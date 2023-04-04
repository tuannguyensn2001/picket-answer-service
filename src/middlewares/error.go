package middlewares

import (
	"context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func HandleGrpcError(ctx context.Context, p interface{}) error {
	err, ok := p.(error)
	if !ok {
		return status.Error(codes.Internal, "server has error")
	}

	return status.Error(codes.Internal, err.Error())
}
