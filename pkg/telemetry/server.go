package telemetry

import (
	"context"
	"net/http"

	"github.com/puppetlabs/relay-pls/pkg/opt"
	"go.opentelemetry.io/otel/exporters/prometheus"
)

type TelemetryServer struct {
	addr     string
	exporter *prometheus.Exporter
}

func (ts *TelemetryServer) Run(ctx context.Context) error {
	hs := &http.Server{
		Handler: ts,
		Addr:    ts.addr,
	}

	go func() {
		_ = hs.ListenAndServe()
	}()

	return nil
}

func (ts *TelemetryServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ts.exporter.ServeHTTP(w, r)
}

func NewTelemetryServer(exporter *prometheus.Exporter, cfg *opt.Config) *TelemetryServer {
	ts := &TelemetryServer{
		addr:     cfg.MetricsAddr,
		exporter: exporter,
	}

	return ts
}
