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
	core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type DockerHandler struct {
	client       *kubernetes.Clientset
	whiteList    *[]string
	pod          core.Pod
	currentImage ImageInfo
	dindPodName  string
}

type ImageInfo struct {
	registry string
	name     string
	tag      string
}

// ExecResult is ...
type ExecResult struct {
	OutBuffer *bytes.Buffer
	ErrBuffer *bytes.Buffer
}

func newDockerHandler(pod core.Pod) (*DockerHandler, error) {
	kubeCfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)
	restCfg, _ := kubeCfg.ClientConfig()
	clientset, _ := kubernetes.NewForConfig(restCfg)
	tmaxiov1.AddToScheme(scheme)

	f, err := ioutil.ReadFile(whitelist)
	if err != nil {
		return nil, fmt.Errorf("reading white list config file failed by %s", err)
	}

	var list []string
	if err := json.Unmarshal(f, &list); err != nil {
		return nil, fmt.Errorf("unmarshaling white list failed by %s", err)
	}

	pods, _ := clientset.CoreV1().Pods(dindNamespace).List(context.TODO(), v1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", dindDeployment),
	})
	dindPod := core.Pod{}
	if len(pods.Items) > 0 {
		dindPod = pods.Items[0]
	}

	return &DockerHandler{
		client:      clientset,
		pod:         pod,
		whiteList:   &list,
		dindPodName: dindPod.GetName(),
	}, nil
}

func (h *DockerHandler) isValid() (bool, string) {
	isValid := true
	name := ""

	containers := append(h.pod.Spec.InitContainers, h.pod.Spec.Containers...)
	for _, container := range containers {
		isValid = h.isInWhiteList(container.Image) || isValid && h.isSignedImage(container.Image)

		if !isValid {
			name = container.Image
			break
		}
	}

	return isValid, name
}

func (h *DockerHandler) isSignedImage(image string) bool {
	imageInfo := getImageInfo(image)
	notaryServer, err := h.findNotaryServer(imageInfo.registry)
	if err != nil {
		log.Printf("Couldn't find notary server by: %s", err)
		return false
	}

	var command string
	if imageInfo.registry == "docker.io" {
		command = fmt.Sprintf("docker trust inspect %s:%s", imageInfo.name, imageInfo.tag)
	} else {
		if err := h.loginToRegistry(imageInfo.registry); err != nil {
			log.Printf("Couldn't login to registry named %s: by %s", imageInfo.registry, err)
		}
		command = fmt.Sprintf("export DOCKER_CONTENT_TRUST_SERVER=%s; docker trust inspect %s/%s:%s", notaryServer, imageInfo.registry, imageInfo.name, imageInfo.tag)
	}

	result, err := h.execToDockerDaemon(command)
	if err != nil {
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

func (h *DockerHandler) execToDockerDaemon(command string) (*ExecResult, error) {
	result := &ExecResult{
		OutBuffer: &bytes.Buffer{},
		ErrBuffer: &bytes.Buffer{},
	}

	if err := k8s.ExecCmd(h.dindPodName, dindContainer, dindNamespace, command, nil, result.OutBuffer, result.ErrBuffer); err != nil {
		return result, err
	}

	return result, nil
}

func (h *DockerHandler) loginToRegistry(registry string) error {
	pullSecrets := h.pod.Spec.ImagePullSecrets
	if len(pullSecrets) <= 0 {
		return fmt.Errorf("There's any pullSecret")
	}

	for _, pullSecret := range pullSecrets {
		secret, err := h.getSecret(pullSecret.Name)
		if err != nil {
			log.Printf("Couldn't get secret named %s by %s", pullSecret.Name, err)
			break
		}
		id, idExist := secret.Data["ID"]
		pw, pwExist := secret.Data["PASSWD"]
		if idExist && pwExist {
			result, err := h.execToDockerDaemon(fmt.Sprintf("docker login %s -u %s -p %s", registry, id, pw))
			if err != nil {
				log.Printf("Couldn't exec docker login command by %s", err)
				continue
			}

			if strings.Contains(result.OutBuffer.String(), "Login Succeeded") {
				return nil
			}
		}
	}

	return fmt.Errorf("There's no pullSecret to login to registry named %s", registry)
}

func (h *DockerHandler) getSecret(secretName string) (*core.Secret, error) {
	allSecrets, err := h.client.CoreV1().Secrets("").List(context.TODO(), v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var result core.Secret
	exist := false
	for _, secret := range allSecrets.Items {
		if secret.Name == secretName {
			result = secret
			exist = true
			break
		}
	}

	if exist {
		return &result, nil
	}

	return nil, fmt.Errorf("There's no secret named %s", secretName)
}

func (h *DockerHandler) isInWhiteList(image string) bool {
	imageInfo := getImageInfo(image)
	validFormatImage := fmt.Sprintf("%s/%s:%s", imageInfo.registry, imageInfo.name, imageInfo.tag)
	for _, whiteListImage := range *h.whiteList {
		return strings.Contains(validFormatImage, whiteListImage)
	}

	return false
}

func (h *DockerHandler) findNotaryServer(registry string) (string, error) {
	if registry == "docker.io" {
		return "", nil
	}

	var targetReg *regv1.Registry
	regList := h.getRegistries()
	for _, reg := range regList.Items {
		if "https://"+registry == reg.Status.ServerURL {
			targetReg = &reg
			break
		}
	}

	if targetReg == nil {
		return "", fmt.Errorf("No matched registry")
	}

	return targetReg.Status.NotaryURL, nil
}

func (h *DockerHandler) getRegistries() *regv1.RegistryList {
	regList := &regv1.RegistryList{}
	if err := h.client.RESTClient().Get().AbsPath("/apis/tmax.io/v1").Resource("registries").Do(context.TODO()).Into(regList); err != nil {
		log.Printf("reg list err %s", err)
	}

	return regList
}

func getImageInfo(image string) ImageInfo {
	var host, name, tag string

	host = image

	if strings.Contains(host, "/") {
		temp := strings.Split(host, "/")
		host = temp[0]
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

	return ImageInfo{
		registry: host,
		name:     name,
		tag:      tag,
	}
}
