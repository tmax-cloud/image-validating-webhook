package server

import (
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"

	"k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	scheme = runtime.NewScheme()
	codecs = serializer.NewCodecFactory(scheme)
)

// AdmissionController is ...
type AdmissionController interface {
	HandleAdmission(review *v1beta1.AdmissionReview) error
}

// AdmissionControllerServer is ...
type AdmissionControllerServer struct {
	AdmissionController AdmissionController
	Decoder             runtime.Decoder
}

func (admissionControllerServer *AdmissionControllerServer) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	body, err := ioutil.ReadAll(request.Body)

	if err != nil {
		log.Panicf("Couldn't read request by %s\n", err)
	}

	review := &v1beta1.AdmissionReview{}
	_, _, err = admissionControllerServer.Decoder.Decode(body, nil, review)

	if err != nil {
		log.Panicf("Couldn't decode request by %s\n", err)
	}

	admissionControllerServer.AdmissionController.HandleAdmission(review)
	responseInBytes, err := json.Marshal(review)

	if _, err := writer.Write(responseInBytes); err != nil {
		log.Panicf("Couldn't write response by %s\n", err)
	}
}

func getAdmissionServerWithoutSSL(admissionController AdmissionController, listenOn string) *http.Server {
	server := &http.Server{
		Handler: &AdmissionControllerServer{
			AdmissionController: admissionController,
			Decoder:             codecs.UniversalDeserializer(),
		},
		Addr: listenOn,
	}

	return server
}

// GetAdmissionValidationServer is ...
func GetAdmissionValidationServer(admissionController AdmissionController, tlsCert, tlsKey, listenOn string) *http.Server {
	serverCert, err := tls.LoadX509KeyPair(tlsCert, tlsKey)
	server := getAdmissionServerWithoutSSL(admissionController, listenOn)
	server.TLSConfig = &tls.Config{
		Certificates: []tls.Certificate{serverCert},
	}

	if err != nil {
		log.Panic(err)
	}

	return server
}
