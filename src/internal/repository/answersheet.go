package repository

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"picket-answersheet-service/src/internal/entities"
)

type answersheetRepository struct {
	mongo *mongo.Client
}

func NewAnswersheetRepository(mongo *mongo.Client) *answersheetRepository {
	return &answersheetRepository{mongo: mongo}
}

func (r *answersheetRepository) Create(ctx context.Context, event *entities.Event) error {
	collection := r.mongo.Database("picket").Collection("events")

	_, err := collection.InsertOne(ctx, event)
	if err != nil {
		return err
	}

	return nil
}

func (r *answersheetRepository) FindByStatusEnd(ctx context.Context, userId int, testId int) (*entities.Event, error) {
	filter := bson.M{
		"event":   entities.END,
		"user_id": userId,
		"test_id": testId,
	}
	resp := r.mongo.Database("picket").Collection("events").FindOne(ctx, filter)
	if resp.Err() != nil {
		return nil, resp.Err()
	}
	var result entities.Event
	if err := resp.Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *answersheetRepository) GetLatestEventWithLimit(ctx context.Context, userId int, testId int, limit int) ([]entities.Event, error) {
	filter := bson.D{
		{
			"$and", bson.A{
				bson.D{{"user_id", userId}},
				bson.D{{"test_id", testId}},
			},
		},
	}
	opts := options.Find().SetLimit(int64(limit)).SetSort(bson.D{{"_id", -1}})
	cursor, err := r.mongo.Database("picket").Collection("events").Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	result := make([]entities.Event, 0)

	err = cursor.All(ctx, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *answersheetRepository) GetLatestEvent(ctx context.Context, userId int, testId int) ([]entities.Event, error) {
	return r.GetLatestEventWithLimit(ctx, userId, testId, 2)
}

func (r *answersheetRepository) GetLatestStartEvent(ctx context.Context, userId int, testId int) (*entities.Event, error) {
	filter := bson.D{
		{
			"$and", bson.A{
				bson.D{{"user_id", userId}},
				bson.D{{"test_id", testId}},
				bson.D{{"event", entities.START}},
			},
		},
	}
	opts := options.FindOne().SetSort(bson.D{{"_id", -1}})
	resp := r.mongo.Database("picket").Collection("events").FindOne(ctx, filter, opts)
	if resp.Err() != nil {
		return nil, resp.Err()
	}
	var result entities.Event
	err := resp.Decode(&result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *answersheetRepository) FindAnswerByUserIdAndTestId(ctx context.Context, userId int, testId int, sessionId string) ([]entities.Event, error) {

	type Field struct {
		Id     int    `bson:"_id,omitempty"`
		Answer string `bson:"answer,omitempty"`
	}
	filter := bson.A{
		bson.M{"$match": bson.M{"user_id": userId, "test_id": testId, "event": entities.ANSWER, "session": sessionId}},
		bson.M{"$group": bson.M{
			"_id":    "$question_id",
			"answer": bson.M{"$last": "$answer"},
		}},
	}
	resp, err := r.mongo.Database("picket").Collection("events").Aggregate(ctx, filter)
	if err != nil {
		return nil, err
	}
	result := make([]Field, 0)

	for resp.Next(ctx) {
		var event Field
		err := resp.Decode(&event)
		if err != nil {
			return nil, err
		}
		result = append(result, event)
	}

	data := make([]entities.Event, len(result))

	for index, item := range result {
		data[index] = entities.Event{
			UserId:     userId,
			TestId:     testId,
			Event:      entities.ANSWER,
			QuestionId: item.Id,
			Answer:     item.Answer,
		}
	}

	return data, nil
}
