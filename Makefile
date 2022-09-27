# Current version
VERSION ?= v5.0.6
REGISTRY ?= tmaxcloudck

# Image URL to use all building/pushing image targets
IMG ?= $(REGISTRY)/image-validation-webhook
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./..." output:crd:artifacts:config=config/crd

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object paths="./..."

# Build the docker image
docker-build:
	docker build . -t ${IMG}:$(VERSION)

docker-build-dev:
	docker build . -t ${IMG}:dev

# Push the docker image
docker-push:
	docker push ${IMG}:$(VERSION)

docker-push-dev:
	docker push ${IMG}:dev

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.1 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

# Run tests
test: test-crd test-gen test-verify test-unit test-lint

# Custom targets for CI/CD operator
.PHONY: test-gen test-crd test-verify test-lint test-unit

# Test if zz_generated.deepcopy.go file is generated
test-gen: save-sha-gen generate compare-sha-gen

# Test if crd yaml files are generated
test-crd: save-sha-crd manifests compare-sha-crd

# Verify if go.sum is valid
test-verify: save-sha-mod verify compare-sha-mod

# Test code lint
test-lint:
	golangci-lint run ./... -v

# Unit test
test-unit:
	go test -v ./... -count=1

save-sha-gen:
	$(eval GENSHA=$(shell sha512sum pkg/type/zz_generated.deepcopy.go))
	$(info GENSHA is $(GENSHA))

compare-sha-gen:
	$(eval GENSHA_AFTER=$(shell sha512sum pkg/type/zz_generated.deepcopy.go))
	$(info GENSHA_AFTER is $(GENSHA_AFTER))
	@if [ "${GENSHA_AFTER}" = "${GENSHA}" ]; then echo "zz_generated.deepcopy.go is not changed"; else echo "zz_generated.deepcopy.go file is changed"; exit 1; fi

save-sha-crd:
	$(eval CRDSHA1=$(shell sha512sum config/crd/tmax.io_registrysecuritypolicies.yaml))
	$(eval CRDSHA2=$(shell sha512sum config/crd/tmax.io_clusterregistrysecuritypolicies.yaml))

compare-sha-crd:
	$(eval CRDSHA1_AFTER=$(shell sha512sum config/crd/tmax.io_registrysecuritypolicies.yaml))
	@if [ "${CRDSHA1_AFTER}" = "${CRDSHA1}" ]; then echo "tmax.io_registrysecuritypolicies.yaml is not changed"; else echo "tmax.io_registrysecuritypolicies.yaml file is changed"; exit 1; fi
	$(eval CRDSHA2_AFTER=$(shell sha512sum config/crd/tmax.io_clusterregistrysecuritypolicies.yaml))
	@if [ "${CRDSHA2_AFTER}" = "${CRDSHA2}" ]; then echo "tmax.io_clusterregistrysecuritypolicies.yaml is not changed"; else echo "tmax.io_clusterregistrysecuritypolicies.yaml file is changed"; exit 1; fi

save-sha-mod:
	$(eval MODSHA=$(shell sha512sum go.mod))
	$(eval SUMSHA=$(shell sha512sum go.sum))

verify:
	go mod verify

compare-sha-mod:
	$(eval MODSHA_AFTER=$(shell sha512sum go.mod))
	$(eval SUMSHA_AFTER=$(shell sha512sum go.sum))
	@if [ "${MODSHA_AFTER}" = "${MODSHA}" ]; then echo "go.mod is not changed"; else echo "go.mod file is changed"; exit 1; fi
	@if [ "${SUMSHA_AFTER}" = "${SUMSHA}" ]; then echo "go.sum is not changed"; else echo "go.sum file is changed"; exit 1; fi
