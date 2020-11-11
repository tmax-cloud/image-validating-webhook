package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"

	"github.com/eddy-kor-92/image-webhook/internal/k8s"
	"k8s.io/api/admission/v1beta1"
	core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	dndPod       = "dnd-pod"
	dndContainer = "dnd-container"
	dndNamespace = "default"
)

// ImageValidationAdmission is ...
type ImageValidationAdmission struct {
}

// ExecResult is ...
type ExecResult struct {
	OutBuffer *bytes.Buffer
	ErrBuffer *bytes.Buffer
}

// HandleAdmission is ...
func (*ImageValidationAdmission) HandleAdmission(review *v1beta1.AdmissionReview) error {
	pod := core.Pod{}

	log.Println("Handling review")

	if err := json.Unmarshal(review.Request.Object.Raw, &pod); err != nil {
		return fmt.Errorf("unmarshaling request failed with %s", err)
	}

	isValid := true
	name := "default image"

	for _, container := range pod.Spec.Containers {
		isValid = isValid && isSignedImage(container.Image)

		if !isValid {
			name = container.Image
			break
		}
	}

	review.Response = &v1beta1.AdmissionResponse{
		Allowed: isValid,
		Result: &v1.Status{
			Message: fmt.Sprintf("Image '%s' is not signed", name),
		},
	}

	return nil
}

func isSignedImage(image string) bool {
	result := &ExecResult{
		OutBuffer: &bytes.Buffer{},
		ErrBuffer: &bytes.Buffer{},
	}

	command := fmt.Sprintf("docker trust inspect %s", image)

	if err := k8s.ExecCmd(dndPod, dndContainer, dndNamespace, command, nil, result.OutBuffer, result.ErrBuffer); err != nil {
		log.Panicf("Failed to execute command to docker daemon by %s", err)
	}

	log.Println(result.OutBuffer.String())

	return false
}
