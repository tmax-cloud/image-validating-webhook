package image

import (
	"log"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type signatureTestCase struct {
	uri            string
	basicAuth      string
	expectedHost   string
	expectedName   string
	expectedTag    string
	expectedDigest string
	expectedErr    string
}

const (
	testLibrary    = "testlibrary/"
	testImage      = "testimage"
	testRepository = "test.io"
	testTag        = "v0.0.1"
	testDigest     = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	wrongDigest    = "sha256:11111111111111111111111111111111111111111111111111111111111"
)

func TestNewImage(t *testing.T) {
	// Set loggers
	if os.Getenv("CI") != "true" {
		logrus.SetLevel(logrus.DebugLevel)
		ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	}

	tc := map[string]signatureTestCase{
		"onlyName": {
			uri:            testImage,
			basicAuth:      "",
			expectedHost:   "docker.io",
			expectedName:   "library/" + testImage,
			expectedTag:    "latest",
			expectedDigest: "",
			expectedErr:    "",
		},
		"withLibraryName": {
			uri:            testLibrary + testImage,
			basicAuth:      "",
			expectedHost:   "docker.io",
			expectedName:   testLibrary + testImage,
			expectedTag:    "latest",
			expectedDigest: "",
			expectedErr:    "",
		},
		"fullURI": {
			uri:            testRepository + "/" + testLibrary + testImage,
			basicAuth:      "",
			expectedHost:   testRepository,
			expectedName:   testLibrary + testImage,
			expectedTag:    "latest",
			expectedDigest: "",
			expectedErr:    "",
		},
		"withTag": {
			uri:            testRepository + "/" + testLibrary + testImage + ":" + testTag,
			basicAuth:      "",
			expectedHost:   testRepository,
			expectedName:   testLibrary + testImage,
			expectedTag:    testTag,
			expectedDigest: "",
			expectedErr:    "",
		},
		"withDigest": {
			uri:            testRepository + "/" + testLibrary + testImage + "@" + testDigest,
			basicAuth:      "",
			expectedHost:   testRepository,
			expectedName:   testLibrary + testImage,
			expectedTag:    "",
			expectedDigest: testDigest,
			expectedErr:    "",
		},
		"withWrongDigest": {
			uri:            testRepository + "/" + testLibrary + testImage + "@" + wrongDigest,
			basicAuth:      "",
			expectedHost:   "",
			expectedName:   "",
			expectedTag:    "",
			expectedDigest: "",
			expectedErr:    "invalid checksum digest length",
		},
	}
	for name, c := range tc {
		t.Run(name, func(t *testing.T) {
			r, err := NewImage(c.uri, c.basicAuth)
			if c.expectedErr == "" {
				require.Equal(t, c.expectedHost, r.Host)
				require.Equal(t, c.expectedName, r.Name)
				require.Equal(t, c.expectedTag, r.Tag)
				require.Equal(t, c.expectedDigest, r.Digest)
			} else {
				log.Println(err.Error())
				require.Equal(t, c.expectedErr, err.Error())
			}
		})
	}
}
