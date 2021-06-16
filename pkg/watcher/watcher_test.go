package watcher

import (
	"bytes"
	"github.com/stretchr/testify/require"
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
	"testing"
)

func TestNew(t *testing.T) {
	cli := testWatcherRestClient()
	wi := New("", "", &corev1.Pod{}, cli, fields.Everything())

	w, ok := wi.(*watcher)
	require.True(t, ok, "assertion")

	// Queue test
	t.Run("queue", func(t *testing.T) {
		require.NotNil(t, w.queue, "queue")

		require.Equal(t, 0, w.queue.Len())
		w.queue.Add(&corev1.Pod{Spec: corev1.PodSpec{SchedulerName: "test"}})
		require.Equal(t, 1, w.queue.Len())
		obj, quit := w.queue.Get()
		require.False(t, quit)
		pod, ok := obj.(*corev1.Pod)
		require.True(t, ok, "assertion")
		require.Equal(t, "test", pod.Spec.SchedulerName, "testVal")
		w.queue.Done(obj)
		require.Equal(t, 0, w.queue.Len())
	})

	// Indexer test
	t.Run("indexer", func(t *testing.T) {
		require.NotNil(t, w.indexer, "indexer")

		pods := []*corev1.Pod{
			{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}, Spec: corev1.PodSpec{SchedulerName: "test-1"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "test2", Namespace: "default"}, Spec: corev1.PodSpec{SchedulerName: "test-2"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "kube-system"}, Spec: corev1.PodSpec{SchedulerName: "system-1"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "test2", Namespace: "kube-system"}, Spec: corev1.PodSpec{SchedulerName: "system-2"}},
		}
		for _, pod := range pods {
			require.NoError(t, w.indexer.Add(pod))
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
					if c.errorOccurs {
						require.Error(t, err, "error occurs")
						require.Equal(t, c.errorMessage, "", "error message")
					} else {
						require.NoError(t, err, "error occurs")
						require.Len(t, objs, c.expectedLen)
					}
				})
			}
		})
	})

	// Informer test
	t.Run("informer", func(t *testing.T) {
		require.NotNil(t, w.informer, "informer")
		require.False(t, w.informer.HasSynced(), "synced")
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
	require.True(t, ok, "assertion")

	wi.SetHandler(hdl)
	require.NotNil(t, w.handler, "handler")

	pod := &corev1.Pod{Spec: corev1.PodSpec{SchedulerName: "test-sche"}}
	require.NoError(t, w.handler.Handle(pod))
	require.Equal(t, hdl.obj, pod, "pod")
}

func TestWatcher_GetClient(t *testing.T) {
	cli := testWatcherRestClient()
	wi := New("", "", &corev1.Pod{}, cli, fields.Everything())

	w, ok := wi.(*watcher)
	require.True(t, ok, "assertion")

	pods := []*corev1.Pod{
		{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}, Spec: corev1.PodSpec{SchedulerName: "test"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "test2", Namespace: "default"}, Spec: corev1.PodSpec{SchedulerName: "test2"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "test1", Namespace: "kube-system"}, Spec: corev1.PodSpec{SchedulerName: "system-1"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "test2", Namespace: "kube-system"}, Spec: corev1.PodSpec{SchedulerName: "system-2"}},
	}
	for _, pod := range pods {
		require.NoError(t, w.indexer.Add(pod))
	}

	c := NewCachedClient(w)

	pod := &corev1.Pod{}
	require.NoError(t, c.Get(types.NamespacedName{Name: "test", Namespace: "default"}, pod))
	require.Equal(t, "test", pod.Spec.SchedulerName, "test val")

	require.Error(t, c.Get(types.NamespacedName{Name: "test3", Namespace: "default"}, pod))
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
