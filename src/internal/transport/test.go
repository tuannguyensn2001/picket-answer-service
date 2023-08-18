package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/avast/retry-go"
	"github.com/rs/zerolog/log"
	"github.com/segmentio/kafka-go"
	"picket-answersheet-service/src/config"
	"picket-answersheet-service/src/internal/entities"
	"strings"
)

type TestTransport struct {
	config      config.IConfig
	testUsecase ITestUsecase
}

type ITestUsecase interface {
	SyncTest(ctx context.Context, test *entities.Test) error
}

func NewTestTransport(ctx context.Context, testUsecase ITestUsecase, config config.IConfig) *TestTransport {

	t := &TestTransport{
		testUsecase: testUsecase,
		config:      config,
	}

	go t.SyncTest(ctx)

	return t
}

func (t *TestTransport) SyncTest(ctx context.Context) {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: strings.Split(t.config.GetKafkaBroker(), ","),
		Topic:   "sync-test",
		GroupID: "sync-test-1",
		//Logger:  kafka.LoggerFunc(log.Debug().Msgf),
	})
	defer func() {
		if err := r.Close(); err != nil {
			log.Error().Err(err).Send()
		}
	}()
	for {
		m, err := r.ReadMessage(ctx)
		if err != nil {
			log.Ctx(ctx).Error().Err(err).Send()
			continue
		}
		var test entities.Test
		if err := json.NewDecoder(bytes.NewBuffer(m.Value)).Decode(&test); err != nil {
			log.Ctx(ctx).Error().Err(err).Send()
			continue
		}
		log.Info().Interface("test", test).Str("topic", "sync-test").Send()
		err = retry.Do(func() error {
			return t.testUsecase.SyncTest(ctx, &test)
		})
		if err != nil {
			log.Error().Err(err).Send()
		}
	}
}
