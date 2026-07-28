// loader.go — ProjectLoader: resolves a source to a local directory.
//
// Counterpart of `project_loader.rs` (rust-mcp-server):
//   - existing local path        → as-is
//   - Git URL (https/git@, *.git) → `git clone --depth 1` to a temp dir
//   - Archive URL (.zip/.tar.gz)  → HTTP download + extraction to a temp dir
package project

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go-mcp-server/internal/model"
)

// Loader resolves sources to local paths. Stateless.
type Loader struct{}

// Resolved indicates the resulting local path and whether it is a temp dir (cleanup).
type Resolved struct {
	Path    string
	TempDir bool
}

// Resolve classifies the source and materializes it as a local directory.
func (l *Loader) Resolve(source string) (*Resolved, error) {
	switch {
	case isGitURL(source):
		return l.cloneGit(source)
	case isArchiveURL(source):
		return l.fetchArchive(source)
	default:
		return l.localPath(source)
	}
}

// localPath validates that the path exists and is a directory.
func (l *Loader) localPath(source string) (*Resolved, error) {
	info, err := os.Stat(source)
	if err != nil || !info.IsDir() {
		return nil, model.NewLoadError(model.ErrSourceNotFound,
			fmt.Sprintf("path not found or not a directory: %s", source))
	}
	abs, err := filepath.Abs(source)
	if err != nil {
		abs = source
	}
	return &Resolved{Path: abs}, nil
}

// isGitURL detects Git URLs: git@host:user/repo, https://.../*.git, or
// known forge hosts without archive extension.
func isGitURL(source string) bool {
	if strings.HasPrefix(source, "git@") {
		return true
	}
	if strings.HasSuffix(source, ".git") {
		return true
	}
	for _, prefix := range []string{"https://github.com/", "https://gitlab.com/", "https://bitbucket.org/"} {
		if strings.HasPrefix(source, prefix) {
			return true
		}
	}
	return false
}

// isArchiveURL detects supported archive URLs.
func isArchiveURL(source string) bool {
	return strings.HasSuffix(source, ".zip") ||
		strings.HasSuffix(source, ".tar.gz") ||
		strings.HasSuffix(source, ".tgz")
}

// cloneGit runs `git clone --depth 1` to a temp dir.
// (Lesson from rust iter 4.5: git CLI is trivial vs a library.)
func (l *Loader) cloneGit(url string) (*Resolved, error) {
	dest, err := os.MkdirTemp("", "go-mcp-clone-*")
	if err != nil {
		return nil, model.NewLoadError(model.ErrGitCloneFailed, err.Error())
	}
	cmd := exec.Command("git", "clone", "--depth", "1", url, dest)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(dest)
		return nil, model.NewLoadError(model.ErrGitCloneFailed,
			fmt.Sprintf("git clone failed: %v: %s", err, strings.TrimSpace(string(out))))
	}
	return &Resolved{Path: dest, TempDir: true}, nil
}

// fetchArchive downloads the URL and extracts it into a temp dir.
// .tar.zst/.tar.xz are not supported (no stdlib; future iter scope).
func (l *Loader) fetchArchive(url string) (*Resolved, error) {
	resp, err := http.Get(url) //nolint:gosec // explicit user URL, same as in rust-mcp-server
	if err != nil {
		return nil, model.NewLoadError(model.ErrDownloadFailed, err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, model.NewLoadError(model.ErrDownloadFailed,
			fmt.Sprintf("HTTP %d from %s", resp.StatusCode, url))
	}

	dest, err := os.MkdirTemp("", "go-mcp-archive-*")
	if err != nil {
		return nil, model.NewLoadError(model.ErrDownloadFailed, err.Error())
	}

	var extractErr error
	switch {
	case strings.HasSuffix(url, ".zip"):
		extractErr = extractZip(resp.Body, dest)
	default: // .tar.gz / .tgz
		extractErr = extractTarGz(resp.Body, dest)
	}
	if extractErr != nil {
		os.RemoveAll(dest)
		return nil, model.NewLoadError(model.ErrArchiveUnsupported, extractErr.Error())
	}

	// If the archive contains a single root directory, descend one level
	// (common convention in GitHub/GitLab archives).
	if single := singleChildDir(dest); single != "" {
		return &Resolved{Path: single, TempDir: true}, nil
	}
	return &Resolved{Path: dest, TempDir: true}, nil
}

// singleChildDir returns the only subdirectory of dir if it is its only
// content, or "" otherwise.
func singleChildDir(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || !entries[0].IsDir() {
		return ""
	}
	return filepath.Join(dir, entries[0].Name())
}

// extractZip extracts a zip stream into dest. archive/zip needs ReaderAt,
// so it is first materialized into a temporary file.
func extractZip(r io.Reader, dest string) error {
	tmp, err := os.CreateTemp("", "go-mcp-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if _, err := io.Copy(tmp, r); err != nil {
		return err
	}

	zr, err := zip.OpenReader(tmp.Name())
	if err != nil {
		return err
	}
	defer zr.Close()

	for _, f := range zr.File {
		target, err := safeJoin(dest, f.Name)
		if err != nil {
			return err
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		err = writeFile(target, rc, f.Mode())
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// extractTarGz extracts a tar.gz stream into dest.
func extractTarGz(r io.Reader, dest string) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target, err := safeJoin(dest, hdr.Name)
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := writeFile(target, tr, hdr.FileInfo().Mode()); err != nil {
				return err
			}
		}
	}
}

// safeJoin joins root+rel rejecting path traversal (ZipSlip).
func safeJoin(root, rel string) (string, error) {
	clean := filepath.Clean(rel)
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe path in archive: %s", rel)
	}
	return filepath.Join(root, clean), nil
}

func writeFile(path string, r io.Reader, mode os.FileMode) error {
	if mode == 0 {
		mode = 0o644
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}
