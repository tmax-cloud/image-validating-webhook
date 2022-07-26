module github.com/tmax-cloud/image-validating-webhook

go 1.14

require (
	github.com/docker/distribution v2.7.1+incompatible
	github.com/fvbommel/sortorder v1.0.2
	github.com/google/go-cmp v0.5.2 // indirect
	github.com/google/uuid v1.1.2 // indirect
	github.com/googleapis/gnostic v0.4.0 // indirect
	github.com/gorilla/mux v1.7.4
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/jinzhu/gorm v1.9.16 // indirect
	github.com/lib/pq v1.8.0 // indirect
	github.com/mattn/go-sqlite3 v1.14.5 // indirect
	github.com/miekg/pkcs11 v1.0.3 // indirect
	github.com/opencontainers/go-digest v1.0.0
	github.com/prometheus/client_golang v1.8.0 // indirect
	github.com/sirupsen/logrus v1.6.0
	github.com/stretchr/testify v1.7.0
	github.com/sykesm/zap-logfmt v0.0.4
	github.com/theupdateframework/notary v0.6.2-0.20200804143915-84287fd8df4f
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	go.uber.org/zap v1.21.0
	golang.org/x/crypto v0.0.0-20201002170205-7f63de1d35b0 // indirect
	golang.org/x/oauth2 v0.0.0-20200902213428-5d25da1a8d43 // indirect
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e // indirect
	gomodules.xyz/jsonpatch/v2 v2.1.0 // indirect
	k8s.io/api v0.19.4
	k8s.io/apiextensions-apiserver v0.18.12 // indirect
	k8s.io/apimachinery v0.19.4
	k8s.io/client-go v0.19.4
	k8s.io/kube-openapi v0.0.0-20200410145947-bcb3869e6f29 // indirect
	k8s.io/utils v0.0.0-20201110183641-67b214c5f920 // indirect
	sigs.k8s.io/controller-runtime v0.6.2
	sigs.k8s.io/structured-merge-diff/v3 v3.0.1-0.20200706213357-43c19bbb7fba // indirect
)

replace (
	github.com/go-logr/logr => github.com/go-logr/logr v0.1.0
	k8s.io/api => k8s.io/api v0.18.8
	k8s.io/apimachinery => k8s.io/apimachinery v0.18.8
	k8s.io/client-go => k8s.io/client-go v0.18.8
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.18.8
)
