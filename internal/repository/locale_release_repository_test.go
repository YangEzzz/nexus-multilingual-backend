package repository

import (
	"encoding/json"
	"testing"

	"choice-matrix-backend/internal/models"
)

func TestLocaleReleaseMergePreservesAbsentAndOverlaysPublishedValues(t *testing.T) {
	previous := models.LocaleData{
		"en": {"common": {"kept": "Old", "updated": "Before"}},
		"cn": {"common": {"kept": "保留"}},
	}
	current := cloneLocaleData(previous)
	mergePublishedTerms(current, []models.Term{
		{Module: "common", Key: "updated", Translations: []models.Translation{
			{LanguageCode: "en", Content: "After"},
			{LanguageCode: "cn", Content: ""},
		}},
		{Module: "common", Key: "added", Translations: []models.Translation{
			{LanguageCode: "en", Content: "New"},
		}},
	}, map[string]bool{"en": true, "cn": true})

	if current["en"]["common"]["kept"] != "Old" || current["en"]["common"]["updated"] != "After" || current["en"]["common"]["added"] != "New" {
		t.Fatalf("unexpected English snapshot: %#v", current["en"])
	}
	if current["cn"]["common"]["kept"] != "保留" {
		t.Fatalf("empty translation should preserve prior value: %#v", current["cn"])
	}
	if previous["en"]["common"]["updated"] != "Before" {
		t.Fatal("previous release was mutated")
	}

	content, err := renderLocaleJSON(current["en"])
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]map[string]string
	if err := json.Unmarshal([]byte(content), &payload); err != nil || payload["common"]["updated"] != "After" {
		t.Fatalf("invalid public JSON: %s, %v", content, err)
	}
}

func TestRenderLocaleJSONRejectsGlobalModuleCollision(t *testing.T) {
	_, err := renderLocaleJSON(map[string]map[string]string{
		"":       {"common": "text"},
		"common": {"confirm": "Confirm"},
	})
	if err == nil {
		t.Fatal("expected a collision error")
	}
}
