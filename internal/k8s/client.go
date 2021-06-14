package k8s

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

// GroupVersionClient a group/version-specific k8s client
type GroupVersionClient interface {
	rest.Interface

	GetGroupVersion() schema.GroupVersion
}

type groupVersionClient struct {
	*rest.RESTClient
	gv schema.GroupVersion
}

// NewGroupVersionClient creates a new GroupVersionClient, given groupVersion and config
func NewGroupVersionClient(cfg *rest.Config, groupVersion schema.GroupVersion) (GroupVersionClient, error) {
	restCfg := rest.CopyConfig(cfg)
	if groupVersion.Group == "" {
		restCfg.APIPath = "/api"
	} else {
		restCfg.APIPath = "/apis"
	}
	restCfg.ContentConfig.GroupVersion = &groupVersion
	restCfg.NegotiatedSerializer = scheme.Codecs.WithoutConversion()

	restCli, err := rest.RESTClientFor(restCfg)
	if err != nil {
		return nil, err
	}
	return &groupVersionClient{RESTClient: restCli, gv: groupVersion}, nil
}

func (g *groupVersionClient) GetGroupVersion() schema.GroupVersion {
	return g.gv
}
