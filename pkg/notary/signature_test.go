package notary

import (
	"fmt"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	notarytest "github.com/tmax-cloud/image-validating-webhook/pkg/notary/test"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	testImageTag     = "test"
	testRegistryHost = "test.registry"

	testImageNotSigned = "not-signed"
	testImageSigned    = "signed"
)

type signatureTestCase struct {
	imgHost string
	imgRepo string
	imgTag  string

	expectedSignatureNil bool
}

func TestFetchSignature(t *testing.T) {
	// Set loggers
	if os.Getenv("CI") != "true" {
		logrus.SetLevel(logrus.DebugLevel)
		ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	}

	testSrv, err := notarytest.New(false)
	require.NoError(t, err)

	_, err = testSrv.SignImage(testSrv.URL, testRegistryHost, testImageSigned, testImageTag, "111111111111111111111111111111")
	require.NoError(t, err)

	tc := map[string]signatureTestCase{
		"signed": {
			imgHost:              testRegistryHost,
			imgRepo:              testImageSigned,
			imgTag:               testImageTag,
			expectedSignatureNil: false,
		},
		"unsigned": {
			imgHost:              testRegistryHost,
			imgRepo:              testImageNotSigned,
			imgTag:               testImageTag,
			expectedSignatureNil: true,
		},
	}

	for name, c := range tc {
		t.Run(name, func(t *testing.T) {
			sig, err := FetchSignature(fmt.Sprintf("%s/%s:%s", c.imgHost, c.imgRepo, c.imgTag), "", testSrv.URL)
			require.NoError(t, err)

			if c.expectedSignatureNil {
				require.Nil(t, sig)
			} else {
				require.NotNil(t, sig)
				require.Equal(t, fmt.Sprintf("%s/%s", c.imgHost, c.imgRepo), sig.Name, "name")
				require.Len(t, sig.SignedTags, 1, "tags length")
				require.Equal(t, c.imgTag, sig.SignedTags[0].SignedTag, "tag")
				require.Len(t, sig.SignedTags[0].Signers, 1, "signer length")
				require.Equal(t, "Repo Admin", sig.SignedTags[0].Signers[0], "signer")
			}
		})
	}
}
