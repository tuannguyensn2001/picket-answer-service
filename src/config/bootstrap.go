package config

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"os"
)

type structure struct {
	AppGrpcPort string `mapstructure:"APP_GRPC_PORT"`
	DatabaseUrl string `mapstructure:"DATABASE_URL"`
	KafkaBroker string `mapstructure:"KAFKA_BROKER"`
}

func bootstrap() structure {
	path, err := os.Getwd()
	if err != nil {
		log.Fatal().Err(err).Msg("config error")
	}

	viper.AddConfigPath(path)
	viper.SetConfigType("env")
	viper.AutomaticEnv()
	viper.SetConfigName(".env")

	viper.ReadInConfig()

	var result structure
	if err := viper.Unmarshal(&result); err != nil {
		log.Error().Err(err).Msg("unmarshal env")
	}

	return result

}
