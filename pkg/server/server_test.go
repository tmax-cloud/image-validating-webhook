package server

import (
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

type testHandler struct{}

func (t *testHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("test-1"))
}

type testHandler2 struct{}

func (t *testHandler2) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("test-2"))
}

func TestAddHandlerInitiator(t *testing.T) {
	AddHandlerInitiator("/test-1", []string{http.MethodGet}, func(_ *HandlerConfig) (http.Handler, error) {
		return &testHandler{}, nil
	})
	require.Len(t, handlerInitiators, 1, "handlerInitiator length")
}

type serverTestCase struct {
	method string
	path   string

	expectedStatusCode int
	expectedOutput     []byte
}

func TestServer(t *testing.T) {
	AddHandlerInitiator("/test-1", []string{http.MethodGet}, func(_ *HandlerConfig) (http.Handler, error) {
		return &testHandler{}, nil
	})
	AddHandlerInitiator("/test-2", []string{http.MethodPost}, func(_ *HandlerConfig) (http.Handler, error) {
		return &testHandler2{}, nil
	})

	s := Server{mux: mux.NewRouter(), server: &http.Server{}}
	require.NoError(t, s.addHandlersToServer())

	testSrv := httptest.NewServer(s.server.Handler)
	testCli := testSrv.Client()

	tc := map[string]serverTestCase{
		"test-1": {
			method:             http.MethodGet,
			path:               "/test-1",
			expectedStatusCode: http.StatusOK,
			expectedOutput:     []byte("test-1"),
		},
		"test-1-post": {
			method:             http.MethodPost,
			path:               "/test-1",
			expectedStatusCode: http.StatusMethodNotAllowed,
			expectedOutput:     []byte{},
		},
		"test-2": {
			method:             http.MethodPost,
			path:               "/test-2",
			expectedStatusCode: http.StatusOK,
			expectedOutput:     []byte("test-2"),
		},
		"test-2-get": {
			method:             http.MethodGet,
			path:               "/test-2",
			expectedStatusCode: http.StatusMethodNotAllowed,
			expectedOutput:     []byte{},
		},
		"not-found": {
			method:             http.MethodGet,
			path:               "/test-3",
			expectedStatusCode: http.StatusNotFound,
			expectedOutput:     []byte("404 page not found\n"),
		},
	}

	for name, c := range tc {
		t.Run(name, func(t *testing.T) {
			req, err := http.NewRequest(c.method, testSrv.URL+c.path, nil)
			require.NoError(t, err)
			resp, err := testCli.Do(req)
			require.NoError(t, err)

			output, err := ioutil.ReadAll(resp.Body)
			require.NoError(t, err)

			require.Equal(t, c.expectedStatusCode, resp.StatusCode, "code")
			require.Equal(t, c.expectedOutput, output, "output")
		})
	}
}
