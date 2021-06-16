package notary

import (
	"fmt"
	"github.com/tmax-cloud/image-validating-webhook/internal/utils"
	"github.com/tmax-cloud/registry-operator/pkg/image"
	"github.com/tmax-cloud/registry-operator/pkg/trust"
	"log"
	"os"
	"strings"
)

// Signature is a sign info of an image
type Signature struct {
	Name               string      `json:"Name"`
	SignedTags         []SignedTag `json:"SignedTags"`
	Signers            []Signer    `json:"Signer"`
	AdministrativeKeys []AdminKey  `json:"AdministrativeKeys"`
}

// SignedTag is a tag-signature info
type SignedTag struct {
	SignedTag string   `json:"SignedTag"`
	Digest    string   `json:"Digest"`
	Signers   []string `json:"Signers"`
}

// Signer is a signer struct
type Signer struct {
}

// AdminKey is a key for admin
type AdminKey struct {
	Name string `json:"Name"`
	Keys []Key  `json:"Keys"`
}

// Key is a key struct
type Key struct {
	ID string `json:"ID"`
}

// GetRepoAdminKey gets admin key for the repository
func (s *Signature) GetRepoAdminKey() string {
	for _, key := range s.AdministrativeKeys {
		if key.Name == "Repository" {
			return key.Keys[0].ID
		}
	}

	return ""
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
	img, err := image.NewImage(imageUri, "", basicAuth, nil)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	// Use notary client
	// Here, we create a new cache directory per requests.
	// (Be aware that FetchSigner is called from inside the http.Handler. It can be called simultaneously as goroutines)
	// By doing so, we can clean the cache directory after the process in easier way.
	tempDir := fmt.Sprintf("%s/notary/%s", os.TempDir(), utils.RandomString(10))
	not, err := trust.NewReadOnly(img, notaryServer, tempDir)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	defer func() {
		if err := not.ClearDir(); err != nil {
			log.Printf("deleting notary temp dir error by %s", err)
		}
	}()

	signedRepo, err := not.GetSignedMetadata(img.Tag)
	if err != nil {
		// If the image is not signed
		// TODO - registry's GetSignedMetadata error handle - not using error string!
		if strings.Contains(err.Error(), "does not have trust data for") {
			return nil, nil
		}
		log.Println(err)
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
	for _, a := range signedRepo.AdministrativeKeys {
		var keys []Key
		for _, k := range a.Keys {
			keys = append(keys, Key{ID: k.ID})
		}

		sig.AdministrativeKeys = append(sig.AdministrativeKeys, AdminKey{
			Name: a.Name,
			Keys: keys,
		})
	}

	return &sig, nil
}
