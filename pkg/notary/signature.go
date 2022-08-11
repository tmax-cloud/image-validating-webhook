package notary

import (
	"fmt"
	"os"
	"strings"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/tmax-cloud/image-validating-webhook/internal/utils"
	"github.com/tmax-cloud/image-validating-webhook/pkg/image"
	"github.com/tmax-cloud/image-validating-webhook/pkg/trust"
)

var (
	signatureLog = logf.Log.WithName("signature")
)

// Signature is a sign info of an image
type Signature struct {
	Name       string      `json:"Name"`
	SignedTags []SignedTag `json:"SignedTags"`
}

// SignedTag is a tag-signature info
type SignedTag struct {
	SignedTag string   `json:"SignedTag"`
	Digest    string   `json:"Digest"`
	Signers   []string `json:"Signers"`
}

// GetDigest gets signed digest for the tag
func (s *Signature) GetDigest(tag string) string {
	digest := ""
	for _, signedTag := range s.SignedTags {
		if signedTag.SignedTag == tag {
			digest = signedTag.Digest
		}
	}
	return digest
}

// FetchSignature fetches a signature from the notary server
func FetchSignature(imageUri, basicAuth, notaryServer string) (*Signature, error) {
	img, err := image.NewImage(imageUri, basicAuth)
	if err != nil {
		signatureLog.Error(err, "failed new image")
		return nil, err
	}

	// Use notary client
	// Here, we create a new cache directory per requests.
	// (Be aware that FetchSigner is called from inside the http.Handler. It can be called simultaneously as goroutines)
	// By doing so, we can clean the cache directory after the process in easier way.
	tempDir := fmt.Sprintf("%s/notary/%s", os.TempDir(), utils.RandomString(10))
	not, err := trust.NewReadOnly(img, notaryServer, tempDir)
	if err != nil {
		signatureLog.Error(err, "failed new image read in notary")
		return nil, err
	}

	defer func() {
		if err := not.ClearDir(); err != nil {
			errMsg := fmt.Sprintf("deleting notary temp dir error by %s", err)
			signatureLog.Error(err, errMsg)
		}
	}()

	signedRepo, err := not.GetSignedMetadata(img.Tag)
	if err != nil {
		// If the image is not signed
		// TODO - registry's GetSignedMetadata error handle - not using error string!
		if strings.Contains(err.Error(), "does not have trust data for") {
			return nil, nil
		}
		signatureLog.Error(err, "failed Get Signed Metadata")
		return nil, err
	}

	// Convert trust.trustRepo to Signature
	sig := Signature{Name: signedRepo.Name}
	for _, t := range signedRepo.SignedTags {
		sig.SignedTags = append(sig.SignedTags, SignedTag{
			SignedTag: t.SignedTag,
			Digest:    t.Digest,
			Signers:   t.Signers,
		})
	}
	return &sig, nil
}
