package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/bmizerany/assert"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/theupdateframework/notary/client"
	"github.com/theupdateframework/notary/cryptoservice"
	"github.com/theupdateframework/notary/passphrase"
	"github.com/theupdateframework/notary/trustmanager"
	"github.com/theupdateframework/notary/trustpinning"
	"github.com/theupdateframework/notary/tuf"
	notarydata "github.com/theupdateframework/notary/tuf/data"
	"github.com/tmax-cloud/image-validating-webhook/internal/k8s"
	"github.com/tmax-cloud/image-validating-webhook/internal/utils"
	whv1 "github.com/tmax-cloud/image-validating-webhook/pkg/type"
	regv1 "github.com/tmax-cloud/registry-operator/api/v1"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/deprecated/scheme"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	restfake "k8s.io/client-go/rest/fake"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"strings"
	"testing"
	"time"
)

const (
	testNsNoPolicy = "testNsNoPolicy"
	testNsPolicy   = "testNsPolicy"

	testTag                     = "test"
	testImageNotSigned          = "image-not-signed"
	testImageSignedMeetPolicy   = "image-signed-meet-policy"
	testImageSignedUnmeetPolicy = "image-signed-unmeet-policy"
	testImageWhitelisted        = "image-whitelisted"

	testSecretDcj = "test-dcj"

	testSigner = "test-signer"

	testNotaryKeyGun     = "gun"
	testNotaryKeyKeyType = "keyType"
)

var (
	testTimestampKey = notarydata.TUFKey{}

	testCrypto = cryptoservice.NewCryptoService(trustmanager.NewKeyMemoryStore(passphrase.ConstantRetriever("pass")))
	testJsons  = map[string]map[string][]byte{}

	regSignerPolicyPath = regexp.MustCompile("/apis/tmax.io/v1(/namespaces/(.*))?/signerpolicies")
	regSignerKeyPath    = regexp.MustCompile("/apis/tmax.io/v1/signerkeys/(.*)")
	registryPath        = regexp.MustCompile("/apis/tmax.io/v1(/namespaces/(.*))?/registries")

	testTargetKeyMeetPolicy string
)

type handlerTestCase struct {
	namespace  string
	image      string
	pullSecret string

	expectedValid    bool
	expectedErrOccur bool
	expectedErrMsg   string
}

type testRoundTrip struct{}

func (rt *testRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Set("Authorization", "Bearer dummy")
	t := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	return t.RoundTrip(r)
}

