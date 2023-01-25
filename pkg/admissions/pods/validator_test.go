package pods

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/tmax-cloud/image-validating-webhook/internal/k8s"
	"github.com/tmax-cloud/image-validating-webhook/internal/utils"
	notarytest "github.com/tmax-cloud/image-validating-webhook/pkg/notary/test"
	whv1 "github.com/tmax-cloud/image-validating-webhook/pkg/type"
	watcherfake "github.com/tmax-cloud/image-validating-webhook/pkg/watcher/fake"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	restfake "k8s.io/client-go/rest/fake"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	testNoCheckSign = "testNoCheckSign"
	testCheckSign   = "testCheckSign"

	testTag              = "test"
	testImageNotSigned   = "image-not-signed"
	testImageSignCheck   = "image-sign-check"
	testImageNoSignCheck = "image-no-sign-check"
	testImageWhitelisted = "image-whitelisted"

	testSecretDcj = "test-dcj"
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

var testSrvHost string
var notarySrv string

func TestHandler_IsValid(t *testing.T) {
	// Set loggers
	if os.Getenv("CI") != "true" {
		logrus.SetLevel(logrus.ErrorLevel)
		ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	}

	// Notary mock up server
	testSrv, err := notarytest.New(true)
	require.NoError(t, err)
	notarySrv = testSrv.URL
	u, err := url.Parse(testSrv.URL)
	require.NoError(t, err)

	testSrvHost = u.Host

	testCli := fake.NewSimpleClientset()
	testRestCli := testValidatorRestClient(u.Host)

	// Init whitelist
	require.NoError(t, createTestWhiteListConfigMap(testCli))
	require.NoError(t, createTestSecret(testCli, testSrv.URL))

	// Sign some images
	testDummyDigest := "111111111111111111111111111111"
	_, err = testSrv.SignImage(testSrv.URL, u.Host, testImageNoSignCheck, testTag, testDummyDigest)
	require.NoError(t, err)
	_, err = testSrv.SignImage(testSrv.URL, u.Host, testImageSignCheck, testTag, testDummyDigest)
	require.NoError(t, err)

	tc := map[string]handlerTestCase{
		"whitelisted": {
			namespace:        testNoCheckSign,
			image:            fmt.Sprintf("%s:%s", testImageWhitelisted, testTag),
			pullSecret:       "",
			expectedValid:    true,
			expectedErrOccur: false,
			expectedErrMsg:   "",
		},
		"noAuth": {
			namespace:        testCheckSign,
			image:            fmt.Sprintf("%s:%s", testImageNotSigned, testTag),
			pullSecret:       "",
			expectedErrOccur: true,
			expectedErrMsg:   "unauthorized: authentication required",
		},
		"noCheckSign": {
			namespace:        testNoCheckSign,
			image:            fmt.Sprintf("%s:%s", testImageNoSignCheck, testTag),
			pullSecret:       testSecretDcj,
			expectedValid:    true,
			expectedErrOccur: false,
			expectedErrMsg:   "",
		},
		"CheckSign": {
			namespace:        testCheckSign,
			image:            fmt.Sprintf("%s:%s", testImageSignCheck, testTag),
			pullSecret:       testSecretDcj,
			expectedValid:    true,
			expectedErrOccur: false,
			expectedErrMsg:   "",
		},
	}

	validator := testValidator(testCli, testRestCli)

	for name, c := range tc {
		t.Run(name, func(t *testing.T) {
			imgURI := fmt.Sprintf("%s/%s", u.Host, c.image)

			pod := generateTestPod(imgURI, c.namespace, c.pullSecret)
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
						ref, _ := parseImage(imgURI)
						if !strings.Contains(pod.Spec.Containers[0].Image, testImageNoSignCheck) {
							ref.digest = fmt.Sprintf("%x", testDummyDigest)
						}
						require.Equal(t, ref.String(), pod.Spec.Containers[0].Image, "image digest")
					}
				}
			}
		})
	}
}

func testValidator(testCli kubernetes.Interface, testRestCli rest.Interface) *validator {
	validator := &validator{client: testCli}
	validator.registryPolicyCache = &RegistryPolicyCache{restClient: testRestCli, clusterCachedClient: &watcherfake.CachedClient{}, namespaceCachedClient: &watcherfake.CachedClient{
		Cache: map[string]runtime.Object{
			testNoCheckSign + "/policy1": &whv1.RegistrySecurityPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "policy1",
					Namespace: testNoCheckSign,
				},
				Spec: whv1.RegistrySecurityPolicySpec{
					Registries: []whv1.RegistrySpec{
						{
							Registry:  testSrvHost,
							Notary:    "",
							SignCheck: false,
						},
					},
				},
			},
			testCheckSign + "/policy2": &whv1.RegistrySecurityPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "policy2",
					Namespace: testCheckSign,
				},
				Spec: whv1.RegistrySecurityPolicySpec{
					Registries: []whv1.RegistrySpec{
						{
							Registry:  testSrvHost,
							Notary:    notarySrv,
							SignCheck: true,
						},
					},
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
	auth := utils.DockerConfigJSON{
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
	if _, err := cli.CoreV1().Secrets(testNoCheckSign).Create(context.Background(), dcj, metav1.CreateOptions{}); err != nil {
		return err
	}
	if _, err := cli.CoreV1().Secrets(testCheckSign).Create(context.Background(), dcj, metav1.CreateOptions{}); err != nil {
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
			return &http.Response{StatusCode: http.StatusOK, Body: ioutil.NopCloser(&bytes.Buffer{})}, nil
		}),
	}
}
