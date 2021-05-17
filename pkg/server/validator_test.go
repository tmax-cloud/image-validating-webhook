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
	"github.com/tmax-cloud/image-validating-webhook/internal/utils"
	whv1 "github.com/tmax-cloud/image-validating-webhook/pkg/type"
	regv1 "github.com/tmax-cloud/registry-operator/api/v1"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	kscheme "k8s.io/client-go/kubernetes/scheme"
	restfake "k8s.io/client-go/rest/fake"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
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

	regSignerPolicyPath = regexp.MustCompile("/apis/tmax.io/v1/namespaces/(.*)/signerpolicies")
	regSignerKeyPath    = regexp.MustCompile("/apis/tmax.io/v1/signerkeys/(.*)")
	registryPath        = regexp.MustCompile("/apis/tmax.io/v1/namespaces/(.*)/registries")

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

func initTestWhitelistDir() error {
	whiteListDir = path.Join(os.TempDir(), "wh-test-"+randomString(5))
	defer func() {
		_ = os.RemoveAll(whiteListDir)
	}()

	if err := os.MkdirAll(whiteListDir, 0755); err != nil {
		return err
	}
	if err := ioutil.WriteFile(whitelistByImageFile(), []byte(testImageWhitelisted), 0644); err != nil {
		return err
	}
	if err := ioutil.WriteFile(whitelistByNamespaceFile(), nil, 0644); err != nil {
		return err
	}
	return nil
}

type testRoundTrip struct{}

func (rt *testRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Set("Authorization", "Bearer dummy")
	t := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	return t.RoundTrip(r)
}

