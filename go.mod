module github.com/tmax-cloud/image-validating-webhook

go 1.14

require (
	github.com/bmizerany/assert v0.0.0-20160611221934-b7ed37b82869
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/tmax-cloud/registry-operator v0.3.3
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
