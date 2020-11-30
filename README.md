# Overview
The image-validating-webhook project is the implementation of validating admission webhook in k8s to validate an image is signed when a pod is creating.

## install
1. On your local machine, clone this repository
```bash
git clone https://github.com/tmax-cloud/image-validating-webhook.git
cd image-validating-webhook
```

2. Create CA certificate and make secret on your k8s cluster. It'll done by webhook-create-signed-cert.sh
```bash
bash deploy/webhook-create-signed-cert.sh
```

3. Patch validating webhook configuration with CA bundle created by previous step.
```bash
cat deploy/validating-webhook.yaml | bash deploy/webhook-patch-ca-bundle.sh > deploy/validating-webhook-ca-bundle.yaml
```

4. Build & push Docker image 
```bash
docker build --tag <image-name> .
docker push <image-name>
```

5. deploy k8s resources
```bash
kubeclt apply -f deploy/docker-daemon.yaml
kubectl apply -f deploy/deployment.yaml
kubectl apply -f deploy/service.yaml
kubectl apply -f deploy/validating-webhook-ca-bundle.yaml
```