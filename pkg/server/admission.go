package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/tmax-cloud/image-validating-webhook/internal/k8s"
	regv1 "github.com/tmax-cloud/registry-operator/api/v1"
	restclient "k8s.io/client-go/rest"
	"log"

	"k8s.io/api/admission/v1beta1"
	core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	registryNamespace = "registry-system"
)

// ImageValidationAdmission is ...
type ImageValidationAdmission struct {
	validator *validator
}

// NewImageValidationAdmissionHandler initiates a new image validation admission handler
func NewImageValidationAdmissionHandler() (*ImageValidationAdmission, error) {
	clientSet, restCli, err := k8s.NewClientSet()
	if err != nil {
		return nil, err
	}

	v, err := newValidator(clientSet, restCli, getFindHyperCloudNotaryServerFn(restCli))
	if err != nil {
		return nil, err
	}

	return &ImageValidationAdmission{validator: v}, nil
}

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

	name := pod.Name
	if name == "" {
		name = pod.GenerateName
	}
	pod.Namespace = review.Request.Namespace

	log.Printf("INFO: Start to handle review of pod %s in %s", name, pod.Namespace)

	// Validate image signers
	isValid, invalidImageName, err := a.validator.CheckIsValidAndAddDigest(pod)
	if err != nil {
		log.Printf("ERROR: Error while validating images by %s", err)
		review.Response = &v1beta1.AdmissionResponse{
			Allowed: false,
			Result: &v1.Status{
				Message: fmt.Sprintf("Internal webhook server error: %s", err),
			},
		}
		return err
	} else if isValid {
		log.Println("INFO: Pod is valid")
		patch, err := createPatch(pod)
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
				Message: fmt.Sprintf("Image '%s' is not signed", invalidImageName),
			},
		}
	}

	return nil
}

func getFindHyperCloudNotaryServerFn(restClient restclient.Interface) findNotaryServerFn {
	return func(registry string) string {
		if registry == "docker.io" {
			return ""
		}

		var targetReg *regv1.Registry
		regList := &regv1.RegistryList{}
		if err := restClient.Get().AbsPath("/apis/tmax.io/v1").Resource("registries").Do(context.Background()).Into(regList); err != nil {
			log.Printf("reg list err %s", err)
		}
		for _, reg := range regList.Items {
			if "https://"+registry == reg.Status.ServerURL {
				targetReg = &reg
				break
			}
		}

		if targetReg == nil {
			log.Printf("No matched registry named: %s. Couldn't find notary server", registry)
			return ""
		}

		return targetReg.Status.NotaryURL
	}
}

func createPatch(patchPod *core.Pod) ([]byte, error) {
	if patchPod == nil {
		return nil, fmt.Errorf("couldn't create patch")
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
