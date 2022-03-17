package main

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/puppetlabs/relay-pls/pkg/opt"
	"github.com/puppetlabs/relay-pls/pkg/plspb"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

func main() {
	ctx := context.Background()

	cfg, err := opt.NewConfig()
	if err != nil {
		log.Fatalf("failed to configure options: %v", err)
	}

	var srv plspb.LogServer
	var cleanup func()

	// TODO Implement cleaner (and more exact) handling for determining the type of server
	if cfg.Table != "" && cfg.Project != "" && cfg.Dataset != "" {
		srv, cleanup, err = NewBigQueryServer(ctx, cfg)
		if err != nil {
			log.Fatal("failed to initialize BigQuery server")
		}
	} else {
		srv, cleanup, err = NewInMemoryServer(ctx, cfg)
		if err != nil {
			log.Fatal("failed to initialize in memory server")
		}

	}

	gs := grpc.NewServer(
		grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor()),
		grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor()),
	)

	plspb.RegisterLogServer(gs, srv)

	defer cleanup()

	telemetryServer, telemetryCleanup, err := NewTelemetryServer(ctx, cfg)
	if err != nil {
		log.Printf("failed to initialize telemetry server: %v", err)
	}
	defer telemetryCleanup()

	if err := telemetryServer.Run(ctx); err != nil {
		log.Printf("failed to run telemetry server: %v", err)
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.ListenPort))
	if err != nil {
		log.Printf("failed to listen: %v", err)
	}

	if err := gs.Serve(lis); err != nil {
		log.Printf("failed to serve gRPC service: %v", err)
	}
}
