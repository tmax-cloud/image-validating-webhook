module github.com/tmax-cloud/image-validating-webhook

go 1.14

require (
	github.com/docker/distribution v2.7.1+incompatible
	github.com/fvbommel/sortorder v1.0.2
	github.com/gorilla/mux v1.7.4
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/jinzhu/gorm v1.9.16 // indirect
	github.com/lib/pq v1.8.0 // indirect
	github.com/mattn/go-sqlite3 v1.14.5 // indirect
	github.com/miekg/pkcs11 v1.0.3 // indirect
	github.com/opencontainers/go-digest v1.0.0
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/viper v1.3.2
	github.com/stretchr/testify v1.6.1
	github.com/theupdateframework/notary v0.6.2-0.20200804143915-84287fd8df4f
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776 // indirect
	k8s.io/api v0.19.4
	k8s.io/apimachinery v0.19.4
	k8s.io/client-go v0.19.4
	k8s.io/utils v0.0.0-20201110183641-67b214c5f920 // indirect
	knative.dev/pkg v0.0.0-20201127013335-0d896b5c87b8
	sigs.k8s.io/controller-runtime v0.6.2
)

replace (
	github.com/go-logr/logr => github.com/go-logr/logr v0.1.0
	k8s.io/api => k8s.io/api v0.18.8
	k8s.io/apimachinery => k8s.io/apimachinery v0.18.8
	k8s.io/client-go => k8s.io/client-go v0.18.8
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.18.8
)
