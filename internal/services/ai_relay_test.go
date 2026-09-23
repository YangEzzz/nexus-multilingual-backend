package services

import "testing"

func TestAIRelayServiceEncryptDecryptKey(t *testing.T) {
	service, err := NewAIRelayService("test-encryption-key", "", "")
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	encrypted, err := service.EncryptKey("sk-test-secret")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if encrypted == "sk-test-secret" {
		t.Fatal("API Key was not encrypted")
	}
	decrypted, err := service.DecryptKey(encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if decrypted != "sk-test-secret" {
		t.Fatalf("unexpected decrypted key: %q", decrypted)
	}
}

func TestParseTranslationsUsesOnlyRequestedLanguages(t *testing.T) {
	translations, err := parseTranslations("```json\n{\"zh-CN\":\"你好\",\"en\":\"hello\",\"debug\":\"ignore\"}\n```", []string{"zh-CN", "en", "ja"})
	if err != nil {
		t.Fatalf("parse translations: %v", err)
	}
	if translations["zh-CN"] != "你好" || translations["en"] != "hello" || translations["ja"] != "" {
		t.Fatalf("unexpected translations: %#v", translations)
	}
	if _, exists := translations["debug"]; exists {
		t.Fatalf("unexpected translation key: %#v", translations)
	}
}

func TestSanitizeRelayMessageMasksAPIKey(t *testing.T) {
	message := sanitizeRelayMessage("invalid key sk-secret", "sk-secret")
	if message != "invalid key [REDACTED]" {
		t.Fatalf("unexpected sanitized message: %q", message)
	}
}
