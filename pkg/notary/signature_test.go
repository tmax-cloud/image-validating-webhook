package notary

import (
	"fmt"
	"github.com/bmizerany/assert"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	notarytest "github.com/tmax-cloud/image-validating-webhook/pkg/notary/test"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"testing"
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
	expectedTargetKey    string
}

func TestFetchSignature(t *testing.T) {
	// Set loggers
	if os.Getenv("CI") != "true" {
		logrus.SetLevel(logrus.DebugLevel)
		ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	}

	testSrv, err := notarytest.New(false)
	require.NoError(t, err)

	signedTargetKey, err := testSrv.SignImage(testSrv.URL, testRegistryHost, testImageSigned, testImageTag, "111111111111111111111111111111")
	require.NoError(t, err)

	tc := map[string]signatureTestCase{
		"signed": {
			imgHost:              testRegistryHost,
			imgRepo:              testImageSigned,
			imgTag:               testImageTag,
			expectedSignatureNil: false,
			expectedTargetKey:    signedTargetKey,
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
				assert.Equal(t, fmt.Sprintf("%s/%s", c.imgHost, c.imgRepo), sig.Name, "name")
				assert.Equal(t, 1, len(sig.SignedTags), "tags length")
				assert.Equal(t, c.imgTag, sig.SignedTags[0].SignedTag, "tag")
				assert.Equal(t, 1, len(sig.SignedTags[0].Signers), "signer length")
				assert.Equal(t, "Repo Admin", sig.SignedTags[0].Signers[0], "signer")
				assert.Equal(t, c.expectedTargetKey, sig.AdministrativeKeys[1].Keys[0].ID, "target key id")
			}
		})
	}
}
