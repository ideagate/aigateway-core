package models

import (
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBatchJobID(t *testing.T) {
	t.Run("length is 15 chars", func(t *testing.T) {
		id := newBatchJobID()
		assert.Len(t, id, 15)
	})

	t.Run("first 12 chars are digits", func(t *testing.T) {
		id := newBatchJobID()
		for i, ch := range id[:12] {
			assert.Truef(t, unicode.IsDigit(ch), "newBatchJobID()[%d] = %q, want digit (id=%q)", i, ch, id)
		}
	})

	t.Run("last 3 chars are lowercase alphanumeric", func(t *testing.T) {
		id := newBatchJobID()
		for i, ch := range id[12:] {
			assert.Truef(t, unicode.IsLetter(ch) || unicode.IsDigit(ch), "newBatchJobID() suffix[%d] = %q, want alphanumeric (id=%q)", i, ch, id)
			if unicode.IsLetter(ch) {
				assert.Truef(t, unicode.IsLower(ch), "newBatchJobID() suffix[%d] = %q, want lowercase (id=%q)", i, ch, id)
			}
		}
	})

	t.Run("generates unique IDs across multiple calls", func(t *testing.T) {
		const n = 100
		seen := make(map[string]struct{}, n)
		for range n {
			id := newBatchJobID()
			_, dup := seen[id]
			assert.Falsef(t, dup, "newBatchJobID() produced duplicate ID %q", id)
			seen[id] = struct{}{}
		}
	})
}

func TestBatchJobBeforeCreate(t *testing.T) {
	t.Run("sets ID when empty", func(t *testing.T) {
		b := &BatchJob{}
		err := b.BeforeCreate(nil)
		require.NoError(t, err)
		assert.NotEmpty(t, b.ID)
		assert.Len(t, b.ID, 15)
	})

	t.Run("does not overwrite an existing ID", func(t *testing.T) {
		const existingID = "existing-id-001"
		b := &BatchJob{ID: existingID}
		err := b.BeforeCreate(nil)
		require.NoError(t, err)
		assert.Equal(t, existingID, b.ID)
	})
}
