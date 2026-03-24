package usecase

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"io"
	"testing"

	aigatewayv1 "github.com/ideagate/aigateway-core/gen/aigateway/v1"
	"github.com/ideagate/aigateway-core/internal/aigateway/models"
	"github.com/ideagate/aigateway-core/internal/aigateway/providers"
	providermock "github.com/ideagate/aigateway-core/internal/aigateway/providers/_mock"
	"github.com/ideagate/aigateway-core/internal/aigateway/repository"
	repomock "github.com/ideagate/aigateway-core/internal/aigateway/repository/_mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	lock := repomock.NewDistributionLock(t)
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

	uc := New(provider, repo, lock)
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
	lock := repomock.NewDistributionLock(t)

	uc := New(provider, repo, lock)
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
	lock := repomock.NewDistributionLock(t)
	repo.EXPECT().
		CreateBatchJob(mock.Anything, mock.Anything).
		Return(errors.New("db write error")).
		Once()

	uc := New(provider, repo, lock)
	_, err := uc.SubmitBulkChatCompletions(context.Background(), testRequests)
	require.Error(t, err)
}

func TestSubmitBulkChatCompletions_FillsEmptyFieldsFromTemplate(t *testing.T) {
	const providerJobID = "prov-job-template-1"

	requests := []*aigatewayv1.SubmitBulkChatCompletionsRequest{
		{
			Content:    &aigatewayv1.Content{Role: "user", Text: "hello"},
			Metadata:   map[string]string{"request_id": "r1", "shared": "request"},
			TemplateId: "tmpl-1",
		},
	}

	provider := providermock.NewProvider(t)
	provider.EXPECT().Name().Return("google")
	provider.EXPECT().
		SubmitBatchJob(mock.Anything, mock.MatchedBy(func(reqs []*aigatewayv1.SubmitBulkChatCompletionsRequest) bool {
			require.Len(t, reqs, 1)
			req := reqs[0]
			assert.Equal(t, "gemini-2.5-flash", req.GetModel())
			assert.Equal(t, "config instruction", req.GetSystemInstruction().GetText())
			assert.InDelta(t, 0.7, req.GetTemperature(), 0.0001)
			assert.Equal(t, `{"type":"object"}`, req.GetJsonSchemaResponse())
			assert.Equal(t, map[string]string{
				"config_only": "yes",
				"request_id":  "r1",
				"shared":      "request",
			}, req.GetMetadata())
			return true
		})).
		Return(&aigatewayv1.SubmitBulkChatCompletionsResponse{JobId: providerJobID}, nil).
		Once()

	repo := repomock.NewRepository(t)
	lock := repomock.NewDistributionLock(t)
	repo.EXPECT().
		GetPromptConfig(mock.Anything, "tmpl-1").
		Return(&models.PromptConfig{
			ID:                 "tmpl-1",
			Model:              sql.NullString{String: "gemini-2.5-flash", Valid: true},
			SystemInstruction:  sql.NullString{String: "config instruction", Valid: true},
			Temperature:        sql.NullFloat64{Float64: 0.7, Valid: true},
			JSONSchemaResponse: sql.NullString{String: `{"type":"object"}`, Valid: true},
			Metadata:           encodeMetadata(map[string]string{"config_only": "yes", "shared": "config"}),
		}, nil).
		Once()
	repo.EXPECT().
		CreateBatchJob(mock.Anything, mock.MatchedBy(func(job *models.BatchJob) bool {
			decoded := decodeRequests(t, job.RequestProto)
			require.Len(t, decoded, 1)
			assert.Equal(t, "gemini-2.5-flash", decoded[0].GetModel())
			assert.Equal(t, "config instruction", decoded[0].GetSystemInstruction().GetText())
			assert.InDelta(t, 0.7, decoded[0].GetTemperature(), 0.0001)
			assert.Equal(t, `{"type":"object"}`, decoded[0].GetJsonSchemaResponse())
			assert.Equal(t, map[string]string{
				"config_only": "yes",
				"request_id":  "r1",
				"shared":      "request",
			}, decoded[0].GetMetadata())
			return true
		})).
		Return(nil).
		Once()

	uc := New(provider, repo, lock)
	resp, err := uc.SubmitBulkChatCompletions(context.Background(), requests)
	require.NoError(t, err)
	assert.Equal(t, providerJobID, resp.GetJobId())

	assert.Empty(t, requests[0].GetModel())
	assert.Empty(t, requests[0].GetSystemInstruction().GetText())
	assert.Zero(t, requests[0].GetTemperature())
	assert.Empty(t, requests[0].GetJsonSchemaResponse())
	assert.Equal(t, map[string]string{"request_id": "r1", "shared": "request"}, requests[0].GetMetadata())
}

