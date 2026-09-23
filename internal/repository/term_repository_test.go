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

func TestEvaluateBulkTerms(t *testing.T) {
	languages := []models.Language{
		{Code: "cn", IsSource: true},
		{Code: "en"},
	}
	terms := []models.Term{
		{
			ID:     1,
			Key:    "welcome",
			Status: models.TermStatusPending,
			Translations: []models.Translation{
				{LanguageCode: "cn", Content: "你好 {name}"},
				{LanguageCode: "en", Content: "Hello {name}"},
			},
		},
		{
			ID:     2,
			Key:    "missing",
			Status: models.TermStatusPending,
			Translations: []models.Translation{
				{LanguageCode: "cn", Content: "缺少英文"},
			},
		},
		{
			ID:     3,
			Key:    "published",
			Status: models.TermStatusPublished,
		},
	}

	result, eligible := evaluateBulkTerms(terms, languages, BulkOperationApproveReview)
	if result.Matched != 3 || result.Executable != 1 || result.Skipped != 2 {
		t.Fatalf("unexpected bulk counts: %+v", result)
	}
	if len(eligible) != 1 || eligible[0].ID != 1 {
		t.Fatalf("unexpected eligible terms: %+v", eligible)
	}
	if result.SkipReasons["missing_translation"] != 1 || result.SkipReasons["status_not_allowed"] != 1 {
		t.Fatalf("unexpected skip reasons: %+v", result.SkipReasons)
	}
}

func TestEvaluateBulkDeleteProtectsPublishedTerms(t *testing.T) {
	terms := []models.Term{
		{ID: 1, Status: models.TermStatusReview},
		{ID: 2, Status: models.TermStatusPublished},
	}
	result, eligible := evaluateBulkTerms(terms, nil, BulkOperationDelete)
	if result.Executable != 1 || len(eligible) != 1 || eligible[0].ID != 1 {
		t.Fatalf("unexpected delete evaluation: result=%+v eligible=%+v", result, eligible)
	}
	if result.SkipReasons["published_protected"] != 1 {
		t.Fatalf("expected published term to be protected: %+v", result.SkipReasons)
	}
}

func TestEvaluateBulkPublishRequiresReviewAndValidPlaceholders(t *testing.T) {
	languages := []models.Language{
		{Code: "cn", IsSource: true},
		{Code: "en"},
	}
	terms := []models.Term{
		{
			ID:     1,
			Key:    "valid",
			Status: models.TermStatusReview,
			Translations: []models.Translation{
				{LanguageCode: "cn", Content: "你好 {name}"},
				{LanguageCode: "en", Content: "Hello {name}"},
			},
		},
		{
			ID:     2,
			Key:    "wrong-status",
			Status: models.TermStatusPending,
			Translations: []models.Translation{
				{LanguageCode: "cn", Content: "你好"},
				{LanguageCode: "en", Content: "Hello"},
			},
		},
		{
			ID:     3,
			Key:    "placeholder-mismatch",
			Status: models.TermStatusReview,
			Translations: []models.Translation{
				{LanguageCode: "cn", Content: "你好 {name}"},
				{LanguageCode: "en", Content: "Hello"},
			},
		},
	}

	result, eligible := evaluateBulkTerms(terms, languages, BulkOperationPublish)
	if result.Executable != 1 || len(eligible) != 1 || eligible[0].ID != 1 {
		t.Fatalf("unexpected publish evaluation: result=%+v eligible=%+v", result, eligible)
	}
	if result.SkipReasons["status_not_allowed"] != 1 || result.SkipReasons["placeholder_mismatch"] != 1 {
		t.Fatalf("unexpected publish skip reasons: %+v", result.SkipReasons)
	}
}
