package main

import (
	"github.com/eddy-kor-92/image-webhook/pkg/server"
	"github.com/kelseyhightower/envconfig"
)

// Config is ...
type Config struct {
	ListenOn string
	Cert     string
	Key      string
}

func main() {
	config := Config{}
	envconfig.Process("", &config)

	admissionController := server.ImageValidationAdmission{}
	webhookServer := server.GetAdmissionValidationServer(&admissionController, config.Cert, config.Key, config.ListenOn)
	webhookServer.ListenAndServeTLS("", "")
}
