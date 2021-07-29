#!/bin/bash

set -e

kubectl apply -f config/crd/tmax.io_signerpolicies.yaml
kubectl apply -f deploy/role/account.yaml
kubectl apply -f deploy/role/role.yaml
kubectl apply -f deploy/role/role-binding.yaml

kubectl apply -f deploy/whitelist-configmap.yaml
kubectl apply -f deploy/deployment.yaml
kubectl apply -f deploy/service.yaml
kubectl apply -f deploy/validating-webhook.yaml

echo "Deloying image-validation-webhook completed"

exit 0
