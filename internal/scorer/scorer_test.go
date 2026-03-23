package scorer

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/genai"
)

func TestParseScore(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    scoreResponse
		wantErr bool
	}{
		{
			name:  "valid JSON",
			input: `{"score": 8, "reason": "Clear and professional description."}`,
			want:  scoreResponse{Score: 8, Reason: "Clear and professional description."},
		},
		{
			name:  "valid JSON with whitespace",
			input: "  { \"score\": 5, \"reason\": \"Average.\" }  ",
			want:  scoreResponse{Score: 5, Reason: "Average."},
		},
		{
			name: "markdown code fence",
			input: "```json\n{\"score\": 9, \"reason\": \"Excellent detail.\"}\n```",
			want:  scoreResponse{Score: 9, Reason: "Excellent detail."},
		},
		{
			name:    "score out of range low",
			input:   `{"score": 0, "reason": "Bad."}`,
			wantErr: true,
		},
		{
			name:    "score out of range high",
			input:   `{"score": 11, "reason": "Too high."}`,
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			input:   `not json at all`,
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseScore(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil (result: %+v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Score != tc.want.Score || got.Reason != tc.want.Reason {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestExtractText(t *testing.T) {
	tests := []struct {
		name string
		resp *genai.GenerateContentResponse
		want string
	}{
		{
			name: "nil response",
			resp: nil,
			want: "",
		},
		{
			name: "response with text",
			resp: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Parts: []*genai.Part{
								{Text: `{"score": 7, "reason": "Good."}`},
							},
						},
					},
				},
			},
			want: `{"score": 7, "reason": "Good."}`,
		},
		{
			name: "candidate with nil content",
			resp: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{Content: nil},
				},
			},
			want: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractText(tc.resp)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBuildInlinedRequests(t *testing.T) {
	merchants := []MerchantInput{
		{ID: "m1", Description: "A great coffee shop."},
		{ID: "m2", Description: "Fast food restaurant."},
	}

	reqs := buildInlinedRequests(merchants, DefaultModel)

	if len(reqs) != len(merchants) {
		t.Fatalf("expected %d requests, got %d", len(merchants), len(reqs))
	}

	for i, req := range reqs {
		if req.Model != "models/"+DefaultModel {
			t.Errorf("request %d: model = %q, want %q", i, req.Model, "models/"+DefaultModel)
		}
		if len(req.Contents) == 0 {
			t.Errorf("request %d: no contents", i)
			continue
		}
		if !contains(req.Contents[0].Parts[0].Text, merchants[i].Description) {
			t.Errorf("request %d: description not in prompt text", i)
		}
		if req.Metadata["merchant_id"] != merchants[i].ID {
			t.Errorf("request %d: metadata merchant_id = %q, want %q", i, req.Metadata["merchant_id"], merchants[i].ID)
		}
	}
}

func TestParseResults_NoResponses(t *testing.T) {
	merchants := []MerchantInput{
		{ID: "m1", Description: "Coffee shop."},
	}
	job := &genai.BatchJob{
		State: genai.JobStateSucceeded,
		Dest:  &genai.BatchJobDestination{},
	}

	results := parseResults(merchants, job)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Error == "" {
		t.Error("expected error message when no responses returned")
	}
}

func TestParseResults_WithValidResponse(t *testing.T) {
	merchants := []MerchantInput{
		{ID: "m1", Description: "A cozy bookstore."},
	}

	scoreJSON := `{"score": 8, "reason": "Well described."}`
	job := &genai.BatchJob{
		State: genai.JobStateSucceeded,
		Dest: &genai.BatchJobDestination{
			InlinedResponses: []*genai.InlinedResponse{
				{
					Response: &genai.GenerateContentResponse{
						Candidates: []*genai.Candidate{
							{
								Content: &genai.Content{
									Parts: []*genai.Part{{Text: scoreJSON}},
								},
							},
						},
					},
				},
			},
		},
	}

	results := parseResults(merchants, job)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Error != "" {
		t.Errorf("unexpected error: %s", r.Error)
	}
	if r.Score != 8 {
		t.Errorf("score = %d, want 8", r.Score)
	}
	if r.Reason != "Well described." {
		t.Errorf("reason = %q, want %q", r.Reason, "Well described.")
	}
	if r.MerchantID != "m1" {
		t.Errorf("merchant_id = %q, want %q", r.MerchantID, "m1")
	}
}

func TestParseResults_WithErrorResponse(t *testing.T) {
	errCode := int32(400)
	merchants := []MerchantInput{
		{ID: "m1", Description: "Bad merchant."},
	}
	job := &genai.BatchJob{
		State: genai.JobStatePartiallySucceeded,
		Dest: &genai.BatchJobDestination{
			InlinedResponses: []*genai.InlinedResponse{
				{
					Error: &genai.JobError{Code: &errCode, Message: "invalid request"},
				},
			},
		},
	}

	results := parseResults(merchants, job)
	if results[0].Error != "invalid request" {
		t.Errorf("error = %q, want %q", results[0].Error, "invalid request")
	}
}

func TestScoreBatch_EmptyInput(t *testing.T) {
	s := &Scorer{model: DefaultModel}
	results, err := s.ScoreBatch(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results for empty input, got %v", results)
	}
}

// contains is a helper to check if s contains substring.
func contains(s, substring string) bool {
	return strings.Contains(s, substring)
}
