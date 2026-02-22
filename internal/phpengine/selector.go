package phpengine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// SelectVersion determines which PHP version to use.
// Priority: explicit > composer.json > default (8.3)
func SelectVersion(projectRoot, explicit string) string {
	// 1. Explicit version takes precedence
	if explicit != "" && explicit != "auto" {
		return explicit
	}

	// 2. Check composer.json
	composerPath := filepath.Join(projectRoot, "composer.json")
	if data, err := os.ReadFile(composerPath); err == nil {
		version := parseComposerPHPVersion(data)
		if version != "" {
			return version
		}
	}

	// 3. Default to latest stable
	return "8.3"
}

// parseComposerPHPVersion extracts PHP version from composer.json
func parseComposerPHPVersion(data []byte) string {
	var composer struct {
		Require map[string]string `json:"require"`
	}

	if err := json.Unmarshal(data, &composer); err != nil {
		return ""
	}

	phpConstraint, ok := composer.Require["php"]
	if !ok {
		return ""
	}

	return resolveVersionConstraint(phpConstraint)
}

// resolveVersionConstraint converts composer constraint to specific version
func resolveVersionConstraint(constraint string) string {
	constraint = strings.TrimSpace(constraint)

	// Handle common patterns
	patterns := []struct {
		regex *regexp.Regexp
	}{
		{regexp.MustCompile(`^>=?(\d+\.\d+)`)},
		{regexp.MustCompile(`^\^(\d+\.\d+)`)},
		{regexp.MustCompile(`^~(\d+\.\d+)`)},
		{regexp.MustCompile(`^(\d+\.\d+)\.\d+$`)},
	}

	for _, p := range patterns {
		if matches := p.regex.FindStringSubmatch(constraint); len(matches) > 1 {
			minVersion := matches[1]
			return getHighestCompatible(minVersion)
		}
	}

	// Check for specific version
	if matched, _ := regexp.MatchString(`^\d+\.\d+$`, constraint); matched {
		return getHighestCompatible(constraint)
	}

	return ""
}

// getHighestCompatible returns the highest PHP version compatible with min
func getHighestCompatible(min string) string {
	versions := []string{"7.4", "8.0", "8.1", "8.2", "8.3", "8.4"}

	for _, v := range versions {
		if compareVersions(v, min) >= 0 {
			// Return highest available
			return "8.3"
		}
	}

	return "8.3"
}

// compareVersions compares two version strings
func compareVersions(a, b string) int {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")

	for i := 0; i < 2; i++ {
		var va, vb int
		if i < len(partsA) {
			va = atoi(partsA[i])
		}
		if i < len(partsB) {
			vb = atoi(partsB[i])
		}
		if va < vb {
			return -1
		}
		if va > vb {
			return 1
		}
	}
	return 0
}

func atoi(s string) int {
	var n int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}
