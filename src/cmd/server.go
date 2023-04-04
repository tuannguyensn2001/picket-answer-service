package cmd

import (
	"context"
	"fmt"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"net"
	"os"
	"os/signal"
	"picket-answersheet-service/src/config"
	"picket-answersheet-service/src/middlewares"
	"picket-answersheet-service/src/routes"
)

func server(config config.IConfig) *cobra.Command {
	return &cobra.Command{
		Use: "server",
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			server := grpc.NewServer(grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
				grpc_recovery.UnaryServerInterceptor([]grpc_recovery.Option{grpc_recovery.WithRecoveryHandlerContext(middlewares.HandleGrpcError)}...),
				otelgrpc.UnaryServerInterceptor(),
			)))

			reflection.Register(server)

			routes.Grpc(ctx, server, config)

			siginit := make(chan os.Signal, 1)
			signal.Notify(siginit, os.Interrupt, os.Kill)

			go func() {
				lis, err := net.Listen("tcp", fmt.Sprintf(":%s", config.GetGrpcPort()))
				if err != nil {
					log.Fatal().Err(err).Send()
				}
				log.Info().Str("port", config.GetGrpcPort()).Msg("server is running")
				err = server.Serve(lis)
				if err != nil {
					log.Fatal().Err(err).Send()
				}
			}()

			<-siginit
			server.GracefulStop()
			log.Info().Msg("shutdown server")

		},
	}
}
