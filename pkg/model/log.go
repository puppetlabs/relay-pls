package model

import (
	"context"
)

type Log struct {
	Context string
	Name    string
}

type LogMetadata struct {
	Key   string
	Log   *Log
	LogID string
}

type KeyManager interface {
	Create(ctx context.Context) (string, error)
	Decrypt(ctx context.Context, key string, data []byte) ([]byte, error)
	Encrypt(ctx context.Context, key string, data []byte) ([]byte, error)
}

type LogMetadataManager interface {
	Create(ctx context.Context, log *Log) (*LogMetadata, error)
	Get(ctx context.Context, id string) (*LogMetadata, error)
}
