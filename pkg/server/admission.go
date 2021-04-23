package server

import (
	"encoding/json"
	"fmt"
	"log"

	"k8s.io/api/admission/v1beta1"
	core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	dindDeployment       = "docker-daemon"
	dindContainer        = "dind-daemon"
	dindNamespace        = "registry-system"
	whitelistByImage     = "/etc/webhook/config/whitelist-image.json"
	whitelistByNamespace = "/etc/webhook/config/whitelist-namespace.json"
)

// ImageValidationAdmission is ...
type ImageValidationAdmission struct{}

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

// HandleAdmission is ...
func (a *ImageValidationAdmission) HandleAdmission(review *v1beta1.AdmissionReview) error {
	pod := &core.Pod{}
	if err := json.Unmarshal(review.Request.Object.Raw, pod); err != nil {
		log.Printf("unmarshaling request failed with %s", err)
		review.Response = &v1beta1.AdmissionResponse{
			Allowed: false,
			Result: &v1.Status{
				Message: fmt.Sprintf("Internal webhook server error: %s", err),
			},
		}
		return err
	}

	pod.Namespace = review.Request.Namespace

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

	if handler.isNamespaceInWhiteList() {
		log.Println("INFO: This pod's namespace is in the white list")
		review.Response = &v1beta1.AdmissionResponse{
			Allowed: true,
		}

		return nil
	}

	isValid, name := handler.isValid()
	if isValid {
		log.Println("INFO: Pod is valid")
		patch, err := createPatch(handler.GetPatch())
		if err != nil {
			log.Printf("ERROR: Couldn't make patched pod by %s", err)
			review.Response = &v1beta1.AdmissionResponse{
				Allowed: false,
				Result: &v1.Status{
					Message: fmt.Sprintf("Internal webhook server error: %s", err),
				},
			}

			return err
		}

		patchType := v1beta1.PatchTypeJSONPatch
		review.Response = &v1beta1.AdmissionResponse{
			Allowed:   true,
			Patch:     patch,
			PatchType: &patchType,
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

func createPatch(patchPod *core.Pod) ([]byte, error) {
	if patchPod == nil {
		return nil, fmt.Errorf("Couldn't create patch")
	}

	patch := []patchOperation{
		{
			Op:    "replace",
			Path:  "/spec/containers",
			Value: patchPod.Spec.Containers,
		},
	}

	if len(patchPod.Spec.InitContainers) > 0 {
		patch = append(patch, patchOperation{
			Op:    "replace",
			Path:  "/spec/initContainers",
			Value: patchPod.Spec.InitContainers,
		})
	}

	return json.Marshal(&patch)
}
