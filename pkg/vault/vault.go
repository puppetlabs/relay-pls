package vault

import (
	"context"

	"github.com/google/wire"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/puppetlabs/relay-pls/pkg/opt"
)

var ProviderSet = wire.NewSet(
	NewClient,
)

func NewClient(ctx context.Context, cfg *opt.Config) (*vaultapi.Client, error) {
	vc, err := vaultapi.NewClient(vaultapi.DefaultConfig())
	if err != nil {
		return nil, err
	}

	if err := vc.SetAddress(cfg.VaultAddr.String()); err != nil {
		return nil, err
	}

	return vc, nil
}
