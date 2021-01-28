package model

import (
	"context"
)

const (
	METRIC_LOG_CREATE_METADATA = "log_create_metadata"
	METRIC_LOG_ENCRYPT_MESSAGE = "log_encrypt_message"
	METRIC_LOG_GET_METADATA    = "log_get_metadata"
	METRIC_LOG_INSERT_MESSAGE  = "log_insert_message"
	METRIC_LOG_SERVICE_STARTUP = "log_service_startup"
	METRIC_LOG_STREAM_MESSAGE  = "log_stream_message"

	METRIC_LABEL_MODULE  = "module"
	METRIC_LABEL_OUTCOME = "outcome"

	METRIC_VALUE_FAILED  = "failed"
	METRIC_VALUE_SUCCESS = "success"
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
	Encrypt(ctx context.Context, key string, data []byte) ([]byte, error)
}

type LogMetadataManager interface {
	Create(ctx context.Context, log *Log) (*LogMetadata, error)
	Get(ctx context.Context, id string) (*LogMetadata, error)
}

type LogMetadataStore interface {
	CreateLogMetadata(ctx context.Context, log *Log) (*LogMetadata, error)
	ReadLogMetadata(ctx context.Context, id string) (*LogMetadata, error)
}
