package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/tmax-cloud/image-validating-webhook/internal/k8s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"log"
	"regexp"
	"strings"
	"sync"
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

var whitelistImageReg = regexp.MustCompile(`^((([^./]+)\.([^/])+)/)?([^:@]+)(:([^@]+))?(@([^:]+:[0-9a-f]+))?`)

// WhiteList stores whitelisted images/namespaces
type WhiteList struct {
	byImages     []imageRef
	byNamespaces []string

	client kubernetes.Interface

	lock sync.Mutex
}

func newWhiteList(client kubernetes.Interface) (*WhiteList, error) {
	wl := &WhiteList{
		client: client,
	}

	var wg sync.WaitGroup
	wg.Add(1)

	// Start to watch white list config map
	go wl.watch(&wg)

	// Block until it's ready
	wg.Wait()

	return wl, nil
}

func (w *WhiteList) watch(initWait *sync.WaitGroup) {
	ns, err := k8s.Namespace()
	if err != nil {
		log.Fatal(err)
	}
	// Get first
	cm, err := w.client.CoreV1().ConfigMaps(ns).Get(context.Background(), whitelistConfigMap, metav1.GetOptions{})
	if err != nil {
		log.Fatal(err)
	}
	if err := w.ParseOrUpdateWhiteList(cm.Data, w.client); err != nil {
		log.Fatal(err)
	}

	initWait.Done()

	lastResourceVersion := cm.ResourceVersion
	for {
		// Watch
		watcher, err := w.client.CoreV1().ConfigMaps(ns).Watch(context.Background(), metav1.ListOptions{FieldSelector: fmt.Sprintf("metadata.name=%s", whitelistConfigMap)})
		if err != nil {
			log.Println(err)
			continue
		}

		for e := range watcher.ResultChan() {
			cm, ok := e.Object.(*corev1.ConfigMap)
			if !ok || cm.Name != whitelistConfigMap {
				continue
			}

			// Check resourceVersion - remove redundant processing
			if lastResourceVersion == cm.ResourceVersion {
				continue
			}

			log.Println("Whitelist is updated")

			if err := w.ParseOrUpdateWhiteList(cm.Data, w.client); err != nil {
				log.Println(err)
				continue
			}

			lastResourceVersion = cm.ResourceVersion
		}
	}
}

// ParseOrUpdateWhiteList reads whitelist from the config map data and updates it if it's still legacy
func (w *WhiteList) ParseOrUpdateWhiteList(data map[string]string, clientSet kubernetes.Interface) error {
	w.lock.Lock()
	defer w.lock.Unlock()

	// Read Image whitelist
	imageWhiteList, iwExist := data[whitelistByImage]
	if iwExist {
		if err := w.UnmarshalImage(imageWhiteList); err != nil {
			return err
		}
	} else {
		// Fallback to legacy
		imageWhiteListLegacy, exist := data[whitelistByImageLegacy]
		if !exist {
			return fmt.Errorf("there are neither %s nor %s in whitelist", whitelistByImage, whitelistByImageLegacy)
		}
		if err := w.UnmarshalLegacyImage(imageWhiteListLegacy); err != nil {
			return err
		}

		// Update ConfigMap!
		converted, _ := w.Marshal()
		b, err := json.Marshal(&corev1.ConfigMap{Data: map[string]string{whitelistByImage: string(converted)}})
		if err != nil {
			return err
		}
		if _, err := clientSet.CoreV1().ConfigMaps(registryNamespace).Patch(context.Background(), whitelistConfigMap, types.StrategicMergePatchType, b, metav1.PatchOptions{}); err != nil {
			return err
		}
	}

	// Read Namespace whitelist
	nsWhiteList, nwExist := data[whitelistByNamespace]
	if nwExist {
		w.UnmarshalNamespace(nsWhiteList)
	} else {
		// Fallback to legacy
		nsWhiteListLegacy, exist := data[whitelistByNamespaceLegacy]
		if !exist {
			return fmt.Errorf("there are neither %s nor %s in whitelist", whitelistByNamespace, whitelistByNamespaceLegacy)
		}
		if err := w.UnmarshalLegacyNamespace(nsWhiteListLegacy); err != nil {
			return err
		}

		// Update ConfigMap!
		_, converted := w.Marshal()
		b, err := json.Marshal(&corev1.ConfigMap{Data: map[string]string{whitelistByNamespace: string(converted)}})
		if err != nil {
			return err
		}
		if _, err := clientSet.CoreV1().ConfigMaps(registryNamespace).Patch(context.Background(), whitelistConfigMap, types.StrategicMergePatchType, b, metav1.PatchOptions{}); err != nil {
			return err
		}
	}

	return nil
}

// IsNamespaceWhiteListed checks if ns is whitelisted
func (w *WhiteList) IsNamespaceWhiteListed(ns string) bool {
	w.lock.Lock()
	defer w.lock.Unlock()

	for _, whiteListNamespace := range w.byNamespaces {
		if ns == whiteListNamespace {
			return true
		}
	}
	return false
}

// IsImageWhiteListed checks if an image is whitelisted
func (w *WhiteList) IsImageWhiteListed(imageUri string) bool {
	w.lock.Lock()
	defer w.lock.Unlock()

	img, err := parseImage(imageUri)
	if err != nil {
		log.Println(err)
		return false
	}

	for _, i := range w.byImages {
		match := (i.host == "" || i.host == img.host) &&
			(i.name == "*" || i.name == img.name) &&
			(i.tag == "" || i.tag == img.tag) &&
			(i.digest == "" || i.digest == img.digest)

		if match {
			return true
		}
	}
	return false
}

// Unmarshal parses whitelist lists from line-separated lists
func (w *WhiteList) Unmarshal(img, ns string) error {
	// Parse byImages
	if err := w.UnmarshalImage(img); err != nil {
		return err
	}
	// Parse byNamespaces
	w.UnmarshalNamespace(ns)
	return nil
}

// UnmarshalImage parses image whitelist from line-separated lists
func (w *WhiteList) UnmarshalImage(img string) error {
	var err error
	w.byImages, err = parseImages(parseLineSeparatedList(img))
	if err != nil {
		return err
	}
	return nil
}

// UnmarshalNamespace parses namespace whitelist from line-separated lists
func (w *WhiteList) UnmarshalNamespace(ns string) {
	w.byNamespaces = parseLineSeparatedList(ns)
}

// Marshal generates whitelist byte arrays from lists
func (w *WhiteList) Marshal() (string, string) {
	var images []string
	for _, i := range w.byImages {
		images = append(images, i.String())
	}
	return strings.Join(images, delimiter), strings.Join(w.byNamespaces, delimiter)
}

// UnmarshalLegacy parses whitelist lists from json array
func (w *WhiteList) UnmarshalLegacy(img, ns string) error {
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
func (w *WhiteList) UnmarshalLegacyImage(img string) error {
	var err error
	var byImages []string
	if err := json.Unmarshal([]byte(img), &byImages); err != nil {
		return err
	}

	w.byImages, err = parseImages(byImages)
	if err != nil {
		return err
	}
	return nil
}

// UnmarshalLegacyNamespace parses namespace whitelist from json array
func (w *WhiteList) UnmarshalLegacyNamespace(ns string) error {
	return json.Unmarshal([]byte(ns), &w.byNamespaces)
}

func parseLineSeparatedList(list string) []string {
	var result []string

	tokens := strings.Split(list, delimiter)
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
