// Package scorer provides merchant description scoring using Google Gemini LLM in batch mode.
package scorer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"google.golang.org/genai"
)

const (
	// DefaultModel is the default Gemini model used for scoring.
	DefaultModel = "gemini-2.5-flash"

	// systemPrompt instructs the model to return a structured JSON score.
	systemPrompt = `You are a merchant description quality evaluator.
Given a merchant description, return a JSON object with the following fields:
- "score": an integer from 1 to 10 representing overall quality (1=very poor, 10=excellent)
- "reason": a brief one-sentence explanation for the score

Evaluation criteria:
- Clarity and readability
- Completeness of business information
- Professionalism of language
- Specificity (avoids vague terms)
- Customer-facing appeal

Respond ONLY with valid JSON. Example:
{"score": 7, "reason": "Clear description with good detail but lacks unique selling points."}`

	// pollInterval is how long to wait between polling the batch job status.
	pollInterval = 5 * time.Second

	// maxPollAttempts limits how many times we poll before giving up.
	maxPollAttempts = 120 // 10 minutes at 5-second intervals
)

// ScoreResult holds the score and explanation for a single merchant description.
type ScoreResult struct {
	// MerchantID is the identifier provided with the request (optional).
	MerchantID string `json:"merchant_id,omitempty"`
	// Description is the original merchant description that was scored.
	Description string `json:"description"`
	// Score is the quality score from 1 (very poor) to 10 (excellent).
	Score int `json:"score"`
	// Reason is a brief explanation for the assigned score.
	Reason string `json:"reason"`
	// Error holds any processing error for this individual request.
	Error string `json:"error,omitempty"`
}

// MerchantInput represents a single merchant description to be scored.
type MerchantInput struct {
	// ID is an optional identifier for the merchant.
	ID string
	// Description is the merchant's business description to score.
	Description string
}

// Scorer scores merchant descriptions using Gemini in batch mode.
type Scorer struct {
	client *genai.Client
	model  string
}

// New creates a new Scorer. Pass a nil config to use application default credentials.
// Set the GOOGLE_API_KEY environment variable for Gemini Developer API, or
// set GOOGLE_CLOUD_PROJECT and GOOGLE_CLOUD_LOCATION for Vertex AI.
func New(ctx context.Context, cfg *genai.ClientConfig) (*Scorer, error) {
	client, err := genai.NewClient(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create genai client: %w", err)
	}
	return &Scorer{client: client, model: DefaultModel}, nil
}

// WithModel returns a copy of the Scorer configured to use the given model.
func (s *Scorer) WithModel(model string) *Scorer {
	return &Scorer{client: s.client, model: model}
}

// ScoreBatch submits all merchant descriptions as a single batch job to Gemini
// and waits for the job to complete before returning the results.
// Results are returned in the same order as the inputs.
func (s *Scorer) ScoreBatch(ctx context.Context, merchants []MerchantInput) ([]ScoreResult, error) {
	if len(merchants) == 0 {
		return nil, nil
	}

	requests := buildInlinedRequests(merchants, s.model)

	job, err := s.client.Batches.Create(
		ctx,
		"models/"+s.model,
		&genai.BatchJobSource{InlinedRequests: requests},
		&genai.CreateBatchJobConfig{DisplayName: "merchant-description-scoring"},
	)
	if err != nil {
		return nil, fmt.Errorf("create batch job: %w", err)
	}

	job, err = s.waitForCompletion(ctx, job.Name)
	if err != nil {
		return nil, err
	}

	return parseResults(merchants, job), nil
}

// buildInlinedRequests creates one InlinedRequest per merchant description.
func buildInlinedRequests(merchants []MerchantInput, model string) []*genai.InlinedRequest {
	requests := make([]*genai.InlinedRequest, len(merchants))
	for i, m := range merchants {
		requests[i] = &genai.InlinedRequest{
			Model: "models/" + model,
			Contents: []*genai.Content{
				{
					Role: "user",
					Parts: []*genai.Part{
						{Text: fmt.Sprintf("Score this merchant description:\n\n%s", m.Description)},
					},
				},
			},
			Config: &genai.GenerateContentConfig{
				SystemInstruction: &genai.Content{
					Parts: []*genai.Part{{Text: systemPrompt}},
				},
				Temperature: genai.Ptr[float32](0.1),
			},
			Metadata: map[string]string{
				"merchant_id": m.ID,
				"index":       fmt.Sprintf("%d", i),
			},
		}
	}
	return requests
}