func TestSubmitBulkChatCompletions_DoesNotOverrideNonEmptyFields(t *testing.T) {
	const providerJobID = "prov-job-template-2"

	requests := []*aigatewayv1.SubmitBulkChatCompletionsRequest{
		{
			Model:              "request-model",
			Content:            &aigatewayv1.Content{Role: "user", Text: "hello"},
			SystemInstruction:  &aigatewayv1.Content{Role: "system", Text: "request instruction"},
			Temperature:        0.2,
			JsonSchemaResponse: `{"type":"array"}`,
			Metadata:           map[string]string{"shared": "request", "request_only": "yes"},
			TemplateId:         "tmpl-1",
		},
	}

	provider := providermock.NewProvider(t)
	provider.EXPECT().Name().Return("google")
	provider.EXPECT().
		SubmitBatchJob(mock.Anything, mock.MatchedBy(func(reqs []*aigatewayv1.SubmitBulkChatCompletionsRequest) bool {
			require.Len(t, reqs, 1)
			req := reqs[0]
			assert.Equal(t, "request-model", req.GetModel())
			assert.Equal(t, "request instruction", req.GetSystemInstruction().GetText())
			assert.InDelta(t, 0.2, req.GetTemperature(), 0.0001)
			assert.Equal(t, `{"type":"array"}`, req.GetJsonSchemaResponse())
			assert.Equal(t, map[string]string{
				"config_only":  "yes",
				"request_only": "yes",
				"shared":       "request",
			}, req.GetMetadata())
			return true
		})).
		Return(&aigatewayv1.SubmitBulkChatCompletionsResponse{JobId: providerJobID}, nil).
		Once()

	repo := repomock.NewRepository(t)
	lock := repomock.NewDistributionLock(t)
	repo.EXPECT().
		GetPromptConfig(mock.Anything, "tmpl-1").
		Return(&models.PromptConfig{
			ID:                 "tmpl-1",
			Model:              sql.NullString{String: "config-model", Valid: true},
			SystemInstruction:  sql.NullString{String: "config instruction", Valid: true},
			Temperature:        sql.NullFloat64{Float64: 0.7, Valid: true},
			JSONSchemaResponse: sql.NullString{String: `{"type":"object"}`, Valid: true},
			Metadata:           encodeMetadata(map[string]string{"config_only": "yes", "shared": "config"}),
		}, nil).
		Once()
	repo.EXPECT().CreateBatchJob(mock.Anything, mock.Anything).Return(nil).Once()

	uc := New(provider, repo, lock)
	resp, err := uc.SubmitBulkChatCompletions(context.Background(), requests)
	require.NoError(t, err)
	assert.Equal(t, providerJobID, resp.GetJobId())
}

func TestSubmitBulkChatCompletions_ReusesPromptConfigLookup(t *testing.T) {
	const providerJobID = "prov-job-template-3"

	requests := []*aigatewayv1.SubmitBulkChatCompletionsRequest{
		{Content: &aigatewayv1.Content{Role: "user", Text: "hello"}, TemplateId: "tmpl-1"},
		{Content: &aigatewayv1.Content{Role: "user", Text: "world"}, TemplateId: "tmpl-1"},
	}

	provider := providermock.NewProvider(t)
	provider.EXPECT().Name().Return("google")
	provider.EXPECT().
		SubmitBatchJob(mock.Anything, mock.MatchedBy(func(reqs []*aigatewayv1.SubmitBulkChatCompletionsRequest) bool {
			require.Len(t, reqs, 2)
			assert.Equal(t, "config-model", reqs[0].GetModel())
			assert.Equal(t, "config-model", reqs[1].GetModel())
			return true
		})).
		Return(&aigatewayv1.SubmitBulkChatCompletionsResponse{JobId: providerJobID}, nil).
		Once()

	repo := repomock.NewRepository(t)
	lock := repomock.NewDistributionLock(t)
	repo.EXPECT().
		GetPromptConfig(mock.Anything, "tmpl-1").
		Return(&models.PromptConfig{ID: "tmpl-1", Model: sql.NullString{String: "config-model", Valid: true}}, nil).
		Once()
	repo.EXPECT().CreateBatchJob(mock.Anything, mock.Anything).Return(nil).Once()

	uc := New(provider, repo, lock)
	resp, err := uc.SubmitBulkChatCompletions(context.Background(), requests)
	require.NoError(t, err)
	assert.Equal(t, providerJobID, resp.GetJobId())
}

