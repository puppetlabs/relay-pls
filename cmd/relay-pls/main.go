package main

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/puppetlabs/relay-pls/pkg/manager"
	"github.com/puppetlabs/relay-pls/pkg/model"
	"github.com/puppetlabs/relay-pls/pkg/opt"
	"github.com/puppetlabs/relay-pls/pkg/plspb"
	"github.com/puppetlabs/relay-pls/pkg/server"
	"github.com/puppetlabs/relay-pls/pkg/util/vaultutil"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/label"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"
)

func main() {
	ctx := context.Background()

	var serverOpts []server.BigQueryServerOption

	cfg, err := opt.NewConfig()
	if err != nil {
		log.Fatalf("failed to configure options: %v", err)
	}

	if cfg.Debug {
		if err = cfg.WithTelemetry(); err != nil {
			log.Fatalf("failed to configure telemetry: %v", err)
		}
	}

	if cfg.MetricsEnabled {
		meter, err := cfg.WithMetrics()
		if err != nil {
			log.Fatalf("failed to configure metrics: %v", err)
		}

		if meter != nil {
			serverOpts = append(serverOpts,
				server.WithMetrics(meter),
			)

			counter := metric.Must(*meter).NewInt64Counter(model.METRIC_LOG_SERVICE_STARTUP)
			counter.Add(ctx, 1, label.String(model.METRIC_LABEL_MODULE, "main"))
		}
	}

	vaultClient, err := cfg.VaultClient()
	if err != nil {
		log.Fatalf("failed to initialize vault client: %v", err)
	}

	keyManager := manager.NewKeyManager()

	if vaultClient != nil {
		vaultEngineMount, err := vaultutil.CheckNormalizeEngineMount(vaultClient, cfg.VaultEngineMount)
		if err != nil {
			log.Fatalf("invalid vault engine mount %q: %+v", vaultEngineMount, err)
		}

		logMetadataStore := manager.NewVaultLogMetadataStore(vaultClient, vaultEngineMount, keyManager)

		serverOpts = append(serverOpts,
			server.WithLogMetadataManager(manager.NewLogMetadataManager(logMetadataStore)),
		)
	}

	bigqueryClient, err := cfg.BigQueryClient()
	if err != nil {
		log.Fatalf("failed to initialize bigquery client: %v", err)
	}

	table, err := cfg.BigQueryTable()
	if err != nil {
		log.Fatalf("failed to initialize bigquery table: %v", err)
	}

	serverOpts = append(serverOpts,
		server.WithBigQueryClient(bigqueryClient),
		server.WithKeyManager(keyManager),
	)

	bqs := server.NewBigQueryServer(table, serverOpts...)

	s := grpc.NewServer(
		grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor()),
		grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor()),
	)

	plspb.RegisterLogService(s, bqs.Svc())

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.ListenPort))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
