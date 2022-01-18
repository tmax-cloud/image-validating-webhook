package server

import (
	"net/http"

	"github.com/gorilla/mux"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// HandlerConfig is a config to be passed to the handler init functions
type HandlerConfig struct {
	RestCfg    *rest.Config
	ClientSet  kubernetes.Interface
	RestClient rest.Interface
}

// HandlerInitFunc is a function for initializing the Handler
type HandlerInitFunc func(cfg *HandlerConfig) (http.Handler, error)

type handlerContainer struct {
	path     string
	methods  []string
	initFunc HandlerInitFunc
}

// handlerInitiators is a list of HandlerInitFunc, which will be called when the Server starts
var handlerInitiators []handlerContainer

// AddHandlerInitiator appends an initiator func to the list
func AddHandlerInitiator(path string, methods []string, handlerInit HandlerInitFunc) {
	handlerInitiators = append(handlerInitiators, handlerContainer{
		path:     path,
		methods:  methods,
		initFunc: handlerInit,
	})
}

// Server is a multi-purpose http server
type Server struct {
	server *http.Server

	certFile string
	keyFile  string

	mux *mux.Router

	cfg        *rest.Config
	clientSet  kubernetes.Interface
	restClient rest.Interface
}

// New initiates a new Server instance
func New(certFile, keyFile, addr string, cfg *rest.Config, clientSet kubernetes.Interface, restClient rest.Interface) *Server {
	srv := &Server{
		server:   &http.Server{Addr: addr},
		certFile: certFile,
		keyFile:  keyFile,
		mux:      mux.NewRouter(),

		cfg:        cfg,
		clientSet:  clientSet,
		restClient: restClient,
	}

	return srv
}

// Start adds all the handlers to the server and starts the server
func (s *Server) Start() {
	if err := s.addHandlersToServer(); err != nil {
		panic(err)
	}
	if err := s.server.ListenAndServeTLS(s.certFile, s.keyFile); err != nil {
		panic(err)
	}
}

func (s *Server) addHandlersToServer() error {
	// Add handlers to the mux
	cfg := &HandlerConfig{RestCfg: s.cfg, ClientSet: s.clientSet, RestClient: s.restClient}
	for _, i := range handlerInitiators {
		h, err := i.initFunc(cfg)
		if err != nil {
			return err
		}
		s.mux.Methods(i.methods...).Path(i.path).Handler(h)
	}
	s.server.Handler = s.mux
	return nil
}
