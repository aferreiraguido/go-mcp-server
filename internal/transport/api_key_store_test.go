package transport

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApiKeyStoreDirectString(t *testing.T) {
	store, err := LoadApiKeyStore("sk-test-1, sk-test-2,sk-test-3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.Count() != 3 {
		t.Fatalf("expected 3 keys, got %d", store.Count())
	}
	if !store.Validate("sk-test-1") {
		t.Error("expected sk-test-1 to be valid")
	}
	if !store.Validate("sk-test-2") {
		t.Error("expected sk-test-2 to be valid")
	}
	if !store.Validate("sk-test-3") {
		t.Error("expected sk-test-3 to be valid")
	}
	if store.Validate("sk-invalid") {
		t.Error("expected sk-invalid to be invalid")
	}
	if store.Validate("") {
		t.Error("expected empty string to be invalid")
	}
}

func TestApiKeyStoreJsonObjectArray(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "keys.json")
	content := `[
		{"name": "agent-1", "apiKey": "sk-json-1"},
		{"name": "agent-2", "api_key": "sk-json-2"},
		{"name": "agent-3", "key": "sk-json-3"}
	]`
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := LoadApiKeyStore(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.Count() != 3 {
		t.Fatalf("expected 3 keys, got %d", store.Count())
	}
	if !store.Validate("sk-json-1") || !store.Validate("sk-json-2") || !store.Validate("sk-json-3") {
		t.Error("expected all json keys to validate")
	}
}

func TestApiKeyStoreJsonStringArray(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "keys_str.json")
	content := `["sk-str-1", "sk-str-2"]`
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := LoadApiKeyStore(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.Count() != 2 {
		t.Fatalf("expected 2 keys, got %d", store.Count())
	}
	if !store.Validate("sk-str-1") || !store.Validate("sk-str-2") {
		t.Error("expected all keys to validate")
	}
}

func TestApiKeyStorePlainText(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "keys.txt")
	content := `# Comment line
sk-plain-1
# Another comment
sk-plain-2, sk-plain-3

`
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := LoadApiKeyStore(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.Count() != 3 {
		t.Fatalf("expected 3 keys, got %d", store.Count())
	}
	if !store.Validate("sk-plain-1") || !store.Validate("sk-plain-2") || !store.Validate("sk-plain-3") {
		t.Error("expected all plain keys to validate")
	}
}

func TestApiKeyStoreEmptyError(t *testing.T) {
	if _, err := LoadApiKeyStore(""); err == nil {
		t.Error("expected error on empty spec")
	}
	if _, err := LoadApiKeyStore("  , , "); err == nil {
		t.Error("expected error on blank spec")
	}
}
