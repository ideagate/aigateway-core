package providers

import (
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClaudeProvider(t *testing.T) {
	t.Run("empty api key returns error", func(t *testing.T) {
		_, err := newClaudeProvider("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "api key is required")
	})

	t.Run("valid api key returns provider", func(t *testing.T) {
		p, err := newClaudeProvider("test-api-key")
		require.NoError(t, err)
		assert.NotNil(t, p)
		assert.Equal(t, "claude", p.Name())
	})
}

func TestExtractTextFromContent(t *testing.T) {
	tests := []struct {
		name     string
		content  []anthropic.ContentBlockUnion
		wantText string
	}{
		{
			name:     "nil slice",
			content:  nil,
			wantText: "",
		},
		{
			name:     "empty slice",
			content:  []anthropic.ContentBlockUnion{},
			wantText: "",
		},
		{
			name: "single text block",
			content: []anthropic.ContentBlockUnion{
				{Type: "text", Text: "Hello, world!"},
			},
			wantText: "Hello, world!",
		},
		{
			name: "multiple text blocks concatenated",
			content: []anthropic.ContentBlockUnion{
				{Type: "text", Text: "Hello"},
				{Type: "text", Text: ", world!"},
			},
			wantText: "Hello, world!",
		},
		{
			name: "non-text blocks skipped",
			content: []anthropic.ContentBlockUnion{
				{Type: "thinking", Thinking: "some reasoning"},
				{Type: "text", Text: "Answer"},
			},
			wantText: "Answer",
		},
		{
			name: "no text blocks returns empty string",
			content: []anthropic.ContentBlockUnion{
				{Type: "thinking", Thinking: "some reasoning"},
			},
			wantText: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractTextFromContent(tc.content)
			assert.Equal(t, tc.wantText, got)
		})
	}
}
