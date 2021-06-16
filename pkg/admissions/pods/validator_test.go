package pods

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/tmax-cloud/image-validating-webhook/internal/k8s"
	"github.com/tmax-cloud/image-validating-webhook/internal/utils"
	notarytest "github.com/tmax-cloud/image-validating-webhook/pkg/notary/test"
	whv1 "github.com/tmax-cloud/image-validating-webhook/pkg/type"
	watcherfake "github.com/tmax-cloud/image-validating-webhook/pkg/watcher/fake"
	regv1 "github.com/tmax-cloud/registry-operator/api/v1"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	restfake "k8s.io/client-go/rest/fake"
	"net/http"
	"net/url"
	"os"
	"regexp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"strings"
	"testing"
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
)

var (
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
	expectedReason   string
	expectedErrOccur bool
	expectedErrMsg   string
}

func TestHandler_IsValid(t *testing.T) {
	// Set loggers
	if os.Getenv("CI") != "true" {
		logrus.SetLevel(logrus.DebugLevel)
		ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	}

	// Notary mock up server
	testSrv, err := notarytest.New(true)
	require.NoError(t, err)

	u, err := url.Parse(testSrv.URL)
	require.NoError(t, err)

	testCli := fake.NewSimpleClientset()
	testRestCli := testValidatorRestClient(u.Host)

	// Init whitelist
	require.NoError(t, createTestWhiteListConfigMap(testCli))
	require.NoError(t, createTestSecret(testCli, testSrv.URL))

	// Sign some images
	testDummyDigest := "111111111111111111111111111111"
	_, err = testSrv.SignImage(testSrv.URL, u.Host, testImageSignedUnmeetPolicy, testTag, testDummyDigest)
	require.NoError(t, err)
	testTargetKeyMeetPolicy, err = testSrv.SignImage(testSrv.URL, u.Host, testImageSignedMeetPolicy, testTag, testDummyDigest)
	require.NoError(t, err)

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
			expectedErrOccur: true,
			expectedErrMsg:   "unauthorized: authentication required",
		},
		"noPolicyNotSigned": {
			namespace:        testNsNoPolicy,
			image:            fmt.Sprintf("%s:%s", testImageNotSigned, testTag),
			pullSecret:       testSecretDcj,
			expectedValid:    false,
			expectedReason:   fmt.Sprintf("Image '%s/image-not-signed:test' is not signed", u.Host),
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
			expectedReason:   fmt.Sprintf("Image '%s/image-signed-unmeet-policy:test' does not meet signer policy. Please check the namespace's SignerPolicy", u.Host),
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
			expectedReason:   fmt.Sprintf("Image '%s/image-signed-meet-policy:test@sha256:48b206d34364518658edac279026be0dc0b87dddda042c9a41ef2e3ef34a2429''s digest is different from the signed digest", u.Host),
			expectedErrOccur: false,
			expectedErrMsg:   "",
		},
	}

	validator := testValidator(testCli, testRestCli, getFindHyperCloudNotaryServerFn(testRestCli))

	for name, c := range tc {
		t.Run(name, func(t *testing.T) {
			imgUri := fmt.Sprintf("%s/%s", u.Host, c.image)

			pod := generateTestPod(imgUri, c.namespace, c.pullSecret)
			valid, reason, err := validator.CheckIsValidAndAddDigest(pod)
			if c.expectedErrOccur {
				require.Error(t, err)
				require.Equal(t, c.expectedErrMsg, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, c.expectedValid, valid)
				if !valid {
					require.Equal(t, c.expectedReason, reason, "reason")
				} else {
					// Whitelisted image does not get digest
					if !strings.Contains(pod.Spec.Containers[0].Image, testImageWhitelisted) {
						ref, _ := parseImage(imgUri)
						ref.digest = fmt.Sprintf("%x", testDummyDigest)
						require.Equal(t, ref.String(), pod.Spec.Containers[0].Image, "image digest")
					}
				}
			}
		})
	}
}

func testValidator(testCli kubernetes.Interface, testRestCli rest.Interface, fn findNotaryServerFn) *validator {
	validator := &validator{client: testCli, findNotaryServer: fn}
	validator.signerPolicyCache = &SignerPolicyCache{restClient: testRestCli, cachedClient: &watcherfake.CachedClient{
		Cache: map[string]runtime.Object{
			testNsPolicy + "/policy1": &whv1.SignerPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "policy1",
					Namespace: testNsPolicy,
				},
				Spec: whv1.SignerPolicySpec{
					Signers: []string{testSigner},
				},
			},
		},
	}}
	validator.whiteList = &WhiteList{byImages: []imageRef{{name: testImageWhitelisted}}}

	return validator
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
