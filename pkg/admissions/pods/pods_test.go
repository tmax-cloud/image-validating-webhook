package pods

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

type imageAdmissionHandlerTestCase struct {
	gvk      metav1.GroupVersionKind
	gvr      metav1.GroupVersionResource
	resource runtime.Object

	expectedAllowed       bool
	expectedResultMessage string
}

func TestImageAdmission_HandleAdmission(t *testing.T) {
	tc := map[string]imageAdmissionHandlerTestCase{
		"podNotSigned": {
			gvk: metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			gvr: metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			resource: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "testns"},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "test-cont", Image: "test-not-signed:test"},
					},
				},
			},
			expectedAllowed:       false,
			expectedResultMessage: "Pod is not valid: \nimage 'test-not-signed:test' is not signed",
		},
		"podSigned": {
			gvk: metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			gvr: metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			resource: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "testns"},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "test-cont", Image: "test-signed:test"},
					},
				},
			},
			expectedAllowed: true,
		},
	}

	for name, c := range tc {
		t.Run(name, func(t *testing.T) {
			im := &ImageAdmission{validator: &dummyValidator{}}

			metaObj, err := meta.Accessor(c.resource)
			require.NoError(t, err)

			review := &admissionv1beta1.AdmissionReview{
				TypeMeta: metav1.TypeMeta{APIVersion: admissionv1beta1.SchemeGroupVersion.String(), Kind: "AdmissionReview"},
				Request: &admissionv1beta1.AdmissionRequest{
					UID:             types.UID("test-uid"),
					Kind:            c.gvk,
					Resource:        c.gvr,
					RequestKind:     &c.gvk,
					RequestResource: &c.gvr,
					Name:            metaObj.GetName(),
					Namespace:       metaObj.GetNamespace(),
					Operation:       admissionv1beta1.Create,
					UserInfo:        authenticationv1.UserInfo{Username: "test-user"},
					Object:          runtime.RawExtension{Object: c.resource},
				},
			}
			review.Request.Object.Raw, err = json.Marshal(c.resource)
			require.NoError(t, err)

			require.NoError(t, im.HandleAdmission(review))
			require.Equal(t, review.Response.Allowed, c.expectedAllowed)
			require.Equal(t, review.Response.Result.Message, c.expectedResultMessage)
		})
	}
}

type dummyValidator struct{}

func (d *dummyValidator) CheckIsValidAndAddDigest(pod *corev1.Pod) (bool, string, error) {
	var containers []corev1.Container
	containers = append(containers, pod.Spec.InitContainers...)
	containers = append(containers, pod.Spec.Containers...)

	for _, c := range containers {
		if strings.HasPrefix(c.Image, "test-not-signed") {
			return false, fmt.Sprintf("image '%s' is not signed", c.Image), nil
		}
	}

	return true, "", nil
}
