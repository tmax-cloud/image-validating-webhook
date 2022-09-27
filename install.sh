#!/bin/bash

set -e

kubectl apply -f deploy/certificate.yaml
kubectl apply -f config/crd/tmax.io_clusterregistrysecuritypolicies.yaml
kubectl apply -f config/crd/tmax.io_registrysecuritypolicies.yaml
kubectl apply -f deploy/role/account.yaml
kubectl apply -f deploy/role/role.yaml
kubectl apply -f deploy/role/role-binding.yaml

kubectl apply -f deploy/whitelist-configmap.yaml
kubectl apply -f deploy/deployment-dev.yaml
kubectl apply -f deploy/service.yaml
kubectl apply -f deploy/validating-webhook.yaml

echo "Deloying image-validation-webhook completed"

exit 0
