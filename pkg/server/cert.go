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
	serviceName      = "image-validation-admission-svc"
	nameSpace        = "registry-system"
	mutationConfName = "image-validation-admission"
	certName          = "cert.pem"
	keyName           = "key.pem"
	baseDir          = "/tmp/certs/"
)

// Create self-signed certification
func createCert(ctx context.Context, client kubernetes.Interface) error {
	svc := serviceName
	ns := nameSpace

	serverKey, serverCrt, caCrt, err := certResources.CreateCerts(ctx, svc, ns, time.Now().AddDate(1, 0, 0))
	if err != nil {
		return err
	}

	if err = os.MkdirAll(baseDir, os.ModePerm); err != nil {
		return err
	}

	if err = ioutil.WriteFile(baseDir + certName, serverCrt, 0644); err != nil {
		return err
	}

	if err = ioutil.WriteFile(baseDir + keyName, serverKey, 0644); err != nil {
		return err
	}

	if err = updateMutationConfig(ctx, caCrt, client); err != nil {
		return err
	}

	return nil
}

func updateMutationConfig(ctx context.Context, caCrt []byte, client kubernetes.Interface) error {
	mutateconfig, err := client.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(ctx, mutationConfName, v1.GetOptions{})
	if err != nil {
		return err
	}

	mutateconfig.Webhooks[0].ClientConfig.CABundle = caCrt

	_, err = client.AdmissionregistrationV1().MutatingWebhookConfigurations().Update(ctx, mutateconfig, v1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}
