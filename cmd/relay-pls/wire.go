//go:build wireinject
// +build wireinject

package main

import (
	"context"

	"github.com/google/wire"
	"github.com/puppetlabs/relay-pls/pkg/manager"
	"github.com/puppetlabs/relay-pls/pkg/opt"
	"github.com/puppetlabs/relay-pls/pkg/plspb"
	"github.com/puppetlabs/relay-pls/pkg/server"
	"github.com/puppetlabs/relay-pls/pkg/telemetry"
	"github.com/puppetlabs/relay-pls/pkg/vault"
)

func NewBigQueryServer(ctx context.Context, cfg *opt.Config) (plspb.LogServer, func(), error) {
	panic(wire.Build(
		telemetry.ProviderSet,
		vault.ProviderSet,
		manager.KeyManagerProviderSet,
		manager.VaultProviderSet,
		server.BigQueryServerSet,
	))
}

func NewInMemoryServer(ctx context.Context, cfg *opt.Config) (plspb.LogServer, func(), error) {
	panic(wire.Build(
		vault.ProviderSet,
		manager.KeyManagerProviderSet,
		manager.VaultProviderSet,
		server.InMemoryServerSet,
	))
}

func NewTelemetryServer(ctx context.Context, cfg *opt.Config) (*telemetry.TelemetryServer, func(), error) {
	panic(wire.Build(
		telemetry.ProviderSet,
		telemetry.ServerSet,
	))
}
