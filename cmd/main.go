package main

import (
	"log"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/tmax-cloud/image-validating-webhook/pkg/server"
)

func main() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	log.Println("Starting server ...")

	cert := "/etc/webhook/certs/cert.pem"
	key := "/etc/webhook/certs/key.pem"
	listenOn := "0.0.0.0:8443"

	admissionController := &server.ImageValidationAdmission{}
	webhookServer := server.GetAdmissionValidationServer(admissionController, cert, key, listenOn)
	if err := webhookServer.ListenAndServeTLS("", ""); err != nil {
		log.Fatal(err)
	}
}
