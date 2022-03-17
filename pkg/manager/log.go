package manager

import (
	"context"
	"path"

	"github.com/google/uuid"
	"github.com/google/wire"
	"github.com/hashicorp/vault/api"
	"github.com/puppetlabs/leg/encoding/transfer"
	"github.com/puppetlabs/leg/timeutil/pkg/retry"
	"github.com/puppetlabs/relay-pls/pkg/model"
	"github.com/puppetlabs/relay-pls/pkg/opt"
	"github.com/puppetlabs/relay-pls/pkg/util/vaultutil"
)

var VaultProviderSet = wire.NewSet(
	NewVaultLogMetadataManager,
)

type VaultLogMetadataManager struct {
	client      *api.Client
	engineMount string

	keyManager model.KeyManager
}

func (lmm *VaultLogMetadataManager) Get(ctx context.Context, id string) (*model.LogMetadata, error) {
	logMetadataPath := path.Join(lmm.engineMount, "data", "logs", id)

	dataPath := path.Join(logMetadataPath, "encryption_key")

	var key *api.Secret
	err := retry.Wait(ctx, func(ctx context.Context) (bool, error) {
		var verr error
		key, verr = lmm.client.Logical().Read(dataPath)
		if verr != nil {
			return false, verr
		}

		return true, nil
	})
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	if key == nil {
		return nil, nil
	}

	if key.Data == nil {
		return nil, nil
	}

	data, ok := key.Data["data"].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	value, found := data["value"].(string)
	if !found {
		return nil, nil
	}

	// FIXME Lookup context and name?
	return &model.LogMetadata{
		Key:   value,
		LogID: id,
	}, nil
}

func (lmm *VaultLogMetadataManager) Create(ctx context.Context, log *model.Log) (*model.LogMetadata, error) {
	logContextPath := path.Join(lmm.engineMount, "data", "contexts", log.Context, "name", log.Name)

	dataPath := path.Join(logContextPath, "log_id")

	var logID *api.Secret
	err := retry.Wait(ctx, func(ctx context.Context) (bool, error) {
		var verr error
		logID, verr = lmm.client.Logical().Read(dataPath)
		if verr != nil {
			return false, verr
		}

		return true, nil
	})
	if err != nil {
		return nil, err
	}

	if logID != nil {
		if logID.Data == nil {
			return nil, nil
		}

		data, ok := logID.Data["data"].(map[string]interface{})
		if !ok {
			return nil, nil
		}

		value, found := data["value"].(string)
		if !found {
			return nil, nil
		}

		// FIXME Add/lookup key or not?
		return &model.LogMetadata{
			Log:   log,
			LogID: value,
		}, nil
	}

	id := uuid.New().String()

	v, err := transfer.EncodeForTransfer([]byte(id))
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"data": map[string]interface{}{
			"value": v,
		},
		"options": map[string]interface{}{
			"cas": 0,
		},
	}

	if _, err := lmm.client.Logical().Write(dataPath, payload); err != nil {
		return nil, err
	}

	logMetadataPath := path.Join(lmm.engineMount, "data", "logs", id)

	key, err := lmm.keyManager.Create(ctx)
	if err != nil {
		return nil, err
	}

	v, err = transfer.EncodeForTransfer([]byte(key))
	if err != nil {
		return nil, err
	}

	keyPath := path.Join(logMetadataPath, "encryption_key")

	payload = map[string]interface{}{
		"data": map[string]interface{}{
			"value": v,
		},
		"options": map[string]interface{}{
			"cas": 0,
		},
	}

	if _, err := lmm.client.Logical().Write(keyPath, payload); err != nil {
		return nil, err
	}

	return &model.LogMetadata{
		Key:   key,
		Log:   log,
		LogID: id,
	}, nil
}

func NewVaultLogMetadataManager(cfg *opt.Config, vaultClient *api.Client) (model.LogMetadataManager, error) {
	vaultEngineMount, err := vaultutil.CheckNormalizeEngineMount(vaultClient, cfg.VaultEngineMount)
	if err != nil {
		return nil, err
	}

	return &VaultLogMetadataManager{
		client:      vaultClient,
		engineMount: vaultEngineMount,

		keyManager: NewKeyManager(),
	}, nil
}
