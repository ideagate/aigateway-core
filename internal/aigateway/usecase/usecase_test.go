package usecase

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"testing"

	aigatewayv1 "github.com/ideagate/aigateway-core/gen/aigateway/v1"
	"github.com/ideagate/aigateway-core/internal/aigateway/models"
	providermock "github.com/ideagate/aigateway-core/internal/aigateway/providers/_mock"
	repomock "github.com/ideagate/aigateway-core/internal/aigateway/repository/_mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// decodeRequests reverses marshalRequests: parses a length-delimited proto binary
// blob back into a slice of SubmitBulkChatCompletionsRequest.
func decodeRequests(t *testing.T, data []byte) []*aigatewayv1.SubmitBulkChatCompletionsRequest {
	t.Helper()
	var result []*aigatewayv1.SubmitBulkChatCompletionsRequest
	r := bytes.NewReader(data)
	for r.Len() > 0 {
		var length uint32
		err := binary.Read(r, binary.BigEndian, &length)
		require.NoError(t, err)

		msgBytes := make([]byte, length)
		_, err = io.ReadFull(r, msgBytes)
		require.NoError(t, err)

		req := &aigatewayv1.SubmitBulkChatCompletionsRequest{}
		err = proto.Unmarshal(msgBytes, req)
		require.NoError(t, err)

		result = append(result, req)
	}
	return result
}

// ── marshalRequests ──────────────────────────────────────────────────────────

func TestMarshalRequests_ProducesLengthDelimitedProtoBytes(t *testing.T) {
	requests := []*aigatewayv1.SubmitBulkChatCompletionsRequest{
		{Model: "gemini-2.5-flash", Content: &aigatewayv1.Content{Role: "user", Text: "hello"}},
		{Model: "gemini-2.5-pro", Content: &aigatewayv1.Content{Role: "user", Text: "world"}, Metadata: map[string]string{"request_id": "r2"}},
	}

	data, err := marshalRequests(requests)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	got := decodeRequests(t, data)

	require.Len(t, got, len(requests))
	for i, want := range requests {
		assert.Truef(t, proto.Equal(got[i], want), "request[%d]: got %v, want %v", i, got[i], want)
	}
}

func TestMarshalRequests_EmptySliceProducesEmptyBytes(t *testing.T) {
	data, err := marshalRequests(nil)
	require.NoError(t, err)
	assert.Empty(t, data)
}

// ── SubmitBulkChatCompletions ────────────────────────────────────────────────

var testRequests = []*aigatewayv1.SubmitBulkChatCompletionsRequest{
	{Model: "gemini-2.5-flash", Content: &aigatewayv1.Content{Role: "user", Text: "hello"}, Metadata: map[string]string{"request_id": "r1"}},
	{Model: "gemini-2.5-flash", Content: &aigatewayv1.Content{Role: "user", Text: "world"}, Metadata: map[string]string{"request_id": "r2"}},
}

func TestSubmitBulkChatCompletions_HappyPath(t *testing.T) {
	const providerJobID = "prov-job-abc123"

	provider := providermock.NewProvider(t)
	provider.EXPECT().Name().Return("google")
	provider.EXPECT().
		SubmitBatchJob(mock.Anything, mock.Anything).
		Return(&aigatewayv1.SubmitBulkChatCompletionsResponse{JobId: providerJobID}, nil).
		Once()

	repo := repomock.NewRepository(t)
	repo.EXPECT().
		CreateBatchJob(mock.Anything, mock.MatchedBy(func(job *models.BatchJob) bool {
			assert.Equal(t, models.BatchJobStatusPending, job.Status)
			assert.Equal(t, len(testRequests), job.LengthData)
			assert.Equal(t, "google", job.Provider)
			if assert.NotNil(t, job.ReferenceID) {
				assert.Equal(t, providerJobID, *job.ReferenceID)
			}
			assert.NotEmpty(t, job.RequestProto)

			// Verify the stored proto bytes round-trip back to the original requests.
			decoded := decodeRequests(t, job.RequestProto)
			assert.Len(t, decoded, len(testRequests))
			for i, want := range testRequests {
				assert.Truef(t, proto.Equal(decoded[i], want), "stored request[%d]: got %v, want %v", i, decoded[i], want)
			}
			return true
		})).
		Return(nil).
		Once()

	uc := New(provider, repo)
	resp, err := uc.SubmitBulkChatCompletions(context.Background(), testRequests)
	require.NoError(t, err)
	assert.Equal(t, providerJobID, resp.GetJobId())
}

func TestSubmitBulkChatCompletions_ProviderError(t *testing.T) {
	provider := providermock.NewProvider(t)
	provider.EXPECT().
		SubmitBatchJob(mock.Anything, mock.Anything).
		Return(nil, errors.New("provider unavailable")).
		Once()

	// Repository must not be called when the provider fails.
	repo := repomock.NewRepository(t)

	uc := New(provider, repo)
	_, err := uc.SubmitBulkChatCompletions(context.Background(), testRequests)
	require.Error(t, err)
}

func TestSubmitBulkChatCompletions_RepositoryError(t *testing.T) {
	const providerJobID = "job-1"

	provider := providermock.NewProvider(t)
	provider.EXPECT().Name().Return("google")
	provider.EXPECT().
		SubmitBatchJob(mock.Anything, mock.Anything).
		Return(&aigatewayv1.SubmitBulkChatCompletionsResponse{JobId: providerJobID}, nil).
		Once()

	repo := repomock.NewRepository(t)
	repo.EXPECT().
		CreateBatchJob(mock.Anything, mock.Anything).
		Return(errors.New("db write error")).
		Once()

	uc := New(provider, repo)
	_, err := uc.SubmitBulkChatCompletions(context.Background(), testRequests)
	require.Error(t, err)
}
