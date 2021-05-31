package watcher

import (
	"fmt"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"reflect"
	"sort"
)

// CachedClient is a reader from the cache
type CachedClient interface {
	Get(types.NamespacedName, runtime.Object) error
	List(Selector, runtime.Object) error
}

type cachedClient struct {
	indexer cache.Indexer
}

// NewCachedClient creates a cache reader client from the given Watcher
func NewCachedClient(w Watcher) CachedClient {
	return &cachedClient{indexer: w.getIndexer()}
}

func (g *cachedClient) Get(key types.NamespacedName, out runtime.Object) error {
	keyStr := ""
	if key.Namespace != "" {
		keyStr += key.Namespace + string(types.Separator)
	}
	keyStr += key.Name

	// Get from cache
	obj, exists, _ := g.indexer.GetByKey(keyStr)
	if !exists {
		return errors.NewNotFound(schema.GroupResource{}, key.Name)
	}

	// Check if it's runtime object
	if _, isObj := obj.(runtime.Object); !isObj {
		// This should never happen
		return fmt.Errorf("cache contained %T, which is not an Object", obj)
	}

	obj = obj.(runtime.Object).DeepCopyObject()

	outVal := reflect.ValueOf(out)
	objVal := reflect.ValueOf(obj)
	if !objVal.Type().AssignableTo(outVal.Type()) {
		return fmt.Errorf("cache had type %s, but %s was asked for", objVal.Type(), outVal.Type())
	}
	reflect.Indirect(outVal).Set(reflect.Indirect(objVal))

	return nil
}

func (g *cachedClient) List(sel Selector, out runtime.Object) error {
	var err error
	var objs []interface{}

	if sel.Namespace != "" {
		objs, err = g.indexer.ByIndex(cache.NamespaceIndex, sel.Namespace)
		if err != nil {
			return err
		}
	} else {
		objs = g.indexer.List()
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
	for _, item := range objs {
		obj, isObj := item.(runtime.Object)
		if !isObj {
			return fmt.Errorf("cache contained %T, which is not an Object", obj)
		}
		runtimeObjs = append(runtimeObjs, obj.DeepCopyObject())
	}
	return meta.SetList(out, runtimeObjs)
}

// Selector is an object-selector for list methods
type Selector struct {
	Namespace string
}
