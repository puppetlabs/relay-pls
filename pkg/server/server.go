package server

import (
	"context"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/golang/protobuf/ptypes"
	"github.com/google/uuid"
	"github.com/puppetlabs/leg/timeutil/pkg/retry"
	"github.com/puppetlabs/relay-pls/pkg/model"
	"github.com/puppetlabs/relay-pls/pkg/plspb"
	"go.opentelemetry.io/otel/label"
	"go.opentelemetry.io/otel/metric"
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

func WithMetrics(meter *metric.Meter) BigQueryServerOption {
	return func(s *BigQueryServer) {
		s.meter = meter
	}
}

type BigQueryServer struct {
	client *bigquery.Client
	table  *bigquery.Table

	keyManager         model.KeyManager
	logMetadataManager model.LogMetadataManager

	meter *metric.Meter
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
	s.countOutcomeMetric(model.MetricLogCreateMetadata, err)
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
	s.countOutcomeMetric(model.MetricLogGetMetadata, err)
	if err != nil {
		return nil, err
	}

	ct, err := s.keyManager.Encrypt(ctx, lm.Key, in.GetPayload())
	s.countOutcomeMetric(model.MetricLogEncryptMessage, err)
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

	err = retry.Wait(ctx, func(ctx context.Context) (bool, error) {
		ierr := inserter.Put(ctx, logs)
		if ierr != nil {
			return false, ierr
		}

		return true, nil
	})
	s.countOutcomeMetric(model.MetricLogInsertMessage, err)
	if err != nil {
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
	s.countOutcomeMetric(model.MetricLogGetMetadata, err)
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

			err = retry.Wait(ctx, func(ctx context.Context) (bool, error) {
				if serr := stream.Send(message); serr != nil {
					return false, serr
				}

				return true, nil
			})
			s.countOutcomeMetric(model.MetricLogStreamMessage, err)
		}

		if !in.GetFollow() {
			break
		}

		qb.After(&ts)
	}

	return nil
}

func (s *BigQueryServer) countOutcomeMetric(name string, err error) {
	if err != nil {
		s.countMetric(name, model.MetricLabelOutcome, model.MetricValueFailed)
		return
	}

	s.countMetric(name, model.MetricLabelOutcome, model.MetricValueSuccess)
}

func (s *BigQueryServer) countMetric(name, key, value string) {
	counter := metric.Must(*s.meter).NewInt64Counter(name)
	counter.Add(context.Background(), 1,
		label.String(model.MetricLabelModule, "big-query-server"),
		label.String(key, value),
	)
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
