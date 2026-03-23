package providers

import (
	"context"

	aigatewayv1 "github.com/ideagate/aigateway-core/gen/aigateway/v1"
	platformconfig "github.com/ideagate/aigateway-core/internal/platform/config"
)

type Provider interface {
	Name() string
	SubmitBatchJob(ctx context.Context, request []*aigatewayv1.SubmitBulkChatCompletionsRequest) (*aigatewayv1.SubmitBulkChatCompletionsResponse, error)
}

func New(geminiCfg platformconfig.GeminiConfig) (Provider, error) {
	return newGoogleProvider(geminiCfg.APIKey)
}