func TestSubmitBulkChatCompletions_TemplateNotFound(t *testing.T) {
	provider := providermock.NewProvider(t)
	repo := repomock.NewRepository(t)
	lock := repomock.NewDistributionLock(t)

	repo.EXPECT().
		GetPromptConfig(mock.Anything, "missing-template").
		Return(nil, repository.ErrNotFound).
		Once()

	uc := New(provider, repo, lock)
	_, err := uc.SubmitBulkChatCompletions(context.Background(), []*aigatewayv1.SubmitBulkChatCompletionsRequest{{
		Content:    &aigatewayv1.Content{Role: "user", Text: "hello"},
		TemplateId: "missing-template",
	}})
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestUpsertPromptConfig_PersistsMetadata(t *testing.T) {
	provider := providermock.NewProvider(t)
	repo := repomock.NewRepository(t)
	lock := repomock.NewDistributionLock(t)

	repo.EXPECT().
		UpsertPromptConfig(mock.Anything, mock.MatchedBy(func(cfg *models.PromptConfig) bool {
			metadata, err := decodeMetadata(cfg.Metadata)
			require.NoError(t, err)
			assert.Equal(t, map[string]string{"tenant": "alpha", "usecase": "submit"}, metadata)
			return true
		})).
		Return(nil).
		Once()

	uc := New(provider, repo, lock)
	resp, err := uc.UpsertPromptConfig(context.Background(), &aigatewayv1.UpsertPromptConfigRequest{
		Id:       "tmpl-1",
		Metadata: map[string]string{"tenant": "alpha", "usecase": "submit"},
	})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"tenant": "alpha", "usecase": "submit"}, resp.GetPromptConfig().GetMetadata())
}

// ── buildTokenJobs ───────────────────────────────────────────────────────────

func TestBuildTokenJobs(t *testing.T) {
	tests := []struct {
		name       string
		jobType    string
		jobID      string
		input      int64
		output     int64
		total      int64
		wantTypes  []string
		wantCounts []int64
	}{
		{
			name:       "all non-zero",
			jobType:    models.TokenJobTypeBatchJob,
			jobID:      "job-1",
			input:      194,
			output:     469,
			total:      663,
			wantTypes:  []string{models.TokenTypeInput, models.TokenTypeOutput, models.TokenTypeTotal},
			wantCounts: []int64{194, 469, 663},
		},
		{
			name:       "zero values omitted",
			jobType:    models.TokenJobTypeBatchJob,
			jobID:      "job-2",
			input:      0,
			output:     0,
			total:      0,
			wantTypes:  nil,
			wantCounts: nil,
		},
		{
			name:       "partial non-zero",
			jobType:    models.TokenJobTypeBatchJob,
			jobID:      "job-3",
			input:      100,
			output:     0,
			total:      100,
			wantTypes:  []string{models.TokenTypeInput, models.TokenTypeTotal},
			wantCounts: []int64{100, 100},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildTokenJobs(tc.jobType, tc.jobID, tc.input, tc.output, tc.total)
			require.Len(t, got, len(tc.wantTypes))
			for i, tj := range got {
				assert.Equal(t, tc.jobType, tj.JobType)
				assert.Equal(t, tc.jobID, tj.JobID)
				assert.Equal(t, tc.wantTypes[i], tj.TokenType)
				assert.Equal(t, tc.wantCounts[i], tj.TokenCount)
			}
		})
	}
}

// ── SyncBatchJobStatus ───────────────────────────────────────────────────────

