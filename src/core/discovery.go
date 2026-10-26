package core

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// exclusions holds the configured directories to skip during script
// discovery: bare names, matched at any depth, and search-root-relative
// paths such as "redirect/core", matched exactly.
type exclusions struct {
	names map[string]bool
	paths map[string]bool
}

// buildExclusions splits excluded_dirs entries into bare directory names and
// root-relative paths. An entry containing a separator is a path; anything
// else keeps the original "skip any directory with this name" behavior.
// Matching is case-insensitive, matching the previous hardcoded map behavior
// (which listed both "Lib" and "lib"), and path entries accept either
// separator.
func buildExclusions(cfg Config) exclusions {
	ex := exclusions{
		names: make(map[string]bool, len(cfg.ExcludedDirs)),
		paths: make(map[string]bool, len(cfg.ExcludedDirs)),
	}
	for _, entry := range cfg.ExcludedDirs {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if strings.ContainsAny(entry, `/\`) {
			if p := normalizeExcludedPath(entry); p != "" {
				ex.paths[p] = true
			}
			continue
		}
		ex.names[strings.ToLower(entry)] = true
	}
	return ex
}

// normalizeExcludedPath converts a root-relative excluded path to the form
// used during the walk: forward slashes, no "." segments or trailing slash,
// lowercase. Empty and root-only entries are dropped; the scan root itself is
// never excluded. Absolute entries (POSIX "/x", Windows "\x" or "//host/share")
// are unsupported and dropped too: they cannot match a path relative to the
// scan root, and stripping the leading separator would silently re-anchor them
// to search_directory, hiding a subtree the user did not name. Drive-qualified
// entries ("D:/x", "D:\x", "D:x") are dropped for the same reason: on POSIX
// "D:" is a legal directory name, so "D:/x" would otherwise exclude
// <search_directory>/D:/x. Prefixing such an entry with "./" opts back in.
func normalizeExcludedPath(entry string) string {
	if isDriveQualified(entry) {
		return ""
	}
	p := path.Clean(strings.ReplaceAll(entry, `\`, "/"))
	if p == "" || p == "." || p == ".." || strings.HasPrefix(p, "/") {
		return ""
	}
	return strings.ToLower(p)
}

// isDriveQualified reports whether entry begins with a Windows drive prefix: a
// letter followed by a colon. Checked on the raw entry, so "./D:/x" is not
// drive-qualified and stays usable as a root-relative path.
func isDriveQualified(entry string) bool {
	if len(entry) < 2 || entry[1] != ':' {
		return false
	}
	c := entry[0]
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
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

	ex := buildExclusions(cfg)
	includedExtensions := buildIncludedExtensions(cfg)
	findScriptsWalk(root, "", &scripts, ex, includedExtensions)
	sort.Strings(scripts)
	return scripts
}

// findScriptsWalk does the actual recursion with prebuilt lookup sets. rel is
// dir's slash-separated path relative to the scan root ("" at the root) and is
// used to match root-relative exclusion entries.
func findScriptsWalk(dir, rel string, scripts *[]string, ex exclusions, includedExtensions map[string]bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		// Silently skip directories that cannot be read
		return
	}

	for _, entry := range entries {
		name := entry.Name()
		fullPath := filepath.Join(dir, name)

		if entry.IsDir() {
			if ex.names[strings.ToLower(name)] {
				continue
			}
			childRel := ""
			if len(ex.paths) > 0 {
				childRel = name
				if rel != "" {
					childRel = rel + "/" + name
				}
				if ex.paths[strings.ToLower(childRel)] {
					continue
				}
			}
			findScriptsWalk(fullPath, childRel, scripts, ex, includedExtensions)
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
