package server

import (
	"context"
	"fmt"
	whv1 "github.com/tmax-cloud/image-validating-webhook/pkg/type"
	regv1 "github.com/tmax-cloud/registry-operator/api/v1"
	"k8s.io/apimachinery/pkg/watch"
	restclient "k8s.io/client-go/rest"
	"log"
	"sync"
)

// SignerPolicyCache is a cache of type.SignerPolicy
type SignerPolicyCache struct {
	policies map[string]map[string]*whv1.SignerPolicy

	restClient restclient.Interface

	lock sync.Mutex
}

func newSignerPolicyCache(restClient restclient.Interface) (*SignerPolicyCache, error) {
	p := &SignerPolicyCache{
		policies: map[string]map[string]*whv1.SignerPolicy{},

		restClient: restClient,
	}

	var wg sync.WaitGroup
	wg.Add(1)

	// Start to watch SignerPolicy
	go p.watch(&wg)

	// Block until it's ready
	wg.Wait()

	return p, nil
}

func (c *SignerPolicyCache) watch(initWait *sync.WaitGroup) {
	lastResourceVersions := map[string]string{}
	// List first
	sp := &whv1.SignerPolicyList{}
	if err := c.restClient.Get().AbsPath("apis/tmax.io/v1").Resource("signerpolicies").Do(context.Background()).Into(sp); err != nil {
		log.Fatal(err)
	}

	c.lock.Lock()
	c.policies = map[string]map[string]*whv1.SignerPolicy{}
	for _, p := range sp.Items {
		if c.policies[p.Namespace] == nil {
			c.policies[p.Namespace] = map[string]*whv1.SignerPolicy{}
		}
		log.Printf("SignerPolicy %s/%s is ADDED\n", p.Namespace, p.Name)
		c.policies[p.Namespace][p.Name] = &p
		lastResourceVersions[fmt.Sprintf("%s/%s", p.Namespace, p.Name)] = p.ResourceVersion
	}
	initWait.Done()
	c.lock.Unlock()

	// Watch forever
	for {
		w, err := c.restClient.Get().AbsPath("apis/tmax.io/v1").Resource("signerpolicies").Param("watch", "").Watch(context.Background())
		if err != nil {
			log.Println(err)
			continue
		}

		for e := range w.ResultChan() {
			policy, ok := e.Object.(*whv1.SignerPolicy)
			if !ok {
				continue
			}

			c.lock.Lock()
			if c.policies[policy.Namespace] == nil {
				c.policies[policy.Namespace] = map[string]*whv1.SignerPolicy{}
			}

			// Check resourceVersion - remove redundant processing
			if lastResourceVersions[fmt.Sprintf("%s/%s", policy.Namespace, policy.Name)] == policy.ResourceVersion {
				c.lock.Unlock()
				continue
			}

			log.Printf("SignerPolicy %s/%s is %s\n", policy.Namespace, policy.Name, e.Type)
			switch e.Type {
			case watch.Added, watch.Modified:
				c.policies[policy.Namespace][policy.Name] = policy
			case watch.Deleted:
				delete(c.policies[policy.Namespace], policy.Name)
			}
			lastResourceVersions[fmt.Sprintf("%s/%s", policy.Namespace, policy.Name)] = policy.ResourceVersion
			c.lock.Unlock()
		}
	}
}

func (c *SignerPolicyCache) doesMatchPolicy(key, namespace string) bool {
	c.lock.Lock()
	defer c.lock.Unlock()

	// If no policy but is signed, it's valid
	if len(c.policies[namespace]) == 0 {
		return true
	}

	for _, signerPolicy := range c.policies[namespace] {
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
