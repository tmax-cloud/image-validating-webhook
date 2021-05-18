package k8s

import (
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// NewClientSet initiates a new rest client set
func NewClientSet() (*kubernetes.Clientset, error) {
	restCfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(restCfg)
}
