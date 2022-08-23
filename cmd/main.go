package main

import (
	"flag"
	"time"

	zaplogfmt "github.com/sykesm/zap-logfmt"
	uzap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/tmax-cloud/image-validating-webhook/pkg/server"

	_ "github.com/tmax-cloud/image-validating-webhook/pkg/admissions"
)

var zlog = logf.Log.WithName("main.go")

func main() {
	// when zap.Options.Development set true, the 'log level' is fixed to debug.
	opts := zap.Options{
		Development: false,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	configLog := uzap.NewProductionEncoderConfig()
	configLog.EncodeTime = func(ts time.Time, encoder zapcore.PrimitiveArrayEncoder) {
		encoder.AppendString(ts.UTC().Local().Format(time.RFC3339))
	}
	logfmtEncoder := zaplogfmt.NewEncoder(configLog)

	logger := zap.New(zap.UseFlagOptions(&opts), zap.Encoder(logfmtEncoder))
	logf.SetLogger(logger)

	zlog.Info("Starting server ...!!")

	cert := "/etc/webhook/certs/tls.crt"
	key := "/etc/webhook/certs/tls.key"
	listenOn := "0.0.0.0:8443"

	// Create config, clients
	cfg, err := config.GetConfig()
	if err != nil {
		panic(err)
	}

	clientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		panic(err)
	}

	webhookServer := server.New(cert, key, listenOn, cfg, clientSet, clientSet.RESTClient())
	webhookServer.Start()
}
