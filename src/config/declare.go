package config

import "go.mongodb.org/mongo-driver/mongo"

type IConfig interface {
	GetGrpcPort() string
	GetMongo() *mongo.Client
	GetKafkaBroker() string
}

func (c config) GetGrpcPort() string {
	return c.appGrpcPort
}

func (c config) GetMongo() *mongo.Client {
	return c.mongo
}

func (c config) GetKafkaBroker() string {
	return c.kafkaBroker
}
