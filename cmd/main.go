package main

import (
	"log"

	"github.com/eddy-kor-92/image-webhook/pkg/server"
	"github.com/kelseyhightower/envconfig"
)

// Config is ...
type Config struct {
	ListenOn string `default: "0.0.0.0:8443"`
	Cert     string `default: "/etc/webhook/certs/cert.pem"`
	Key      string `default: "/etc/webhook/certs/key.pem"`
}

func main() {
	config := Config{}
	envconfig.Process("", &config)

	log.Println("Starting server ...")

	admissionController := server.ImageValidationAdmission{}
	webhookServer := server.GetAdmissionValidationServer(&admissionController, config.Cert, config.Key, config.ListenOn)
	webhookServer.ListenAndServeTLS("", "")
}
