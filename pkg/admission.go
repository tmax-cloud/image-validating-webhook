package server

import (
	"encoding/json"

	"k8s.io/api/admission/v1beta1"
	core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ImageValidationAdmission is ...
type ImageValidationAdmission struct {
}

// HandleAdmission is ...
func (*ImageValidationAdmission) HandleAdmission(review *v1beta1.AdmissionReview) error {
	pod := core.Pod{}

	if err := json.Unmarshal(review, &pod); err != nil {
		// logging error
	}

	isValid := true
	name := "default image" // testing code

	for _, container := range pod.Spec.Containers {
		name = container.Image // testing code
		isValid &= isSignedImage(container.Image)
	}

	review.Response = &v1beta1.AdmissionResponse{
		Allowed: isValid,
		Result: &v1.Status{
			Message: name,
		},
	}

	return nil
}

func isSignedImage(image string) bool {
	return true
}
