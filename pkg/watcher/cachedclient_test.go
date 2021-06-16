package watcher

import (
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"testing"
)

type testCachedClientGetCase struct {
	objs []runtime.Object

	getKey       types.NamespacedName
	expectedObj  runtime.Object
	errorOccurs  bool
	errorMessage string
}

func TestCachedClient_Get(t *testing.T) {
	tc := map[string]testCachedClientGetCase{
		"pods": {
			objs: []runtime.Object{
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "test"}}}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test2", Namespace: "default"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "test2"}}}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "controller", Namespace: "kube-system"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "controller"}}}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api-server", Namespace: "kube-system"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "api-server"}}}},
			},
			getKey:      types.NamespacedName{Name: "test2", Namespace: "default"},
			expectedObj: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test2", Namespace: "default"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "test2"}}}},
			errorOccurs: false,
		},
		"pods2": {
			objs: []runtime.Object{
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "test"}}}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test2", Namespace: "default"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "test2"}}}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "controller", Namespace: "kube-system"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "controller"}}}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api-server", Namespace: "kube-system"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "api-server"}}}},
			},
			getKey:      types.NamespacedName{Name: "controller", Namespace: "kube-system"},
			expectedObj: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "controller", Namespace: "kube-system"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "controller"}}}},
			errorOccurs: false,
		},
		"podsNotFound": {
			objs: []runtime.Object{
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "test"}}}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test2", Namespace: "default"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "test2"}}}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "controller", Namespace: "kube-system"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "controller"}}}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api-server", Namespace: "kube-system"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "api-server"}}}},
			},
			getKey:       types.NamespacedName{Name: "controller2", Namespace: "kube-system"},
			expectedObj:  &corev1.Pod{},
			errorOccurs:  true,
			errorMessage: " \"controller2\" not found",
		},
	}

	for name, c := range tc {
		t.Run(name, func(t *testing.T) {
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			for _, o := range c.objs {
				require.NoError(t, indexer.Add(o))
			}

			cc := &cachedClient{indexer: indexer}

			out := c.expectedObj.DeepCopyObject()
			err := cc.Get(c.getKey, out)
			if c.errorOccurs {
				require.Error(t, err, "error occurs")
				require.Equal(t, c.errorMessage, err.Error(), "error message")
			} else {
				require.NoError(t, err)
				require.Equal(t, c.expectedObj, out, "output")
			}
		})
	}
}

type testCachedClientListCase struct {
	objs []runtime.Object

	namespaceKey       string
	errorOccurs        bool
	errorMessage       string
	expectedObjectList runtime.Object
}

func TestCachedClient_List(t *testing.T) {
	tc := map[string]testCachedClientListCase{
		"podsAll": {
			objs: []runtime.Object{
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "test"}}}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test2", Namespace: "default"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "test2"}}}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api-server", Namespace: "kube-system"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "api-server"}}}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "controller", Namespace: "kube-system"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "controller"}}}},
			},
			namespaceKey: "",
			expectedObjectList: &corev1.PodList{Items: []corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "test"}}}},
				{ObjectMeta: metav1.ObjectMeta{Name: "test2", Namespace: "default"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "test2"}}}},
				{ObjectMeta: metav1.ObjectMeta{Name: "api-server", Namespace: "kube-system"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "api-server"}}}},
				{ObjectMeta: metav1.ObjectMeta{Name: "controller", Namespace: "kube-system"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "controller"}}}},
			}},
			errorOccurs: false,
		},
		"podsDefault": {
			objs: []runtime.Object{
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "test"}}}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test2", Namespace: "default"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "test2"}}}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api-server", Namespace: "kube-system"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "api-server"}}}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "controller", Namespace: "kube-system"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "controller"}}}},
			},
			namespaceKey: "default",
			expectedObjectList: &corev1.PodList{Items: []corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "test"}}}},
				{ObjectMeta: metav1.ObjectMeta{Name: "test2", Namespace: "default"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "test2"}}}},
			}},
			errorOccurs: false,
		},
		"podsSystem": {
			objs: []runtime.Object{
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "test"}}}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test2", Namespace: "default"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "test2"}}}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api-server", Namespace: "kube-system"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "api-server"}}}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "controller", Namespace: "kube-system"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "controller"}}}},
			},
			namespaceKey: "kube-system",
			expectedObjectList: &corev1.PodList{Items: []corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Name: "api-server", Namespace: "kube-system"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "api-server"}}}},
				{ObjectMeta: metav1.ObjectMeta{Name: "controller", Namespace: "kube-system"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "controller"}}}},
			}},
			errorOccurs: false,
		},
	}

	for name, c := range tc {
		t.Run(name, func(t *testing.T) {
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			for _, o := range c.objs {
				require.NoError(t, indexer.Add(o))
			}

			cc := &cachedClient{indexer: indexer}

			out := c.expectedObjectList.DeepCopyObject()
			err := cc.List(Selector{Namespace: c.namespaceKey}, out)
			if c.errorOccurs {
				require.Error(t, err, "error occurs")
				require.Equal(t, c.errorMessage, err.Error(), "error message")
			} else {
				require.NoError(t, err, "error occurs")
				require.Equal(t, c.expectedObjectList, out, "output")
			}
		})
	}
}
