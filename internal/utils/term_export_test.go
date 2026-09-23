package utils

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"testing"

	"choice-matrix-backend/internal/models"
)

func exportTestTerms() []models.Term {
	return []models.Term{
		{Key: "confirm", Module: "common", Status: models.TermStatusReview, Translations: []models.Translation{{LanguageCode: "cn", Content: "确认"}, {LanguageCode: "en", Content: "Confirm"}}},
		{Key: "cancel", Status: models.TermStatusPublished, Translations: []models.Translation{{LanguageCode: "cn", Content: "取消"}, {LanguageCode: "en", Content: "Cancel"}}},
	}
}

func TestBuildLanguageJSON(t *testing.T) {
	content, err := BuildLanguageJSON(exportTestTerms(), "en")
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(content, &payload); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if payload["cancel"] != "Cancel" {
		t.Fatalf("unexpected global term: %+v", payload)
	}
	common, ok := payload["common"].(map[string]interface{})
	if !ok || common["confirm"] != "Confirm" {
		t.Fatalf("unexpected module terms: %+v", payload)
	}
}

func TestBuildLanguageJSONArchive(t *testing.T) {
	content, err := BuildLanguageJSONArchive(exportTestTerms(), []string{"cn", "en"})
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("invalid ZIP: %v", err)
	}
	if len(archive.File) != 2 {
		t.Fatalf("archive contains %d files", len(archive.File))
	}
}

func TestBuildTermXLSX(t *testing.T) {
	content, err := BuildTermXLSX(exportTestTerms(), []models.Language{{Code: "cn"}, {Code: "en"}})
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("invalid XLSX ZIP: %v", err)
	}
	var sheet []byte
	for _, file := range archive.File {
		if file.Name == "xl/worksheets/sheet1.xml" {
			reader, openErr := file.Open()
			if openErr != nil {
				t.Fatal(openErr)
			}
			sheet, err = io.ReadAll(reader)
			reader.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if !bytes.Contains(sheet, []byte("Confirm")) || !bytes.Contains(sheet, []byte("已发布")) {
		t.Fatalf("worksheet is missing expected values: %s", sheet)
	}
}
