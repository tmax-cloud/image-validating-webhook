package server

import (
	"bytes"
	"encoding/json"
	"github.com/bmizerany/assert"
	whv1 "github.com/tmax-cloud/image-validating-webhook/pkg/type"
	regv1 "github.com/tmax-cloud/registry-operator/api/v1"
	"io/ioutil"
	"k8s.io/client-go/kubernetes/scheme"
	restfake "k8s.io/client-go/rest/fake"
	"net/http"
	"testing"
)

type doesMatchPolicyTestCase struct {
	key       string
	namespace string

	expectedMatch bool
}

func TestSignerPolicyCache_doesMatchPolicy(t *testing.T) {
	tc := map[string]doesMatchPolicyTestCase{
		"noPolicy": {
			key:           "dummy",
			namespace:     testNsNoPolicy,
			expectedMatch: true,
		},
		"notMatchPolicy": {
			key:           "no-match-key",
			namespace:     testNsPolicy,
			expectedMatch: false,
		},
		"matchPolicy": {
			key:           "match-key",
			namespace:     testNsPolicy,
			expectedMatch: true,
		},
	}

	cache := SignerPolicyCache{policies: map[string]map[string]*whv1.SignerPolicy{}, restClient: testPolicyRestClient()}

	cache.policies[testNsPolicy] = map[string]*whv1.SignerPolicy{
		"policy1": {
			Spec: whv1.SignerPolicySpec{
				Signers: []string{"signer1"},
			},
		},
	}

	for name, c := range tc {
		t.Run(name, func(t *testing.T) {
			match := cache.doesMatchPolicy(c.key, c.namespace)
			assert.Equal(t, c.expectedMatch, match, "match policy")
		})
	}
}

func testPolicyRestClient() *restfake.RESTClient {
	_ = whv1.AddToScheme(scheme.Scheme)
	return &restfake.RESTClient{
		GroupVersion:         whv1.GroupVersion,
		NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
		Client: restfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			// SignerKeys
			if sk := regSignerKeyPath.FindAllStringSubmatch(req.URL.Path, -1); len(sk) == 1 {
				b, err := json.Marshal(&regv1.SignerKey{
					Spec: regv1.SignerKeySpec{
						Targets: map[string]regv1.TrustKey{
							"dummyKey": {ID: "match-key"},
						},
					},
				})
				if err != nil {
					return nil, err
				}
				return &http.Response{StatusCode: http.StatusOK, Body: ioutil.NopCloser(bytes.NewReader(b))}, nil
			}
			return &http.Response{StatusCode: http.StatusNotFound, Body: ioutil.NopCloser(&bytes.Buffer{})}, nil
		}),
	}
}
