package manager

import (
	"context"
	"path"

	"github.com/google/uuid"
	"github.com/hashicorp/vault/api"
	"github.com/puppetlabs/horsehead/v2/encoding/transfer"
	"github.com/puppetlabs/relay-pls/pkg/model"
)

type LogMetadataManager struct {
	store model.LogMetadataStore
}

func (m *LogMetadataManager) Get(ctx context.Context, id string) (*model.LogMetadata, error) {
	return m.store.ReadLogMetadata(ctx, id)
}

func (m *LogMetadataManager) Create(ctx context.Context, log *model.Log) (*model.LogMetadata, error) {
	return m.store.CreateLogMetadata(ctx, log)
}

func NewLogMetadataManager(store model.LogMetadataStore) *LogMetadataManager {
	return &LogMetadataManager{
		store: store,
	}
}

type VaultLogMetadataStore struct {
	client      *api.Client
	engineMount string

	keyManager model.KeyManager
}

func (vlms *VaultLogMetadataStore) ReadLogMetadata(ctx context.Context, id string) (*model.LogMetadata, error) {
	logMetadataPath := path.Join(vlms.engineMount, "data", "logs", id)

	dataPath := path.Join(logMetadataPath, "encryption_key")

	key, err := vlms.client.Logical().Read(dataPath)
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

func (vlms *VaultLogMetadataStore) CreateLogMetadata(ctx context.Context, log *model.Log) (*model.LogMetadata, error) {
	logContextPath := path.Join(vlms.engineMount, "data", "contexts", log.Context, "name", log.Name)

	dataPath := path.Join(logContextPath, "log_id")

	logID, err := vlms.client.Logical().Read(dataPath)
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

	if _, err := vlms.client.Logical().Write(dataPath, payload); err != nil {
		return nil, err
	}

	logMetadataPath := path.Join(vlms.engineMount, "data", "logs", id)

	key, err := vlms.keyManager.Create(ctx)
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

	if _, err := vlms.client.Logical().Write(keyPath, payload); err != nil {
		return nil, err
	}

	return &model.LogMetadata{
		Key:   key,
		Log:   log,
		LogID: id,
	}, nil
}

func NewVaultLogMetadataStore(client *api.Client, engineMount string, keyManager model.KeyManager) *VaultLogMetadataStore {
	return &VaultLogMetadataStore{
		client:      client,
		engineMount: engineMount,

		keyManager: keyManager,
	}
}
