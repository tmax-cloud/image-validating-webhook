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
	pod := core.Pod{}
	if err := json.Unmarshal(review.Request.Object.Raw, &pod); err != nil {
		log.Printf("unmarshaling request failed with %s", err)
		review.Response = &v1beta1.AdmissionResponse{
			Allowed: false,
			Result: &v1.Status{
				Message: fmt.Sprintf("Internal webhook server error: %s", err),
			},
		}
		return err
	}

	log.Printf("INFO: Start to handle review of pod %s in %s", pod.Name, pod.Namespace)

	handler, err := newDockerHandler(pod)
	if err != nil {
		log.Printf("ERROR: Couldn't make docker handler by %s", err)
		review.Response = &v1beta1.AdmissionResponse{
			Allowed: false,
			Result: &v1.Status{
				Message: fmt.Sprintf("Internal webhook server error: %s", err),
			},
		}
		return err
	}

	isValid := handler.isNamespaceInWhiteList()
	var name = ""
	if !isValid {
		log.Println("INFO: This pod's namespace is not in the white list")
		isValid, name = handler.isValid()
	}

	if isValid {
		log.Println("INFO: Pod is valid")
		review.Response = &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	} else {
		log.Println("INFO: Pod is invalid")
		review.Response = &v1beta1.AdmissionResponse{
			Allowed: false,
			Result: &v1.Status{
				Message: fmt.Sprintf("Image '%s' is not signed", name),
			},
		}
	}

	return nil
}
