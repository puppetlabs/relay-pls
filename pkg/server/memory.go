package server

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/google/wire"
	"github.com/puppetlabs/leg/timeutil/pkg/retry"
	"github.com/puppetlabs/relay-pls/pkg/model"
	"github.com/puppetlabs/relay-pls/pkg/opt"
	"github.com/puppetlabs/relay-pls/pkg/plspb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var InMemoryServerSet = wire.NewSet(
	NewInMemoryServer,
)

type InMemoryServer struct {
	plspb.UnimplementedLogServer
	logMetadataManager model.LogMetadataManager
	keyManager         model.KeyManager
	messages           map[string][]*LogMessage
}

func (s *InMemoryServer) Create(ctx context.Context, in *plspb.LogCreateRequest) (*plspb.LogCreateResponse, error) {
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

func (s *InMemoryServer) Delete(ctx context.Context, in *plspb.LogDeleteRequest) (*plspb.LogDeleteResponse, error) {
	return nil, nil
}

func (s *InMemoryServer) List(in *plspb.LogListRequest, stream plspb.Log_ListServer) error {
	return nil
}

func (s *InMemoryServer) MessageAppend(ctx context.Context, in *plspb.LogMessageAppendRequest) (*plspb.LogMessageAppendResponse, error) {
	lmm, err := s.logMetadataManager.Get(ctx, in.GetLogId())
	if err != nil {
		return nil, err
	}

	ct, err := s.keyManager.Encrypt(ctx, lmm.Key, in.GetPayload())
	if err != nil {
		return nil, err
	}

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

	if s.messages == nil {
		s.messages = make(map[string][]*LogMessage)
	}
	if s.messages[in.GetLogId()] == nil {
		s.messages[in.GetLogId()] = make([]*LogMessage, 0)
	}

	s.messages[in.GetLogId()] = append(s.messages[in.GetLogId()], message)

	return &plspb.LogMessageAppendResponse{
		LogId:        message.LogID,
		LogMessageId: message.LogMessageID,
	}, nil
}

func (s *InMemoryServer) MessageList(in *plspb.LogMessageListRequest, stream plspb.Log_MessageListServer) error {
	ctx := context.Background()

	lmm, err := s.logMetadataManager.Get(ctx, in.GetLogId())
	if err != nil {
		return err
	}

	for _, message := range s.messages[in.GetLogId()] {
		payload, err := s.keyManager.Decrypt(ctx, lmm.Key, message.EncryptedPayload)
		if err != nil {
			return err
		}
		resp := &plspb.LogMessageListResponse{
			LogMessageId: message.LogMessageID,
			Payload:      payload,
			Timestamp:    timestamppb.New(message.Timestamp),
		}
		err = retry.Wait(ctx, func(ctx context.Context) (bool, error) {
			if serr := stream.Send(resp); serr != nil {
				return false, serr
			}

			return true, nil
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func NewInMemoryServer(cfg *opt.Config,
	keyManager model.KeyManager, logMetadataManager model.LogMetadataManager) plspb.LogServer {
	s := &InMemoryServer{
		logMetadataManager: logMetadataManager,
		keyManager:         keyManager,
	}

	return s
}
