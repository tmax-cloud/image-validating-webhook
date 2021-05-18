package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"os"
	"regexp"
	"strings"
)

const (
	whitelistConfigMap = "image-validation-webhook-whitelist"

	whitelistByImage     = "whitelist-images"
	whitelistByNamespace = "whitelist-namespaces"

	whitelistByImageLegacy     = "whitelist-image.json"
	whitelistByNamespaceLegacy = "whitelist-namespace.json"
)

const (
	delimiter = "\n"
)

var whiteListDir = "/etc/webhook/config/"

var whitelistImageReg = regexp.MustCompile(`^((([^./]+)\.([^/])+)/)?([^:@]+)(:([^@]+))?(@([^:]+:[0-9a-f]+))?`)

func whitelistByImageFile() string {
	return whiteListDir + whitelistByImage
}

func whitelistByNamespaceFile() string {
	return whiteListDir + whitelistByNamespace
}

func whitelistByImageLegacyFile() string {
	return whiteListDir + whitelistByImageLegacy
}

func whitelistByNamespaceLegacyFile() string {
	return whiteListDir + whitelistByNamespaceLegacy
}

// ReadWhiteList reads whitelist from config map files
func ReadWhiteList(clientSet kubernetes.Interface) (*WhiteList, error) {
	wl := &WhiteList{}
	// Read Image whitelist
	imagef, err := ioutil.ReadFile(whitelistByImageFile())
	if err != nil {
		// If is NOT notFound, return error
		if !os.IsNotExist(err) {
			return nil, err
		}

		// Fallback to legacy
		imageLegacyf, err := ioutil.ReadFile(whitelistByImageLegacyFile())
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
		if _, err := clientSet.CoreV1().ConfigMaps(registryNamespace).Patch(context.Background(), whitelistConfigMap, types.StrategicMergePatchType, b, metav1.PatchOptions{}); err != nil {
			return nil, err
		}
	} else {
		if err := wl.UnmarshalImage(imagef); err != nil {
			return nil, err
		}
	}

	// Read
	namespacef, err := ioutil.ReadFile(whitelistByNamespaceFile())
	if err != nil {
		// If is NOT notFound, return error
		if !os.IsNotExist(err) {
			return nil, err
		}

		// Fallback to legacy
		namespaceLegacyf, err := ioutil.ReadFile(whitelistByNamespaceLegacyFile())
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
		if _, err := clientSet.CoreV1().ConfigMaps(registryNamespace).Patch(context.Background(), whitelistConfigMap, types.StrategicMergePatchType, b, metav1.PatchOptions{}); err != nil {
			return nil, err
		}
	} else {
		wl.UnmarshalNamespace(namespacef)
	}

	return wl, nil
}

// WhiteList stores whitelisted images/namespaces
type WhiteList struct {
	byImages     []imageRef
	byNamespaces []string
}

// Unmarshal parses whitelist lists from line-separated lists
func (w *WhiteList) Unmarshal(img, ns []byte) error {
	// Parse byImages
	if err := w.UnmarshalImage(img); err != nil {
		return err
	}
	// Parse byNamespaces
	w.UnmarshalNamespace(ns)
	return nil
}

// UnmarshalImage parses image whitelist from line-separated lists
func (w *WhiteList) UnmarshalImage(img []byte) error {
	var err error
	w.byImages, err = parseImages(parseLineSeparatedList(img))
	if err != nil {
		return err
	}
	return nil
}

// UnmarshalNamespace parses namespace whitelist from line-separated lists
func (w *WhiteList) UnmarshalNamespace(ns []byte) {
	w.byNamespaces = parseLineSeparatedList(ns)
}

// Marshal generates whitelist byte arrays from lists
func (w *WhiteList) Marshal() ([]byte, []byte) {
	var images []string
	for _, i := range w.byImages {
		images = append(images, i.String())
	}
	return []byte(strings.Join(images, delimiter)), []byte(strings.Join(w.byNamespaces, delimiter))
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
	var err error
	var byImages []string
	if err := json.Unmarshal(img, &byImages); err != nil {
		return err
	}

	w.byImages, err = parseImages(byImages)
	if err != nil {
		return err
	}
	return nil
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

type imageRef struct {
	host   string
	name   string
	tag    string
	digest string
}

func (r *imageRef) String() string {
	var b bytes.Buffer

	if r.host != "" {
		b.WriteString(r.host)
		b.WriteString("/")
	}

	b.WriteString(r.name)

	if r.tag != "" {
		b.WriteString(":")
		b.WriteString(r.tag)
	}

	if r.digest != "" {
		b.WriteString("@")
		b.WriteString(r.digest)
	}

	return b.String()
}

func parseImages(images []string) ([]imageRef, error) {
	var results []imageRef
	for _, i := range images {
		ref, err := parseImage(i)
		if err != nil {
			return nil, err
		}
		results = append(results, *ref)
	}
	return results, nil
}

func parseImage(image string) (*imageRef, error) {
	matched := whitelistImageReg.FindAllStringSubmatch(image, -1)
	if len(matched) != 1 || len(matched[0]) != 10 {
		return nil, fmt.Errorf("image is not in right form")
	}

	ref := &imageRef{
		host:   strings.TrimSpace(matched[0][2]),
		name:   strings.TrimSpace(matched[0][5]),
		tag:    strings.TrimSpace(matched[0][7]),
		digest: strings.TrimSpace(matched[0][9]),
	}

	if ref.name == "" {
		return nil, fmt.Errorf("image name is required")
	}

	return ref, nil
}
