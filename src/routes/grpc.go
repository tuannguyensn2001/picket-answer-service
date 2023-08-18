package routes

import (
	"context"
	"google.golang.org/grpc"
	"picket-answersheet-service/src/config"
	"picket-answersheet-service/src/internal/repository"
	"picket-answersheet-service/src/internal/transport"
	"picket-answersheet-service/src/internal/usecase"
	answersheetpb "picket-answersheet-service/src/pb/answer_sheet"
)

func Grpc(ctx context.Context, s *grpc.Server, config config.IConfig) {

	testRepository := repository.NewTestRepository(config.GetMongo())
	testUsecase := usecase.NewTestUsecase(testRepository)
	transport.NewTestTransport(ctx, testUsecase, config)

	answersheetRepository := repository.NewAnswersheetRepository(config.GetMongo())
	answersheetUsecase := usecase.NewAnswersheetUsecase(answersheetRepository, config, testUsecase)
	answersheetTransport := transport.NewAnswerSheetTransport(ctx, answersheetUsecase, config)

	answersheetpb.RegisterAnswerSheetServiceServer(s, answersheetTransport)
}
