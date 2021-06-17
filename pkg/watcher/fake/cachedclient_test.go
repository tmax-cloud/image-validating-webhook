package fake

import (
	"github.com/stretchr/testify/require"
	whv1 "github.com/tmax-cloud/image-validating-webhook/pkg/type"
	"github.com/tmax-cloud/image-validating-webhook/pkg/watcher"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"testing"
)

type testCachedClientGetTestCase struct {
	cacheMap map[string]runtime.Object

	key types.NamespacedName

	errorOccurs    bool
	errorMessage   string
	expectedObject runtime.Object
}

func TestCachedClient_Get(t *testing.T) {
	tc := map[string]testCachedClientGetTestCase{
		"pod": {
			cacheMap: map[string]runtime.Object{
				"default/test": &corev1.Pod{
					Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "test"}}},
				},
			},
			key:         types.NamespacedName{Name: "test", Namespace: "default"},
			errorOccurs: false,
			expectedObject: &corev1.Pod{
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "test"}}},
			},
		},
		"configmap": {
			cacheMap: map[string]runtime.Object{
				"default/test": &corev1.ConfigMap{
					Data: map[string]string{"testKey": "testVal"},
				},
			},
			key:         types.NamespacedName{Name: "test", Namespace: "default"},
			errorOccurs: false,
			expectedObject: &corev1.ConfigMap{
				Data: map[string]string{"testKey": "testVal"},
			},
		},
		"signerPolicy": {
			cacheMap: map[string]runtime.Object{
				"default/test": &whv1.SignerPolicy{
					Spec: whv1.SignerPolicySpec{Signers: []string{"test-signer"}},
				},
			},
			key:         types.NamespacedName{Name: "test", Namespace: "default"},
			errorOccurs: false,
			expectedObject: &whv1.SignerPolicy{
				Spec: whv1.SignerPolicySpec{Signers: []string{"test-signer"}},
			},
		},
		"notFound": {
			cacheMap: map[string]runtime.Object{
				"default/test": &corev1.ConfigMap{
					Data: map[string]string{"testKey": "testVal"},
				},
			},
			key:          types.NamespacedName{Name: "not-found", Namespace: "default"},
			errorOccurs:  true,
			errorMessage: "not found",
			expectedObject: &corev1.ConfigMap{
				Data: map[string]string{"testKey": "testVal"},
			},
		},
	}

	for name, c := range tc {
		t.Run(name, func(t *testing.T) {
			cli := &CachedClient{Cache: c.cacheMap}

			out := c.expectedObject.DeepCopyObject()
			err := cli.Get(c.key, out)
			if c.errorOccurs {
				require.Error(t, err, "error occurs")
				require.Equal(t, c.errorMessage, err.Error(), "error message")
			} else {
				require.NoError(t, err, "error occurs")
				require.Equal(t, c.expectedObject, out, "output")
			}
		})
	}
}

type testCachedClientListCase struct {
	cacheMap map[string]runtime.Object

	namespaceKey       string
	errorOccurs        bool
	errorMessage       string
	expectedObjectList runtime.Object
}

func TestCachedClient_List(t *testing.T) {
	tc := map[string]testCachedClientListCase{
		"podsAll": {
			cacheMap: map[string]runtime.Object{
				"default/test":  &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "test"}}}},
				"default/test2": &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test2", Namespace: "default"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "test2"}}}},
				"test/test2":    &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test2", Namespace: "test"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "test23"}}}},
			},
			namespaceKey: "",
			errorOccurs:  false,
			expectedObjectList: &corev1.PodList{
				Items: []corev1.Pod{
					{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "test"}}}},
					{ObjectMeta: metav1.ObjectMeta{Name: "test2", Namespace: "default"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "test2"}}}},
					{ObjectMeta: metav1.ObjectMeta{Name: "test2", Namespace: "test"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "test23"}}}},
				},
			},
		},
		"podsDefault": {
			cacheMap: map[string]runtime.Object{
				"default/test":  &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "test"}}}},
				"default/test2": &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test2", Namespace: "default"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "test2"}}}},
				"test/test2":    &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test2", Namespace: "test"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "test23"}}}},
			},
			namespaceKey: "default",
			errorOccurs:  false,
			expectedObjectList: &corev1.PodList{
				Items: []corev1.Pod{
					{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "test"}}}},
					{ObjectMeta: metav1.ObjectMeta{Name: "test2", Namespace: "default"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "test2"}}}},
				},
			},
		},
	}

	for name, c := range tc {
		t.Run(name, func(t *testing.T) {
			cli := &CachedClient{Cache: c.cacheMap}

			out := c.expectedObjectList.DeepCopyObject()
			err := cli.List(watcher.Selector{Namespace: c.namespaceKey}, out)
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
