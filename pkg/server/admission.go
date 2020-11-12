package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/eddy-kor-92/image-webhook/internal/k8s"
	"k8s.io/api/admission/v1beta1"
	core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	dindDeployment = "internal-docker-daemon"
	dindContainer  = "dind-daemon"
	dindNamespace  = "default"
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

	command := fmt.Sprintf("docker trust inspect %s", makeTaggedImage(image))

	if err := k8s.ExecCmd(dindDeployment, dindContainer, dindNamespace, command, nil, result.OutBuffer, result.ErrBuffer); err != nil {
		log.Printf("Failed to execute command to docker daemon by %s", err)
	}

	if result.OutBuffer == nil {
		log.Panicf("Failed to get signature of image %s", image)
	}

	return !strings.Contains(result.OutBuffer.String(), "No signatures")
}

func makeTaggedImage(image string) string {
	if strings.Contains(image, ":") {
		return image
	}

	return image + ":latest"
}
