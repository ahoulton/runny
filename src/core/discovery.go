package core

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// buildIgnoredDirs builds the set of directory names to skip during script
// discovery. Names are compared case-insensitively, matching the previous
// hardcoded map behavior (which listed both "Lib" and "lib").
func buildIgnoredDirs(cfg Config) map[string]bool {
	m := make(map[string]bool, len(cfg.ExcludedDirs))
	for _, name := range cfg.ExcludedDirs {
		m[strings.ToLower(name)] = true
	}
	return m
}

// buildIncludedExtensions normalizes the configured script extensions to
// lowercase with a leading dot (e.g. "PYW" -> ".pyw").
func buildIncludedExtensions(cfg Config) map[string]bool {
	m := make(map[string]bool, len(cfg.IncludedExtensions))
	for _, ext := range cfg.IncludedExtensions {
		ext = strings.ToLower(strings.TrimSpace(ext))
		if ext == "" {
			continue
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		m[ext] = true
	}
	return m
}

// FindScripts recursively discovers Python scripts in the given root directory
// (per the config's excluded dirs and included extensions).
// Returns a sorted list of absolute paths.
func FindScripts(root string, cfg Config) []string {
	var scripts []string

	ignoredDirs := buildIgnoredDirs(cfg)
	includedExtensions := buildIncludedExtensions(cfg)
	findScriptsWalk(root, &scripts, ignoredDirs, includedExtensions)
	sort.Strings(scripts)
	return scripts
}

// findScriptsWalk does the actual recursion with prebuilt lookup sets.
func findScriptsWalk(dir string, scripts *[]string, ignoredDirs, includedExtensions map[string]bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		// Silently skip directories that cannot be read
		return
	}

	for _, entry := range entries {
		name := entry.Name()
		fullPath := filepath.Join(dir, name)

		if entry.IsDir() {
			if !ignoredDirs[strings.ToLower(name)] {
				findScriptsWalk(fullPath, scripts, ignoredDirs, includedExtensions)
			}
			continue
		}

		// Check if the file matches a configured script extension
		ext := strings.ToLower(filepath.Ext(name))
		if includedExtensions[ext] {
			absPath, err := filepath.Abs(fullPath)
			if err != nil {
				continue
			}
			*scripts = append(*scripts, absPath)
		}
	}
}
