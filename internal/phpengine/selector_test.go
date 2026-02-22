package phpengine_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sadewadee/maboo/internal/phpengine"
)

func TestSelectVersionFromConfig(t *testing.T) {
	// When config specifies a version, use it
	version := phpengine.SelectVersion("/some/path", "8.2")
	if version != "8.2" {
		t.Errorf("expected 8.2, got %s", version)
	}
}

func TestSelectVersionFromComposer(t *testing.T) {
	// Create temp dir with composer.json
	tmpDir := t.TempDir()
	composerJSON := `{
        "require": {
            "php": "^8.1"
        }
    }`
	err := os.WriteFile(filepath.Join(tmpDir, "composer.json"), []byte(composerJSON), 0644)
	if err != nil {
		t.Fatal(err)
	}

	version := phpengine.SelectVersion(tmpDir, "auto")
	// ^8.1 should resolve to 8.3 (latest compatible)
	if version != "8.3" {
		t.Errorf("expected 8.3 for ^8.1, got %s", version)
	}
}

func TestSelectVersionDefault(t *testing.T) {
	tmpDir := t.TempDir()
	version := phpengine.SelectVersion(tmpDir, "auto")
	if version != "8.3" {
		t.Errorf("expected default 8.3, got %s", version)
	}
}

func TestSelectVersionExplicit(t *testing.T) {
	// Explicit version takes precedence over composer.json
	tmpDir := t.TempDir()
	composerJSON := `{"require": {"php": "^7.4"}}`
	os.WriteFile(filepath.Join(tmpDir, "composer.json"), []byte(composerJSON), 0644)

	version := phpengine.SelectVersion(tmpDir, "8.4")
	if version != "8.4" {
		t.Errorf("expected explicit 8.4, got %s", version)
	}
}
