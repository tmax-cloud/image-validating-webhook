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

## Usage

1. for administrator of cluster :
   - If you want to except some images or namespaces from validation, add it to white list config map named `image-validation-webhook-whitelist` in `registry-system` namespace.
   - In the configmap, there're two json data: `whitelist-images`, `whitelist-namespaces`. Add an image's name to `whitelist-images` or a namespace's name to `whitelist-namespaces`. (Refer to the [example](./deploy/whitelist-configmap.yaml))  
     `CAUTION`: Multiple whitelist entries must be separated by a newline(\n)
   - For `whitelist-images`, wildcard for image name is supported.  
     e.g., if `whitelist-image` contains `registry-example.com/*`, then `registry-example.com/image-1` `registry-example.com/image-2` are treated as whitelisted.
   - For `whitelist-images`, host, tag, digest can be omitted. They will be treated as a wildcard.  
     e.g., `registry` in `whitelist-images` will treat `registry-1.com/registry:tag1` and `registry-2.com/registry:tag2` as whitelisted.
2. for user :

   - Default policy of image-validation-webhook is permitting pod creation with images from any registries.
   - You can restrict which registries to pull the images from: Use CRD named RegistySecurityPolicy & ClusterRegistrySecurityPolicy: Sample is
     ```yaml
     apiVersion: tmax.io/v1
     kind: RegistrySecurityPolicy
     metadata:
       name: sample-policy
       namespace: some-namespace
     spec:
       registries:
         - registry: core.harbor.domain.io
           notary: https://notary.harbor.domain.io
           signCheck: true
     ---
     apiVersion: tmax.io/v1
     kind: ClusterRegistrySecurityPolicy
     metadata:
       name: sample-cluster-policy
     spec:
       registries:
         - registry: docker.io
           signCheck: true
     ```
   - RegistrySecurityPolicy is a namespaced scope resource and you can add the trusted registries to `registries` list
   - ClusterRegistrySecurityPolicy is a cluster scope resource and works exactly same as RegistrySecurityPolicy in all namespaces
   - registries array consists of

     - Registry: Registry's url
     - Notary: Registry's corresponding notary server url
     - Signcheck: If it is false, all images from this registry are allowed without checking their signature

   - Example flows of image validity check
     1. Image is whitelisted : VALID
     2. No Policy : VALID
     3. Policies exists & image registry source is not contained in the policies : INVALID
     4. Policies exists & image registry source is allow by the policies & signcheck is false : VALID
     5. Policies exists & image registry source is allow by the policies & signcheck is true -> check sign
        - Image is signed : VALID
        - Image is not signed : INVALID

## Uninstall

1. Execute uninstall.sh
   ```bash
   bash uninstall.sh
   ```
