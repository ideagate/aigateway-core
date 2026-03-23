package grpcserver

import (
	"context"
	"io"

	aigatewayv1 "github.com/ideagate/aigateway-core/gen/aigateway/v1"
	aigatewayusecase "github.com/ideagate/aigateway-core/internal/aigateway/usecase"
	"google.golang.org/grpc"
)

// Server adapts the AIGateway gRPC contract to the application usecase layer.
type Server struct {
	aigatewayv1.UnimplementedAIGatewayServiceServer
	usecase aigatewayusecase.Usecase
}

// New creates a new AIGateway gRPC server.
func New(usecase aigatewayusecase.Usecase) *Server {
	if usecase == nil {
		panic("aigateway usecase is nil")
	}

	return &Server{usecase: usecase}
}

// SubmitBulkChatCompletions receives the full client stream, then delegates to the usecase layer.
func (s *Server) SubmitBulkChatCompletions(stream grpc.ClientStreamingServer[aigatewayv1.SubmitBulkChatCompletionsRequest, aigatewayv1.SubmitBulkChatCompletionsResponse]) error {
	requests, err := recvSubmitBulkChatCompletionsRequests(stream)
	if err != nil {
		return err
	}

	resp, err := s.usecase.SubmitBulkChatCompletions(stream.Context(), requests)
	if err != nil {
		return err
	}
	if resp == nil {
		resp = &aigatewayv1.SubmitBulkChatCompletionsResponse{}
	}

	return stream.SendAndClose(resp)
}

// GetJobStatus delegates the unary request to the usecase layer.
func (s *Server) GetJobStatus(ctx context.Context, req *aigatewayv1.GetJobStatusRequest) (*aigatewayv1.GetJobStatusResponse, error) {
	resp, err := s.usecase.GetJobStatus(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		resp = &aigatewayv1.GetJobStatusResponse{}
	}

	return resp, nil
}

// GetJobResults delegates the request to the usecase layer and streams the results back to the caller.
func (s *Server) GetJobResults(req *aigatewayv1.GetJobResultsRequest, stream grpc.ServerStreamingServer[aigatewayv1.GetJobResultsResponse]) error {
	results, err := s.usecase.GetJobResults(stream.Context(), req)
	if err != nil {
		return err
	}

	for _, result := range results {
		if result == nil {
			continue
		}
		if err := stream.Send(result); err != nil {
			return err
		}
	}

	return nil
}

func recvSubmitBulkChatCompletionsRequests(stream grpc.ClientStreamingServer[aigatewayv1.SubmitBulkChatCompletionsRequest, aigatewayv1.SubmitBulkChatCompletionsResponse]) ([]*aigatewayv1.SubmitBulkChatCompletionsRequest, error) {
	var requests []*aigatewayv1.SubmitBulkChatCompletionsRequest
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return requests, nil
		}
		if err != nil {
			return nil, err
		}
		requests = append(requests, req)
	}
}
