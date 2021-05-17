module github.com/tmax-cloud/image-validating-webhook

go 1.14

require (
	github.com/bmizerany/assert v0.0.0-20160611221934-b7ed37b82869
	github.com/gorilla/mux v1.7.4
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/sirupsen/logrus v1.6.0
	github.com/theupdateframework/notary v0.6.2-0.20200804143915-84287fd8df4f
	github.com/tmax-cloud/registry-operator v0.3.4-0.20210513064405-950fb7ad5930
	k8s.io/api v0.19.4
	k8s.io/apimachinery v0.19.4
	k8s.io/client-go v0.19.4
	k8s.io/utils v0.0.0-20201110183641-67b214c5f920 // indirect
	sigs.k8s.io/controller-runtime v0.6.2
)

replace (
	github.com/go-logr/logr => github.com/go-logr/logr v0.1.0
	k8s.io/api => k8s.io/api v0.18.8
	k8s.io/apimachinery => k8s.io/apimachinery v0.18.8
	k8s.io/client-go => k8s.io/client-go v0.18.8
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.18.8
)
