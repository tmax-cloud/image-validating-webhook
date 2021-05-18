#!bin/bash

set -e

kubectl delete -f deploy/validating-webhook-ca-bundle.yaml
kubectl delete -f deploy/service.yaml
kubectl delete -f deploy/deployment.yaml
kubectl delete -f deploy/whitelist-configmap.yaml

kubectl delete -f deploy/role/role-binding.yaml
kubectl delete -f deploy/role/role.yaml
kubectl delete -f deploy/role/account.yaml

kubectl delete secret image-validation-admission-certs

echo "Uninstalling image-validation-webhook completed"

exit 0
