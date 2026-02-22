package config_test

import (
	"testing"

	"github.com/sadewadee/maboo/internal/config"
)

func TestPHPVersionConfig(t *testing.T) {
	cfg := &config.Config{
		PHP: config.PHPConfig{
			Version: "8.3",
			Mode:    "worker",
		},
		App: config.AppConfig{
			Root:  ".",
			Entry: "auto",
		},
	}

	if cfg.PHP.Version != "8.3" {
		t.Errorf("expected PHP version 8.3, got %s", cfg.PHP.Version)
	}
	if cfg.PHP.Mode != "worker" {
		t.Errorf("expected worker mode, got %s", cfg.PHP.Mode)
	}
}

func TestAppConfigDefaults(t *testing.T) {
	cfg := config.Default()

	if cfg.App.Root != "." {
		t.Errorf("expected default root '.', got %s", cfg.App.Root)
	}
	if cfg.App.Entry != "auto" {
		t.Errorf("expected default entry 'auto', got %s", cfg.App.Entry)
	}
}

func TestPHPConfigDefaults(t *testing.T) {
	cfg := config.Default()

	if cfg.PHP.Version != "auto" {
		t.Errorf("expected default PHP version 'auto', got %s", cfg.PHP.Version)
	}
	if cfg.PHP.Mode != "worker" {
		t.Errorf("expected default PHP mode 'worker', got %s", cfg.PHP.Mode)
	}
}

func TestValidatePHPMode(t *testing.T) {
	tests := []struct {
		mode      string
		expectErr bool
	}{
		{"worker", false},
		{"request", false},
		{"invalid", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			cfg := config.Default()
			cfg.PHP.Mode = tt.mode
			cfg.PHP.Version = "8.3"

			// Remove the Worker requirement for validation test
			cfg.PHP.Worker = "index.php"

			err := cfg.Validate()
			if tt.expectErr && err == nil {
				t.Error("expected error for invalid mode")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidatePHPVersion(t *testing.T) {
	tests := []struct {
		version   string
		expectErr bool
	}{
		{"auto", false},
		{"7.4", false},
		{"8.0", false},
		{"8.1", false},
		{"8.2", false},
		{"8.3", false},
		{"8.4", false},
		{"9.0", true},
		{"invalid", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			cfg := config.Default()
			cfg.PHP.Version = tt.version
			cfg.PHP.Worker = "index.php"

			err := cfg.Validate()
			if tt.expectErr && err == nil {
				t.Error("expected error for invalid version")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
