package server

import (
	"context"
	"net/http"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/google/uuid"
	"github.com/google/wire"
	"github.com/puppetlabs/leg/timeutil/pkg/retry"
	"github.com/puppetlabs/relay-pls/pkg/model"
	"github.com/puppetlabs/relay-pls/pkg/opt"
	"github.com/puppetlabs/relay-pls/pkg/plspb"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iterator"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var BigQueryServerSet = wire.NewSet(
	NewBigQueryServer,
	NewBigQueryClient,
	NewBigQueryTable,
)

type BigQueryServer struct {
	plspb.UnimplementedLogServer
	client             *bigquery.Client
	table              *bigquery.Table
	logMetadataManager model.LogMetadataManager
	keyManager         model.KeyManager
	meter              *metric.Meter
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
	s.countOutcomeMetric(ctx, model.MetricLogCreateMetadata, err)
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
	lmm, err := s.logMetadataManager.Get(ctx, in.GetLogId())
	s.countOutcomeMetric(ctx, model.MetricLogGetMetadata, err)
	if err != nil {
		return nil, err
	}

	ct, err := s.keyManager.Encrypt(ctx, lmm.Key, in.GetPayload())
	s.countOutcomeMetric(ctx, model.MetricLogEncryptMessage, err)
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
	s.countOutcomeMetric(ctx, model.MetricLogInsertMessage, err)
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
	s.countOutcomeMetric(ctx, model.MetricLogGetMetadata, err)
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
				message.Timestamp = timestamppb.New(ts)
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
			s.countOutcomeMetric(ctx, model.MetricLogStreamMessage, err)
		}

		if !in.GetFollow() {
			break
		}

		qb.After(&ts)
	}

	return nil
}

func (s *BigQueryServer) countOutcomeMetric(ctx context.Context, name string, err error) {
	attrs := []attribute.KeyValue{
		attribute.String(model.MetricLabelOutcome, model.MetricValueSuccess),
	}
	if err != nil {
		attrs = []attribute.KeyValue{
			attribute.String(model.MetricLabelOutcome, model.MetricValueFailed),
		}
	}

	s.countMetric(ctx, name, attrs...)
}

func (s *BigQueryServer) countMetric(ctx context.Context, name string, additionalAttrs ...attribute.KeyValue) {
	if s.meter == nil {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String(model.MetricLabelModule, "big-query-server"),
	}
	attrs = append(attrs, additionalAttrs...)

	counter := metric.Must(*s.meter).NewInt64Counter(name)
	counter.Add(ctx, 1, attrs...)
}

func NewBigQueryServer(cfg *opt.Config,
	keyManager model.KeyManager, logMetadataManager model.LogMetadataManager,
	bigQueryClient *bigquery.Client, bigQueryTable *bigquery.Table,
	meter *metric.Meter) plspb.LogServer {

	s := &BigQueryServer{
		client: bigQueryClient,
		table:  bigQueryTable,

		keyManager:         keyManager,
		logMetadataManager: logMetadataManager,
		meter:              meter,
	}

	return s
}

func NewBigQueryTable(ctx context.Context, cfg *opt.Config, client *bigquery.Client) (*bigquery.Table, error) {
	schema := bigquery.Schema{
		{Name: "log_id", Type: bigquery.StringFieldType, Required: true},
		{Name: "log_message_id", Type: bigquery.StringFieldType, Required: true},
		{Name: "timestamp", Type: bigquery.TimestampFieldType, Required: true},
		{Name: "encrypted_payload", Type: bigquery.BytesFieldType},
	}

	metadata := &bigquery.TableMetadata{
		Schema: schema,
		Clustering: &bigquery.Clustering{
			Fields: []string{
				"log_id",
			},
		},
	}

	dataset := client.Dataset(cfg.Dataset)
	table := dataset.Table(cfg.Table)
	err := table.Create(ctx, metadata)
	if e, ok := err.(*googleapi.Error); ok && e.Code != http.StatusConflict {
		return nil, err
	}

	return table, nil
}

func NewBigQueryClient(ctx context.Context, cfg *opt.Config) (*bigquery.Client, error) {
	return bigquery.NewClient(ctx, cfg.Project)
}
