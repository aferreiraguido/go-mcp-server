package transport

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// keyEntry represents an object in a JSON API key array.
type keyEntry struct {
	Name      string `json:"name"`
	APIKey    string `json:"apiKey"`
	APIKeyAlt string `json:"api_key"`
	KeyAlt    string `json:"key"`
}

func (e keyEntry) getKey() string {
	if e.APIKey != "" {
		return e.APIKey
	}
	if e.APIKeyAlt != "" {
		return e.APIKeyAlt
	}
	return e.KeyAlt
}

// ApiKeyStore holds a set of valid Bearer tokens in memory for fast lookup.
type ApiKeyStore struct {
	keys map[string]struct{}
}

// NewApiKeyStore creates an ApiKeyStore from a slice of tokens.
func NewApiKeyStore(tokens []string) *ApiKeyStore {
	m := make(map[string]struct{}, len(tokens))
	for _, tok := range tokens {
		t := strings.TrimSpace(tok)
		if t != "" {
			m[t] = struct{}{}
		}
	}
	return &ApiKeyStore{keys: m}
}

// Count returns the number of unique valid keys in the store.
func (s *ApiKeyStore) Count() int {
	if s == nil {
		return 0
	}
	return len(s.keys)
}

// Validate returns true if the token matches any stored API key.
func (s *ApiKeyStore) Validate(token string) bool {
	if s == nil {
		return false
	}
	t := strings.TrimSpace(token)
	if t == "" {
		return false
	}
	_, ok := s.keys[t]
	return ok
}

// Load loads API keys from either a file path (JSON or line-delimited)
// or a direct comma-separated string of tokens.
func LoadApiKeyStore(spec string) (*ApiKeyStore, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, errors.New("empty API keys specification")
	}

	info, err := os.Stat(spec)
	if err == nil && !info.IsDir() {
		return loadFromFile(spec)
	}

	// Not a file; parse as comma-separated tokens.
	var tokens []string
	for _, part := range strings.Split(spec, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			tokens = append(tokens, trimmed)
		}
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("no valid API keys found in specification: %s", spec)
	}
	return NewApiKeyStore(tokens), nil
}

func loadFromFile(path string) (*ApiKeyStore, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read API keys file %s: %w", path, err)
	}

	// 1. Try parsing as JSON array of objects [{"name": "...", "apiKey": "..."}]
	var entries []keyEntry
	if err := json.Unmarshal(data, &entries); err == nil {
		var tokens []string
		for _, e := range entries {
			k := strings.TrimSpace(e.getKey())
			if k != "" {
				tokens = append(tokens, k)
			}
		}
		if len(tokens) > 0 {
			return NewApiKeyStore(tokens), nil
		}
	}

	// 2. Try parsing as JSON array of strings ["key1", "key2"]
	var strEntries []string
	if err := json.Unmarshal(data, &strEntries); err == nil {
		var tokens []string
		for _, s := range strEntries {
			k := strings.TrimSpace(s)
			if k != "" {
				tokens = append(tokens, k)
			}
		}
		if len(tokens) > 0 {
			return NewApiKeyStore(tokens), nil
		}
	}

	// 3. Fallback: line-by-line or comma-separated text
	var tokens []string
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		for _, part := range strings.Split(line, ",") {
			k := strings.TrimSpace(part)
			if k != "" {
				tokens = append(tokens, k)
			}
		}
	}

	if len(tokens) == 0 {
		return nil, fmt.Errorf("no valid API keys found in file: %s", path)
	}
	return NewApiKeyStore(tokens), nil
}
