package utils

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"net/url"
)

// Keys for docker configs
const (
	DockerConfigAuthKey     = "auth"
	DockerConfigUserKey     = "user"
	DockerConfigPasswordKey = "password"
)

// DockerConfigJson is a top-level dcj
type DockerConfigJson struct {
	Auths map[string]DockerLoginCredential `json:"auths"`
}

// DockerLoginCredential is a [basic|id|pw]:auth map
type DockerLoginCredential map[string]string

// ImagePullSecret is a secret and dcj struct
type ImagePullSecret struct {
	secret *corev1.Secret
	json   *DockerConfigJson
}

// NewImagePullSecret creates a new ImagePullSecret from a secret
func NewImagePullSecret(secret *corev1.Secret) (*ImagePullSecret, error) {
	if secret.Type != corev1.SecretTypeDockerConfigJson {
		return nil, fmt.Errorf("unsupported secret type")
	}

	imagePullSecretData, ok := secret.Data[corev1.DockerConfigJsonKey]
	if !ok {
		return nil, fmt.Errorf("failed to get dockerconfig from ImagePullSecret")
	}

	var dockerConfigJson DockerConfigJson
	if err := json.Unmarshal(imagePullSecretData, &dockerConfigJson); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ImagePullSecret(%s)'s dockerconfig", secret.Name)
	}

	return &ImagePullSecret{
		secret: secret,
		json:   &dockerConfigJson,
	}, nil
}

// GetHostBasicAuth parses a PullSecret for the given host
func (s *ImagePullSecret) GetHostBasicAuth(host string) (string, error) {
	loginAuth, ok := s.json.Auths[host]
	if !ok {
		u, _ := url.Parse(host)
		loginAuth, ok = s.json.Auths[u.Host]
		// DO NOT return error, image may be public
		if !ok {
			return "", nil
		}
	}

	if basicAuth, isBasicPresent := loginAuth[DockerConfigAuthKey]; isBasicPresent {
		return basicAuth, nil
	}

	username, isUserPresent := loginAuth[DockerConfigUserKey]
	password, isPasswordPresent := loginAuth[DockerConfigPasswordKey]
	if isUserPresent && isPasswordPresent {
		return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password))), nil
	}
	return "", fmt.Errorf("there is neither basic auth nor id/pw in docker config json for host %s", host)
}
