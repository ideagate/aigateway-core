package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	aigatewayv1 "github.com/ideagate/aigateway-core/gen/aigateway/v1"
	"github.com/ideagate/aigateway-core/internal/aigateway/models"
	models2 "github.com/ideagate/aigateway-core/models"
)

const claudeDefaultMaxTokens = 4096

type ClaudeProvider struct {
	client anthropic.Client
}

func newClaudeProvider(apiKey string) (*ClaudeProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("claude api key is required")
	}

	return &ClaudeProvider{
		client: anthropic.NewClient(option.WithAPIKey(apiKey)),
	}, nil
}

func (c *ClaudeProvider) Name() string {
	return "claude"
}

func (c *ClaudeProvider) ChatCompletion(ctx context.Context, request *models2.ChatCompletionRequest) (*models2.ChatCompletionResponse, error) {
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(request.Model),
		MaxTokens: claudeDefaultMaxTokens,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(request.Content.Text)),
		},
	}

	if request.Temperature != 0 {
		params.Temperature = param.NewOpt(float64(request.Temperature))
	}

	if request.SystemInstruction.Text != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: request.SystemInstruction.Text},
		}
	}

	msg, err := c.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("create message: %w", err)
	}

	text := extractTextFromContent(msg.Content)

	return &models2.ChatCompletionResponse{
		Content: models2.Content{
			Text: text,
		},
	}, nil
}

func (c *ClaudeProvider) SubmitBatchJob(ctx context.Context, requests []*aigatewayv1.SubmitBulkChatCompletionsRequest) (*aigatewayv1.SubmitBulkChatCompletionsResponse, error) {
	if len(requests) == 0 {
		return nil, errors.New("no requests provided")
	}

	batchRequests := make([]anthropic.MessageBatchNewParamsRequest, len(requests))
	for i, request := range requests {
		reqParams := anthropic.MessageBatchNewParamsRequestParams{
			Model:     anthropic.Model(request.GetModel()),
			MaxTokens: claudeDefaultMaxTokens,
			Messages: []anthropic.MessageParam{
				anthropic.NewUserMessage(anthropic.NewTextBlock(request.GetContent().GetText())),
			},
		}

		if request.GetTemperature() != 0 {
			reqParams.Temperature = param.NewOpt(float64(request.GetTemperature()))
		}

		if request.GetSystemInstruction().GetText() != "" {
			reqParams.System = []anthropic.TextBlockParam{
				{Text: request.GetSystemInstruction().GetText()},
			}
		}

		batchRequests[i] = anthropic.MessageBatchNewParamsRequest{
			CustomID: fmt.Sprintf("%d", i),
			Params:   reqParams,
		}
	}

	batch, err := c.client.Messages.Batches.New(ctx, anthropic.MessageBatchNewParams{
		Requests: batchRequests,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create batch job: %w", err)
	}

	return &aigatewayv1.SubmitBulkChatCompletionsResponse{
		JobId: batch.ID,
	}, nil
}

// GetBatchJobStatus polls the provider once and returns the current status.
// On completion the full individual responses slice is JSON-serialised into
// ResultsJSON so callers can persist it directly.
func (c *ClaudeProvider) GetBatchJobStatus(ctx context.Context, referenceID string) (*BatchJobStatusResult, error) {
	batch, err := c.client.Messages.Batches.Get(ctx, referenceID)
	if err != nil {
		return nil, fmt.Errorf("get batch job %s: %w", referenceID, err)
	}

	result := &BatchJobStatusResult{}

	switch batch.ProcessingStatus {
	case anthropic.MessageBatchProcessingStatusEnded:
		// Collect all individual results by streaming the results file.
		stream := c.client.Messages.Batches.ResultsStreaming(ctx, referenceID)
		var responses []anthropic.MessageBatchIndividualResponse
		for stream.Next() {
			responses = append(responses, stream.Current())
		}
		if err := stream.Err(); err != nil {
			return nil, fmt.Errorf("stream batch results %s: %w", referenceID, err)
		}

		// Determine overall status and aggregate token counts.
		// Batches with at least one succeeded request are marked completed,
		// matching the GoogleProvider behaviour for partial success. Callers
		// can inspect the individual ResultsJSON entries for per-request errors.
		succeeded := batch.RequestCounts.Succeeded
		if succeeded == 0 {
			result.Status = models.BatchJobStatusFailed
		} else {
			result.Status = models.BatchJobStatusCompleted
		}

		for _, r := range responses {
			if r.Result.Type == "succeeded" {
				usage := r.Result.Message.Usage
				result.InputTokenCount += usage.InputTokens
				result.OutputTokenCount += usage.OutputTokens
				result.TotalTokenCount += usage.InputTokens + usage.OutputTokens
			}
		}

		resultsJSON, err := json.Marshal(responses)
		if err != nil {
			return nil, fmt.Errorf("marshal batch results: %w", err)
		}
		result.ResultsJSON = resultsJSON

	case anthropic.MessageBatchProcessingStatusCanceling:
		result.Status = models.BatchJobStatusProcessing

	default:
		// in_progress — still in flight.
		result.Status = models.BatchJobStatusProcessing
	}

	return result, nil
}

// extractTextFromContent returns the concatenated text from all text-type
// content blocks in the message response.
func extractTextFromContent(content []anthropic.ContentBlockUnion) string {
	var text string
	for _, block := range content {
		if block.Type == "text" {
			text += block.Text
		}
	}
	return text
}
