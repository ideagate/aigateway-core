package providers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/genai"
)

func makeUsage(prompt, candidates, total int32) *genai.GenerateContentResponseUsageMetadata {
	return &genai.GenerateContentResponseUsageMetadata{
		PromptTokenCount:     prompt,
		CandidatesTokenCount: candidates,
		TotalTokenCount:      total,
	}
}

func TestAggregateTokenCounts(t *testing.T) {
	tests := []struct {
		name           string
		responses      []*genai.InlinedResponse
		wantInput      int64
		wantOutput     int64
		wantTotal      int64
	}{
		{
			name:      "nil slice",
			responses: nil,
		},
		{
			name:      "empty slice",
			responses: []*genai.InlinedResponse{},
		},
		{
			name: "single response",
			responses: []*genai.InlinedResponse{
				{Response: &genai.GenerateContentResponse{UsageMetadata: makeUsage(194, 469, 663)}},
			},
			wantInput:  194,
			wantOutput: 469,
			wantTotal:  663,
		},
		{
			name: "multiple responses summed",
			responses: []*genai.InlinedResponse{
				{Response: &genai.GenerateContentResponse{UsageMetadata: makeUsage(100, 200, 300)}},
				{Response: &genai.GenerateContentResponse{UsageMetadata: makeUsage(50, 150, 200)}},
			},
			wantInput:  150,
			wantOutput: 350,
			wantTotal:  500,
		},
		{
			name: "nil response entry skipped",
			responses: []*genai.InlinedResponse{
				nil,
				{Response: &genai.GenerateContentResponse{UsageMetadata: makeUsage(10, 20, 30)}},
			},
			wantInput:  10,
			wantOutput: 20,
			wantTotal:  30,
		},
		{
			name: "nil Response field skipped",
			responses: []*genai.InlinedResponse{
				{Response: nil},
				{Response: &genai.GenerateContentResponse{UsageMetadata: makeUsage(5, 10, 15)}},
			},
			wantInput:  5,
			wantOutput: 10,
			wantTotal:  15,
		},
		{
			name: "nil UsageMetadata skipped",
			responses: []*genai.InlinedResponse{
				{Response: &genai.GenerateContentResponse{UsageMetadata: nil}},
				{Response: &genai.GenerateContentResponse{UsageMetadata: makeUsage(3, 7, 10)}},
			},
			wantInput:  3,
			wantOutput: 7,
			wantTotal:  10,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotInput, gotOutput, gotTotal := aggregateTokenCounts(tc.responses)
			assert.Equal(t, tc.wantInput, gotInput, "input token count")
			assert.Equal(t, tc.wantOutput, gotOutput, "output token count")
			assert.Equal(t, tc.wantTotal, gotTotal, "total token count")
		})
	}
}
