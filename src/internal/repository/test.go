package repository

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"picket-answersheet-service/src/internal/entities"
)

type testRepository struct {
	mongo *mongo.Client
}

func NewTestRepository(mongo *mongo.Client) *testRepository {
	return &testRepository{mongo: mongo}
}

func (r *testRepository) Create(ctx context.Context, test *entities.Test) error {
	collection := r.mongo.Database("picket").Collection("tests")

	_, err := collection.InsertOne(ctx, test)
	if err != nil {
		return err
	}

	return nil
}

func (r *testRepository) FindByTestId(ctx context.Context, testId int) (*entities.Test, error) {
	filter := bson.M{
		"test_id": testId,
	}
	result := r.mongo.Database("picket").Collection("tests").FindOne(ctx, filter)
	if result.Err() != nil {
		return nil, result.Err()
	}
	var test entities.Test
	if err := result.Decode(&test); err != nil {
		return nil, err
	}
	return &test, nil

}
