package server

import (
	"context"
	"encoding/json"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"os"
	"strings"
)

const (
	whitelistConfigMap = "image-validation-webhook-whitelist"

	whiteListDir = "/etc/webhook/config/"

	whitelistByImage     = "whitelist-images"
	whitelistByNamespace = "whitelist-namespaces"

	whitelistByImageFile     = whiteListDir + whitelistByImage
	whitelistByNamespaceFile = whiteListDir + whitelistByNamespace

	whitelistByImageLegacy     = "whitelist-image.json"
	whitelistByNamespaceLegacy = "whitelist-namespace.json"

	whitelistByImageLegacyFile     = whiteListDir + whitelistByImageLegacy
	whitelistByNamespaceLegacyFile = whiteListDir + whitelistByNamespaceLegacy
)

const (
	delimiter = "\n"
)

// ReadWhiteList reads whitelist from config map files
func ReadWhiteList(clientSet *kubernetes.Clientset) (*WhiteList, error) {
	wl := &WhiteList{}
	// Read Image whitelist
	imagef, err := ioutil.ReadFile(whitelistByImageFile)
	if err != nil {
		// If is NOT notFound, return error
		if !os.IsNotExist(err) {
			return nil, err
		}

		// Fallback to legacy
		imageLegacyf, err := ioutil.ReadFile(whitelistByImageLegacyFile)
		if err != nil {
			return nil, err
		}
		if err := wl.UnmarshalLegacyImage(imageLegacyf); err != nil {
			return nil, err
		}

		// Update ConfigMap!
		converted, _ := wl.Marshal()
		b, err := json.Marshal(&corev1.ConfigMap{Data: map[string]string{whitelistByImage: string(converted)}})
		if err != nil {
			return nil, err
		}
		if _, err := clientSet.CoreV1().ConfigMaps(dindNamespace).Patch(context.Background(), whitelistConfigMap, types.StrategicMergePatchType, b, metav1.PatchOptions{}); err != nil {
			return nil, err
		}
	} else {
		wl.UnmarshalImage(imagef)
	}

	// Read
	namespacef, err := ioutil.ReadFile(whitelistByNamespaceFile)
	if err != nil {
		// If is NOT notFound, return error
		if !os.IsNotExist(err) {
			return nil, err
		}

		// Fallback to legacy
		namespaceLegacyf, err := ioutil.ReadFile(whitelistByNamespaceLegacyFile)
		if err != nil {
			return nil, err
		}
		if err := wl.UnmarshalLegacyNamespace(namespaceLegacyf); err != nil {
			return nil, err
		}

		// Update ConfigMap!
		_, converted := wl.Marshal()
		b, err := json.Marshal(&corev1.ConfigMap{Data: map[string]string{whitelistByNamespace: string(converted)}})
		if err != nil {
			return nil, err
		}
		if _, err := clientSet.CoreV1().ConfigMaps(dindNamespace).Patch(context.Background(), whitelistConfigMap, types.StrategicMergePatchType, b, metav1.PatchOptions{}); err != nil {
			return nil, err
		}
	} else {
		wl.UnmarshalNamespace(namespacef)
	}

	return wl, nil
}

// WhiteList stores whitelisted images/namespaces
type WhiteList struct {
	byImages     []string
	byNamespaces []string
}

// Unmarshal parses whitelist lists from line-separated lists
func (w *WhiteList) Unmarshal(img, ns []byte) {
	// Parse byImages
	w.UnmarshalImage(img)
	// Parse byNamespaces
	w.UnmarshalNamespace(ns)
}

// UnmarshalImage parses image whitelist from line-separated lists
func (w *WhiteList) UnmarshalImage(img []byte) {
	w.byImages = parseLineSeparatedList(img)
}

// UnmarshalNamespace parses namespace whitelist from line-separated lists
func (w *WhiteList) UnmarshalNamespace(ns []byte) {
	w.byNamespaces = parseLineSeparatedList(ns)
}

// Marshal generates whitelist byte arrays from lists
func (w *WhiteList) Marshal() ([]byte, []byte) {
	return []byte(strings.Join(w.byImages, delimiter)), []byte(strings.Join(w.byNamespaces, delimiter))
}

// UnmarshalLegacy parses whitelist lists from json array
func (w *WhiteList) UnmarshalLegacy(img, ns []byte) error {
	// Parse byImages
	if err := w.UnmarshalLegacyImage(img); err != nil {
		return err
	}
	// Parse byNamespace
	if err := w.UnmarshalLegacyNamespace(ns); err != nil {
		return err
	}
	return nil
}

// UnmarshalLegacyImage parses image whitelist from json array
func (w *WhiteList) UnmarshalLegacyImage(img []byte) error {
	return json.Unmarshal(img, &w.byImages)
}

// UnmarshalLegacyNamespace parses namespace whitelist from json array
func (w *WhiteList) UnmarshalLegacyNamespace(ns []byte) error {
	return json.Unmarshal(ns, &w.byNamespaces)
}

func parseLineSeparatedList(list []byte) []string {
	var result []string

	tokens := strings.Split(string(list), delimiter)
	for _, line := range tokens {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}
