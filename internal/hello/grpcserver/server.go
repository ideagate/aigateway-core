package grpcserver

import (
	"context"

	hellov1 "github.com/ideagate/aigateway-core/gen/hello/v1"
	hellousecase "github.com/ideagate/aigateway-core/internal/hello/usecase"
)

// Server adapts the Hello gRPC contract to the application usecase layer.
type Server struct {
	hellov1.UnimplementedHelloServiceServer
	usecase hellousecase.Usecase
}

// New creates a new Hello gRPC server.
func New(usecase hellousecase.Usecase) *Server {
	if usecase == nil {
		panic("hello usecase is nil")
	}

	return &Server{usecase: usecase}
}

// SayHello delegates the request to the usecase layer.
func (s *Server) SayHello(ctx context.Context, req *hellov1.SayHelloRequest) (*hellov1.SayHelloResponse, error) {
	resp, err := s.usecase.SayHello(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		resp = &hellov1.SayHelloResponse{}
	}

	return resp, nil
}
