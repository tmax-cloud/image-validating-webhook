package pods

import (
	"github.com/stretchr/testify/require"
	"testing"
)

type imageWhiteListTestCase struct {
	list  []imageRef
	image string

	expectedWhitelisted bool
}

func TestWhiteList_IsImageWhiteListed(t *testing.T) {
	tc := map[string]imageWhiteListTestCase{
		"noTagWhiteListed": {
			list:                []imageRef{{name: "notary"}},
			image:               "registry-test.registry.ipip.nip.io/notary",
			expectedWhitelisted: true,
		},
		"noTagNotWhiteListed": {
			list:                []imageRef{{name: "notary"}},
			image:               "registry-test.registry.ipip.nip.io/notary-2",
			expectedWhitelisted: false,
		},
		"yesTagNotWhiteListed": {
			list:                []imageRef{{name: "notary", tag: "test"}},
			image:               "registry-test.registry.ipip.nip.io/notary",
			expectedWhitelisted: false,
		},
		"yesTagWhiteListed": {
			list:                []imageRef{{name: "notary", tag: "test"}},
			image:               "registry-test.registry.ipip.nip.io/notary:test",
			expectedWhitelisted: true,
		},
		"registry": {
			list:                []imageRef{{name: "registry"}},
			image:               "registry-test.registry.ipip.nip.io/test-image",
			expectedWhitelisted: false,
		},
		"wildcard": {
			list:                []imageRef{{host: "registry-2.registry.ipip.nip.io", name: "*"}},
			image:               "registry-2.registry.ipip.nip.io/test-image",
			expectedWhitelisted: true,
		},
		"wildcard2": {
			list:                []imageRef{{host: "registry-2.registry.ipip.nip.io", name: "*"}},
			image:               "registry-2.registry.ipip.nip.io/test-image:test",
			expectedWhitelisted: true,
		},
		"containsSlashSuccess": {
			list:                []imageRef{{host: "registry-2.registry.ipip.nip.io", name: "tmaxcloudck/notary_mysql", tag: "0.6.2-rc2"}},
			image:               "registry-2.registry.ipip.nip.io/tmaxcloudck/notary_mysql:0.6.2-rc2",
			expectedWhitelisted: true,
		},
		"containsSlashFailure": {
			list:                []imageRef{{host: "registry-2.registry.ipip.nip.io", name: "tmaxcloudck/notary_mysql", tag: "0.6.2-rc2"}},
			image:               "registry-2.registry.ipip.nip.io/tmaxcloudck/notary_mysql:0.6.2-rc1",
			expectedWhitelisted: false,
		},
	}

	for name, c := range tc {
		t.Run(name, func(t *testing.T) {
			w := &WhiteList{byImages: c.list}
			isWhitelisted := w.IsImageWhiteListed(c.image)
			require.Equal(t, c.expectedWhitelisted, isWhitelisted)
		})
	}
}

type whitelistTestCase struct {
	marshalledImage   string
	unmarshalledImage []imageRef
	marshalledNs      string
	unmarshalledNs    []string
	fail              bool
}

func TestWhiteList_Marshal(t *testing.T) {
	tc := map[string]whitelistTestCase{
		"normal": {
			unmarshalledImage: []imageRef{
				{name: "test-img"},
				{name: "img-validating-webhook"},
			},
			marshalledImage: `test-img
img-validating-webhook`,
			unmarshalledNs: []string{"test-ns", "default"},
			marshalledNs: `test-ns
default`,
		},
	}

	for name, c := range tc {
		t.Run(name, func(t *testing.T) {
			wl := &WhiteList{
				byImages:     c.unmarshalledImage,
				byNamespaces: c.unmarshalledNs,
			}
			img, ns := wl.Marshal()
			require.Equal(t, c.marshalledImage, img, "image")
			require.Equal(t, c.marshalledNs, ns, "ns")
		})
	}
}

