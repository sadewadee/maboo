package phpengine_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sadewadee/maboo/internal/phpengine"
)

func TestDetectEntryPointLaravel(t *testing.T) {
	tmpDir := t.TempDir()

	// Create Laravel structure
	os.MkdirAll(filepath.Join(tmpDir, "public"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "public", "index.php"), []byte("<?php"), 0644)

	entry := phpengine.DetectEntryPoint(tmpDir, "auto")
	if entry != "public/index.php" {
		t.Errorf("expected public/index.php for Laravel, got %s", entry)
	}
}

func TestDetectEntryPointWordPress(t *testing.T) {
	tmpDir := t.TempDir()

	// Create WordPress structure
	os.WriteFile(filepath.Join(tmpDir, "index.php"), []byte("<?php"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "wp-config.php"), []byte("<?php"), 0644)

	entry := phpengine.DetectEntryPoint(tmpDir, "auto")
	if entry != "index.php" {
		t.Errorf("expected index.php for WordPress, got %s", entry)
	}
}

func TestDetectEntryPointExplicit(t *testing.T) {
	tmpDir := t.TempDir()

	entry := phpengine.DetectEntryPoint(tmpDir, "app.php")
	if entry != "app.php" {
		t.Errorf("expected explicit app.php, got %s", entry)
	}
}

func TestDetectEntryPointDefault(t *testing.T) {
	tmpDir := t.TempDir()

	entry := phpengine.DetectEntryPoint(tmpDir, "auto")
	if entry != "index.php" {
		t.Errorf("expected default index.php, got %s", entry)
	}
}

func TestDetectFramework(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(string)
		expected  string
	}{
		{
			name: "laravel",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "artisan"), []byte("#!/bin/php"), 0755)
			},
			expected: "laravel",
		},
		{
			name: "wordpress",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "wp-config.php"), []byte("<?php"), 0644)
			},
			expected: "wordpress",
		},
		{
			name:     "generic",
			setup:    func(dir string) {},
			expected: "generic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tt.setup(tmpDir)

			framework := phpengine.DetectFramework(tmpDir)
			if framework != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, framework)
			}
		})
	}
}
