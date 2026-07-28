package model

import "fmt"

// LoadErrorCode classifies project loading errors.
// Counterpart of `ProjectLoadErrorCode` (rust) / `ProjectLoadException` (java).
type LoadErrorCode string

const (
	// ErrSourceNotFound: the local path does not exist.
	ErrSourceNotFound LoadErrorCode = "SOURCE_NOT_FOUND"
	// ErrGoModMissing: no go.mod after resolution.
	ErrGoModMissing LoadErrorCode = "GO_MOD_MISSING"
	// ErrGoModMalformed: go.mod cannot be parsed.
	ErrGoModMalformed LoadErrorCode = "GO_MOD_MALFORMED"
	// ErrGitCloneFailed: `git clone` failed.
	ErrGitCloneFailed LoadErrorCode = "GIT_CLONE_FAILED"
	// ErrArchiveUnsupported: unsupported archive extension.
	ErrArchiveUnsupported LoadErrorCode = "ARCHIVE_UNSUPPORTED"
	// ErrDownloadFailed: HTTP download of archive failed.
	ErrDownloadFailed LoadErrorCode = "DOWNLOAD_FAILED"
)

// LoadError is a load error with a stable code, designed so the LLM
// can reason about it (appears in OKError Markdown).
type LoadError struct {
	Code    LoadErrorCode
	Message string
}

func (e *LoadError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// NewLoadError constructs a LoadError.
func NewLoadError(code LoadErrorCode, message string) *LoadError {
	return &LoadError{Code: code, Message: message}
}
