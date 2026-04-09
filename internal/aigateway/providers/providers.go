package providers

import (
	"context"

	aigatewayv1 "github.com/ideagate/aigateway-core/gen/aigateway/v1"
	platformconfig "github.com/ideagate/aigateway-core/internal/platform/config"
	"github.com/ideagate/aigateway-core/models"
)

// BatchJobStatusResult carries the current status of a provider-side batch job
// and, on terminal success, the raw JSON-serialised responses and token counts.
type BatchJobStatusResult struct {
	// Status is one of the BatchJobStatus* constants from the models package.
	Status string
	// ResultsJSON holds the serialised responses; non-nil only on completion.
	ResultsJSON []byte
	// Token counts aggregated across all responses; non-zero only on completion.
	InputTokenCount  int64
	OutputTokenCount int64
	TotalTokenCount  int64
}

type Provider interface {
	Name() string
	ChatCompletion(ctx context.Context, request *models.ChatCompletionRequest) (*models.ChatCompletionResponse, error)
	SubmitBatchJob(ctx context.Context, request []*aigatewayv1.SubmitBulkChatCompletionsRequest) (*aigatewayv1.SubmitBulkChatCompletionsResponse, error)
	// GetBatchJobStatus fetches the current status of a previously submitted batch
	// job from the provider. referenceID is the value stored in BatchJob.ReferenceID.
	GetBatchJobStatus(ctx context.Context, referenceID string) (*BatchJobStatusResult, error)
}

func New(geminiCfg platformconfig.GeminiConfig) (Provider, error) {
	return newGoogleProvider(geminiCfg.APIKey)
}

func NewClaude(claudeCfg platformconfig.ClaudeConfig) (Provider, error) {
	return newClaudeProvider(claudeCfg.APIKey, claudeCfg.MaxTokens)
}