func TestWhiteList_Unmarshal(t *testing.T) {
	tc := map[string]whitelistTestCase{
		"normal": {
			marshalledImage: `test-img
img-validating-webhook`,
			unmarshalledImage: []imageRef{
				{name: "test-img"},
				{name: "img-validating-webhook"},
			},
			marshalledNs: `test-ns
default`,
			unmarshalledNs: []string{"test-ns", "default"},
		},
	}

	for name, c := range tc {
		t.Run(name, func(t *testing.T) {
			wl := &WhiteList{}
			require.NoError(t, wl.Unmarshal(c.marshalledImage, c.marshalledNs))
			require.Equal(t, c.unmarshalledImage, wl.byImages, "image")
			require.Equal(t, c.unmarshalledNs, wl.byNamespaces, "ns")
		})
	}
}

func TestWhiteList_UnmarshalImage(t *testing.T) {
	tc := map[string]whitelistTestCase{
		"normal": {
			marshalledImage: `test-img
img-validating-webhook`,
			unmarshalledImage: []imageRef{
				{name: "test-img"},
				{name: "img-validating-webhook"},
			},
		},
		"empty": {},
	}

	for name, c := range tc {
		t.Run(name, func(t *testing.T) {
			wl := &WhiteList{}
			require.NoError(t, wl.UnmarshalImage(c.marshalledImage))
			require.Equal(t, c.unmarshalledImage, wl.byImages, "result")
		})
	}
}

func TestWhiteList_UnmarshalNamespace(t *testing.T) {
	tc := map[string]whitelistTestCase{
		"normal": {
			marshalledNs: `test-ns
default`,
			unmarshalledNs: []string{"test-ns", "default"},
		},
		"empty": {},
	}

	for name, c := range tc {
		t.Run(name, func(t *testing.T) {
			wl := &WhiteList{}
			wl.UnmarshalNamespace(c.marshalledNs)
			require.Equal(t, c.unmarshalledNs, wl.byNamespaces, "result")
		})
	}
}

func TestWhiteList_UnmarshalLegacy(t *testing.T) {
	tc := map[string]whitelistTestCase{
		"normal": {
			marshalledImage: `["test-img", "img-validating-webhook"]`,
			unmarshalledImage: []imageRef{
				{name: "test-img"},
				{name: "img-validating-webhook"},
			},
			marshalledNs:   `["test-ns", "default"]`,
			unmarshalledNs: []string{"test-ns", "default"},
			fail:           false,
		},
		"fail": {
			marshalledImage: `{"asd": "asd"}`,
			marshalledNs:    `{"asd": "asd"}`,
			fail:            true,
		},
		"failEmpty": {
			fail: true,
		},
	}

	for name, c := range tc {
		t.Run(name, func(t *testing.T) {
			wl := &WhiteList{}
			err := wl.UnmarshalLegacy(c.marshalledImage, c.marshalledNs)
			if c.fail {
				require.Error(t, err, "error occurs")
			} else {
				require.NoError(t, err, "error occurs")
			}
			require.Equal(t, c.unmarshalledImage, wl.byImages, "image")
			require.Equal(t, c.unmarshalledNs, wl.byNamespaces, "ns")
		})
	}
}

func TestWhiteList_UnmarshalLegacyImage(t *testing.T) {
	tc := map[string]whitelistTestCase{
		"normal": {
			marshalledImage: "[\"test-img\", \"img-validating-webhook\"]",
			fail:            false,
			unmarshalledImage: []imageRef{
				{name: "test-img"},
				{name: "img-validating-webhook"},
			},
		},
		"fail": {
			fail: true,
		},
		"failObject": {
			marshalledImage: "{\"test-img\", \"img-validating-webhook\"}",
			fail:            true,
		},
	}

	for name, c := range tc {
		t.Run(name, func(t *testing.T) {
			wl := &WhiteList{}
			err := wl.UnmarshalLegacyImage(c.marshalledImage)
			if c.fail {
				require.Error(t, err, "error occurs")
			} else {
				require.NoError(t, err, "error occurs")
			}
			require.Equal(t, c.unmarshalledImage, wl.byImages, "result")
		})
	}
}