func TestSyncBatchJobStatus_SavesTokenJobsOnCompletion(t *testing.T) {
	const jobID = "job-abc"
	const refID = "ref-xyz"

	providerResult := &providers.BatchJobStatusResult{
		Status:           models.BatchJobStatusCompleted,
		ResultsJSON:      []byte(`[{"response":{"text":"ok"}}]`),
		InputTokenCount:  194,
		OutputTokenCount: 469,
		TotalTokenCount:  663,
	}

	provider := providermock.NewProvider(t)
	provider.EXPECT().
		GetBatchJobStatus(mock.Anything, refID).
		Return(providerResult, nil).
		Once()

	repo := repomock.NewRepository(t)
	lock := repomock.NewDistributionLock(t)

	lock.EXPECT().
		Acquire(mock.Anything, lockKeyPrefix+jobID, lockTTL).
		Return(true, nil).
		Once()
	lock.EXPECT().
		Release(mock.Anything, lockKeyPrefix+jobID).
		Return(nil).
		Once()

	repo.EXPECT().
		UpdateBatchJob(mock.Anything, mock.Anything).
		Return(nil).
		Once()

	repo.EXPECT().
		UpsertTokenJobs(mock.Anything, mock.MatchedBy(func(jobs []*models.TokenJob) bool {
			if !assert.Len(t, jobs, 3) {
				return false
			}
			byType := make(map[string]int64, len(jobs))
			for _, j := range jobs {
				assert.Equal(t, models.TokenJobTypeBatchJob, j.JobType)
				assert.Equal(t, jobID, j.JobID)
				byType[j.TokenType] = j.TokenCount
			}
			assert.Equal(t, int64(194), byType[models.TokenTypeInput])
			assert.Equal(t, int64(469), byType[models.TokenTypeOutput])
			assert.Equal(t, int64(663), byType[models.TokenTypeTotal])
			return true
		})).
		Return(nil).
		Once()

	uc := New(provider, repo, lock)
	activeRef := refID
	job := &models.BatchJob{
		ID:          jobID,
		ReferenceID: &activeRef,
		Status:      models.BatchJobStatusProcessing,
	}
	err := uc.(*usecase).syncJobStatus(context.Background(), job)
	require.NoError(t, err)
}

func TestSyncBatchJobStatus_NoTokenJobsOnFailure(t *testing.T) {
	const jobID = "job-fail"
	const refID = "ref-fail"

	provider := providermock.NewProvider(t)
	provider.EXPECT().
		GetBatchJobStatus(mock.Anything, refID).
		Return(&providers.BatchJobStatusResult{Status: models.BatchJobStatusFailed}, nil).
		Once()

	repo := repomock.NewRepository(t)
	lock := repomock.NewDistributionLock(t)

	lock.EXPECT().
		Acquire(mock.Anything, lockKeyPrefix+jobID, lockTTL).
		Return(true, nil).
		Once()
	lock.EXPECT().
		Release(mock.Anything, lockKeyPrefix+jobID).
		Return(nil).
		Once()

	repo.EXPECT().
		UpdateBatchJob(mock.Anything, mock.Anything).
		Return(nil).
		Once()

	// UpsertTokenJobs must NOT be called for a failed job.

	uc := New(provider, repo, lock)
	activeRef := refID
	job := &models.BatchJob{
		ID:          jobID,
		ReferenceID: &activeRef,
		Status:      models.BatchJobStatusProcessing,
	}
	err := uc.(*usecase).syncJobStatus(context.Background(), job)
	require.NoError(t, err)
}

func TestSyncBatchJobStatus_TokenJobUpsertErrorIsLogged(t *testing.T) {
	const jobID = "job-token-err"
	const refID = "ref-token-err"

	providerResult := &providers.BatchJobStatusResult{
		Status:          models.BatchJobStatusCompleted,
		InputTokenCount: 10,
		TotalTokenCount: 10,
	}

	provider := providermock.NewProvider(t)
	provider.EXPECT().
		GetBatchJobStatus(mock.Anything, refID).
		Return(providerResult, nil).
		Once()

	repo := repomock.NewRepository(t)
	lock := repomock.NewDistributionLock(t)

	lock.EXPECT().
		Acquire(mock.Anything, lockKeyPrefix+jobID, lockTTL).
		Return(true, nil).
		Once()
	lock.EXPECT().
		Release(mock.Anything, lockKeyPrefix+jobID).
		Return(nil).
		Once()

	repo.EXPECT().
		UpdateBatchJob(mock.Anything, mock.Anything).
		Return(nil).
		Once()

	repo.EXPECT().
		UpsertTokenJobs(mock.Anything, mock.Anything).
		Return(errors.New("db error")).
		Once()

	// The overall syncJobStatus should still succeed (error is logged, not returned).
	uc := New(provider, repo, lock)
	activeRef := refID
	job := &models.BatchJob{
		ID:          jobID,
		ReferenceID: &activeRef,
		Status:      models.BatchJobStatusProcessing,
	}
	err := uc.(*usecase).syncJobStatus(context.Background(), job)
	require.NoError(t, err)
}