func testSignImage(srvHost, srvURL, image string) error {
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
		Name:   "test",
		Hashes: notarydata.Hashes{"sha256": []byte("asdasd")},
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
	if os.Getenv("CI") == "true" {
		logrus.SetLevel(logrus.DebugLevel)
		ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	}

	// Init whitelist
	if err := initTestWhitelistDir(); err != nil {
		t.Fatal(err)
	}

	// Init timestamp key
	pub, err := testCrypto.Create(notarydata.CanonicalTimestampRole, "", notarydata.ECDSAKey)
	if err != nil {
		t.Fatal(err)
	}
	testTimestampKey.Type = pub.Algorithm()
	testTimestampKey.Value.Public = pub.Public()

	// Notary mock up server
	testSrv := httptest.NewTLSServer(testNotaryHandler())
	u, err := url.Parse(testSrv.URL)
	if err != nil {
		t.Fatal(err)
	}

	// Sign some images
	if err := testSignImage(u.Host, testSrv.URL, testImageSignedUnmeetPolicy); err != nil {
		t.Fatal(err)
	}
	if err := testSignImage(u.Host, testSrv.URL, testImageSignedMeetPolicy); err != nil {
		t.Fatal(err)
	}

	testCli := fake.NewSimpleClientset()
	testRestCli := testRestClient(u.Host)

	if err := createTestSecret(testCli, testSrv.URL); err != nil {
		t.Fatal(err)
	}

	tc := map[string]handlerTestCase{
		"whitelisted": {
			namespace:        testNsNoPolicy,
			image:            testImageWhitelisted,
			pullSecret:       "",
			expectedValid:    true,
			expectedErrOccur: false,
			expectedErrMsg:   "",
		},
		"noAuth": {
			namespace:        testNsNoPolicy,
			image:            testImageNotSigned,
			pullSecret:       "",
			expectedValid:    false,
			expectedErrOccur: true,
			expectedErrMsg:   "unauthorized: authentication required",
		},
		"noPolicyNotSigned": {
			namespace:        testNsNoPolicy,
			image:            testImageNotSigned,
			pullSecret:       testSecretDcj,
			expectedValid:    false,
			expectedErrOccur: false,
			expectedErrMsg:   "",
		},
		"noPolicySigned": {
			namespace:        testNsNoPolicy,
			image:            testImageSignedUnmeetPolicy,
			pullSecret:       testSecretDcj,
			expectedValid:    true,
			expectedErrOccur: false,
			expectedErrMsg:   "",
		},
		"signedNotMeetPolicy": {
			namespace:        testNsPolicy,
			image:            testImageSignedUnmeetPolicy,
			pullSecret:       testSecretDcj,
			expectedValid:    false,
			expectedErrOccur: false,
			expectedErrMsg:   "",
		},
		"signedMeetPolicy": {
			namespace:        testNsPolicy,
			image:            testImageSignedMeetPolicy,
			pullSecret:       testSecretDcj,
			expectedValid:    true,
			expectedErrOccur: false,
			expectedErrMsg:   "",
		},
	}

	for name, c := range tc {
		t.Run(name, func(t *testing.T) {
			imgUri := fmt.Sprintf("%s/%s:test", u.Host, c.image)
			handler, err := newValidator(testCli, testRestCli, getFindHyperCloudNotaryServerFn(testRestCli), generateTestPod(imgUri, c.namespace, c.pullSecret))
			if err != nil {
				t.Fatal(err)
			}

			valid, image, err := handler.IsValid()
			assert.Equal(t, c.expectedErrOccur, err != nil, "error occurs")
			if err != nil {
				assert.Equal(t, c.expectedErrMsg, err.Error(), "error message")
			} else {
				assert.Equal(t, c.expectedValid, valid, "validity")
				if !valid {
					assert.Equal(t, imgUri, image, "failed image")
				}
			}
		})
	}
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

func testRestClient(regHost string) *restfake.RESTClient {
	return &restfake.RESTClient{
		NegotiatedSerializer: kscheme.Codecs,
		Client: restfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			// SignerPolicies
			if sp := regSignerPolicyPath.FindAllStringSubmatch(req.URL.Path, -1); len(sp) == 1 && len(sp[0]) == 2 {
				ns := sp[0][1]
				list := &whv1.SignerPolicyList{}

				// Yes policy
				if ns == testNsPolicy {
					list.Items = append(list.Items, whv1.SignerPolicy{
						Spec: whv1.SignerPolicySpec{
							Signers: []string{testSigner},
						},
					})
				}

				b, err := json.Marshal(list)
				if err != nil {
					return nil, err
				}
				return &http.Response{StatusCode: http.StatusOK, Body: ioutil.NopCloser(bytes.NewReader(b))}, nil
			}

			// SignerKeys
			if sk := regSignerKeyPath.FindAllStringSubmatch(req.URL.Path, -1); len(sk) == 1 && len(sk[0]) == 2 {
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
			if rs := registryPath.FindAllStringSubmatch(req.URL.Path, -1); len(rs) == 1 && len(rs[0]) == 2 {
				ns := rs[0][1]
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

			if req.Body != nil {
				defer func() {
					_ = req.Body.Close()
				}()
				b, err := ioutil.ReadAll(req.Body)
				if err != nil {
					return nil, err
				}
				fmt.Println(string(b))
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

type imageWhiteListTestCase struct {
	list  []imageRef
	image string

	expectedWhitelisted bool
}

func TestHandler_IsImageInWhiteList(t *testing.T) {
	tc := map[string]imageWhiteListTestCase{
		"noTagWhiteListed": {
			list:                []imageRef{{name: "notary"}},
			image:               "registry-test.registry.ipip.nip.io/notary",
			expectedWhitelisted: true,
		},
		"noTagNotWhiteListed": {
			list:                []imageRef{{name: "notary"}},
			image:               "registry-test.registry.ipip.nip.io/notary-2",
			expectedWhitelisted: false,
		},
		"yesTagNotWhiteListed": {
			list:                []imageRef{{name: "notary", tag: "test"}},
			image:               "registry-test.registry.ipip.nip.io/notary",
			expectedWhitelisted: false,
		},
		"yesTagWhiteListed": {
			list:                []imageRef{{name: "notary", tag: "test"}},
			image:               "registry-test.registry.ipip.nip.io/notary:test",
			expectedWhitelisted: true,
		},
		"registry": {
			list:                []imageRef{{name: "registry"}},
			image:               "registry-test.registry.ipip.nip.io/test-image",
			expectedWhitelisted: false,
		},
		"wildcard": {
			list:                []imageRef{{host: "registry-2.registry.ipip.nip.io", name: "*"}},
			image:               "registry-2.registry.ipip.nip.io/test-image",
			expectedWhitelisted: true,
		},
		"wildcard2": {
			list:                []imageRef{{host: "registry-2.registry.ipip.nip.io", name: "*"}},
			image:               "registry-2.registry.ipip.nip.io/test-image:test",
			expectedWhitelisted: true,
		},
		"containsSlashSuccess": {
			list:                []imageRef{{host: "registry-2.registry.ipip.nip.io", name: "tmaxcloudck/notary_mysql", tag: "0.6.2-rc2"}},
			image:               "registry-2.registry.ipip.nip.io/tmaxcloudck/notary_mysql:0.6.2-rc2",
			expectedWhitelisted: true,
		},
		"containsSlashFailure": {
			list:                []imageRef{{host: "registry-2.registry.ipip.nip.io", name: "tmaxcloudck/notary_mysql", tag: "0.6.2-rc2"}},
			image:               "registry-2.registry.ipip.nip.io/tmaxcloudck/notary_mysql:0.6.2-rc1",
			expectedWhitelisted: false,
		},
	}

	for name, c := range tc {
		t.Run(name, func(t *testing.T) {
			h := &validator{
				whiteList: WhiteList{byImages: c.list},
			}
			isWhitelisted := h.isImageInWhiteList(c.image)
			assert.Equal(t, c.expectedWhitelisted, isWhitelisted)
		})
	}
}
