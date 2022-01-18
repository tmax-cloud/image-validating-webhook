package fake

import (
	"fmt"
	"github.com/tmax-cloud/image-validating-webhook/pkg/watcher"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sort"
	"strings"
)

// CachedClient is a fake watcher.CachedClient for testing
type CachedClient struct {
	Cache map[string]runtime.Object
}

// Get gets an object from the cache
func (c *CachedClient) Get(key types.NamespacedName, out runtime.Object) error {
	keyStr := ""
	if key.Namespace != "" {
		keyStr += key.Namespace + "/"
	}
	keyStr += key.Name

	obj, ok := c.Cache[keyStr]
	if !ok {
		return fmt.Errorf("not found")
	}

	obj = obj.DeepCopyObject()

	outVal := reflect.ValueOf(out)
	objVal := reflect.ValueOf(obj)
	if !objVal.Type().AssignableTo(outVal.Type()) {
		return fmt.Errorf("cache had type %s, but %s was asked for", objVal.Type(), outVal.Type())
	}
	reflect.Indirect(outVal).Set(reflect.Indirect(objVal))

	return nil
}

// List lists objects from the cache
func (c *CachedClient) List(sel watcher.Selector, out runtime.Object) error {
	var objs []runtime.Object
	for k, obj := range c.Cache {
		if sel.Namespace == "" || strings.HasPrefix(k, fmt.Sprintf("%s/", sel.Namespace)) {
			objs = append(objs, obj)
		}
	}

	sort.Slice(objs, func(i, j int) bool {
		aMeta, err := apimeta.Accessor(objs[i])
		if err != nil {
			return false
		}
		bMeta, err := apimeta.Accessor(objs[j])
		if err != nil {
			return false
		}

		return fmt.Sprintf("%s/%s", aMeta.GetNamespace(), aMeta.GetName()) < fmt.Sprintf("%s/%s", bMeta.GetNamespace(), bMeta.GetName())
	})

	runtimeObjs := make([]runtime.Object, 0, len(objs))
	for _, obj := range objs {
		outObj := obj.DeepCopyObject()
		runtimeObjs = append(runtimeObjs, outObj)
	}
	return apimeta.SetList(out, runtimeObjs)
}
