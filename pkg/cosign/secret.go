package cosign

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	KeyReference = "k8s://"
)

// GetKeyPairSecret get cosign key-pair from secret resource in k8s cluster
func GetKeyPairSecret(ctx context.Context, client kubernetes.Interface, k8sKeyRef string) (*v1.Secret, error) {
	namespace, name, err := parseRef(k8sKeyRef)
	if err != nil {
		return nil, err
	}

	var s *v1.Secret
	if s, err = client.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{}); err != nil {
		return nil, errors.Wrap(err, "Cosign: checking if secret exists")
	}

	return s, nil
}

// the reference should be formatted as <namespace>/<secret name>
func parseRef(k8sRef string) (string, string, error) {
	s := strings.Split(strings.TrimPrefix(k8sRef, KeyReference), "/")
	if len(s) != 2 {
		return "", "", errors.New("Cosign: kubernetes specification should be in the format k8s://<namespace>/<secret>")
	}
	return s[0], s[1], nil
}
