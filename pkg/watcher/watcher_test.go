package watcher

import (
	"bytes"
	"github.com/bmizerany/assert"
	whv1 "github.com/tmax-cloud/image-validating-webhook/pkg/type"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	restfake "k8s.io/client-go/rest/fake"
	"net/http"
	"reflect"
	"testing"
)

func TestNew(t *testing.T) {
	cli := testWatcherRestClient()
	wi := New("", "", &corev1.Pod{}, cli, fields.Everything())

	w, ok := wi.(*watcher)
	assert.Equal(t, true, ok, "assertion")

	// Queue test
	t.Run("queue", func(t *testing.T) {
		assert.Equal(t, true, w.queue != nil, "queue")

		assert.Equal(t, 0, w.queue.Len(), "len")
		w.queue.Add(&corev1.Pod{Spec: corev1.PodSpec{SchedulerName: "test"}})
		assert.Equal(t, 1, w.queue.Len(), "len")
		obj, quit := w.queue.Get()
		assert.Equal(t, false, quit, "quit")
		pod, ok := obj.(*corev1.Pod)
		assert.Equal(t, true, ok, "assertion")
		assert.Equal(t, "test", pod.Spec.SchedulerName, "testVal")
		w.queue.Done(obj)
		assert.Equal(t, 0, w.queue.Len(), "len")
	})

	// Indexer test
	t.Run("indexer", func(t *testing.T) {
		assert.Equal(t, true, w.indexer != nil, "indexer")

		pods := []*corev1.Pod{
			{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}, Spec: corev1.PodSpec{SchedulerName: "test-1"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "test2", Namespace: "default"}, Spec: corev1.PodSpec{SchedulerName: "test-2"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "kube-system"}, Spec: corev1.PodSpec{SchedulerName: "system-1"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "test2", Namespace: "kube-system"}, Spec: corev1.PodSpec{SchedulerName: "system-2"}},
		}
		for _, pod := range pods {
			if err := w.indexer.Add(pod); err != nil {
				t.Fatal(err)
			}
		}

		t.Run("namespaceIndex", func(t *testing.T) {
			type testCase struct {
				key string
				val string

				errorOccurs  bool
				errorMessage string
				expectedLen  int
			}

			tc := map[string]testCase{
				"namespaceDefault": {
					key:         "namespace",
					val:         "default",
					errorOccurs: false,
					expectedLen: 2,
				},
				"namespaceSystem": {
					key:         "namespace",
					val:         "kube-system",
					errorOccurs: false,
					expectedLen: 2,
				},
				"namespaceTest": {
					key:         "namespace",
					val:         "test",
					errorOccurs: false,
					expectedLen: 0,
				},
			}

			for name, c := range tc {
				t.Run(name, func(t *testing.T) {
					objs, err := w.indexer.ByIndex(c.key, c.val)
					if err != nil {
						assert.Equal(t, c.errorOccurs, true, "error occurs")
						assert.Equal(t, c.errorMessage, "", "error message")
					} else {
						assert.Equal(t, c.errorOccurs, false, "error occurs")
						assert.Equal(t, c.expectedLen, len(objs), "len")
					}
				})
			}
		})
	})

	// Informer test
	t.Run("informer", func(t *testing.T) {
		assert.Equal(t, true, w.informer != nil, "informer")
		assert.Equal(t, false, w.informer.HasSynced(), "synced")
	})
}

type testWatcherHandler struct {
	obj runtime.Object
}

func (t *testWatcherHandler) Handle(object runtime.Object) error {
	t.obj = object
	return nil
}

func TestWatcher_SetHandler(t *testing.T) {
	cli := testWatcherRestClient()
	wi := New("", "", &corev1.Pod{}, cli, fields.Everything())

	hdl := &testWatcherHandler{}

	w, ok := wi.(*watcher)
	assert.Equal(t, true, ok, "assertion")

	wi.SetHandler(hdl)
	assert.Equal(t, true, w.handler != nil, "handler")

	pod := &corev1.Pod{Spec: corev1.PodSpec{SchedulerName: "test-sche"}}
	if err := w.handler.Handle(pod); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, true, reflect.DeepEqual(pod, hdl.obj), "deep equal")
}

func TestWatcher_GetClient(t *testing.T) {
	cli := testWatcherRestClient()
	wi := New("", "", &corev1.Pod{}, cli, fields.Everything())

	w, ok := wi.(*watcher)
	assert.Equal(t, true, ok, "assertion")

	pods := []*corev1.Pod{
		{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}, Spec: corev1.PodSpec{SchedulerName: "test"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "test2", Namespace: "default"}, Spec: corev1.PodSpec{SchedulerName: "test2"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "test1", Namespace: "kube-system"}, Spec: corev1.PodSpec{SchedulerName: "system-1"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "test2", Namespace: "kube-system"}, Spec: corev1.PodSpec{SchedulerName: "system-2"}},
	}
	for _, pod := range pods {
		if err := w.indexer.Add(pod); err != nil {
			t.Fatal(err)
		}
	}

	c := NewCachedClient(w)

	pod := &corev1.Pod{}
	if err := c.Get(types.NamespacedName{Name: "test", Namespace: "default"}, pod); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "test", pod.Spec.SchedulerName, "test val")

	if err := c.Get(types.NamespacedName{Name: "test3", Namespace: "default"}, pod); err == nil {
		t.Fatal("not found error should occur")
	}
}

func TestWatcher_Start(t *testing.T) {
	cli := testWatcherRestClient()
	wi := New("", "", &corev1.Pod{}, cli, fields.Everything())

	go wi.Start(nil)

	// TODO
}

func testWatcherRestClient() *restfake.RESTClient {
	return &restfake.RESTClient{
		GroupVersion:         whv1.GroupVersion,
		NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
		Client: restfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusNotFound, Body: ioutil.NopCloser(&bytes.Buffer{})}, nil
		}),
	}
}
