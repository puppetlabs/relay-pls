package main

import (
	"fmt"
	"log"
	"net"

	"github.com/puppetlabs/relay-pls/pkg/manager"
	"github.com/puppetlabs/relay-pls/pkg/opt"
	"github.com/puppetlabs/relay-pls/pkg/plspb"
	"github.com/puppetlabs/relay-pls/pkg/server"
	"github.com/puppetlabs/relay-pls/pkg/util/vaultutil"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

func main() {
	cfg, err := opt.NewConfig()
	if err != nil {
		log.Fatalf("failed to configure options: %v", err)
	}

	if cfg.Debug {
		if err = cfg.WithTelemetry(); err != nil {
			log.Fatalf("failed to configure telemetry: %v", err)
		}
	}

	var serverOpts []server.BigQueryServerOption

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
