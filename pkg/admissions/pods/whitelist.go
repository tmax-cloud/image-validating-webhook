package pods

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/tmax-cloud/image-validating-webhook/internal/k8s"
	"github.com/tmax-cloud/image-validating-webhook/pkg/watcher"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
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
var wlog = ctrl.Log.WithName("whitelist.go")

// WhiteList stores whitelisted images/namespaces
type WhiteList struct {
	byImages     []imageRef
	byNamespaces []string

	lock sync.Mutex

	clientSet    kubernetes.Interface
	cachedClient watcher.CachedClient
}

func newWhiteList(cfg *rest.Config, clientSet kubernetes.Interface) (*WhiteList, error) {
	wl := &WhiteList{
		clientSet: clientSet,
	}

	// Create watcher client for corev1
	watchCli, err := k8s.NewGroupVersionClient(cfg, corev1.SchemeGroupVersion)
	if err != nil {
		panic(err)
	}

	// Initiate watcher
	w := watcher.New(registryNamespace, string(corev1.ResourceConfigMaps), &corev1.ConfigMap{}, watchCli, fields.ParseSelectorOrDie(fmt.Sprintf("metadata.name=%s", whitelistConfigMap)))
	wl.cachedClient = watcher.NewCachedClient(w)

	w.SetHandler(wl)

	waitCh := make(chan struct{})

	// Start to watch white list config map
	go w.Start(waitCh)

	// Block until it's ready
	<-waitCh

	return wl, nil
}

// Handle handles a whitelist configmap update event
func (w *WhiteList) Handle(object runtime.Object) error {
	cm, ok := object.(*corev1.ConfigMap)
	if !ok {
		return fmt.Errorf("object is not a ConfigMap")
	}

	if err := w.ParseOrUpdateWhiteList(cm); err != nil {
		return err
	}
	return nil
}

// ParseOrUpdateWhiteList reads whitelist from the config map data and updates it if it's still legacy
func (w *WhiteList) ParseOrUpdateWhiteList(cm *corev1.ConfigMap) error {
	w.lock.Lock()
	defer w.lock.Unlock()

	wlog.Info("Whitelist is updated. Parsing...")

	// Read Image whitelist
	imageWhiteList, iwExist := cm.Data[whitelistByImage]
	if iwExist {
		if err := w.UnmarshalImage(imageWhiteList); err != nil {
			return err
		}
	} else {
		// Fallback to legacy
		imageWhiteListLegacy, exist := cm.Data[whitelistByImageLegacy]
		if !exist {
			return fmt.Errorf("there are neither %s nor %s in whitelist", whitelistByImage, whitelistByImageLegacy)
		}
		if err := w.UnmarshalLegacyImage(imageWhiteListLegacy); err != nil {
			return err
		}

		// Update ConfigMap!
		converted, _ := w.Marshal()
		b, err := json.Marshal(&corev1.ConfigMap{Data: map[string]string{whitelistByImage: converted}})
		if err != nil {
			return err
		}
		if _, err := w.clientSet.CoreV1().ConfigMaps(registryNamespace).Patch(context.Background(), whitelistConfigMap, types.StrategicMergePatchType, b, metav1.PatchOptions{}); err != nil {
			return err
		}
	}

	// Read Namespace whitelist
	nsWhiteList, nwExist := cm.Data[whitelistByNamespace]
	if nwExist {
		w.UnmarshalNamespace(nsWhiteList)
	} else {
		// Fallback to legacy
		nsWhiteListLegacy, exist := cm.Data[whitelistByNamespaceLegacy]
		if !exist {
			return fmt.Errorf("there are neither %s nor %s in whitelist", whitelistByNamespace, whitelistByNamespaceLegacy)
		}
		if err := w.UnmarshalLegacyNamespace(nsWhiteListLegacy); err != nil {
			return err
		}

		// Update ConfigMap!
		_, converted := w.Marshal()
		b, err := json.Marshal(&corev1.ConfigMap{Data: map[string]string{whitelistByNamespace: converted}})
		if err != nil {
			return err
		}
		if _, err := w.clientSet.CoreV1().ConfigMaps(registryNamespace).Patch(context.Background(), whitelistConfigMap, types.StrategicMergePatchType, b, metav1.PatchOptions{}); err != nil {
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
func (w *WhiteList) IsImageWhiteListed(imageURI string) bool {
	w.lock.Lock()
	defer w.lock.Unlock()

	img, err := parseImage(imageURI)
	if err != nil {
		wlog.Error(err, "Image WhiteListed Error")
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
