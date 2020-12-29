package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	"github.com/tmax-cloud/image-validating-webhook/internal/k8s"
	"k8s.io/api/admission/v1beta1"
	core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	dindDeployment = "docker-daemon"
	dindContainer  = "dind-daemon"
	dindNamespace  = "registry-system"
	whitelist      = "/etc/webhook/config/whitelist.json"
)

// ImageValidationAdmission is ...
type ImageValidationAdmission struct {
	whiteList *[]string
}

// ExecResult is ...
type ExecResult struct {
	OutBuffer *bytes.Buffer
	ErrBuffer *bytes.Buffer
}

// HandleAdmission is ...
func (a *ImageValidationAdmission) HandleAdmission(review *v1beta1.AdmissionReview) error {
	log.Println("Handling review")

	f, err := ioutil.ReadFile(whitelist)
	if err != nil {
		return fmt.Errorf("reading white list config file failed by %s", err)
	}

	var list []string
	if err := json.Unmarshal(f, &list); err != nil {
		return fmt.Errorf("unmarshaling white list failed by %s", err)
	}

	a.whiteList = &list
	log.Println(list)

	pod := core.Pod{}
	if err := json.Unmarshal(review.Request.Object.Raw, &pod); err != nil {
		return fmt.Errorf("unmarshaling request failed with %s", err)
	}

	isValid := true
	name := "default image"

	containers := append(pod.Spec.InitContainers, pod.Spec.Containers...)

	for _, container := range containers {
		isValid = a.isInWhiteList(container.Image) || isValid && isSignedImage(container.Image)

		if !isValid {
			name = container.Image
			break
		}
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

func (a *ImageValidationAdmission) isInWhiteList(image string) bool {
	for _, whiteListImage := range *a.whiteList {
		// TODO: 추가적인 상황 고려하여 로직 개선할 것
		return strings.Contains(image, whiteListImage)
	}

	return false
}

func isSignedImage(image string) bool {
	kubeCfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)

	restCfg, _ := kubeCfg.ClientConfig()

	clientset, _ := kubernetes.NewForConfig(restCfg)

	pods, _ := clientset.CoreV1().Pods(dindNamespace).List(context.TODO(), v1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", dindDeployment),
	})

	pod := core.Pod{}
	if len(pods.Items) > 0 {
		pod = pods.Items[0]
	}

	result := &ExecResult{
		OutBuffer: &bytes.Buffer{},
		ErrBuffer: &bytes.Buffer{},
	}

	command := fmt.Sprintf("docker trust inspect %s", makeTaggedImage(image))

	if err := k8s.ExecCmd(pod.GetName(), dindContainer, dindNamespace, command, nil, result.OutBuffer, result.ErrBuffer); err != nil {
		log.Printf("Failed to execute command to docker daemon by %s", err)
	}

	if result.OutBuffer.Len() <= 0 {
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
