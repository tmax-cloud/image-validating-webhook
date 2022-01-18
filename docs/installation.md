# Installation Guide

This documentation guides how to install image validation webhook.

* [Prerequisites](#prerequisistes)
* [Installing Image Validation Webhook](#installing-image-validation-webhook)
* [Uninstall](#uninstall)

## Prerequisites
- [Install Cert Manager](https://github.com/tmax-cloud/install-cert-manager)

## Installing Image Validation Webhook

1. On your local machine, clone this repository

```bash
git clone https://github.com/tmax-cloud/image-validating-webhook.git
cd image-validating-webhook
```

2. Set TLS Certificates
      - Edit & apply deploy/certificate.yaml
   ```yaml
   spec:
      secretName: image-validation-webhook-cert
      ...
      issuerRef:
         kind: ClusterIssuer
         group: cert-manager.io
         name: <Name of Cert Issuer>
   ```
3. Execute install.sh

   ```bash
   bash install.sh
   ```

   If your docker needs sudo, execute install.sh with sudo

   ```
   sudo bash install.sh
   ```

## Uninstall

1. Execute uninstall.sh
   ```bash
   bash uninstall.sh
   ```