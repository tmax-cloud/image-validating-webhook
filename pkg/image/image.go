package image

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/opencontainers/go-digest"
	"github.com/tmax-cloud/image-validating-webhook/pkg/auth"

	"github.com/docker/distribution/reference"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Logger is log util with name image-client
var Logger = log.Log.WithName("image-client")

const (
	// DefaultServerHostName is the default registry server hostname
	DefaultServerHostName = "registry-1.docker.io"
	// DefaultServer is the default registry server
	DefaultServer = "https://" + DefaultServerHostName
	// DefaultHostname is the default built-in hostname
	DefaultHostname = "docker.io"

	// LegacyDefaultDomain is ...
	LegacyDefaultDomain = "index.docker.io"
	// LegacyV1Server is FQDN of legacy v1 server
	LegacyV1Server = "https://index.docker.io/v1"
	// LegacyV2Server is FQDN of legacy v2 server
	LegacyV2Server = "https://index.docker.io/v2"
)

// Image is a struct containing info of image
type Image struct {
	ServerURL string

	Host         string
	Name         string
	FamiliarName string
	Tag          string
	Digest       string

	// username:password string encrypted by base64
	BasicAuth string
	Token     *auth.Token

	HttpClient http.Client
}

// NewImage creates new image client
func NewImage(uri, basicAuth string) (*Image, error) {
	r := &Image{}

	// Set image
	if uri != "" {
		if err := r.setImage(uri); err != nil {
			Logger.Error(err, "failed to set image", "uri", uri)
			return nil, err
		}
	}

	// Auth
	r.BasicAuth = basicAuth
	Logger.Info("Auth", "auth", basicAuth)
	r.Token = &auth.Token{}

	// Generate HTTPS client
	var tlsConfig = &tls.Config{InsecureSkipVerify: true}

	r.HttpClient = http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}
	return r, nil
}

// setImage sets image from "[<server>/]<imageName>[:<tag>|@<digest>]" form argument
func (r *Image) setImage(image string) error {
	// Parse image
	var img reference.Named
	var err error

	r.ServerURL = DefaultServer
	img, err = reference.ParseNamed(image)
	if err == nil {
		domain := reference.Domain(img)
		if r.isDefaultServerDomain(domain) {
			domain = DefaultServer
		}
		if !r.isValidDomain(domain) {
			r.setServerURL(domain)
		}
	}

	if r.ServerURL == DefaultServer {
		img, err = r.normalizedNamed(image)
		if err != nil {
			Logger.Error(err, "failed to normalize image", "image", image)
			return err
		}

		r.FamiliarName = reference.FamiliarName(img)
		Logger.Info("Image: ", "registry", reference.Domain(img), "image", reference.Path(img))
	} else {
		img, err = reference.ParseNamed(image)
		if err != nil {
			uri := r.ServerURL
			uri = strings.TrimPrefix(uri, "http://")
			uri = strings.TrimPrefix(uri, "https://")
			uri = path.Join(uri, image)
			img, err = reference.ParseNamed(uri)
			if err != nil {
				Logger.Error(err, "failed to parse uri", "uri", uri)
				return err
			}
		}
		r.FamiliarName = reference.Path(img)
	}

	r.Host, r.Name = reference.SplitHostname(img)
	referred := false
	r.Digest = ""
	r.Tag = ""
	if canonical, isCanonical := img.(reference.Canonical); isCanonical {
		r.Digest = canonical.Digest().String()
		referred = true
	}

	img = reference.TagNameOnly(img)
	if tagged, isTagged := img.(reference.NamedTagged); isTagged {
		r.Tag = tagged.Tag()
		referred = true
	}

	if !referred {
		return fmt.Errorf("no tag and digest given")
	}

	return nil
}

// isDefaultServerDomain returns whether the image's domain is docker.io
func (r *Image) isDefaultServerDomain(domain string) bool {
	if domain != DefaultHostname &&
		domain != DefaultServer &&
		domain != DefaultServerHostName &&
		domain != LegacyDefaultDomain {
		return false
	}
	return true
}

// isValidDomain returns whether the domain is contained in server url
func (r *Image) isValidDomain(domain string) bool {
	return strings.Contains(r.ServerURL, domain)
}

// setServerURL sets registry server URL
func (r *Image) setServerURL(url string) {
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		url = "https://" + url
	}
	r.ServerURL = url
}

// normalizedNamed normalize image for default server
func (r *Image) normalizedNamed(image string) (reference.Named, error) {
	var named, norm reference.Named
	var err error

	named, err = reference.ParseNormalizedNamed(image)
	if err != nil {
		Logger.Error(err, "failed to parse image", "image", image)
		return nil, err
	}

	tag, digest := r.getTagOrDigest(named)

	norm, err = reference.ParseNormalizedNamed(reference.Path(named))
	if err != nil {
		Logger.Error(err, "failed to parse image", "image", image)
		return nil, err
	}

	image = path.Join(reference.Domain(named), reference.Path(norm))
	named, err = reference.ParseNormalizedNamed(image)
	if err != nil {
		Logger.Error(err, "failed to parse image", "image", image)
		return nil, err
	}

	if tag != "" {
		named, err = reference.WithTag(named, tag)
		if err != nil {
			Logger.Error(err, "failed to tag image", "image", image, "tag", tag)
			return nil, err
		}
	} else if digest != "" {
		named, err = reference.WithDigest(named, digest)
		if err != nil {
			Logger.Error(err, "failed to digest image", "image", image, "digest", digest)
			return nil, err
		}
	}

	return named, nil
}

// getTagOrDigest returns image's tag or digest
func (r *Image) getTagOrDigest(named reference.Named) (tag string, digest digest.Digest) {
	if tagged, isTagged := named.(reference.NamedTagged); isTagged {
		tag = tagged.Tag()
		return
	}

	if digested, isDigested := named.(reference.Digested); isDigested {
		digest = digested.Digest()
	}

	return
}

// GetImageNameWithHost add host in front of image name with slash
func (r *Image) GetImageNameWithHost() string {
	return path.Join(r.Host, r.Name)
}
