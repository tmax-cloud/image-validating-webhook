package server

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"k8s.io/client-go/kubernetes/scheme"
	"log"
	"net/http"

	"k8s.io/api/admission/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

	log.Println("Handling request")

	if err != nil {
		errMsg := fmt.Sprintf("Couldn't read request by %s", err)
		log.Println(errMsg)
		http.Error(writer, errMsg, http.StatusBadRequest)
		return
	}

	review := &v1beta1.AdmissionReview{}
	if _, _, err = admissionControllerServer.Decoder.Decode(body, nil, review); err != nil {
		errMsg := fmt.Sprintf("Couldn't decode request by %s", err)
		log.Println(errMsg)
		review.Response = &v1beta1.AdmissionResponse{
			Allowed: false,
			Result: &v1.Status{
				Message: errMsg,
			},
		}
	}

	_ = admissionControllerServer.AdmissionController.HandleAdmission(review)
	responseInBytes, err := json.Marshal(review)
	if err != nil {
		errMsg := fmt.Sprintf("Couldn't encode response by %s", err)
		log.Println(errMsg)
		http.Error(writer, errMsg, http.StatusInternalServerError)
		return
	}

	if _, err := writer.Write(responseInBytes); err != nil {
		errMsg := fmt.Sprintf("Couldn't write response by %s", err)
		log.Println(errMsg)
		http.Error(writer, errMsg, http.StatusInternalServerError)
		return
	}
}

// GetAdmissionValidationServer is ...
func GetAdmissionValidationServer(admissionController AdmissionController, tlsCert, tlsKey, listenOn string) *http.Server {
	serverCert, err := tls.LoadX509KeyPair(tlsCert, tlsKey)
	mux := http.NewServeMux()
	mux.Handle("/validate", &AdmissionControllerServer{
		AdmissionController: admissionController,
		Decoder:             scheme.Codecs.UniversalDeserializer(),
	})

	server := &http.Server{
		Handler: mux,
		Addr:    listenOn,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{serverCert},
		},
	}

	if err != nil {
		log.Printf("params: %s %s %s", tlsCert, tlsKey, listenOn)
		log.Panic(err)
	}

	return server
}