func TestWhiteList_UnmarshalLegacyNamespace(t *testing.T) {
	tc := map[string]whitelistTestCase{
		"normal": {
			marshalledNs:   "[\"test-ns\", \"default\"]",
			fail:           false,
			unmarshalledNs: []string{"test-ns", "default"},
		},
		"fail": {
			fail: true,
		},
		"failObject": {
			marshalledNs: "{\"test-ns\": \"default\"}",
			fail:         true,
		},
	}

	for name, c := range tc {
		t.Run(name, func(t *testing.T) {
			wl := &WhiteList{}
			err := wl.UnmarshalLegacyNamespace(c.marshalledNs)
			if c.fail {
				require.Error(t, err, "error occurs")
			} else {
				require.NoError(t, err, "error occurs")
			}
			require.Equal(t, c.unmarshalledNs, wl.byNamespaces, "result")
		})
	}
}

type parseImageTestCase struct {
	image string
	ref   imageRef
}

func TestParseImage(t *testing.T) {
	tc := map[string]parseImageTestCase{
		"full": {
			image: "reg-test.registry.ipip.nip.io/alpine:3@sha256:def822f9851ca422481ec6fee59a9966f12b351c62ccb9aca841526ffaa9f748", // Input
			ref: imageRef{ // Expected output
				host:   "reg-test.registry.ipip.nip.io",
				name:   "alpine",
				tag:    "3",
				digest: "sha256:def822f9851ca422481ec6fee59a9966f12b351c62ccb9aca841526ffaa9f748",
			},
		},
		"noHost": {
			image: "alpine:3",
			ref: imageRef{
				host:   "",
				name:   "alpine",
				tag:    "3",
				digest: "",
			},
		},
		"digest": {
			image: "alpine:3@sha256:def822f9851ca422481ec6fee59a9966f12b351c62ccb9aca841526ffaa9f748",
			ref: imageRef{
				host:   "",
				name:   "alpine",
				tag:    "3",
				digest: "sha256:def822f9851ca422481ec6fee59a9966f12b351c62ccb9aca841526ffaa9f748",
			},
		},
		"containsSlash": {
			image: "tmax-cloud/alpine:3",
			ref: imageRef{
				host:   "",
				name:   "tmax-cloud/alpine",
				tag:    "3",
				digest: "",
			},
		},
	}

	for name, c := range tc {
		t.Run(name, func(t *testing.T) {
			ref, err := parseImage(c.image)
			require.NoError(t, err, "error occurs")
			require.Equal(t, c.ref, *ref)
		})
	}
}

func TestImageRef_String(t *testing.T) {
	tc := map[string]parseImageTestCase{
		"full": {
			ref:   imageRef{host: "docker.io", name: "alpine", tag: "3", digest: "sha256:def822f9851ca422481ec6fee59a9966f12b351c62ccb9aca841526ffaa9f748"},
			image: "docker.io/alpine:3@sha256:def822f9851ca422481ec6fee59a9966f12b351c62ccb9aca841526ffaa9f748",
		},
		"noHost": {
			ref:   imageRef{name: "alpine", tag: "3", digest: "sha256:def822f9851ca422481ec6fee59a9966f12b351c62ccb9aca841526ffaa9f748"},
			image: "alpine:3@sha256:def822f9851ca422481ec6fee59a9966f12b351c62ccb9aca841526ffaa9f748",
		},
		"noTag": {
			ref:   imageRef{name: "alpine", digest: "sha256:def822f9851ca422481ec6fee59a9966f12b351c62ccb9aca841526ffaa9f748"},
			image: "alpine@sha256:def822f9851ca422481ec6fee59a9966f12b351c62ccb9aca841526ffaa9f748",
		},
		"noDigest": {
			ref:   imageRef{name: "alpine", tag: "3"},
			image: "alpine:3",
		},
		"onlyName": {
			ref:   imageRef{name: "alpine"},
			image: "alpine",
		},
	}

	for name, c := range tc {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, c.image, c.ref.String())
		})
	}
}
