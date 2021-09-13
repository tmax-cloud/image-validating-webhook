package auth

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type roundTripTestCase struct {
	token         *Token
	url           *url.URL
	expectedError bool
}

func TestRoundTrip(t *testing.T) {
	// Set loggers
	if os.Getenv("CI") != "true" {
		logrus.SetLevel(logrus.DebugLevel)
		ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	}
	rt := &RegistryTransport{
		Base: &http.Transport{ // Base is DefaultTransport, added TLSClientConfig
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
		},
	}
	tc := map[string]roundTripTestCase{
		"TokenExist": {
			token: &Token{
				Type:  TokenTypeBearer,
				Value: "adf02j2sfd0",
			},
			url: &url.URL{
				Scheme: "https",
				Host:   "google.com",
			},
		},
		"TokenNoExist": {
			url: &url.URL{
				Scheme: "https",
				Host:   "google.com",
			},
		},
		"wrongURL": {
			url: &url.URL{
				Scheme: "http",
				Host:   "fake.abc",
			},
			expectedError: true,
		},
	}
	for name, c := range tc {
		t.Run(name, func(t *testing.T) {
			req := new(http.Request)
			rt.Token = c.token
			req.URL = c.url
			resp, err := rt.RoundTrip(req)
			if c.expectedError {
				require.Contains(t, err.Error(), "no such host")
			} else {
				require.NoError(t, err)
				if c.token != nil {
					require.Equal(t, resp.Request.Header["Authorization"][0], fmt.Sprintf("%s %s", c.token.Type, c.token.Value))
				} else {
					require.Equal(t, len(resp.Request.Header["Authorization"]), 0)
				}
			}
		})
	}
}
