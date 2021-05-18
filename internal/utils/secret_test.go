package utils

import (
	"encoding/base64"
	"github.com/bmizerany/assert"
	corev1 "k8s.io/api/core/v1"
	"reflect"
	"testing"
)

type newImagePullSecretTestCase struct {
	secret *corev1.Secret

	expectedAuths       map[string]DockerLoginCredential
	expectedErrorOccurs bool
	expectedErrorString string
}

func TestNewImagePullSecret(t *testing.T) {
	tc := map[string]newImagePullSecretTestCase{
		"empty": {
			secret: &corev1.Secret{
				Type: corev1.SecretTypeDockerConfigJson,
				Data: map[string][]byte{
					corev1.DockerConfigJsonKey: []byte(`{"auths": {}}`),
				},
			},
			expectedAuths: map[string]DockerLoginCredential{},
		},
		"normal": {
			secret: &corev1.Secret{
				Type: corev1.SecretTypeDockerConfigJson,
				Data: map[string][]byte{
					corev1.DockerConfigJsonKey: []byte(`{"auths": {"https://reg-test.registry.ipip.nip.io": {"auth": "valval"}}}`),
				},
			},
			expectedAuths: map[string]DockerLoginCredential{
				"https://reg-test.registry.ipip.nip.io": {
					"auth": "valval",
				},
			},
		},
		"noProtocol": {
			secret: &corev1.Secret{
				Type: corev1.SecretTypeDockerConfigJson,
				Data: map[string][]byte{
					corev1.DockerConfigJsonKey: []byte(`{"auths": {"reg-test.registry.ipip.nip.io": {"auth": "valval"}}}`),
				},
			},
			expectedAuths: map[string]DockerLoginCredential{
				"reg-test.registry.ipip.nip.io": {
					"auth": "valval",
				},
			},
		},
		"dockerio": {
			secret: &corev1.Secret{
				Type: corev1.SecretTypeDockerConfigJson,
				Data: map[string][]byte{
					corev1.DockerConfigJsonKey: []byte(`{"auths": {"https://registry-1.docker.io": {"auth": "valval"}}}`),
				},
			},
			expectedAuths: map[string]DockerLoginCredential{
				"https://registry-1.docker.io": {
					"auth": "valval",
				},
			},
		},
		"notProperType": {
			secret: &corev1.Secret{
				Type: corev1.SecretTypeBasicAuth,
				Data: map[string][]byte{
					corev1.DockerConfigJsonKey: []byte(`{"auths": {"registry-1.docker.io": {"auth": "valval"}}}`),
				},
			},
			expectedErrorOccurs: true,
			expectedErrorString: "unsupported secret type",
		},
		"noData": {
			secret: &corev1.Secret{
				Type: corev1.SecretTypeDockerConfigJson,
				Data: map[string][]byte{
					"keyTest": []byte(`{"auths": {"registry-1.docker.io": {"auth": "valval"}}}`),
				},
			},
			expectedErrorOccurs: true,
			expectedErrorString: "failed to get dockerconfig from ImagePullSecret",
		},
	}

	for name, c := range tc {
		t.Run(name, func(t *testing.T) {
			ps, err := NewImagePullSecret(c.secret)
			assert.Equal(t, c.expectedErrorOccurs, err != nil, "error occurs")
			if err != nil {
				assert.Equal(t, c.expectedErrorString, err.Error(), "error string")
			} else {
				assert.Equal(t, true, reflect.DeepEqual(ps.json.Auths, c.expectedAuths), "auth result")
			}
		})
	}
}

type getHostBasicAuthTestCase struct {
	host  string
	auths map[string]DockerLoginCredential

	expectedAuth        string
	expectedErrorOccurs bool
	expectedErrorString string
}

func TestImagePullSecret_GetHostBasicAuth(t *testing.T) {
	tc := map[string]getHostBasicAuthTestCase{
		"notFound": {
			host: "https://not-found-host",
			auths: map[string]DockerLoginCredential{
				"https://found-host": {"auth": "dummy"},
			},
			expectedAuth:        "",
			expectedErrorOccurs: false,
			expectedErrorString: "",
		},
		"onlyHost": {
			host: "https://found-host",
			auths: map[string]DockerLoginCredential{
				"found-host": {"auth": "dummy"},
			},
			expectedAuth:        "dummy",
			expectedErrorOccurs: false,
			expectedErrorString: "",
		},
		"basicAuth": {
			host: "https://found-host",
			auths: map[string]DockerLoginCredential{
				"https://found-host": {"auth": "dummy"},
			},
			expectedAuth:        "dummy",
			expectedErrorOccurs: false,
			expectedErrorString: "",
		},
		"idpw": {
			host: "https://found-host",
			auths: map[string]DockerLoginCredential{
				"https://found-host": {"user": "testID", "password": "testPW"},
			},
			expectedAuth:        base64.StdEncoding.EncodeToString([]byte("testID:testPW")),
			expectedErrorOccurs: false,
			expectedErrorString: "",
		},
		"noProperKeys": {
			host: "https://found-host",
			auths: map[string]DockerLoginCredential{
				"https://found-host": {"id": "testID", "password": "testPW"},
			},
			expectedAuth:        "",
			expectedErrorOccurs: true,
			expectedErrorString: "there is neither basic auth nor id/pw in docker config json for host https://found-host",
		},
	}

	for name, c := range tc {
		t.Run(name, func(t *testing.T) {
			ps := ImagePullSecret{
				json: &DockerConfigJson{
					Auths: c.auths,
				},
			}

			auth, err := ps.GetHostBasicAuth(c.host)
			assert.Equal(t, c.expectedErrorOccurs, err != nil, "error occurs")
			if err != nil {
				assert.Equal(t, c.expectedErrorString, err.Error(), "error string")
			} else {
				assert.Equal(t, c.expectedAuth, auth, "basic auth")
			}
		})
	}
}
