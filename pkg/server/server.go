package server

import (
	"context"
	"log"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/golang/protobuf/ptypes"
	"github.com/google/uuid"
	"github.com/puppetlabs/relay-pls/pkg/model"
	"github.com/puppetlabs/relay-pls/pkg/plspb"
	"google.golang.org/api/iterator"
)

type BigQueryServerOption func(s *BigQueryServer)

func WithBigQueryClient(client *bigquery.Client) BigQueryServerOption {
	return func(s *BigQueryServer) {
		s.client = client
	}
}

func WithLogMetadataManager(lmm model.LogMetadataManager) BigQueryServerOption {
	return func(s *BigQueryServer) {
		s.logMetadataManager = lmm
	}
}

func WithKeyManager(km model.KeyManager) BigQueryServerOption {
	return func(s *BigQueryServer) {
		s.keyManager = km
	}
}

type BigQueryServer struct {
	client *bigquery.Client
	table  *bigquery.Table

	keyManager         model.KeyManager
	logMetadataManager model.LogMetadataManager
}

func (s *BigQueryServer) Svc() *plspb.LogService {
	return &plspb.LogService{
		Create:        s.Create,
		Delete:        s.Delete,
		List:          s.List,
		MessageAppend: s.MessageAppend,
		MessageList:   s.MessageList,
	}
}

func (s *BigQueryServer) Create(ctx context.Context, in *plspb.LogCreateRequest) (*plspb.LogCreateResponse, error) {
	if in.GetContext() == "" || in.GetName() == "" {
		return nil, ErrInvalid
	}

	lm, err := s.logMetadataManager.Create(ctx,
		&model.Log{
			Context: in.Context,
			Name:    in.Name,
		})
	if err != nil {
		return nil, err
	}

	return &plspb.LogCreateResponse{
		LogId: lm.LogID,
	}, nil
}

func (s *BigQueryServer) Delete(ctx context.Context, in *plspb.LogDeleteRequest) (*plspb.LogDeleteResponse, error) {
	return nil, nil
}

func (s *BigQueryServer) List(in *plspb.LogListRequest, stream plspb.Log_ListServer) error {
	return nil
}

func (s *BigQueryServer) MessageAppend(ctx context.Context, in *plspb.LogMessageAppendRequest) (*plspb.LogMessageAppendResponse, error) {
	lm, err := s.logMetadataManager.Get(ctx, in.GetLogId())
	if err != nil {
		return nil, err
	}

	ct, err := s.keyManager.Encrypt(ctx, lm.Key, in.GetPayload())
	if err != nil {
		return nil, err
	}

	logs := make([]*LogMessage, 0)

	ts := time.Now()
	if in.GetTimestamp() != nil {
		ts = in.GetTimestamp().AsTime()
	}

	message := &LogMessage{
		LogID:            in.GetLogId(),
		LogMessageID:     uuid.New().String(),
		Timestamp:        ts,
		EncryptedPayload: ct,
	}

	logs = append(logs, message)

	inserter := s.table.Inserter()
	if err := inserter.Put(ctx, logs); err != nil {
		return nil, err
	}

	return &plspb.LogMessageAppendResponse{
		LogId:        message.LogID,
		LogMessageId: message.LogMessageID,
	}, nil
}

func (s *BigQueryServer) MessageList(in *plspb.LogMessageListRequest, stream plspb.Log_MessageListServer) error {
	ctx := context.Background()

	lm, err := s.logMetadataManager.Get(ctx, in.GetLogId())
	if err != nil {
		return err
	}

	qb := NewBigQueryTableQueryBuilder()
	qb.WithClient(s.client)
	qb.WithTable(s.table)

	qb.WithLog(in.GetLogId())
	qb.WithEncryptionKey(lm.Key)

	if in.GetStartAt() != nil {
		startAt := in.GetStartAt().AsTime()
		qb.WithStartAt(&startAt)
	}

	if in.GetEndAt() != nil {
		endAt := in.GetEndAt().AsTime()
		qb.WithEndAt(&endAt)
	}

	for {
		q, err := qb.Build()
		if err != nil {
			return err
		}

		it, err := q.Read(ctx)
		if err != nil {
			return err
		}

		ts := time.Now().UTC()
		for {
			if it == nil {
				break
			}

			var values []bigquery.Value
			err := it.Next(&values)

			if err == iterator.Done {
				break
			}
			if err != nil {
				continue
			}

			message := &plspb.LogMessageListResponse{}

			if payload, ok := values[QueryColumnPayload].([]byte); ok {
				message.Payload = payload
			}

			ts, ok := values[QueryColumnTimestamp].(time.Time)
			if ok {
				if thisTime, err := ptypes.TimestampProto(ts); err == nil {
					message.Timestamp = thisTime
				}
			}

			if logMessageID, ok := values[QueryColumnLogMessageID].(string); ok {
				message.LogMessageId = logMessageID
			}

			// FIXME Add retry logic
			// FIXME Collate errors
			if err := stream.Send(message); err != nil {
				// FIXME Do not log within this context
				log.Printf("failed to send message: %v", err)
			}
		}

		if !in.GetFollow() {
			break
		}

		qb.After(&ts)
	}

	return nil
}

func NewBigQueryServer(table *bigquery.Table, opts ...BigQueryServerOption) *BigQueryServer {
	s := &BigQueryServer{
		table: table,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}
