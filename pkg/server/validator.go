package server

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/tmax-cloud/image-validating-webhook/internal/utils"
	"github.com/tmax-cloud/registry-operator/pkg/image"
	restclient "k8s.io/client-go/rest"
	"log"
	"math/rand"
	"strings"
	"time"

	whv1 "github.com/tmax-cloud/image-validating-webhook/pkg/type"
	regv1 "github.com/tmax-cloud/registry-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// validator handles overall process to check signs
type validator struct {
	client         kubernetes.Interface
	restClient     restclient.Interface
	pod            *corev1.Pod
	patch          *corev1.Pod
	whiteList      WhiteList
	signerPolicies []whv1.SignerPolicy

	findNotaryServer findNotaryServerFn
}

type findNotaryServerFn func(registryHost, namespace string) string

func newValidator(clientset kubernetes.Interface, restClient restclient.Interface, findNotaryFn findNotaryServerFn, pod *corev1.Pod) (*validator, error) {
	// Read whitelist
	wl, err := ReadWhiteList(clientset)
	if err != nil {
		return nil, err
	}

	signerPolicies := &whv1.SignerPolicyList{}
	if err := restClient.
		Get().AbsPath("apis/tmax.io/v1").
		Resource("signerpolicies").
		Namespace(pod.Namespace).
		Do(context.Background()).
		Into(signerPolicies); err != nil {
		return nil, fmt.Errorf("signer policies error, %s", err)
	}

	return &validator{
		client:           clientset,
		restClient:       restClient,
		pod:              pod,
		patch:            pod.DeepCopy(),
		whiteList:        *wl,
		signerPolicies:   signerPolicies.Items,
		findNotaryServer: findNotaryFn,
	}, nil
}

// GetPatch generates a patch to update pod spec
func (h *validator) GetPatch() *corev1.Pod {
	return h.patch
}

func getDigest(tag string, signature Signature) string {
	digest := ""
	for _, signedTag := range signature.SignedTags {
		if signedTag.SignedTag == tag {
			digest = signedTag.Digest
		}
	}

	return digest
}

// IsValid checks if images of initContainers and containers are valid
func (h *validator) IsValid() (bool, string, error) {
	// Check initContainers
	if isValid, name, err := h.addDigestWhenImageValid(h.patch.Spec.InitContainers); err != nil {
		return false, "", err
	} else if !isValid {
		return false, name, nil
	}
	// Check containers
	if isValid, name, err := h.addDigestWhenImageValid(h.patch.Spec.Containers); err != nil {
		return false, "", err
	} else if !isValid {
		return false, name, nil
	}

	return true, "", nil
}

func (h *validator) addDigestWhenImageValid(containers []corev1.Container) (bool, string, error) {
	for i, container := range containers {
		if !h.isImageInWhiteList(container.Image) {
			isValid, digest, err := h.isSignedImage(container.Image)
			if err != nil {
				return false, "", err
			}
			if !isValid {
				return false, container.Image, nil
			}
			containers[i].Image = fmt.Sprintf("%s@sha256:%s", container.Image, digest)
		}
	}

	return true, "", nil
}

func (h *validator) isSignedImage(imageUri string) (bool, string, error) {
	img, err := image.NewImage(imageUri, "", "", nil)
	if err != nil {
		log.Println(err)
		return false, "", err
	}

	// Get registry basic auth
	img.BasicAuth, err = h.getBasicAuthForRegistry(img.Host)
	if err != nil {
		log.Println(err)
		return false, "", err
	}

	// Get trust info of the image
	sig, err := fetchSignature(img, h.findNotaryServer(img.Host, h.pod.Namespace))
	if err != nil {
		log.Println(err)
		return false, "", err
	}

	// If not signed, sig is nil
	if sig == nil {
		return false, "", nil
	}

	// Check if it meets signer policy
	if h.hasMatchedSigner(*sig) {
		digest := getDigest(img.Tag, *sig)
		return true, digest, nil
	}

	// Is NOT valid if it does not match any policy
	return false, "", nil
}

func (h *validator) hasMatchedSigner(signature Signature) bool {
	// If no policy but is signed, it's valid
	if len(h.signerPolicies) == 0 {
		return true
	}

	key := signature.getRepoAdminKey()

	for _, signerPolicy := range h.signerPolicies {
		for _, signerName := range signerPolicy.Spec.Signers {
			signer := &regv1.SignerKey{}
			if err := h.restClient.Get().AbsPath("apis/tmax.io/v1").Resource("signerkeys").Name(signerName).Do(context.Background()).Into(signer); err != nil {
				log.Printf("signer getting error by %s", err)
			}

			for _, targetKey := range signer.Spec.Targets {
				if targetKey.ID == key {
					return true
				}
			}
		}
	}

	return false
}

func (h *validator) getBasicAuthForRegistry(host string) (string, error) {
	for _, pullSecret := range h.pod.Spec.ImagePullSecrets {
		secret, err := h.client.CoreV1().Secrets(h.pod.Namespace).Get(context.Background(), pullSecret.Name, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("couldn't get secret named %s by %s", pullSecret.Name, err)
		}
		imagePullSecret, err := utils.NewImagePullSecret(secret)
		if err != nil {
			return "", err
		}
		basicAuth, err := imagePullSecret.GetHostBasicAuth(h.findRegistryServer(host))
		if err != nil {
			return "", err
		}
		if basicAuth == "" {
			continue
		}

		return base64.StdEncoding.EncodeToString([]byte(basicAuth)), nil
	}

	// DO NOT return error - the image may be public
	return "", nil
}

func (h *validator) isImageInWhiteList(imageUri string) bool {
	img, err := image.NewImage(imageUri, "", "", nil)
	if err != nil {
		log.Println(err)
		return false
	}
	validImageName := fmt.Sprintf("%s/%s:%s", img.Host, img.Name, img.Tag)
	for _, whiteListImage := range h.whiteList.byImages {
		if strings.Contains(validImageName, whiteListImage) {
			return true
		}
	}
	return false
}

func (h *validator) IsNamespaceInWhiteList(ns string) bool {
	for _, whiteListNamespace := range h.whiteList.byNamespaces {
		if ns == whiteListNamespace {
			return true
		}
	}
	return false
}

func (h *validator) findRegistryServer(registry string) string {
	if registry == "docker.io" {
		return "https://registry-1.docker.io"
	}
	return "https://" + registry
}

func randomString(length int) string {
	rand.Seed(time.Now().UnixNano())
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	charset := "abcdefghijklmnopqrstuvwxyz1234567890"
	str := make([]byte, length)

	for i := range str {
		str[i] = charset[seededRand.Intn(len(charset))]
	}

	return string(str)
}
