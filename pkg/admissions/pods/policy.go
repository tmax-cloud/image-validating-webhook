package pods

import (
	"context"
	"github.com/tmax-cloud/image-validating-webhook/internal/k8s"
	whv1 "github.com/tmax-cloud/image-validating-webhook/pkg/type"
	"github.com/tmax-cloud/image-validating-webhook/pkg/watcher"
	regv1 "github.com/tmax-cloud/registry-operator/api/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/rest"
	"log"
)

// SignerPolicyCache is a cache of type.SignerPolicy
type SignerPolicyCache struct {
	restClient rest.Interface

	cachedClient watcher.CachedClient
}

func newSignerPolicyCache(cfg *rest.Config, restClient rest.Interface) (*SignerPolicyCache, error) {
	// Create watcher client for whv1
	watchCli, err := k8s.NewGroupVersionClient(cfg, whv1.GroupVersion)
	if err != nil {
		panic(err)
	}

	// Initiate watcher
	w := watcher.New("", "signerpolicies", &whv1.SignerPolicy{}, watchCli, fields.Everything())

	p := &SignerPolicyCache{
		restClient:   restClient,
		cachedClient: watcher.NewCachedClient(w),
	}

	waitCh := make(chan struct{})

	// Start to watch SignerPolicy
	go w.Start(waitCh)

	// Block until it's ready
	<-waitCh

	return p, nil
}

func (c *SignerPolicyCache) doesMatchPolicy(key, namespace string) bool {
	objs := &whv1.SignerPolicyList{}
	if err := c.cachedClient.List(watcher.Selector{Namespace: namespace}, objs); err != nil {
		log.Println(err)
		return false
	}

	// If no policy but is signed, it's valid
	if len(objs.Items) == 0 {
		return true
	}

	for _, signerPolicy := range objs.Items {
		for _, signerName := range signerPolicy.Spec.Signers {
			signer := &regv1.SignerKey{}
			if err := c.restClient.Get().AbsPath("apis/tmax.io/v1").Resource("signerkeys").Name(signerName).Do(context.Background()).Into(signer); err != nil {
				log.Printf("signer getting error by %s", err)
				continue
			}

			for _, targetKey := range signer.Spec.Targets {
				if targetKey.ID == key {
					return true
				}
			}
		}
	}

	return false
}
