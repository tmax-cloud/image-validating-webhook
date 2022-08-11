package pods

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/tmax-cloud/image-validating-webhook/pkg/server"
	"k8s.io/client-go/kubernetes/scheme"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	registryNamespace = "registry-system"
)

var (
	plog = logf.Log.WithName("pods")
)

func init() {
	// Add validating-mutating-admission handler initiator
	server.AddHandlerInitiator("/validate", []string{http.MethodPost}, NewPodsAdmissionHandler)
}

// ImageAdmission is ...
type ImageAdmission struct {
	validator Validator
}

// NewPodsAdmissionHandler initiates a new image validation admission handler
func NewPodsAdmissionHandler(cfg *server.HandlerConfig) (http.Handler, error) {
	v, err := newValidator(cfg.RestCfg, cfg.ClientSet, cfg.RestClient)
	if err != nil {
		return nil, err
	}

	return &ImageAdmission{validator: v}, nil
}

func (a *ImageAdmission) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		errMsg := fmt.Sprintf("Couldn't read request by %s", err)
		plog.Error(err, errMsg)
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	plog.Info("Handling request")

	review := &admissionv1beta1.AdmissionReview{}
	if _, _, err = scheme.Codecs.UniversalDeserializer().Decode(body, nil, review); err != nil {
		errMsg := fmt.Sprintf("Couldn't decode request by %s", err)
		plog.Error(err, errMsg)
		setReviewResponseNotAllowed(review, errMsg)
		if err := writeReviewResponse(review, w); err != nil {
			plog.Error(err, "")
		}
		return
	}

	// Handle Admission
	if err := a.HandleAdmission(review); err != nil {
		errMsg := fmt.Sprintf("Couldn't handle admission request by %s", err)
		plog.Error(err, errMsg)
		setReviewResponseNotAllowed(review, errMsg)
		if err := writeReviewResponse(review, w); err != nil {
			plog.Error(err, "")
		}
		return
	}

	// Return response
	if err := writeReviewResponse(review, w); err != nil {
		plog.Error(err, "")
	}
}

// HandleAdmission is ...
func (a *ImageAdmission) HandleAdmission(review *admissionv1beta1.AdmissionReview) error {
	pod := &core.Pod{}
	if err := json.Unmarshal(review.Request.Object.Raw, pod); err != nil {
		errMsg := fmt.Sprintf("unmarshaling request failed with %s", err)
		plog.Error(err, errMsg)
		setReviewResponseNotAllowed(review, fmt.Sprintf("Internal webhook server error: %s", err))
		return err
	}
	pod.Namespace = review.Request.Namespace

	infoMsg := fmt.Sprintf("Start to handle review of pod %s(%s) in %s", pod.Name, pod.GenerateName, pod.Namespace)
	plog.Info(infoMsg)

	// Validate image signers
	isValid, invalidReason, err := a.validator.CheckIsValidAndAddDigest(pod)
	if err != nil {
		errMsg := fmt.Sprintf("Error while validating images by %s", err)
		plog.Error(err, errMsg)
		setReviewResponseNotAllowed(review, fmt.Sprintf("Internal webhook server error: %s", err))
		return err
	} else if isValid {
		plog.Info("Pod is valid")
		patch, err := createPatch(pod)
		if err != nil {
			errMsg := fmt.Sprintf("Couldn't make patched pod by %s", err)
			plog.Error(err, errMsg)
			setReviewResponseNotAllowed(review, fmt.Sprintf("Internal webhook server error: %s", err))
			return err
		}

		patchType := admissionv1beta1.PatchTypeJSONPatch
		review.Response = &admissionv1beta1.AdmissionResponse{
			Allowed:   true,
			Result:    &metav1.Status{},
			Patch:     patch,
			PatchType: &patchType,
		}
	} else {
		plog.Info("Pod is invalid")
		setReviewResponseNotAllowed(review, fmt.Sprintf("Pod is not valid: %s", invalidReason))
	}

	return nil
}

func setReviewResponseNotAllowed(review *admissionv1beta1.AdmissionReview, message string) {
	review.Response = &admissionv1beta1.AdmissionResponse{
		Allowed: false,
		Result: &metav1.Status{
			Message: message,
		},
	}
}

func writeReviewResponse(review *admissionv1beta1.AdmissionReview, w http.ResponseWriter) error {
	responseInBytes, err := json.Marshal(review)
	if err != nil {
		return err
	}
	if _, err := w.Write(responseInBytes); err != nil {
		return err
	}
	return nil
}

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
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
