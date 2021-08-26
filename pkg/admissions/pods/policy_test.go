package pods

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	whv1 "github.com/tmax-cloud/image-validating-webhook/pkg/type"
	"github.com/tmax-cloud/image-validating-webhook/pkg/watcher/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	restfake "k8s.io/client-go/rest/fake"
)

type doesMatchPolicyTestCase struct {
	registry  string
	namespace string

	expectedValid  bool
	expectedPolicy whv1.RegistrySpec
}

func TestRegistryPolicyCache_doesMatchPolicy(t *testing.T) {
	tc := map[string]doesMatchPolicyTestCase{
		"notMatchPolicy": {
			registry:       "no-match-registry",
			namespace:      testCheckSign,
			expectedValid:  false,
			expectedPolicy: whv1.RegistrySpec{},
		},
		"clusterPolicy": {
			registry:      "testRegistry1",
			namespace:     testCheckSign,
			expectedValid: true,
			expectedPolicy: whv1.RegistrySpec{
				Registry:  "testRegistry1",
				Notary:    "",
				SignCheck: false,
			},
		},
		"namespacePolicy": {
			registry:      "testRegistry2",
			namespace:     testCheckSign,
			expectedValid: true,
			expectedPolicy: whv1.RegistrySpec{
				Registry:  "testRegistry2",
				Notary:    "",
				SignCheck: false,
			},
		},
	}

	cache := RegistryPolicyCache{restClient: testPolicyRestClient(), clusterCachedClient: &fake.CachedClient{
		Cache: map[string]runtime.Object{
			testCheckSign + "/policy1": &whv1.ClusterRegistrySecurityPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "policy1",
				},
				Spec: whv1.ClusterRegistrySecurityPolicySpec{
					Registries: []whv1.RegistrySpec{
						{
							Registry:  "testRegistry1",
							Notary:    "",
							SignCheck: false,
						},
					},
				},
			},
		},
	}, namespaceCachedClient: &fake.CachedClient{
		Cache: map[string]runtime.Object{
			testCheckSign + "/policy2": &whv1.RegistrySecurityPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "policy2",
					Namespace: testCheckSign,
				},
				Spec: whv1.RegistrySecurityPolicySpec{
					Registries: []whv1.RegistrySpec{
						{
							Registry:  "testRegistry2",
							Notary:    "",
							SignCheck: false,
						},
					},
				},
			},
		},
	}}

	for name, c := range tc {
		t.Run(name, func(t *testing.T) {
			valid, policy := cache.doesMatchPolicy(c.registry, c.namespace)
			require.Equal(t, c.expectedValid, valid)
			require.Equal(t, c.expectedPolicy, policy)
		})
	}
}

func testPolicyRestClient() *restfake.RESTClient {
	_ = whv1.AddToScheme(scheme.Scheme)
	return &restfake.RESTClient{
		GroupVersion:         whv1.GroupVersion,
		NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
		Client: restfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: ioutil.NopCloser(&bytes.Buffer{})}, nil
		}),
	}
}
