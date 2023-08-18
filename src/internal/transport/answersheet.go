package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/avast/retry-go"
	"github.com/rs/zerolog/log"
	"github.com/segmentio/kafka-go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"picket-answersheet-service/src/config"
	"picket-answersheet-service/src/internal/dto"
	"picket-answersheet-service/src/internal/entities"
	"picket-answersheet-service/src/internal/usecase"
	answersheetpb "picket-answersheet-service/src/pb/answer_sheet"
	"strings"
	"time"
)

type IAnswerSheetUsecase interface {
	StartTest(ctx context.Context, input dto.StartTestInput) error
	UserAnswer(ctx context.Context, input dto.UserAnswerInput) error
	PushToDeadLetterQueue(ctx context.Context, value []byte) error
	CheckUserDoingTest(ctx context.Context, userId int, testId int) (bool, error)
	NotifyJobSuccess(ctx context.Context, jobId int) error
	NotifyJobFail(ctx context.Context, jobId int, errFail error) error
	GetLatestStartTime(ctx context.Context, testId int, userId int) (*time.Time, error)
	GetCurrentTest(ctx context.Context, testId int, userId int) ([]entities.Event, error)
	CheckUserSubmitted(ctx context.Context, userId int, testId int) (bool, error)
	SubmitTest(ctx context.Context, input dto.SubmitTestInput) error
	GetScore(ctx context.Context, input dto.GetScoreInput) (*dto.GetScoreOutput, error)
}

type answersheetTransport struct {
	usecase IAnswerSheetUsecase
	answersheetpb.UnimplementedAnswerSheetServiceServer
	config config.IConfig
}

func NewAnswerSheetTransport(ctx context.Context, usecase IAnswerSheetUsecase, iConfig config.IConfig) *answersheetTransport {
	t := answersheetTransport{usecase: usecase, config: iConfig}
	go t.StartTest(ctx)
	go func() {
		for i := 1; i <= 10; i++ {
			go t.UserAnswer(ctx)
		}
	}()
	go t.SubmitTest(ctx)
	return &t
}

func (t *answersheetTransport) UserAnswer(ctx context.Context) {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: strings.Split(t.config.GetKafkaBroker(), ","),
		Topic:   "answer-test",
		GroupID: "answer-test-2",
		//Logger:  kafka.LoggerFunc(log.Info().Msgf),
	})

	for {
		m, err := r.FetchMessage(ctx)
		log.Info().Str("message", string(m.Value)).Str("topic", m.Topic).Msg("fetch message")
		if err != nil {
			log.Error().Err(err).Send()
			continue
		}
		var input dto.UserAnswerInput
		if err := json.NewDecoder(bytes.NewBuffer(m.Value)).Decode(&input); err != nil {
			log.Error().Err(err).Send()
			r.CommitMessages(ctx, m)
			continue
		}

		if input.JobId == 0 {
			log.Error().Interface("payload", input.Payload).Msg("message not valid")
			r.CommitMessages(ctx, m)
			continue
		}

		err = retry.Do(func() error {
			return t.usecase.UserAnswer(ctx, input)
		}, retry.Attempts(10))
		if err != nil {
			log.Error().Err(err).Send()
			continue
		}

		go retry.Do(func() error {
			return t.usecase.NotifyJobSuccess(ctx, input.JobId)
		}, retry.Attempts(20))

		if err := r.CommitMessages(ctx, m); err != nil {
			log.Error().Err(err).Send()
			continue
		}
	}
}

func (t *answersheetTransport) SubmitTest(ctx context.Context) {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: strings.Split(t.config.GetKafkaBroker(), ","),
		Topic:   "submit-test",
		GroupID: "submit-test-1",
		//Logger:  kafka.LoggerFunc(log.Info().Msgf),
	})
	defer func() {
		if err := r.Close(); err != nil {
			log.Error().Err(err).Send()
		}
	}()

	for {
		m, err := r.FetchMessage(ctx)
		if err != nil {
			log.Error().Err(err).Send()
			continue
		}
		var input dto.SubmitTestInput
		if err := json.NewDecoder(bytes.NewBuffer(m.Value)).Decode(&input); err != nil {
			log.Error().Err(err).Send()
			r.CommitMessages(ctx, m)
			continue
		}
		if input.JobId == 0 {
			log.Error().Interface("payload", input.Payload).Msg("job_id = 0")
			r.CommitMessages(ctx, m)
			continue
		}
		log.Info().Str("topic", "submit-test").Interface("payload", input).Send()
		err = retry.Do(func() error {
			return t.usecase.SubmitTest(ctx, input)
		}, retry.Attempts(10))
		if err != nil {
			log.Error().Err(err).Send()
			continue
		}

		go retry.Do(func() error {
			err := t.usecase.NotifyJobSuccess(ctx, input.JobId)
			if err != nil && errors.Is(err, usecase.ErrUserSubmitted) {
				return nil
			}
			return err
		}, retry.Attempts(20))
		if err := r.CommitMessages(ctx, m); err != nil {
			log.Error().Err(err).Send()
			continue
		}
	}

}

