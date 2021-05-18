package server

import (
	"github.com/bmizerany/assert"
	"testing"
)

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
			assert.Equal(t, c.marshalledImage, string(img), "image")
			assert.Equal(t, c.marshalledNs, string(ns), "ns")
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
			if err := wl.Unmarshal([]byte(c.marshalledImage), []byte(c.marshalledNs)); err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, c.unmarshalledImage, wl.byImages, "image")
			assert.Equal(t, c.unmarshalledNs, wl.byNamespaces, "ns")
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
			if err := wl.UnmarshalImage([]byte(c.marshalledImage)); err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, c.unmarshalledImage, wl.byImages, "result")
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
			wl.UnmarshalNamespace([]byte(c.marshalledNs))
			assert.Equal(t, c.unmarshalledNs, wl.byNamespaces, "result")
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
			err := wl.UnmarshalLegacy([]byte(c.marshalledImage), []byte(c.marshalledNs))
			assert.Equal(t, c.fail, err != nil, "error occurs")
			assert.Equal(t, c.unmarshalledImage, wl.byImages, "image")
			assert.Equal(t, c.unmarshalledNs, wl.byNamespaces, "ns")
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
			err := wl.UnmarshalLegacyImage([]byte(c.marshalledImage))
			assert.Equal(t, c.fail, err != nil, "error occurs")
			assert.Equal(t, c.unmarshalledImage, wl.byImages, "result")
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
			err := wl.UnmarshalLegacyNamespace([]byte(c.marshalledNs))
			assert.Equal(t, c.fail, err != nil, "error occurs")
			assert.Equal(t, c.unmarshalledNs, wl.byNamespaces, "result")
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
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, c.ref, *ref)
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
			assert.Equal(t, c.image, c.ref.String())
		})
	}
}
