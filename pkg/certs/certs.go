package certs

import (
	"context"

	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// Config is cluster's configuration
var Config *viper.Viper

const (
	// K8sPrefix is hypercloud prefix
	K8sPrefix = "hpcd-"
	// OperatorNamespace is default operator namespace
	OperatorNamespace = "registry-system"
	// TLSPrefix is TLS secret prefix
	TLSPrefix = "tls-"
	// K8sRegistryPrefix is registry's image pull secret resource prefix
	K8sRegistryPrefix = "registry-"
	// K8sNotaryPrefix is notary resource prefix
	K8sNotaryPrefix = "notary-"
	// K8sKeycloakPrefix is keycloak resource prefix
	K8sKeycloakPrefix = "keycloak-"
	// CustomObjectGroup is custom resource group
	CustomObjectGroup = "tmax.io"

	// RegistryRootCASecretName is OpenSSL Cert File Name
	RegistryRootCASecretName = "registry-ca"
	// KeycloakCASecretName is keycloak cert secret name
	KeycloakCASecretName = "keycloak-cert"
)

const (
	// RootCACert is secret's CA cert file name
	RootCACert = "ca.crt"
	// RootCAPriv is secret's CA key file name
	RootCAPriv = "ca.key"
)

// CAData returns ca data in secret
func CAData(secret *corev1.Secret) ([]byte, []byte) {
	if secret == nil {
		return nil, nil
	}
	return secret.Data[RootCACert], secret.Data[RootCAPriv]
}

// GetSystemKeycloakCert returns client's keycloak secret
func GetSystemKeycloakCert(c client.Client) (*corev1.Secret, error) {
	if c == nil {
		cli, err := client.New(config.GetConfigOrDie(), client.Options{})
		if err != nil {
			return nil, err
		}

		c = cli
	}

	opNamespace := Config.GetString("operator.namespace")
	if opNamespace == "" {
		opNamespace = OperatorNamespace
	}

	sysKeycloakCA := &corev1.Secret{}
	if err := c.Get(context.TODO(), types.NamespacedName{Name: KeycloakCASecretName, Namespace: opNamespace}, sysKeycloakCA); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return sysKeycloakCA, nil
}
