package test

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/theupdateframework/notary/client"
	"github.com/theupdateframework/notary/cryptoservice"
	"github.com/theupdateframework/notary/passphrase"
	"github.com/theupdateframework/notary/trustmanager"
	"github.com/theupdateframework/notary/trustpinning"
	"github.com/theupdateframework/notary/tuf"
	notarydata "github.com/theupdateframework/notary/tuf/data"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"
)

const (
	testNotaryKeyGun     = "gun"
	testNotaryKeyKeyType = "keyType"
)

// Server is a notary mock server for the test purpose
type Server struct {
	// needAuth is true if you want a bearer token authentication
	needAuth bool

	// files is a map of file contents - key: gun/role name
	files map[string]map[notarydata.RoleName][]byte

	// crypto is a server-side key store
	crypto *cryptoservice.CryptoService

	// timestampKey is a time stamp key of the server
	timestampKey notarydata.TUFKey

	*httptest.Server
}

// New instantiates a new notary mock server
func New(needAuth bool) (*Server, error) {
	crypto := cryptoservice.NewCryptoService(trustmanager.NewKeyMemoryStore(passphrase.ConstantRetriever("pass")))

	// Init timestamp key
	pub, err := crypto.Create(notarydata.CanonicalTimestampRole, "", notarydata.ECDSAKey)
	if err != nil {
		return nil, err
	}

	srv := &Server{
		needAuth:     needAuth,
		files:        map[string]map[notarydata.RoleName][]byte{},
		crypto:       crypto,
		timestampKey: notarydata.TUFKey{Type: pub.Algorithm(), Value: notarydata.KeyPair{Public: pub.Public()}},
	}
	srv.Server = httptest.NewTLSServer(srv.notaryHandler())

	return srv, nil
}

func (s *Server) authHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		logrus.Debug(req.Method + ": " + req.URL.String())
		token := req.Header.Get("Authorization")

		if strings.HasPrefix(req.URL.String(), "/token") || token != "" {
			h.ServeHTTP(w, req)
		} else {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf("Bearer realm=\"https://%s/token\",service=\"notary.docker.io\"", req.Host))
			w.WriteHeader(http.StatusUnauthorized)
		}
	})
}

func (s *Server) tokenHandler(w http.ResponseWriter, req *http.Request) {
	basicAuth := req.Header.Get("Authorization")
	if basicAuth == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	b, _ := json.Marshal(struct {
		Token string `json:"token"`
	}{
		Token: "dummy",
	})
	_, _ = w.Write(b)
}

func (s *Server) notaryHandler() http.Handler {
	m := mux.NewRouter()

	if s.needAuth {
		// Auth handler
		m.Use(s.authHandler)

		// Token
		m.Methods(http.MethodGet).Subrouter().HandleFunc("/token", s.tokenHandler)
	}

	// Ping
	m.Methods(http.MethodGet).Subrouter().HandleFunc("/v2", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Timestamp key
	m.Methods(http.MethodGet).Path(fmt.Sprintf("/v2/{%s:[^*]+}/_trust/tuf/timestamp.key", testNotaryKeyGun)).HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		b, _ := json.Marshal(s.timestampKey)
		_, _ = w.Write(b)
	})

	// Get keys json
	m.Methods(http.MethodGet).Path(fmt.Sprintf("/v2/{%s:[^*]+}/_trust/tuf/{%s}.json", testNotaryKeyGun, testNotaryKeyKeyType)).HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		keys, ok := s.files[vars[testNotaryKeyGun]]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		keyType := vars[testNotaryKeyKeyType]
		if strings.HasPrefix(keyType, string(notarydata.CanonicalSnapshotRole)) {
			keyType = string(notarydata.CanonicalSnapshotRole)
		} else if strings.HasPrefix(keyType, string(notarydata.CanonicalTargetsRole)) {
			keyType = string(notarydata.CanonicalTargetsRole)
		}

		key, ok := keys[notarydata.RoleName(keyType)]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write(key)
	})

	// Post keys json
	m.Methods(http.MethodPost).Path(fmt.Sprintf("/v2/{%s:[^*]+}/_trust/tuf/", testNotaryKeyGun)).HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		_ = req.ParseMultipartForm(32 << 20)

		gun := vars[testNotaryKeyGun]
		s.files[gun] = map[notarydata.RoleName][]byte{}
		tufRepo := tuf.NewRepo(s.crypto)

		files := req.MultipartForm.File["files"]
		for _, f := range files {
			file, err := f.Open()
			if err != nil {
				log.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			fileName := notarydata.RoleName(f.Filename)
			s.files[gun][fileName], err = ioutil.ReadAll(file)
			if err != nil {
				log.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			switch fileName {
			case notarydata.CanonicalRootRole:
				if err := json.Unmarshal(s.files[gun][fileName], &tufRepo.Root); err != nil {
					log.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			case notarydata.CanonicalSnapshotRole:
				if err := json.Unmarshal(s.files[gun][fileName], &tufRepo.Snapshot); err != nil {
					log.Println(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}
		}

		// Create timestamp
		if err := tufRepo.InitTimestamp(); err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		ts, err := tufRepo.SignTimestamp(time.Now().Add(10 * time.Hour))
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		tsBytes, _ := json.Marshal(ts)
		s.files[gun][notarydata.CanonicalTimestampRole] = tsBytes

	})
	return m
}

type testRoundTrip struct{}

func (rt *testRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Set("Authorization", "Bearer dummy")
	t := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	return t.RoundTrip(r)
}

// SignImage signs an image and publish it to the notary mock server
func (s *Server) SignImage(srvURL, imgHost, imgRepo, imgTag, digest string) (string, error) {
	// Init notary client and sign images
	tempDir := fmt.Sprintf("%s/notary/test-sign-image", os.TempDir())
	rt := &testRoundTrip{}
	repo, err := client.NewFileCachedRepository(tempDir, notarydata.GUN(fmt.Sprintf("%s/%s", imgHost, imgRepo)), srvURL, rt, passphrase.ConstantRetriever("test"), trustpinning.TrustPinConfig{})
	if err != nil {
		return "", err
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()
	rootPub, err := repo.GetCryptoService().Create(notarydata.CanonicalRootRole, "", notarydata.ECDSAKey)
	if err != nil {
		return "", err
	}

	if _, err := repo.ListTargets(); err != nil {
		switch err.(type) {
		case client.ErrRepoNotInitialized, client.ErrRepositoryNotExist:
			if err := repo.Initialize([]string{rootPub.ID()}); err != nil {
				return "", err
			}
		default:
			return "", err
		}
	}

	target := &client.Target{
		Name:   imgTag,
		Hashes: notarydata.Hashes{"sha256": []byte(digest)},
		Length: 32,
	}
	if err := repo.AddTarget(target, notarydata.CanonicalTargetsRole); err != nil {
		return "", err
	}

	if err := repo.Publish(); err != nil {
		return "", err
	}

	var targetKey string
	for _, k := range repo.GetCryptoService().ListKeys(notarydata.CanonicalTargetsRole) {
		targetKey = k
		break
	}

	return targetKey, nil
}
