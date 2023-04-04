package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/segmentio/kafka-go"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"picket-answersheet-service/src/config"
	"picket-answersheet-service/src/internal/dto"
	"picket-answersheet-service/src/internal/entities"
	"strings"
	"time"
)

type IAnswersheetRepository interface {
	Create(ctx context.Context, event *entities.Event) error
	GetLatestEvent(ctx context.Context, userId int, testId int) ([]entities.Event, error)
	GetLatestStartEvent(ctx context.Context, userId int, testId int) (*entities.Event, error)
	GetLatestEventWithLimit(ctx context.Context, userId int, testId int, limit int) ([]entities.Event, error)
	FindAnswerByUserIdAndTestId(ctx context.Context, userId int, testId int, sessionId string) ([]entities.Event, error)
}

type answersheetUsecase struct {
	repository IAnswersheetRepository
	config     config.IConfig
}

func NewAnswersheetUsecase(repository IAnswersheetRepository, iConfig config.IConfig) *answersheetUsecase {
	return &answersheetUsecase{repository: repository, config: iConfig}
}

var tracer = otel.Tracer("usecase")

func (u *answersheetUsecase) StartTest(ctx context.Context, input dto.StartTestInput) error {

	session := uuid.New()
	e := entities.Event{
		UserId:    input.Payload.UserId,
		TestId:    input.Payload.TestId,
		CreatedAt: input.Payload.CreatedAt,
		UpdatedAt: input.Payload.UpdatedAt,
		Event:     entities.START,
		Id:        primitive.NewObjectID(),
		Session:   session.String(),
	}
	err := u.repository.Create(ctx, &e)
	if err != nil {
		log.Error().Err(err).Send()
		return err
	}

	log.Info().Int("user_id", e.UserId).Int("test_id", e.TestId).Msg("user start test")

	return nil

}

func (u *answersheetUsecase) CheckUserDoingTest(ctx context.Context, userId int, testId int) (bool, error) {
	ctx, span := tracer.Start(ctx, "get lastest event ", trace.WithAttributes(attribute.Int("userId", userId), attribute.Int("testId", testId)))
	defer span.End()
	list, err := u.repository.GetLatestEvent(ctx, userId, testId)
	if err != nil {
		log.Error().Err(err).Send()
		return false, err
	}

	if len(list) == 0 {
		return false, nil
	}

	if len(list) == 1 {
		if list[0].Event == entities.START || list[0].Event == entities.DOING {
			return true, nil
		}
		return false, nil
	}

	first, second := list[0], list[1]

	if first.Event == entities.END && second.Event == entities.START {
		return false, nil
	}

	return true, nil
}

func (u *answersheetUsecase) GetCurrentTest(ctx context.Context, testId int, userId int) ([]entities.Event, error) {
	event, err := u.repository.GetLatestStartEvent(ctx, userId, testId)
	if err != nil {
		log.Error().Err(err).Send()
		return nil, err
	}
	return u.repository.FindAnswerByUserIdAndTestId(ctx, userId, testId, event.Session)
}

func (u *answersheetUsecase) GetLatestStartTime(ctx context.Context, testId int, userId int) (*time.Time, error) {
	ctx, span := tracer.Start(ctx, "get latest from mongodb")
	defer span.End()
	event, err := u.repository.GetLatestStartEvent(ctx, userId, testId)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		log.Error().Err(err).Send()
		return nil, err
	}
	if err != nil && errors.Is(err, mongo.ErrNoDocuments) {
		log.Error().Err(err).Send()
		return nil, nil
	}

	result := event.CreatedAt

	//t, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	//convert := result.In(t)

	//zap.S().Info(convert.Format("15:04:05 02/01/2006"))
	return result, nil
}

func (u *answersheetUsecase) NotifyJobFail(ctx context.Context, jobId int, errFail error) error {
	jobFail := &kafka.Writer{
		Addr:                   kafka.TCP(strings.Split(u.config.GetKafkaBroker(), ",")...),
		Topic:                  "job-fail",
		AllowAutoTopicCreation: true,
		BatchSize:              1,
	}

	b := new(bytes.Buffer)
	err := json.NewEncoder(b).Encode(map[string]interface{}{
		"job_id":        jobId,
		"error_message": errFail.Error(),
	})
	if err != nil {
		log.Error().Err(err).Send()
		return err
	}

	err = jobFail.WriteMessages(ctx, kafka.Message{
		Value: b.Bytes(),
	})
	if err != nil {
		return err
	}

	return nil
}

func (u *answersheetUsecase) NotifyJobSuccess(ctx context.Context, jobId int) error {
	jobSuccess := &kafka.Writer{
		Addr:                   kafka.TCP(strings.Split(u.config.GetKafkaBroker(), ",")...),
		Topic:                  "job-success",
		AllowAutoTopicCreation: true,
		Balancer:               &kafka.LeastBytes{},
		BatchSize:              1,
	}

	b := new(bytes.Buffer)
	err := json.NewEncoder(b).Encode(map[string]interface{}{
		"job_id": jobId,
	})
	if err != nil {
		log.Error().Err(err).Send()
		return err
	}

	err = jobSuccess.WriteMessages(ctx, kafka.Message{
		Value: b.Bytes(),
	})
	if err != nil {
		return err
	}

	return nil
}

func (u *answersheetUsecase) PushToDeadLetterQueue(ctx context.Context, value []byte) error {
	w := &kafka.Writer{
		Addr:                   kafka.TCP(strings.Split(u.config.GetKafkaBroker(), ",")...),
		Topic:                  "dead-letter-queue",
		AllowAutoTopicCreation: true,
		Balancer:               &kafka.LeastBytes{},
		BatchSize:              1,
	}

	err := w.WriteMessages(ctx, kafka.Message{
		Value: value,
	})
	if err != nil {
		return err
	}

	return nil

}

func (u *answersheetUsecase) UserAnswer(ctx context.Context, input dto.UserAnswerInput) error {
	validate := validator.New()
	if err := validate.Struct(input); err != nil {
		return err
	}
	event, err := u.repository.GetLatestStartEvent(ctx, input.Payload.UserId, input.Payload.TestId)
	if err != nil {
		return err
	}
	sessionId := event.Session

	e := entities.Event{
		Id:             primitive.NewObjectID(),
		UserId:         input.Payload.UserId,
		TestId:         input.Payload.TestId,
		Event:          input.Payload.Event,
		Session:        sessionId,
		Answer:         input.Payload.Answer,
		CreatedAt:      input.Payload.CreatedAt,
		UpdatedAt:      input.Payload.UpdatedAt,
		PreviousAnswer: input.Payload.PreviousAnswer,
		QuestionId:     input.Payload.QuestionId,
	}

	if err := u.repository.Create(ctx, &e); err != nil {
		return err
	}

	return nil
}
