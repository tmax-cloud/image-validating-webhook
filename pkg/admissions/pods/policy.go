package pods

import (
	"fmt"

	"github.com/tmax-cloud/image-validating-webhook/internal/k8s"
	whv1 "github.com/tmax-cloud/image-validating-webhook/pkg/type"
	"github.com/tmax-cloud/image-validating-webhook/pkg/watcher"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/rest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// RegistryPolicyCache is a cache of type.RegistrySecurityPolicy
type RegistryPolicyCache struct {
	restClient rest.Interface

	clusterCachedClient   watcher.CachedClient
	namespaceCachedClient watcher.CachedClient
}

var (
	policylog = logf.Log.WithName("policy")
)

func newRegistryPolicyCache(cfg *rest.Config, restClient rest.Interface) (*RegistryPolicyCache, error) {
	// Create watcher client for whv1
	watchCli, err := k8s.NewGroupVersionClient(cfg, whv1.GroupVersion)
	if err != nil {
		panic(err)
	}

	// Initiate watcher
	nw := watcher.New("", "registrysecuritypolicies", &whv1.RegistrySecurityPolicy{}, watchCli, fields.Everything())
	cw := watcher.New("", "clusterregistrysecuritypolicies", &whv1.ClusterRegistrySecurityPolicy{}, watchCli, fields.Everything())

	p := &RegistryPolicyCache{
		restClient:            restClient,
		clusterCachedClient:   watcher.NewCachedClient(cw),
		namespaceCachedClient: watcher.NewCachedClient(nw),
	}

	waitChCluster := make(chan struct{})
	waitChNamespace := make(chan struct{})

	// Start to watch RegistrySecurityPolicy
	go cw.Start(waitChCluster)
	go nw.Start(waitChNamespace)

	// Block until it's ready
	<-waitChCluster
	<-waitChNamespace

	return p, nil
}

func (c *RegistryPolicyCache) doesMatchPolicy(registry string, namespace string) (bool, whv1.RegistrySpec) {
	clusterObjs := &whv1.ClusterRegistrySecurityPolicyList{}
	namespaceObjs := &whv1.RegistrySecurityPolicyList{}

	if err := c.clusterCachedClient.List(watcher.Selector{Namespace: ""}, clusterObjs); err != nil {
		policylog.Error(err, "")
		return false, whv1.RegistrySpec{}
	}
	if err := c.namespaceCachedClient.List(watcher.Selector{Namespace: namespace}, namespaceObjs); err != nil {
		policylog.Error(err, "")
		return false, whv1.RegistrySpec{}
	}

	if registry == "" {
		registry = "docker.io"
	}

	if len(clusterObjs.Items) == 0 && len(namespaceObjs.Items) == 0 {
		return true, whv1.RegistrySpec{}
	}
	for i := range clusterObjs.Items {
		for j := range clusterObjs.Items[i].Spec.Registries {
			if clusterObjs.Items[i].Spec.Registries[j].Registry == registry {
				return true, clusterObjs.Items[i].Spec.Registries[j]
			}
		}
	}
	for i := range namespaceObjs.Items {
		for j := range namespaceObjs.Items[i].Spec.Registries {
			if namespaceObjs.Items[i].Spec.Registries[j].Registry == registry {
				return true, namespaceObjs.Items[i].Spec.Registries[j]
			}
		}
	}
	err := fmt.Errorf("no matching registry security policy")
	policylog.Error(err, "")

	return false, whv1.RegistrySpec{}
}
