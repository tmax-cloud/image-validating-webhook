package k8s

import (
	whv1 "github.com/tmax-cloud/image-validating-webhook/pkg/type"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// NewClientSet initiates a new rest client set
func NewClientSet() (*kubernetes.Clientset, rest.Interface, error) {
	// Common client set
	comCfg, err := config.GetConfig()
	if err != nil {
		return nil, nil, err
	}
	cliSet, err := kubernetes.NewForConfig(comCfg)
	if err != nil {
		return nil, nil, err
	}

	// Rest client for tmax.io/v1
	restCfg, err := config.GetConfig()
	if err != nil {
		return nil, nil, err
	}
	restCfg.ContentConfig.GroupVersion = &whv1.GroupVersion
	restCfg.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	restCli, err := rest.RESTClientFor(restCfg)
	if err != nil {
		return nil, nil, err
	}

	return cliSet, restCli, nil
}