func testSignImage(srvHost, srvURL, image, dummyDigest string) error {
	// Init notary client and sign images
	tempDir := fmt.Sprintf("%s/notary/%s", os.TempDir(), randomString(10))
	rt := &testRoundTrip{}
	repo, err := client.NewFileCachedRepository(tempDir, notarydata.GUN(fmt.Sprintf("%s/%s", srvHost, image)), srvURL, rt, passphrase.ConstantRetriever("pass"), trustpinning.TrustPinConfig{})
	if err != nil {
		return err
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()
	rootPub, err := repo.GetCryptoService().Create(notarydata.CanonicalRootRole, "", notarydata.ECDSAKey)
	if err != nil {
		return err
	}

	if _, err := repo.ListTargets(); err != nil {
		switch err.(type) {
		case client.ErrRepoNotInitialized, client.ErrRepositoryNotExist:
			if err := repo.Initialize([]string{rootPub.ID()}); err != nil {
				return err
			}
		default:
			return err
		}
	}

	target := &client.Target{
		Name:   testTag,
		Hashes: notarydata.Hashes{"sha256": []byte(dummyDigest)},
		Length: 32,
	}
	if err := repo.AddTarget(target, notarydata.CanonicalTargetsRole); err != nil {
		return err
	}

	if image == testImageSignedMeetPolicy {
		for _, k := range repo.GetCryptoService().ListKeys(notarydata.CanonicalTargetsRole) {
			testTargetKeyMeetPolicy = k
			break
		}
	}

	if err := repo.Publish(); err != nil {
		return err
	}

	return nil
}

func TestHandler_IsValid(t *testing.T) {
	// Set loggers
	if os.Getenv("CI") != "true" {
		logrus.SetLevel(logrus.DebugLevel)
		ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	}

	// Notary mock up server
	testSrv := httptest.NewTLSServer(testNotaryHandler())
	u, err := url.Parse(testSrv.URL)
	if err != nil {
		t.Fatal(err)
	}

	testCli := fake.NewSimpleClientset()
	testRestCli := testValidatorRestClient(u.Host)

	// Init whitelist
	if err := createTestWhiteListConfigMap(testCli); err != nil {
		t.Fatal(err)
	}

	if err := createTestSecret(testCli, testSrv.URL); err != nil {
		t.Fatal(err)
	}

	// Init timestamp key
	pub, err := testCrypto.Create(notarydata.CanonicalTimestampRole, "", notarydata.ECDSAKey)
	if err != nil {
		t.Fatal(err)
	}
	testTimestampKey.Type = pub.Algorithm()
	testTimestampKey.Value.Public = pub.Public()

	// Sign some images
	testDummyDigest := randomString(30)
	if err := testSignImage(u.Host, testSrv.URL, testImageSignedUnmeetPolicy, testDummyDigest); err != nil {
		t.Fatal(err)
	}
	if err := testSignImage(u.Host, testSrv.URL, testImageSignedMeetPolicy, testDummyDigest); err != nil {
		t.Fatal(err)
	}

	tc := map[string]handlerTestCase{
		"whitelisted": {
			namespace:        testNsNoPolicy,
			image:            fmt.Sprintf("%s:%s", testImageWhitelisted, testTag),
			pullSecret:       "",
			expectedValid:    true,
			expectedErrOccur: false,
			expectedErrMsg:   "",
		},
		"noAuth": {
			namespace:        testNsNoPolicy,
			image:            fmt.Sprintf("%s:%s", testImageNotSigned, testTag),
			pullSecret:       "",
			expectedValid:    false,
			expectedErrOccur: true,
			expectedErrMsg:   "unauthorized: authentication required",
		},
		"noPolicyNotSigned": {
			namespace:        testNsNoPolicy,
			image:            fmt.Sprintf("%s:%s", testImageNotSigned, testTag),
			pullSecret:       testSecretDcj,
			expectedValid:    false,
			expectedErrOccur: false,
			expectedErrMsg:   "",
		},
		"noPolicySigned": {
			namespace:        testNsNoPolicy,
			image:            fmt.Sprintf("%s:%s", testImageSignedUnmeetPolicy, testTag),
			pullSecret:       testSecretDcj,
			expectedValid:    true,
			expectedErrOccur: false,
			expectedErrMsg:   "",
		},
		"signedNotMeetPolicy": {
			namespace:        testNsPolicy,
			image:            fmt.Sprintf("%s:%s", testImageSignedUnmeetPolicy, testTag),
			pullSecret:       testSecretDcj,
			expectedValid:    false,
			expectedErrOccur: false,
			expectedErrMsg:   "",
		},
		"signedMeetPolicy": {
			namespace:        testNsPolicy,
			image:            fmt.Sprintf("%s:%s", testImageSignedMeetPolicy, testTag),
			pullSecret:       testSecretDcj,
			expectedValid:    true,
			expectedErrOccur: false,
			expectedErrMsg:   "",
		},
		"unmatchedDigest": {
			namespace:        testNsNoPolicy,
			image:            fmt.Sprintf("%s:%s@sha256:48b206d34364518658edac279026be0dc0b87dddda042c9a41ef2e3ef34a2429", testImageSignedMeetPolicy, testTag),
			pullSecret:       testSecretDcj,
			expectedValid:    false,
			expectedErrOccur: false,
			expectedErrMsg:   "",
		},
	}

	validator, err := newValidator(testCli, testRestCli, getFindHyperCloudNotaryServerFn(testRestCli))
	if err != nil {
		t.Fatal(err)
	}
	for name, c := range tc {
		t.Run(name, func(t *testing.T) {
			imgUri := fmt.Sprintf("%s/%s", u.Host, c.image)

			pod := generateTestPod(imgUri, c.namespace, c.pullSecret)
			valid, image, err := validator.CheckIsValidAndAddDigest(pod)
			assert.Equal(t, c.expectedErrOccur, err != nil, "error occurs")
			if err != nil {
				assert.Equal(t, c.expectedErrMsg, err.Error(), "error message")
			} else {
				assert.Equal(t, c.expectedValid, valid, "validity")
				if !valid {
					assert.Equal(t, imgUri, image, "failed image")
				} else {
					// Whitelisted image does not get digest
					if !strings.Contains(pod.Spec.Containers[0].Image, testImageWhitelisted) {
						t.Log(pod.Spec.Containers[0].Image)
						ref, _ := parseImage(imgUri)
						ref.digest = fmt.Sprintf("%x", testDummyDigest)
						assert.Equal(t, ref.String(), pod.Spec.Containers[0].Image, "image digest")
					}
				}
			}
		})
	}
}

func createTestWhiteListConfigMap(cli kubernetes.Interface) error {
	ns, err := k8s.Namespace()
	if err != nil {
		return err
	}

	secret := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      whitelistConfigMap,
			Namespace: ns,
		},
		Data: map[string]string{
			whitelistByImage:     testImageWhitelisted,
			whitelistByNamespace: "",
		},
	}

	if _, err := cli.CoreV1().ConfigMaps(ns).Create(context.Background(), secret, metav1.CreateOptions{}); err != nil {
		return err
	}
	return nil
}

