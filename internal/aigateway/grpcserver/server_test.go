package grpcserver

import (
	"context"
	"io"
	"net"
	"testing"

	aigatewayv1 "github.com/ideagate/aigateway-core/gen/aigateway/v1"
	aigatewayusecase "github.com/ideagate/aigateway-core/internal/aigateway/usecase"
	aigatewaymock "github.com/ideagate/aigateway-core/internal/aigateway/usecase/_mock"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const testBufSize = 1024 * 1024

func TestSubmitBulkChatCompletionsDelegatesToUsecase(t *testing.T) {
	uc := aigatewaymock.NewUsecase(t)
	uc.EXPECT().
		SubmitBulkChatCompletions(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, reqs []*aigatewayv1.SubmitBulkChatCompletionsRequest) (*aigatewayv1.SubmitBulkChatCompletionsResponse, error) {
			if len(reqs) != 2 {
				t.Fatalf("expected 2 streamed requests, got %d", len(reqs))
			}
			if reqs[0].GetModel() != "gemini-2.5-flash" {
				t.Fatalf("first request model = %q, want %q", reqs[0].GetModel(), "gemini-2.5-flash")
			}
			if reqs[1].GetMetadata()["request_id"] != "req-2" {
				t.Fatalf("second request metadata request_id = %q, want %q", reqs[1].GetMetadata()["request_id"], "req-2")
			}
			return &aigatewayv1.SubmitBulkChatCompletionsResponse{JobId: "job-123"}, nil
		}).
		Once()

	conn := newTestClientConn(t, uc)
	client := aigatewayv1.NewAIGatewayServiceClient(conn)

	ctx := context.Background()
	stream, err := client.SubmitBulkChatCompletions(ctx)
	if err != nil {
		t.Fatalf("SubmitBulkChatCompletions() error = %v", err)
	}

	for _, req := range []*aigatewayv1.SubmitBulkChatCompletionsRequest{
		{
			Model: "gemini-2.5-flash",
			Content: &aigatewayv1.Content{
				Role: "user",
				Text: "hello",
			},
			Metadata: map[string]string{"request_id": "req-1"},
		},
		{
			Model: "gemini-2.5-flash",
			Content: &aigatewayv1.Content{
				Role: "user",
				Text: "world",
			},
			Metadata: map[string]string{"request_id": "req-2"},
		},
	} {
		if err := stream.Send(req); err != nil {
			t.Fatalf("stream.Send() error = %v", err)
		}
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		t.Fatalf("stream.CloseAndRecv() error = %v", err)
	}
	if resp.GetJobId() != "job-123" {
		t.Fatalf("job_id = %q, want %q", resp.GetJobId(), "job-123")
	}
}

func TestGetJobStatusDelegatesToUsecase(t *testing.T) {
	uc := aigatewaymock.NewUsecase(t)
	uc.EXPECT().
		GetJobStatus(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, req *aigatewayv1.GetJobStatusRequest) (*aigatewayv1.GetJobStatusResponse, error) {
			if req.GetJobId() != "job-456" {
				t.Fatalf("job_id = %q, want %q", req.GetJobId(), "job-456")
			}
			return &aigatewayv1.GetJobStatusResponse{
				JobId:  req.GetJobId(),
				Status: aigatewayv1.JobStatus_JOB_STATUS_IN_PROGRESS,
			}, nil
		}).
		Once()

	conn := newTestClientConn(t, uc)
	client := aigatewayv1.NewAIGatewayServiceClient(conn)

	resp, err := client.GetJobStatus(context.Background(), &aigatewayv1.GetJobStatusRequest{JobId: "job-456"})
	if err != nil {
		t.Fatalf("GetJobStatus() error = %v", err)
	}
	if resp.GetStatus() != aigatewayv1.JobStatus_JOB_STATUS_IN_PROGRESS {
		t.Fatalf("status = %v, want %v", resp.GetStatus(), aigatewayv1.JobStatus_JOB_STATUS_IN_PROGRESS)
	}
}

func TestGetJobResultsDelegatesToUsecase(t *testing.T) {
	uc := aigatewaymock.NewUsecase(t)
	uc.EXPECT().
		GetJobResults(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, req *aigatewayv1.GetJobResultsRequest) ([]*aigatewayv1.GetJobResultsResponse, error) {
			if req.GetJobId() != "job-789" {
				t.Fatalf("job_id = %q, want %q", req.GetJobId(), "job-789")
			}
			return []*aigatewayv1.GetJobResultsResponse{
				{
					Model: "gemini-2.5-flash",
					Content: &aigatewayv1.Content{
						Role: "assistant",
						Text: "first result",
					},
					Metadata: map[string]string{"request_id": "req-1"},
				},
				{
					Model: "gemini-2.5-flash",
					Content: &aigatewayv1.Content{
						Role: "assistant",
						Text: "second result",
					},
					Metadata: map[string]string{"request_id": "req-2"},
				},
			}, nil
		}).
		Once()

	conn := newTestClientConn(t, uc)
	client := aigatewayv1.NewAIGatewayServiceClient(conn)

	stream, err := client.GetJobResults(context.Background(), &aigatewayv1.GetJobResultsRequest{JobId: "job-789"})
	if err != nil {
		t.Fatalf("GetJobResults() error = %v", err)
	}

	var got []string
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("stream.Recv() error = %v", err)
		}
		got = append(got, resp.GetContent().GetText())
	}

	want := []string{"first result", "second result"}
	if len(got) != len(want) {
		t.Fatalf("received %d results, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("result %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestNewPanicsWithNilUsecase(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for nil usecase")
		}
	}()

	_ = New(nil)
}

func newTestClientConn(t *testing.T, uc aigatewayusecase.Usecase) *grpc.ClientConn {
	t.Helper()

	listener := bufconn.Listen(testBufSize)
	server := grpc.NewServer()
	aigatewayv1.RegisterAIGatewayServiceServer(server, New(uc))

	go func() {
		if err := server.Serve(listener); err != nil {
			panic(err)
		}
	}()

	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.DialContext() error = %v", err)
	}

	t.Cleanup(func() {
		_ = conn.Close()
	})

	return conn
}
