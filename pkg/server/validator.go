package server

import (
	"context"
	"fmt"
	"github.com/tmax-cloud/image-validating-webhook/internal/utils"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"log"
	"math/rand"
	"time"

	whv1 "github.com/tmax-cloud/image-validating-webhook/pkg/type"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func init() {
	if err := whv1.AddToScheme(scheme.Scheme); err != nil {
		log.Fatal(err)
	}
}

// validator handles overall process to check signs
type validator struct {
	client     kubernetes.Interface
	restClient restclient.Interface

	findNotaryServer findNotaryServerFn

	signerPolicyCache *SignerPolicyCache
	whiteList         *WhiteList
}

type findNotaryServerFn func(registryHost string) string

func newValidator(clientset kubernetes.Interface, restClient restclient.Interface, findNotaryFn findNotaryServerFn) (*validator, error) {
	v := &validator{
		client:           clientset,
		restClient:       restClient,
		findNotaryServer: findNotaryFn,
	}

	var err error

	// Initiate SignerPolicy cache
	v.signerPolicyCache, err = newSignerPolicyCache(restClient)
	if err != nil {
		return nil, err
	}

	// Initiate WhiteList cache
	v.whiteList, err = newWhiteList(clientset)
	if err != nil {
		return nil, err
	}

	return v, nil
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

// CheckIsValidAndAddDigest checks if images of initContainers and containers are valid
func (h *validator) CheckIsValidAndAddDigest(pod *corev1.Pod) (bool, string, error) {
	// Check namespace whitelist
	if h.whiteList.IsNamespaceWhiteListed(pod.Namespace) {
		return true, "", nil
	}

	// Check initContainers
	if isValid, name, err := h.addDigestWhenImageValid(pod.Spec.InitContainers, pod.Namespace, pod.Spec.ImagePullSecrets); err != nil {
		return false, "", err
	} else if !isValid {
		return false, name, nil
	}
	// Check containers
	if isValid, name, err := h.addDigestWhenImageValid(pod.Spec.Containers, pod.Namespace, pod.Spec.ImagePullSecrets); err != nil {
		return false, "", err
	} else if !isValid {
		return false, name, nil
	}

	return true, "", nil
}

func (h *validator) addDigestWhenImageValid(containers []corev1.Container, namespace string, pullSecrets []corev1.LocalObjectReference) (bool, string, error) {
	for i, container := range containers {
		// Check if it's whitelisted
		if h.whiteList.IsImageWhiteListed(container.Image) {
			continue
		}

		ref, err := parseImage(container.Image)
		if err != nil {
			return false, "", err
		}

		// Get registry basic auth
		basicAuth, err := h.getBasicAuthForRegistry(ref.host, namespace, pullSecrets)
		if err != nil {
			return false, "", err
		}

		// Get trust info of the image
		sig, err := fetchSignature(container.Image, basicAuth, h.findNotaryServer(ref.host))
		if err != nil {
			log.Println(err)
			return false, "", err
		}
		// sig is nil if it's not signed
		if sig == nil {
			return false, container.Image, nil
		}

		// Check if it meets signer policy
		if h.hasMatchedSigner(*sig, namespace) {
			digest := getDigest(ref.tag, *sig)

			// If digest is different from user-specified one, return error
			if ref.digest != "" && ref.digest != digest {
				return false, container.Image, nil
			}

			ref.digest = digest
			containers[i].Image = ref.String()
			return true, "", nil
		}

		// Does NOT match signer policy
		return false, container.Image, nil
	}

	return true, "", nil
}

func (h *validator) hasMatchedSigner(signature Signature, namespace string) bool {
	return h.signerPolicyCache.doesMatchPolicy(signature.getRepoAdminKey(), namespace)
}

func (h *validator) getBasicAuthForRegistry(host, namespace string, pullSecrets []corev1.LocalObjectReference) (string, error) {
	for _, pullSecret := range pullSecrets {
		secret, err := h.client.CoreV1().Secrets(namespace).Get(context.Background(), pullSecret.Name, metav1.GetOptions{})
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

		return basicAuth, nil
	}

	// DO NOT return error - the image may be public
	return "", nil
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
