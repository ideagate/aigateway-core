package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	aigatewayv1 "github.com/ideagate/aigateway-core/gen/aigateway/v1"
	"google.golang.org/genai"
)

type GoogleProvider struct {
	client *genai.Client
}

func newGoogleProvider(apiKey string) (*GoogleProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("providers.gemini.api_key is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend: genai.BackendGeminiAPI,
		APIKey:  apiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("create genai client: %w", err)
	}

	return &GoogleProvider{
		client: client,
	}, nil
}

func (g *GoogleProvider) Name() string {
	return "google"
}

func (g *GoogleProvider) SubmitBatchJob(ctx context.Context, requests []*aigatewayv1.SubmitBulkChatCompletionsRequest) (*aigatewayv1.SubmitBulkChatCompletionsResponse, error) {
	if len(requests) == 0 {
		return nil, errors.New("no requests provided")
	}

	batchRequests := make([]*genai.InlinedRequest, len(requests))
	for i, request := range requests {
		batchRequests[i] = &genai.InlinedRequest{
			Config: &genai.GenerateContentConfig{},
			Model:  request.GetModel(),
			Contents: []*genai.Content{
				{
					Role: "user",
					Parts: []*genai.Part{
						{Text: request.GetContent().GetText()},
					},
				},
			},
		}

		if request.GetTemperature() != 0 {
			batchRequests[i].Config.Temperature = genai.Ptr(request.GetTemperature())
		}

		if request.GetSystemInstruction().GetText() != "" {
			batchRequests[i].Config.SystemInstruction = &genai.Content{
				Parts: []*genai.Part{{Text: request.GetSystemInstruction().GetText()}},
			}
		}

		if request.GetJsonSchemaResponse() != "" {
			genAIResponseSchema := genai.Schema{}
			if err := json.Unmarshal([]byte(request.GetJsonSchemaResponse()), &genAIResponseSchema); err != nil {
				return nil, fmt.Errorf("failed to unmarshal json schema: %w", err)
			}
			batchRequests[i].Config.ResponseJsonSchema = genAIResponseSchema
		}

		if request.GetMetadata() != nil {
			batchRequests[i].Metadata = request.GetMetadata()
		}
	}

	job, err := g.client.Batches.Create(
		ctx,
		"",
		&genai.BatchJobSource{InlinedRequests: batchRequests},
		&genai.CreateBatchJobConfig{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to submit batch job: %w", err)
	}

	return &aigatewayv1.SubmitBulkChatCompletionsResponse{
		JobId: job.DisplayName,
	}, nil
}