func createTestSecret(cli kubernetes.Interface, registryHost string) error {
	auth := utils.DockerConfigJson{
		Auths: map[string]utils.DockerLoginCredential{
			registryHost: {
				utils.DockerConfigAuthKey: "dummy",
			},
		},
	}
	authB, err := json.Marshal(auth)
	if err != nil {
		return err
	}

	dcj := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: testSecretDcj,
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: authB,
		},
	}
	if _, err := cli.CoreV1().Secrets(testNsNoPolicy).Create(context.Background(), dcj, metav1.CreateOptions{}); err != nil {
		return err
	}
	if _, err := cli.CoreV1().Secrets(testNsPolicy).Create(context.Background(), dcj, metav1.CreateOptions{}); err != nil {
		return err
	}
	return nil
}

func generateTestPod(img, ns, secretName string) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test-cont",
					Image: img,
				},
			},
		},
	}
	if secretName != "" {
		pod.Spec.ImagePullSecrets = []corev1.LocalObjectReference{
			{Name: secretName},
		}
	}
	return pod
}

func testValidatorRestClient(regHost string) *restfake.RESTClient {
	_ = whv1.AddToScheme(scheme.Scheme)
	return &restfake.RESTClient{
		GroupVersion:         whv1.GroupVersion,
		NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
		Client: restfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			isWatch := len(req.URL.Query()["watch"]) > 0
			// SignerPolicies
			if sp := regSignerPolicyPath.FindAllStringSubmatch(req.URL.Path, -1); len(sp) == 1 {
				allPolicies := map[string][]whv1.SignerPolicy{}

				// Yes Policy
				allPolicies[testNsPolicy] = append(allPolicies[testNsPolicy], whv1.SignerPolicy{
					TypeMeta: metav1.TypeMeta{APIVersion: "tmax.io/v1", Kind: "SignerPolicy"},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNsPolicy,
					},
					Spec: whv1.SignerPolicySpec{
						Signers: []string{testSigner},
					},
				})

				ns := sp[0][2]

				list := &whv1.SignerPolicyList{}
				// All namespace
				if sp[0][2] == "" {
					for _, v := range allPolicies {
						list.Items = append(list.Items, v...)
					}
				} else {
					list.Items = append(list.Items, allPolicies[ns]...)
				}

				if isWatch {
					b := bytes.NewBuffer([]byte{})

					for _, policies := range allPolicies {
						for _, p := range policies {
							pBytes, err := json.Marshal(p)
							if err != nil {
								return nil, err
							}
							ev := metav1.WatchEvent{
								Type:   string(watch.Added),
								Object: runtime.RawExtension{Raw: json.RawMessage(pBytes)},
							}
							bb, err := json.Marshal(ev)
							if err != nil {
								return nil, err
							}
							b.Write(bb)
						}
					}

					header := http.Header{}
					header.Set("Content-Type", "application/json")
					header.Set("Transfer-Encoding", "chunked")

					return &http.Response{StatusCode: http.StatusOK, Header: header, Body: ioutil.NopCloser(b)}, nil
				}

				b, err := json.Marshal(list)
				if err != nil {
					return nil, err
				}

				return &http.Response{StatusCode: http.StatusOK, Body: ioutil.NopCloser(bytes.NewReader(b))}, nil
			}

			// SignerKeys
			if sk := regSignerKeyPath.FindAllStringSubmatch(req.URL.Path, -1); len(sk) == 1 {
				b, err := json.Marshal(&regv1.SignerKey{
					Spec: regv1.SignerKeySpec{
						Targets: map[string]regv1.TrustKey{
							"dummy": {ID: testTargetKeyMeetPolicy},
						},
					},
				})
				if err != nil {
					return nil, err
				}
				return &http.Response{StatusCode: http.StatusOK, Body: ioutil.NopCloser(bytes.NewReader(b))}, nil
			}

			// Registries
			if rs := registryPath.FindAllStringSubmatch(req.URL.Path, -1); len(rs) == 1 {
				ns := rs[0][2]
				list := &regv1.RegistryList{
					Items: []regv1.Registry{{
						ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "test-reg"},
						Spec:       regv1.RegistrySpec{},
						Status: regv1.RegistryStatus{
							ServerURL: "https://" + regHost,
							NotaryURL: "https://" + regHost,
						},
					}},
				}

				b, err := json.Marshal(list)
				if err != nil {
					return nil, err
				}
				return &http.Response{StatusCode: http.StatusOK, Body: ioutil.NopCloser(bytes.NewReader(b))}, nil
			}
			return &http.Response{StatusCode: http.StatusNotFound, Body: ioutil.NopCloser(&bytes.Buffer{})}, nil
		}),
	}
}

