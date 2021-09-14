package trust

import (
	"fmt"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/tmax-cloud/image-validating-webhook/internal/utils"
	"github.com/tmax-cloud/image-validating-webhook/pkg/image"
	notarytest "github.com/tmax-cloud/image-validating-webhook/pkg/notary/test"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type trustTestCase struct {
	notaryURL            string
	image                *image.Image
	path                 string
	expectedSignatureNil bool
	expectedErrMsg       string
}

func TestNewReadOnly(t *testing.T) {
	// Set loggers
	if os.Getenv("CI") != "true" {
		logrus.SetLevel(logrus.DebugLevel)
		ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	}
	testSrv, err := notarytest.New(false)
	require.NoError(t, err)
	tc := map[string]trustTestCase{
		"signedTag": {
			notaryURL: testSrv.URL,
			image: &image.Image{
				Host: "test.io",
				Name: "signed-repo",
				Tag:  "signed-tag",
			},
			path:                 fmt.Sprintf("%s/notary/%s", os.TempDir(), utils.RandomString(10)),
			expectedSignatureNil: false,
		},
		"unsignedTag": {
			notaryURL: testSrv.URL,
			image: &image.Image{
				Host: "test.io",
				Name: "signed-repo",
				Tag:  "unsigned-tag",
			},
			path:                 fmt.Sprintf("%s/notary/%s", os.TempDir(), utils.RandomString(10)),
			expectedSignatureNil: true,
			expectedErrMsg:       "No valid trust data",
		},
		"unsignedRepo": {
			notaryURL: testSrv.URL,
			image: &image.Image{
				Host: "test.io",
				Name: "unsigned-repo",
				Tag:  "unsigned-tag",
			},
			path:                 fmt.Sprintf("%s/notary/%s", os.TempDir(), utils.RandomString(10)),
			expectedSignatureNil: true,
			expectedErrMsg:       "does not have trust data",
		},
	}
	_, err = testSrv.SignImage(testSrv.URL, "test.io", "signed-repo", "signed-tag", "111111111111111111111111111111")
	require.NoError(t, err)

	for name, c := range tc {
		t.Run(name, func(t *testing.T) {
			img, _ := image.NewImage(fmt.Sprintf("%s/%s:%s", c.image.Host, c.image.Name, c.image.Tag), "")
			n, err := NewReadOnly(img, c.notaryURL, c.path)
			require.NoError(t, err)
			defer func() {
				err = n.ClearDir()
				require.NoError(t, err)
			}()
			if !c.expectedSignatureNil {
				repo, err := n.GetSignedMetadata(c.image.Tag)
				require.NoError(t, err)
				require.Equal(t, repo.Name, fmt.Sprintf("%s/%s", c.image.Host, c.image.Name))
			} else {
				_, err = n.GetSignedMetadata(c.image.Tag)
				require.Contains(t, err.Error(), c.expectedErrMsg)
			}
		})
	}
}
