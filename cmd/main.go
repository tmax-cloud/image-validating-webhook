package main

import (
	"log"

	"github.com/tmax-cloud/image-validating-webhook/pkg/server"
)

func main() {
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
