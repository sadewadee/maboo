package phpengine

import (
	"os"
	"path/filepath"
)

// DetectEntryPoint finds the PHP entry point for the project.
// Priority: explicit > auto-detect > default
func DetectEntryPoint(docRoot, explicit string) string {
	// 1. Explicit entry point
	if explicit != "" && explicit != "auto" {
		return explicit
	}

	// 2. Auto-detect candidates (in priority order)
	candidates := []string{
		"public/index.php", // Laravel, Symfony, most frameworks
		"index.php",        // WordPress, plain PHP
		"app.php",          // Symfony (old structure)
		"frontend.php",     // Custom
		"main.php",         // Custom
	}

	for _, candidate := range candidates {
		fullPath := filepath.Join(docRoot, candidate)
		if _, err := os.Stat(fullPath); err == nil {
			return candidate
		}
	}

	// 3. Default fallback
	return "index.php"
}

// DetectFramework attempts to identify the PHP framework.
func DetectFramework(docRoot string) string {
	// Check for Laravel
	if _, err := os.Stat(filepath.Join(docRoot, "artisan")); err == nil {
		return "laravel"
	}

	// Check for Symfony
	if _, err := os.Stat(filepath.Join(docRoot, "bin", "console")); err == nil {
		return "symfony"
	}

	// Check for WordPress
	if _, err := os.Stat(filepath.Join(docRoot, "wp-config.php")); err == nil {
		return "wordpress"
	}

	// Check for Drupal
	if _, err := os.Stat(filepath.Join(docRoot, "core", "lib", "Drupal.php")); err == nil {
		return "drupal"
	}

	return "generic"
}
