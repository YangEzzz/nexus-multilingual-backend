package repository

import (
	"testing"

	"choice-matrix-backend/internal/models"
)

func TestBatchCreateStatus(t *testing.T) {
	languages := []models.Language{{Code: "cn"}, {Code: "en"}}
	tests := []struct {
		name   string
		filled map[string]bool
		want   models.TermStatus
	}{
		{name: "no translation stays draft", filled: map[string]bool{}, want: models.TermStatusDraft},
		{name: "partial translation is pending", filled: map[string]bool{"cn": true}, want: models.TermStatusPending},
		{name: "complete translation is ready for review", filled: map[string]bool{"cn": true, "en": true}, want: models.TermStatusReview},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := batchCreateStatus(languages, tt.filled); got != tt.want {
				t.Fatalf("batchCreateStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}
