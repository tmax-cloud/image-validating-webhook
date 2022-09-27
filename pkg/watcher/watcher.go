package watcher

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	watcherLog = logf.Log.WithName("watcher.go")
)

// Handler an interface for watch events
type Handler interface {
	Handle(runtime.Object) error
}

// Watcher is an interface of k8s object watcher
type Watcher interface {
	Start(chan struct{})
	SetHandler(Handler)

	getIndexer() cache.Indexer
}

type watcher struct {
	queue    workqueue.RateLimitingInterface
	indexer  cache.Indexer
	informer cache.Controller

	stopCh chan struct{}

	handler Handler
}

// New creates a new watcher for the given object
func New(namespace, resourceKind string, obj runtime.Object, restCli rest.Interface, selector fields.Selector) Watcher {
	listWatcher := cache.NewListWatchFromClient(restCli, resourceKind, namespace, selector)
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	//cache.NewSharedIndexInformer()
	indexer, informer := cache.NewIndexerInformer(listWatcher, obj, 0, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(newObj)
			if err == nil {
				queue.Add(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
	}, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})

	w := &watcher{
		queue:    queue,
		indexer:  indexer,
		informer: informer,

		stopCh: make(chan struct{}),
	}

	return w
}

func (w *watcher) Start(waitCh chan struct{}) {
	// Start informer sync
	go w.informer.Run(w.stopCh)

	// Wait until cache is synced
	if !cache.WaitForCacheSync(w.stopCh, w.informer.HasSynced) {
		panic(fmt.Errorf("timed out waiting for caches to sync"))
	}

	waitCh <- struct{}{}

	// Start watcher func
	wait.Until(w.watch, time.Second, w.stopCh)
}

func (w *watcher) SetHandler(handler Handler) {
	w.handler = handler
}

func (w *watcher) getIndexer() cache.Indexer {
	return w.indexer
}

func (w *watcher) watch() {
	for w.processNextItem() {
	}
}

func (w *watcher) processNextItem() bool {
	key, quit := w.queue.Get()
	if quit {
		return false
	}

	defer w.queue.Done(key)

	keyStr, ok := key.(string)
	if !ok {
		watcherLog.Info("key is not a string")
		return true
	}

	// Get from cache
	obj, exists, _ := w.indexer.GetByKey(keyStr)
	if !exists {
		msg := fmt.Sprintf("resource %s not found\n", keyStr)
		watcherLog.Info(msg)
		return true
	}

	// Check if it's runtime object
	rObj, isObj := obj.(runtime.Object)
	if !isObj {
		msg := fmt.Sprintf("cache contained %T, which is not an Object\n", obj)
		watcherLog.Info(msg)
		return true
	}

	if w.handler != nil {
		if err := w.handler.Handle(rObj.DeepCopyObject()); err != nil {
			watcherLog.Error(err, "")
			return true
		}
	}

	return true
}