// waitForCompletion polls the batch job until it reaches a terminal state.
func (s *Scorer) waitForCompletion(ctx context.Context, jobName string) (*genai.BatchJob, error) {
	for attempt := 0; attempt < maxPollAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}

		job, err := s.client.Batches.Get(ctx, jobName, &genai.GetBatchJobConfig{})
		if err != nil {
			return nil, fmt.Errorf("poll batch job %s: %w", jobName, err)
		}

		switch job.State {
		case genai.JobStateSucceeded, genai.JobStatePartiallySucceeded:
			return job, nil
		case genai.JobStateFailed:
			msg := "batch job failed"
			if job.Error != nil {
				msg = fmt.Sprintf("batch job failed: %s", job.Error.Message)
			}
			return nil, fmt.Errorf("%s", msg)
		case genai.JobStateCancelled, genai.JobStateCancelling:
			return nil, fmt.Errorf("batch job was cancelled")
		}
		// Still running – keep polling.
	}
	return nil, fmt.Errorf("timed out waiting for batch job %s", jobName)
}

// parseResults extracts ScoreResult values from the completed job's inlined responses.
func parseResults(merchants []MerchantInput, job *genai.BatchJob) []ScoreResult {
	results := make([]ScoreResult, len(merchants))
	for i, m := range merchants {
		results[i] = ScoreResult{
			MerchantID:  m.ID,
			Description: m.Description,
		}
	}

	if job.Dest == nil || len(job.Dest.InlinedResponses) == 0 {
		for i := range results {
			results[i].Error = "no response returned from batch job"
		}
		return results
	}

	for i, resp := range job.Dest.InlinedResponses {
		if i >= len(results) {
			break
		}
		if resp.Error != nil && resp.Error.Message != "" {
			results[i].Error = resp.Error.Message
			continue
		}
		if resp.Response == nil {
			results[i].Error = "empty response"
			continue
		}
		text := extractText(resp.Response)
		sr, err := parseScore(text)
		if err != nil {
			results[i].Error = fmt.Sprintf("parse score: %v (raw: %s)", err, text)
			continue
		}
		results[i].Score = sr.Score
		results[i].Reason = sr.Reason
	}

	return results
}

// extractText pulls the first text part from a GenerateContentResponse.
func extractText(resp *genai.GenerateContentResponse) string {
	if resp == nil {
		return ""
	}
	for _, c := range resp.Candidates {
		if c.Content == nil {
			continue
		}
		for _, p := range c.Content.Parts {
			if p.Text != "" {
				return p.Text
			}
		}
	}
	return ""
}

// scoreResponse is the expected JSON shape returned by the model.
type scoreResponse struct {
	Score  int    `json:"score"`
	Reason string `json:"reason"`
}

// parseScore attempts to parse the model's text output as a scoreResponse JSON.
// It also handles code-fenced JSON (```json ... ```).
func parseScore(text string) (scoreResponse, error) {
	text = strings.TrimSpace(text)
	// Strip markdown code fences if present.
	if strings.HasPrefix(text, "```") {
		start := strings.Index(text, "\n")
		end := strings.LastIndex(text, "```")
		if start != -1 && end > start {
			text = strings.TrimSpace(text[start:end])
		}
	}
	var sr scoreResponse
	if err := json.Unmarshal([]byte(text), &sr); err != nil {
		return scoreResponse{}, fmt.Errorf("unmarshal: %w", err)
	}
	if sr.Score < 1 || sr.Score > 10 {
		return scoreResponse{}, fmt.Errorf("score %d out of range [1,10]", sr.Score)
	}
	return sr, nil
}
