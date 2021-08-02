package main

import (
	"log"

	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/tmax-cloud/image-validating-webhook/pkg/server"

	_ "github.com/tmax-cloud/image-validating-webhook/pkg/admissions"
)

func main() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	log.Println("Starting server ...!!")

	cert := "/tmp/certs/cert.pem"
	key := "/tmp/certs/key.pem"
	listenOn := "0.0.0.0:8443"

	// Create config, clients
	cfg, err := config.GetConfig()
	if err != nil {
		panic(err)
	}

	clientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		panic(err)
	}

	webhookServer := server.New(cert, key, listenOn, cfg, clientSet, clientSet.RESTClient())
	webhookServer.Start()
}
