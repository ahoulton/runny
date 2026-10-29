package core

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ncruces/zenity"
)

// SearchPaths represents one or more directories to search for scripts.
// In JSON, it can be unmarshaled from either a single string or an array of strings.
type SearchPaths []string

// UnmarshalJSON unmarshals either a JSON string or a JSON array of strings into SearchPaths,
// preserving path values as written.
func (s *SearchPaths) UnmarshalJSON(data []byte) error {
	// Try single string first
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		if single == "" {
			*s = nil
		} else {
			*s = SearchPaths{single}
		}
		return nil
	}

	// Try array of strings
	var list []string
	if err := json.Unmarshal(data, &list); err == nil {
		var paths SearchPaths
		for _, item := range list {
			if item != "" {
				paths = append(paths, item)
			}
		}
		*s = paths
		return nil
	}

	return fmt.Errorf("search_directory must be a string or list of strings")
}

// MarshalJSON marshals SearchPaths as a single string if there is at most one path,
// or as an array of strings if there are multiple paths.
func (s SearchPaths) MarshalJSON() ([]byte, error) {
	if len(s) == 0 {
		return json.Marshal("")
	}
	if len(s) == 1 {
		return json.Marshal(s[0])
	}
	return json.Marshal([]string(s))
}

// Config represents the persistent configuration for Runny.
type Config struct {
	SearchDirectory    SearchPaths `json:"search_directory"`
	ExcludedDirs       []string    `json:"excluded_dirs"` // names at any depth, or paths relative to SearchDirectory
	IncludedExtensions []string    `json:"included_extensions"`
	VenvDir            string      `json:"venv_dir"`
	PythonExecutable   string      `json:"python_executable"`
	AutoRefresh        *bool       `json:"auto_refresh"`
}

// DefaultExcludedDirs returns the built-in directory names skipped during
// script discovery, used when excluded_dirs is absent from the config file.
func DefaultExcludedDirs() []string {
	return []string{
		".venv",
		"venv",
		"env",
		".env",
		"Lib",
		"lib",
		"site-packages",
		"__pycache__",
		".git",
	}
}

// DefaultIncludedExtensions returns the script file extensions Runny
// discovers, used when included_extensions is absent or invalid.
func DefaultIncludedExtensions() []string {
	return []string{".py", ".pyw"}
}

// DefaultVenvDir is the virtual environment directory name searched for
// while resolving the Python interpreter.
const DefaultVenvDir = ".venv"

// DefaultAutoRefresh is whether the script list is automatically refreshed in the background.
const DefaultAutoRefresh = true

// applyDefaults fills in defaults for any option fields that are absent
// from the config file, keeping old config files working unchanged.
func (c *Config) applyDefaults() {
	if c.ExcludedDirs == nil {
		c.ExcludedDirs = DefaultExcludedDirs()
	}
	if len(c.IncludedExtensions) == 0 {
		c.IncludedExtensions = DefaultIncludedExtensions()
	}
	if c.VenvDir == "" {
		c.VenvDir = DefaultVenvDir
	}
	if c.AutoRefresh == nil {
		v := DefaultAutoRefresh
		c.AutoRefresh = &v
	}
	// PythonExecutable intentionally has no default: an empty value means
	// auto-detect the interpreter (venv walk, then system Python).
}

// LoadConfig reads the config file and returns it with defaults applied for
// any missing option. If the on-disk file lacks one of the newer options, the
// completed set of options is written back to the file so the user can see
// and edit them. The saved search_directory value is preserved verbatim —
// even when it points to a directory that no longer exists (it is only
// dropped at load time, not erased from disk).
func LoadConfig(dir string) Config {
	configPath := filepath.Join(dir, configFileName)

	var cfg Config
	var raw map[string]json.RawMessage

	data, err := os.ReadFile(configPath)
	fileExists := err == nil
	if fileExists {
		_ = json.Unmarshal(data, &cfg)
		_ = json.Unmarshal(data, &raw)
	}

	// Remember the on-disk search_directory so the upgrade write-back below,
	// and nothing else, loses it when the directory is temporarily missing.
	savedSearch := slices.Clone(cfg.SearchDirectory)

	// Auto-upgrade: an old config (or any config missing an option) gets the
	// newer option keys added with their defaults, once, right here.
	_, hasPythonExe := raw["python_executable"]
	_, hasAutoRefresh := raw["auto_refresh"]
	needsUpgrade := fileExists &&
		(cfg.ExcludedDirs == nil || len(cfg.IncludedExtensions) == 0 || cfg.VenvDir == "" || !hasPythonExe || !hasAutoRefresh)
	cfg.applyDefaults()

	// Convert forward slashes back to OS-specific path and validate that
	// directories still exist; otherwise drop them in memory (the launcher will
	// prompt for one if none remain).
	var validDirs SearchPaths
	for _, p := range cfg.SearchDirectory {
		nativePath := filepath.FromSlash(p)
		if nativePath != "" {
			info, err := os.Stat(nativePath)
			if err == nil && info.IsDir() {
				validDirs = append(validDirs, nativePath)
			}
		}
	}
	cfg.SearchDirectory = validDirs
	if needsUpgrade {
		upgraded := cfg
		upgraded.SearchDirectory = savedSearch
		SaveConfig(dir, upgraded)
	}

	return cfg
}

// SaveConfig writes the configuration to the config file, preserving any
// user options (search_directory, excluded_dirs, ...) rather than erasing
// them with a fresh struct.
func SaveConfig(dir string, cfg Config) {
	data, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return
	}

	configPath := filepath.Join(dir, configFileName)
	_ = os.WriteFile(configPath, data, 0644)
}

// SaveSearchDirectory updates only the search directory in the config file,
// keeping all other user options exactly as they currently are on disk.
func SaveSearchDirectory(dir, searchDir string) {
	SaveSearchDirectories(dir, []string{searchDir})
}

// SaveSearchDirectories updates the search directories in the config file,
// keeping all other user options exactly as they currently are on disk.
func SaveSearchDirectories(dir string, searchDirs []string) {
	// Load the on-disk config so custom options survive the save.
	cfg := LoadConfig(dir)

	var paths SearchPaths
	for _, searchDir := range searchDirs {
		paths = append(paths, filepath.ToSlash(searchDir))
	}
	cfg.SearchDirectory = paths

	SaveConfig(dir, cfg)
}

// PickDirectory opens a native directory picker dialogue.
// Returns the selected directory path or an empty string if cancelled.
func PickDirectory() string {
	// Try native dialog first
	dir, err := zenity.SelectFile(
		zenity.Directory(),
		zenity.Title("Select Python Script Directory"),
	)
	if err == nil && dir != "" {
		return dir
	}

	// Fall back to text prompt on all platforms if dialog is unavailable/cancelled
	fmt.Print("Enter directory path to search for scripts (blank to cancel): ")
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)

	if line == "" {
		return ""
	}

	absPath, err := filepath.Abs(line)
	if err != nil {
		return ""
	}

	info, err := os.Stat(absPath)
	if err != nil || !info.IsDir() {
		fmt.Println("Invalid directory path.")
		return ""
	}

	return absPath
}
