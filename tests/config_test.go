// Package tests contains the formal test suite that Runny must always pass
// (run with `go test ./tests`). It exercises the exported behavior of the
// src/core package: configuration loading/defaults/auto-upgrade/saving.
package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"runny/src/core"
)

// writeConfigFile is a helper that writes a config file with slash-normalized
// values for use in a test.
func writeConfigFile(t *testing.T, dir string, content string) string {
	t.Helper()
	configPath := filepath.Join(dir, "runny_config.json")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return configPath
}

// mustParseConfig parses an on-disk config file into a core.Config.
func mustParseConfig(t *testing.T, path string) core.Config {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg core.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("config file is not valid JSON (%v):\n%s", err, data)
	}
	return cfg
}

// TestDefaultsAppliedWhenConfigMissing checks that loading with no config
// file at all yields the built-in defaults in memory (and creates nothing).
func TestDefaultsAppliedWhenConfigMissing(t *testing.T) {
	dir := t.TempDir()

	cfg := core.LoadConfig(dir)

	if cfg.SearchDirectory != "" {
		t.Fatalf("SearchDirectory = %q, want empty", cfg.SearchDirectory)
	}
	if cfg.VenvDir != core.DefaultVenvDir {
		t.Fatalf("VenvDir = %q, want %q", cfg.VenvDir, core.DefaultVenvDir)
	}
	if len(cfg.ExcludedDirs) != len(core.DefaultExcludedDirs()) {
		t.Fatalf("ExcludedDirs = %v, want built-in defaults", cfg.ExcludedDirs)
	}
	if len(cfg.IncludedExtensions) != len(core.DefaultIncludedExtensions()) {
		t.Fatalf("IncludedExtensions = %v, want built-in defaults", cfg.IncludedExtensions)
	}
	if cfg.PythonExecutable != "" {
		t.Fatalf("PythonExecutable = %q, want empty (auto-detect)", cfg.PythonExecutable)
	}

	// No config file should be created just by loading.
	if _, err := os.Stat(filepath.Join(dir, "runny_config.json")); !os.IsNotExist(err) {
		t.Fatalf("LoadConfig must not create a config file when none exists")
	}
}

// TestOldConfigAutoUpgradesOnDisk checks that a pre-2026 config file
// (search_directory only) gains the newer option keys with defaults the
// first time it is loaded, and is left alone on subsequent loads.
func TestOldConfigAutoUpgradesOnDisk(t *testing.T) {
	dir := t.TempDir()
	scriptDir := filepath.Join(dir, "scripts")
	if err := os.MkdirAll(scriptDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := writeConfigFile(t, dir,
		`{"search_directory": "`+filepath.ToSlash(scriptDir)+`"}`)

	cfg := core.LoadConfig(dir)

	// In memory: full defaults
	if cfg.VenvDir != core.DefaultVenvDir || len(cfg.IncludedExtensions) == 0 || len(cfg.ExcludedDirs) == 0 {
		t.Fatalf("defaults not applied in memory: %+v", cfg)
	}
	if cfg.SearchDirectory != scriptDir {
		t.Fatalf("SearchDirectory = %q, want %q", cfg.SearchDirectory, scriptDir)
	}

	// On disk: same search directory, plus the new option keys
	disk := mustParseConfig(t, configPath)
	if disk.SearchDirectory != filepath.ToSlash(scriptDir) {
		t.Fatalf("upgrade must preserve search_directory on disk, got %q", disk.SearchDirectory)
	}
	if disk.VenvDir != core.DefaultVenvDir || len(disk.ExcludedDirs) == 0 || len(disk.IncludedExtensions) == 0 {
		t.Fatalf("config file not upgraded with defaults:\n%s", mustReadAll(t, configPath))
	}

	// Second load: everything present, file must not be rewritten again
	infoBefore, err := os.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)

	before := mustReadAll(t, configPath)
	_ = core.LoadConfig(dir)
	if after := mustReadAll(t, configPath); before != after {
		t.Fatalf("config rewritten despite all options present:\n%s\n===\n%s", before, after)
	}
	infoAfter, err := os.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !infoBefore.ModTime().Equal(infoAfter.ModTime()) {
		t.Fatalf("config file was rewritten on disk during second load: modtime changed from %v to %v",
			infoBefore.ModTime(), infoAfter.ModTime())
	}
}

