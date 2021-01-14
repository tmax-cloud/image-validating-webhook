package server

import (
	"encoding/json"
	"fmt"
	"log"

	"k8s.io/api/admission/v1beta1"
	core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	dindDeployment       = "docker-daemon"
	dindContainer        = "dind-daemon"
	dindNamespace        = "registry-system"
	whitelistByImage     = "/etc/webhook/config/whitelist-image.json"
	whitelistByNamespace = "/etc/webhook/config/whitelist-namespace.json"
)

// ImageValidationAdmission is ...
type ImageValidationAdmission struct {
	client *kubernetes.Clientset
}

// HandleAdmission is ...
func (a *ImageValidationAdmission) HandleAdmission(review *v1beta1.AdmissionReview) error {
	log.Println("Handling review")

	pod := core.Pod{}
	if err := json.Unmarshal(review.Request.Object.Raw, &pod); err != nil {
		return fmt.Errorf("unmarshaling request failed with %s", err)
	}

	handler, err := newDockerHandler(pod)
	if err != nil {
		return fmt.Errorf("Couldn't create handler by %s", err)
	}

	isValid := handler.isNamespaceInWhiteList()
	var name = ""
	if !isValid {
		isValid, name = handler.isValid()
	}

	if isValid {
		review.Response = &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	} else {
		review.Response = &v1beta1.AdmissionResponse{
			Allowed: false,
			Result: &v1.Status{
				Message: fmt.Sprintf("Image '%s' is not signed", name),
			},
		}
	}

	return nil
}
