package config

import (
	"context"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type config struct {
	appGrpcPort string
	mongo       *mongo.Client
	kafkaBroker string
}

func GetConfig() (*config, error) {
	structure := bootstrap()

	mongo, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(structure.DatabaseUrl))
	if err != nil {
		log.Error().Err(err).Send()
		return nil, err
	}

	result := config{
		appGrpcPort: structure.AppGrpcPort,
		mongo:       mongo,
		kafkaBroker: structure.KafkaBroker,
	}

	return &result, nil
}