func testNotaryAuthMW(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		logrus.Debug(req.Method + ": " + req.URL.String())
		token := req.Header.Get("Authorization")

		if strings.HasPrefix(req.URL.String(), "/token") || token != "" {
			h.ServeHTTP(w, req)
		} else {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf("Bearer realm=\"https://%s/token\",service=\"notary.docker.io\"", req.Host))
			w.WriteHeader(http.StatusUnauthorized)
		}
	})
}

func testNotaryAuthHandler(w http.ResponseWriter, req *http.Request) {
	basicAuth := req.Header.Get("Authorization")
	if basicAuth == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	b, _ := json.Marshal(struct {
		Token string `json:"token"`
	}{
		Token: "dummy",
	})
	_, _ = w.Write(b)
}

func testNotaryHandler() http.Handler {
	m := mux.NewRouter()

	// Auth handler
	m.Use(testNotaryAuthMW)

	// Ping
	m.Methods(http.MethodGet).Subrouter().HandleFunc("/v2", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Token
	m.Methods(http.MethodGet).Subrouter().HandleFunc("/token", testNotaryAuthHandler)

	// Timestamp key
	m.Methods(http.MethodGet).Path(fmt.Sprintf("/v2/{%s:[^*]+}/_trust/tuf/timestamp.key", testNotaryKeyGun)).HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		b, _ := json.Marshal(testTimestampKey)
		_, _ = w.Write(b)
	})

	// Get keys json
	m.Methods(http.MethodGet).Path(fmt.Sprintf("/v2/{%s:[^*]+}/_trust/tuf/{%s}.json", testNotaryKeyGun, testNotaryKeyKeyType)).HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		keys, ok := testJsons[vars[testNotaryKeyGun]]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		keyType := vars[testNotaryKeyKeyType]
		if strings.HasPrefix(keyType, "snapshot") {
			keyType = "snapshot"
		} else if strings.HasPrefix(keyType, "targets") {
			keyType = "targets"
		}

		key, ok := keys[keyType]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write(key)
	})

	// Post keys json
	m.Methods(http.MethodPost).Path(fmt.Sprintf("/v2/{%s:[^*]+}/_trust/tuf/", testNotaryKeyGun)).HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		_ = req.ParseMultipartForm(32 << 20)

		gun := vars[testNotaryKeyGun]
		testJsons[gun] = map[string][]byte{}
		tufRepo := tuf.NewRepo(testCrypto)

		files := req.MultipartForm.File["files"]
		for _, f := range files {
			file, err := f.Open()
			if err != nil {
				log.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			testJsons[gun][f.Filename], err = ioutil.ReadAll(file)
			if err != nil {
				log.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			switch f.Filename {
			case "root":
				if err := json.Unmarshal(testJsons[gun][f.Filename], &tufRepo.Root); err != nil {
					log.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			case "targets":
			case "snapshot":
				if err := json.Unmarshal(testJsons[gun][f.Filename], &tufRepo.Snapshot); err != nil {
					log.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}
		}

		// Create timestamp
		if err := tufRepo.InitTimestamp(); err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		ts, err := tufRepo.SignTimestamp(time.Now().Add(10 * time.Hour))
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		tsBytes, _ := json.Marshal(ts)
		testJsons[gun]["timestamp"] = tsBytes
	})
	return m
}
