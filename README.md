# Overview
The image-validating-webhook project is the implementation of validating admission webhook in k8s to validate an image is signed when a pod is creating.

## Install
1. On your local machine, clone this repository
```bash
git clone https://github.com/tmax-cloud/image-validating-webhook.git
cd image-validating-webhook
```

2. Execute install.sh
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