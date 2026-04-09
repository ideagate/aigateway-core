package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	aigatewayv1 "github.com/ideagate/aigateway-core/gen/aigateway/v1"
	"github.com/ideagate/aigateway-core/internal/aigateway/models"
	models2 "github.com/ideagate/aigateway-core/models"
)

const defaultClaudeMaxTokens int64 = 8192

// ClaudeProvider implements the Provider interface using the Anthropic Claude API.
type ClaudeProvider struct {
	client    anthropic.Client
	maxTokens int64
}

func newClaudeProvider(apiKey string, maxTokens int64) (*ClaudeProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("claude api key is required")
	}
	if maxTokens <= 0 {
		maxTokens = defaultClaudeMaxTokens
	}
	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &ClaudeProvider{
		client:    client,
		maxTokens: maxTokens,
	}, nil
}

func (c *ClaudeProvider) Name() string {
	return "claude"
}

func (c *ClaudeProvider) ChatCompletion(ctx context.Context, request *models2.ChatCompletionRequest) (*models2.ChatCompletionResponse, error) {
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(request.Model),
		MaxTokens: c.maxTokens,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(request.Content.Text)),
		},
	}

	if request.SystemInstruction.Text != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: request.SystemInstruction.Text},
		}
	}

	if request.Temperature != 0 {
		params.Temperature = param.NewOpt(float64(request.Temperature))
	}

	msg, err := c.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("claude chat completion: %w", err)
	}

	text := ""
	for _, block := range msg.Content {
		if block.Type == "text" {
			text += block.Text
		}
	}

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
	for i, req := range requests {
		reqParams := anthropic.MessageBatchNewParamsRequestParams{
			Model:     anthropic.Model(req.GetModel()),
			MaxTokens: c.maxTokens,
			Messages: []anthropic.MessageParam{
				anthropic.NewUserMessage(anthropic.NewTextBlock(req.GetContent().GetText())),
			},
		}

		if req.GetSystemInstruction().GetText() != "" {
			reqParams.System = []anthropic.TextBlockParam{
				{Text: req.GetSystemInstruction().GetText()},
			}
		}

		if req.GetTemperature() != 0 {
			reqParams.Temperature = param.NewOpt(float64(req.GetTemperature()))
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
		return nil, fmt.Errorf("failed to create claude batch job: %w", err)
	}

	return &aigatewayv1.SubmitBulkChatCompletionsResponse{
		JobId: batch.ID,
	}, nil
}

// GetBatchJobStatus polls the provider once and returns the current status.
// When the batch has ended, results are streamed and JSON-serialised into
// ResultsJSON so callers can persist it directly.
func (c *ClaudeProvider) GetBatchJobStatus(ctx context.Context, referenceID string) (*BatchJobStatusResult, error) {
	batch, err := c.client.Messages.Batches.Get(ctx, referenceID)
	if err != nil {
		return nil, fmt.Errorf("get claude batch job %s: %w", referenceID, err)
	}

	result := &BatchJobStatusResult{}

	switch batch.ProcessingStatus {
	case anthropic.MessageBatchProcessingStatusEnded:
		stream := c.client.Messages.Batches.ResultsStreaming(ctx, referenceID)
		var responses []anthropic.MessageBatchIndividualResponse
		for stream.Next() {
			item := stream.Current()
			responses = append(responses, item)
			if item.Result.Type == "succeeded" {
				msg := item.Result.AsSucceeded().Message
				result.InputTokenCount += msg.Usage.InputTokens
				result.OutputTokenCount += msg.Usage.OutputTokens
				result.TotalTokenCount += msg.Usage.InputTokens + msg.Usage.OutputTokens
			}
		}
		if err := stream.Err(); err != nil {
			return nil, fmt.Errorf("stream claude batch results %s: %w", referenceID, err)
		}

		resultsJSON, err := json.Marshal(responses)
		if err != nil {
			return nil, fmt.Errorf("marshal claude batch results: %w", err)
		}
		result.ResultsJSON = resultsJSON
		result.Status = models.BatchJobStatusCompleted

	default:
		// in_progress or canceling — still in flight.
		result.Status = models.BatchJobStatusProcessing
	}

	return result, nil
}
