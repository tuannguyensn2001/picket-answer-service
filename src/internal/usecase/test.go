package usecase

import (
	"context"
	"errors"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"picket-answersheet-service/src/internal/entities"
)

type ITestRepository interface {
	FindByTestId(ctx context.Context, testId int) (*entities.Test, error)
	Create(ctx context.Context, test *entities.Test) error
}

type testUsecase struct {
	repository ITestRepository
}

func NewTestUsecase(repository ITestRepository) *testUsecase {
	return &testUsecase{repository: repository}
}

func (u *testUsecase) SyncTest(ctx context.Context, test *entities.Test) error {
	check, err := u.repository.FindByTestId(ctx, test.TestId)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		log.Error().Err(err).Send()
		return err
	}
	if check != nil {
		log.Info().Int("test_id", test.TestId).Send()
		return nil
	}

	test.Id = primitive.NewObjectID()
	err = u.repository.Create(ctx, test)
	if err != nil {
		log.Error().Err(err).Send()
	}

	return nil
}

func (u *testUsecase) GetByTestId(ctx context.Context, testId int) (*entities.Test, error) {
	result, err := u.repository.FindByTestId(ctx, testId)
	if err != nil {
		log.Error().Err(err).Send()
		return nil, err
	}
	return result, nil
}
