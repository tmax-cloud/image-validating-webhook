package server

import (
	"context"
	"io/ioutil"
	"os"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	certResources "knative.dev/pkg/webhook/certificates/resources"
)

const (
	serviceName = "image-validation-admission-svc"
	nameSpace = "registry-system"
	mutationConfName = "image-validation-admission"
	certDir = "/tmp/certs/cert.pem"
	keyDir = "/tmp/certs/key.pem"
	baseDir = "/tmp/certs/"
)

// Create self-signed certification
func createCert(ctx context.Context, client kubernetes.Interface) error {
	svc := serviceName
	ns := nameSpace

	serverKey, serverCrt, caCrt, err := certResources.CreateCerts(ctx, svc, ns, time.Now().AddDate(1,0,0))
	if err != nil {
		return err
	}

	if err = os.MkdirAll(baseDir, os.ModePerm); err != nil {
		return err
	}

	if err = ioutil.WriteFile(certDir, serverCrt, 0644); err != nil {
		return err
	}

	if err = ioutil.WriteFile(keyDir, serverKey, 0644); err != nil {
		return err
	}

	if err = createMutationConfig(ctx, caCrt, client); err != nil {
		return err
	}
	
	return nil
}

func createMutationConfig(ctx context.Context, caCrt []byte, client kubernetes.Interface) error {
	mutateconfig, err := client.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(ctx, mutationConfName, v1.GetOptions{})
	if err != nil {
		return err
	}

	mutateconfig.Webhooks[0].ClientConfig.CABundle = caCrt
	
	_ , err = client.AdmissionregistrationV1().MutatingWebhookConfigurations().Update(ctx, mutateconfig, v1.UpdateOptions{})
	if err != nil {
		return err
	}
	
	return nil
}