package server_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/ptypes"
	"github.com/google/uuid"
	"github.com/puppetlabs/relay-pls/pkg/manager"
	"github.com/puppetlabs/relay-pls/pkg/model"
	"github.com/puppetlabs/relay-pls/pkg/opt"
	"github.com/puppetlabs/relay-pls/pkg/plspb"
	"github.com/puppetlabs/relay-pls/pkg/server"
	"github.com/puppetlabs/relay-pls/pkg/test/mock"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

const (
	MAX_LOG_COUNT         = 5
	MAX_LOG_MESSAGE_COUNT = 5
)

type mockListService_ListMessageServer struct {
	grpc.ServerStream
	Messages []*plspb.LogMessageListResponse
}

func (mls *mockListService_ListMessageServer) Send(m *plspb.LogMessageListResponse) error {
	mls.Messages = append(mls.Messages, m)
	return nil
}

func TestBigQueryServer(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	ctrl := gomock.NewController(t)

	cfg, err := opt.NewConfig()
	assert.NoError(t, err)

	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" {
		assert.FailNow(t, "GOOGLE_APPLICATION_CREDENTIALS must be set for this integration test")
	}

	if cfg.Project == "" || cfg.Dataset == "" || cfg.Table == "" {
		assert.FailNow(t, "BigQuery configuration must be set for this integration test")
	}

	bigqueryClient, err := cfg.BigQueryClient()
	assert.NoError(t, err)

	table, err := cfg.BigQueryTable()
	assert.NoError(t, err)

	km := manager.NewKeyManager()

	logs := []*model.Log{}
	for logIndex := 0; logIndex < MAX_LOG_COUNT; logIndex++ {
		logContext := uuid.New().String()
		logs = append(logs,
			&model.Log{
				Context: logContext,
				Name:    "stdout",
			},
			&model.Log{
				Context: logContext,
				Name:    "stderr",
			})
	}

	ctx := context.Background()

	logMetadata, err := createLogMetadata(ctx, logs, km)
	assert.NoError(t, err)

	m := mock.NewMockLogMetadataStore(ctrl)

	setExpectations(ctx, logs, logMetadata, m)

	lmm := manager.NewLogMetadataManager(m)

	var serverOpts []server.BigQueryServerOption

	serverOpts = append(serverOpts,
		server.WithBigQueryClient(bigqueryClient),
		server.WithKeyManager(km),
		server.WithLogMetadataManager(lmm),
	)

	s := server.NewBigQueryServer(table, serverOpts...)

	for _, log := range logs {
		createResponse, err := s.Create(ctx, &plspb.LogCreateRequest{Context: log.Context, Name: log.Name})
		assert.NoError(t, err)
		assert.NotNil(t, createResponse)
		assert.NotEmpty(t, createResponse.GetLogId())

		expectedMessages := make(map[string]*plspb.LogMessageListResponse)
		for messageIndex := 0; messageIndex < MAX_LOG_MESSAGE_COUNT; messageIndex++ {
			payload := []byte(fmt.Sprintf("test-message %d", messageIndex))

			ts, err := ptypes.TimestampProto(time.Now().UTC())
			assert.NoError(t, err)

			messageResponse, err := s.MessageAppend(ctx,
				&plspb.LogMessageAppendRequest{
					LogId:     createResponse.GetLogId(),
					Payload:   payload,
					Timestamp: ts,
				},
			)
			assert.NoError(t, err)
			assert.NotNil(t, messageResponse)
			assert.NotEmpty(t, messageResponse.GetLogMessageId())
			assert.Equal(t, messageResponse.GetLogId(), createResponse.GetLogId())

			expectedMessages[messageResponse.GetLogMessageId()] = &plspb.LogMessageListResponse{
				LogMessageId: messageResponse.GetLogMessageId(),
				Payload:      payload,
				Timestamp:    ts,
			}
		}

		stream := &mockListService_ListMessageServer{}
		err = s.MessageList(&plspb.LogMessageListRequest{Follow: false, LogId: createResponse.GetLogId()}, stream)
		assert.NoError(t, err)
		assert.Equal(t, MAX_LOG_MESSAGE_COUNT, len(stream.Messages))

		var lastTimestamp time.Time
		for index, message := range stream.Messages {
			expected, ok := expectedMessages[message.GetLogMessageId()]
			assert.True(t, ok)

			assert.Equal(t, expected.GetLogMessageId(), message.GetLogMessageId())
			assert.Equal(t, expected.GetPayload(), message.GetPayload())
			assert.Equal(t, expected.GetTimestamp().AsTime().Truncate(time.Microsecond), message.GetTimestamp().AsTime().Truncate(time.Microsecond))

			if index > 0 {
				assert.False(t, message.GetTimestamp().AsTime().Truncate(time.Microsecond).Before(lastTimestamp))
			}

			lastTimestamp = message.GetTimestamp().AsTime().Truncate(time.Microsecond)
		}
	}
}

func createLogMetadata(ctx context.Context, logs []*model.Log, km model.KeyManager) ([]*model.LogMetadata, error) {
	lm := make([]*model.LogMetadata, len(logs))

	for index, log := range logs {
		key, err := km.Create(ctx)
		if err != nil {
			return nil, err
		}

		logMetadata := &model.LogMetadata{
			Log:   log,
			LogID: uuid.New().String(),
			Key:   key,
		}

		lm[index] = logMetadata
	}

	return lm, nil
}

func setExpectations(ctx context.Context, logs []*model.Log, logMetadata []*model.LogMetadata, m *mock.MockLogMetadataStore) {
	for index, log := range logs {
		m.
			EXPECT().
			CreateLogMetadata(ctx, gomock.Eq(log)).
			Return(logMetadata[index], nil).
			AnyTimes()

		m.
			EXPECT().
			ReadLogMetadata(ctx, gomock.Eq(logMetadata[index].LogID)).
			Return(logMetadata[index], nil).
			AnyTimes()
	}
}