// TestUpgradePreservesMissingSearchDir checks the unavailable-drive/USB
// scenario: an on-disk search_directory that points at a directory that no
// longer keeps existing on disk, even though LoadConfig reports it as empty.
func TestUpgradePreservesMissingSearchDir(t *testing.T) {
	dir := t.TempDir()
	goneDir := filepath.Join(dir, "usb-gone")
	configPath := writeConfigFile(t, dir,
		`{"search_directory": "`+filepath.ToSlash(goneDir)+`"}`)

	cfg := core.LoadConfig(dir)

	if cfg.SearchDirectory != "" {
		t.Fatalf("SearchDirectory = %q, want empty for a missing directory", cfg.SearchDirectory)
	}

	disk := mustParseConfig(t, configPath)
	if disk.SearchDirectory != filepath.ToSlash(goneDir) {
		t.Fatalf("upgrade must not erase an unavailable search_directory, got %q", disk.SearchDirectory)
	}
	if len(disk.ExcludedDirs) == 0 || disk.VenvDir != core.DefaultVenvDir {
		t.Fatalf("upgrade options missing:\n%s", mustReadAll(t, configPath))
	}
}

// TestExclusionListFullReplacement checks that a config-supplied excluded_dirs
// replaces the built-in list entirely: custom names are skipped, and previously
// default-excluded names (env, lib) are discovered again.
func TestExclusionListFullReplacement(t *testing.T) {
	dir := t.TempDir()
	scriptDir := filepath.Join(dir, "scripts")
	for _, name := range []string{"node_modules", "lib", "env", "ns", ".git"} {
		if err := os.MkdirAll(filepath.Join(scriptDir, name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{
		"top.py",
		"ns/in_ns.py",
		"node_modules/in_nm.py",
		"lib/in_lib.py",
		"env/in_env.py",
		".git/hidden.py",
	} {
		if err := os.WriteFile(filepath.Join(scriptDir, filepath.FromSlash(p)), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := core.LoadConfig(dir) // defaults
	cfg.ExcludedDirs = []string{"node_modules", ".git"}
	cfg.SearchDirectory = scriptDir

	have := map[string]bool{}
	for _, s := range core.FindScripts(scriptDir, cfg) {
		rel, err := filepath.Rel(scriptDir, s)
		if err != nil {
			t.Fatal(err)
		}
		have[filepath.ToSlash(rel)] = true
	}

	for _, must := range []string{"top.py", "ns/in_ns.py", "lib/in_lib.py", "env/in_env.py"} {
		if !have[must] {
			t.Errorf("%s missing: full replacement must re-include previously-default skipped dirs (have %v)", must, have)
		}
	}
	for _, banned := range []string{"node_modules/in_nm.py", ".git/hidden.py"} {
		if have[banned] {
			t.Errorf("%s must stay excluded by the replacement list (have %v)", banned, have)
		}
	}
}

// TestExcludedDirsCaseInsensitive checks that directory-name exclusion matches
// case-insensitively on every platform.
func TestExcludedDirsCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	scriptDir := filepath.Join(dir, "scripts")
	for _, name := range []string{"Build", "logs"} {
		if err := os.MkdirAll(filepath.Join(scriptDir, name), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(scriptDir, name, "x.py"), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := core.LoadConfig(dir)
	cfg.SearchDirectory = scriptDir
	cfg.ExcludedDirs = []string{"BUILD", "Logs"}

	if got := core.FindScripts(scriptDir, cfg); len(got) != 0 {
		t.Fatalf("case-insensitive exclusion failed, found: %v", got)
	}
}

// TestSavePreservesOptionsAndUpdatesSearchDir checks that re-picking a
// directory never wipes user options, and adds them for a from-scratch config.
func TestSavePreservesOptionsAndUpdatesSearchDir(t *testing.T) {
	dir := t.TempDir()
	scriptDir := filepath.Join(dir, "scripts")
	otherDir := filepath.Join(dir, "other")
	configPath := writeConfigFile(t, dir,
		`{"search_directory": "`+filepath.ToSlash(scriptDir)+`",`+
			`"excluded_dirs": ["node_modules"],`+
			`"included_extensions": [".py"],`+
			`"venv_dir": "virtual",`+
			`"python_executable": "custom.exe"}`)

	_ = core.LoadConfig(dir) // triggers the write-back of identical values
	core.SaveSearchDirectory(dir, otherDir)

	disk := mustParseConfig(t, configPath)
	if disk.SearchDirectory != filepath.ToSlash(otherDir) {
		t.Fatalf("search_directory not updated, got %q", disk.SearchDirectory)
	}
	if len(disk.ExcludedDirs) != 1 || disk.ExcludedDirs[0] != "node_modules" {
		t.Fatalf("excluded_dirs wiped by save: %v", disk.ExcludedDirs)
	}
	if len(disk.IncludedExtensions) != 1 || disk.IncludedExtensions[0] != ".py" {
		t.Fatalf("included_extensions wiped by save: %v", disk.IncludedExtensions)
	}
	if disk.VenvDir != "virtual" {
		t.Fatalf("venv_dir wiped by save: %q", disk.VenvDir)
	}
	if disk.PythonExecutable != "custom.exe" {
		t.Fatalf("python_executable wiped by save: %q", disk.PythonExecutable)
	}
}

// TestSaveCreatesConfigWithFullOptions checks that saving into a directory
// with no config file writes one containing every option.
func TestSaveCreatesConfigWithFullOptions(t *testing.T) {
	dir := t.TempDir()

	core.SaveSearchDirectory(dir, filepath.Join(dir, "scripts"))

	disk := mustParseConfig(t, filepath.Join(dir, "runny_config.json"))
	if disk.SearchDirectory == "" {
		t.Fatal("saved search directory missing")
	}
	if len(disk.ExcludedDirs) == 0 || len(disk.IncludedExtensions) == 0 || disk.VenvDir != core.DefaultVenvDir {
		t.Fatalf("saved config incomplete: %+v", disk)
	}
}

// TestSaveConfigDirect checks that SaveConfig directly serializes a Config struct
// with all fields to runny_config.json.
func TestSaveConfigDirect(t *testing.T) {
	dir := t.TempDir()
	cfg := core.Config{
		SearchDirectory:    filepath.ToSlash(filepath.Join(dir, "scripts")),
		ExcludedDirs:       []string{"custom_skip"},
		IncludedExtensions: []string{".py"},
		VenvDir:            "custom_venv",
		PythonExecutable:   "C:/custom/python.exe",
	}

	core.SaveConfig(dir, cfg)

	disk := mustParseConfig(t, filepath.Join(dir, "runny_config.json"))
	if disk.SearchDirectory != cfg.SearchDirectory {
		t.Fatalf("SearchDirectory = %q, want %q", disk.SearchDirectory, cfg.SearchDirectory)
	}
	if len(disk.ExcludedDirs) != 1 || disk.ExcludedDirs[0] != "custom_skip" {
		t.Fatalf("ExcludedDirs = %v, want %v", disk.ExcludedDirs, cfg.ExcludedDirs)
	}
	if len(disk.IncludedExtensions) != 1 || disk.IncludedExtensions[0] != ".py" {
		t.Fatalf("IncludedExtensions = %v, want %v", disk.IncludedExtensions, cfg.IncludedExtensions)
	}
	if disk.VenvDir != "custom_venv" {
		t.Fatalf("VenvDir = %q, want %q", disk.VenvDir, "custom_venv")
	}
	if disk.PythonExecutable != "C:/custom/python.exe" {
		t.Fatalf("PythonExecutable = %q, want %q", disk.PythonExecutable, "C:/custom/python.exe")
	}
}

// TestLoadConfigMalformedJSON checks that if runny_config.json contains invalid
// JSON, LoadConfig does not panic and returns in-memory defaults.
func TestLoadConfigMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	writeConfigFile(t, dir, "{invalid-json-content")

	cfg := core.LoadConfig(dir)

	if cfg.SearchDirectory != "" {
		t.Fatalf("SearchDirectory = %q, want empty", cfg.SearchDirectory)
	}
	if cfg.VenvDir != core.DefaultVenvDir {
		t.Fatalf("VenvDir = %q, want %q", cfg.VenvDir, core.DefaultVenvDir)
	}
	if len(cfg.ExcludedDirs) != len(core.DefaultExcludedDirs()) {
		t.Fatalf("ExcludedDirs = %v, want defaults", cfg.ExcludedDirs)
	}
	if len(cfg.IncludedExtensions) != len(core.DefaultIncludedExtensions()) {
		t.Fatalf("IncludedExtensions = %v, want defaults", cfg.IncludedExtensions)
	}
}

// TestLoadConfigSearchDirPointsToFile checks that if search_directory points to
// a regular file rather than a directory, LoadConfig treats it as empty in memory.
func TestLoadConfigSearchDirPointsToFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "some_file.txt")
	if err := os.WriteFile(filePath, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	configPath := writeConfigFile(t, dir,
		`{"search_directory": "`+filepath.ToSlash(filePath)+`"}`)

	cfg := core.LoadConfig(dir)

	if cfg.SearchDirectory != "" {
		t.Fatalf("SearchDirectory = %q, want empty when pointing to a regular file", cfg.SearchDirectory)
	}

	disk := mustParseConfig(t, configPath)
	if disk.SearchDirectory != filepath.ToSlash(filePath) {
		t.Fatalf("on-disk search_directory = %q, want %q preserved", disk.SearchDirectory, filepath.ToSlash(filePath))
	}
}

func mustReadAll(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
