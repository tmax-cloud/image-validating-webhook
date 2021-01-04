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
	regv1 "github.com/tmax-cloud/registry-operator/api/v1"
	tmaxiov1 "github.com/tmax-cloud/registry-operator/api/v1"
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
	client    *kubernetes.Clientset
}

// ExecResult is ...
type ExecResult struct {
	OutBuffer *bytes.Buffer
	ErrBuffer *bytes.Buffer
}

// NewAdmissionController is ...
func NewAdmissionController() *ImageValidationAdmission {
	kubeCfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)
	restCfg, _ := kubeCfg.ClientConfig()
	clientset, _ := kubernetes.NewForConfig(restCfg)
	tmaxiov1.AddToScheme(scheme)

	return &ImageValidationAdmission{
		client: clientset,
	}
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

	pod := core.Pod{}
	if err := json.Unmarshal(review.Request.Object.Raw, &pod); err != nil {
		return fmt.Errorf("unmarshaling request failed with %s", err)
	}

	isValid := true
	name := "default image"
	containers := append(pod.Spec.InitContainers, pod.Spec.Containers...)
	for _, container := range containers {
		isValid = a.isInWhiteList(container.Image) || isValid && a.isSignedImage(container.Image)

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
	host, name, tag := getHostAndTag(image)
	validFormatImage := fmt.Sprintf("%s/%s:%s", host, name, tag)
	for _, whiteListImage := range *a.whiteList {
		return strings.Contains(validFormatImage, whiteListImage)
	}

	return false
}

func (a *ImageValidationAdmission) isSignedImage(image string) bool {
	pods, _ := a.client.CoreV1().Pods(dindNamespace).List(context.TODO(), v1.ListOptions{
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
	host, name, tag := getHostAndTag(image)
	notaryServer, err := a.findNotaryServer(host)
	if err != nil {
		return false
	}

	var command string
	if host == "docker.io" {
		command = fmt.Sprintf("docker trust inspect %s:%s", name, tag)
	} else {
		command = fmt.Sprintf("export DOCKER_CONTENT_TRUST_SERVER=%s; docker trust inspect %s/%s:%s", notaryServer, host, name, tag)
	}

	if err := k8s.ExecCmd(pod.GetName(), dindContainer, dindNamespace, command, nil, result.OutBuffer, result.ErrBuffer); err != nil {
		log.Printf("Failed to execute command to docker daemon by %s", err)
	}

	if result.OutBuffer.Len() <= 0 {
		log.Panicf("Failed to get signature of image %s", image)
	}

	signatures, err := getSignatures(result.OutBuffer.String())
	if err != nil {
		log.Printf("Failed to get signature by %s", err)
		return false
	}

	return len(signatures) != 0
}

func (a *ImageValidationAdmission) getRegistries() *regv1.RegistryList {
	regList := &regv1.RegistryList{}
	if err := a.client.RESTClient().Get().AbsPath("/apis/tmax.io/v1").Resource("registries").Do(context.TODO()).Into(regList); err != nil {
		log.Printf("reg list err %s", err)
	}

	return regList
}

func (a *ImageValidationAdmission) findNotaryServer(host string) (string, error) {
	if host == "docker.io" {
		return "", nil
	}

	var targetReg *regv1.Registry
	regList := a.getRegistries()
	for _, reg := range regList.Items {
		if host == reg.Status.ServerURL {
			targetReg = &reg
			break
		}
	}

	if targetReg == nil {
		return "", fmt.Errorf("No matched registry")
	}

	return targetReg.Status.NotaryURL, nil
}

func getHostAndTag(image string) (string, string, string) {
	var host, name, tag, protocol string

	if strings.Contains(image, "https://") {
		protocol = "https://"
	} else if strings.Contains(image, "http://") {
		protocol = "http://"
	} else {
		protocol = ""
	}

	if protocol != "" {
		host = strings.Split(image, protocol)[1]
	}

	if strings.Contains(host, "/") {
		temp := strings.Split(host, "/")
		host = protocol + temp[0]
		name = temp[1]
	} else {
		host = "docker.io"
		name = image
	}

	if strings.Contains(name, ":") {
		temp := strings.Split(name, ":")
		name = temp[0]
		tag = temp[1]
	} else {
		tag = "latest"
	}

	return host, name, tag
}