func (t *answersheetTransport) StartTest(ctx context.Context) {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: strings.Split(t.config.GetKafkaBroker(), ","),
		Topic:   "start-test",
		GroupID: "start-test-1",
		//Logger:  kafka.LoggerFunc(log.Info().Msgf),
	})

	defer func() {
		if err := r.Close(); err != nil {
			log.Error().Err(err).Send()
		}
	}()
	for {
		m, err := r.FetchMessage(ctx)
		log.Info().Str("message", string(m.Value)).Str("topic", m.Topic).Msg("fetch message")

		if err != nil {
			log.Error().Err(err).Send()
			continue
		}
		var input dto.StartTestInput
		// message json khong hop le
		if err := json.NewDecoder(bytes.NewBuffer(m.Value)).Decode(&input); err != nil {
			log.Error().Err(err).Send()
			r.CommitMessages(ctx, m)
			continue
		}
		// message chua duoc insert
		if input.JobId == 0 {
			log.Error().Interface("payload", input.Payload).Msg("message not valid")
			r.CommitMessages(ctx, m)
			continue
		}

		// retry start
		err = retry.Do(func() error {
			return t.usecase.StartTest(ctx, input)
		}, retry.Attempts(10))
		if err != nil {
			log.Error().Err(err).Send()
			continue
		}

		go retry.Do(func() error {
			return t.usecase.NotifyJobSuccess(ctx, input.JobId)
		}, retry.Attempts(20))

		if err := r.CommitMessages(ctx, m); err != nil {
			log.Error().Err(err).Send()
			continue
		}
	}

}

func (t *answersheetTransport) CheckUserDoingTest(ctx context.Context, request *answersheetpb.CheckUserDoingTestRequest) (*answersheetpb.CheckUserDoingTestResponse, error) {

	check, err := t.usecase.CheckUserDoingTest(ctx, int(request.UserId), int(request.TestId))
	if err != nil {
		return nil, status.Error(codes.Internal, "server has error")
	}

	return &answersheetpb.CheckUserDoingTestResponse{
		Check:   check,
		Message: "success",
	}, nil
}

func (t *answersheetTransport) GetLatestStartTime(ctx context.Context, request *answersheetpb.GetLatestStartTimeRequest) (*answersheetpb.GetLatestStartTimeResponse, error) {
	result, err := t.usecase.GetLatestStartTime(ctx, int(request.TestId), int(request.UserId))

	if err != nil {
		panic(err)
	}

	resp := answersheetpb.GetLatestStartTimeResponse{
		Message: "success1",
	}

	if result != nil {
		//zap.S().Info(result.Format("15:04:05 02/01/2006"))
		resp.Data = timestamppb.New(*result)
	} else {
		resp.Data = nil
	}

	return &resp, nil
}

func (t *answersheetTransport) GetCurrentTest(ctx context.Context, request *answersheetpb.GetCurrentTestRequest) (*answersheetpb.GetCurrentTestResponse, error) {
	data, err := t.usecase.GetCurrentTest(ctx, int(request.TestId), int(request.UserId))
	if err != nil {
		panic(err)
	}
	list := make([]*answersheetpb.Answer, len(data))
	for index, item := range data {
		list[index] = &answersheetpb.Answer{
			QuestionId: int64(item.QuestionId),
			Answer:     item.Answer,
		}
	}
	resp := answersheetpb.GetCurrentTestResponse{
		Message: "success",
		Data:    list,
	}
	return &resp, nil
}

func (t *answersheetTransport) CheckUserSubmitted(ctx context.Context, request *answersheetpb.CheckUserSubmittedRequest) (*answersheetpb.CheckUserSubmittedResponse, error) {
	check, err := t.usecase.CheckUserSubmitted(ctx, int(request.UserId), int(request.TestId))
	if err != nil {
		return nil, err
	}
	return &answersheetpb.CheckUserSubmittedResponse{
		Data: check,
	}, nil

}

func (t *answersheetTransport) GetScore(ctx context.Context, request *answersheetpb.GetScoreRequest) (*answersheetpb.GetScoreResponse, error) {
	input := dto.GetScoreInput{
		Session: request.Session,
		UserId:  int(request.UserId),
		TestId:  int(request.TestId),
	}
	result, err := t.usecase.GetScore(ctx, input)
	if err != nil {
		return nil, err
	}

	return &answersheetpb.GetScoreResponse{
		Score: result.Score,
	}, nil
}

// KAFKA
