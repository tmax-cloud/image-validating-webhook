package server

import (
	"github.com/bmizerany/assert"
	"testing"
)

type whitelistTestCase struct {
	marshalledImage   string
	unmarshalledImage []string
	marshalledNs      string
	unmarshalledNs    []string
	fail              bool
}

func TestWhiteList_Marshal(t *testing.T) {
	tc := map[string]whitelistTestCase{
		"normal": {
			unmarshalledImage: []string{"test-img", "img-validating-webhook"},
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
			unmarshalledImage: []string{"test-img", "img-validating-webhook"},
			marshalledNs: `test-ns
default`,
			unmarshalledNs: []string{"test-ns", "default"},
		},
	}

	for name, c := range tc {
		t.Run(name, func(t *testing.T) {
			wl := &WhiteList{}
			wl.Unmarshal([]byte(c.marshalledImage), []byte(c.marshalledNs))
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
			unmarshalledImage: []string{"test-img", "img-validating-webhook"},
		},
		"empty": {},
	}

	for name, c := range tc {
		t.Run(name, func(t *testing.T) {
			wl := &WhiteList{}
			wl.UnmarshalImage([]byte(c.marshalledImage))
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
			marshalledImage:   `["test-img", "img-validating-webhook"]`,
			unmarshalledImage: []string{"test-img", "img-validating-webhook"},
			marshalledNs:      `["test-ns", "default"]`,
			unmarshalledNs:    []string{"test-ns", "default"},
			fail:              false,
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
			marshalledImage:   "[\"test-img\", \"img-validating-webhook\"]",
			fail:              false,
			unmarshalledImage: []string{"test-img", "img-validating-webhook"},
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
