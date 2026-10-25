// Extension filtering, discovery recursion, and Python interpreter
// resolution tests for the suite Runny must always pass.
package tests

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"runny/src/core"
)

// TestIncludedExtensionsNormalize checks that extension input is normalized
// (lowercase, leading dot) and that discovery only accepts configured types.
func TestIncludedExtensionsNormalize(t *testing.T) {
	dir := t.TempDir()
	scriptDir := filepath.Join(dir, "scripts")
	if err := os.MkdirAll(scriptDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.py", "b.PYW", "c.pyi", "d.txt", "e.pyw"} {
		if err := os.WriteFile(filepath.Join(scriptDir, name), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := core.LoadConfig(dir)
	cfg.SearchDirectory = scriptDir
	cfg.IncludedExtensions = []string{" PYW ", ".PYI", "", "   "} // whitespace, no leading dots, mixed case, blanks

	base := map[string]bool{}
	for _, s := range core.FindScripts(scriptDir, cfg) {
		base[filepath.Base(s)] = true
	}
	if !base["e.pyw"] || !base["b.PYW"] || !base["c.pyi"] {
		t.Fatalf("configured extensions not honored (case/leading-dot normalization): %v", base)
	}
	if base["a.py"] || base["d.txt"] {
		t.Fatalf("non-configured extensions discovered: %v", base)
	}
}

// TestDiscoveryIsRecursiveAndSorted checks that scripts are found at depth and
// returned in sorted absolute-path order.
func TestDiscoveryIsRecursiveAndSorted(t *testing.T) {
	dir := t.TempDir()
	scriptDir := filepath.Join(dir, "scripts")
	deep := filepath.Join(scriptDir, "proj", "sub")
	if err := os.MkdirAll(deep, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scriptDir, "b.py"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(deep, "a.py"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(deep, "readme.md"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := core.LoadConfig(dir)
	cfg.SearchDirectory = scriptDir
	got := core.FindScripts(scriptDir, cfg)

	want := []string{
		filepath.Join(scriptDir, "b.py"),
		filepath.Join(deep, "a.py"),
	}
	if len(got) != len(want) {
		t.Fatalf("found %d scripts, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q (scripts = %v)", i, got[i], want[i], got)
		}
	}
}

// TestVenvWalksUpwardFromScript checks that the configured venv_dir is found
// walking upward from the script directory.
func TestVenvWalksUpwardFromScript(t *testing.T) {
	dir := t.TempDir()
	var venvPython string
	if runtime.GOOS == "windows" {
		venvPython = filepath.Join(dir, "virtual", "Scripts", "python.exe")
	} else {
		venvPython = filepath.Join(dir, "virtual", "bin", "python3")
	}
	if err := os.MkdirAll(filepath.Dir(venvPython), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(venvPython, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := core.LoadConfig(dir)
	cfg.VenvDir = "virtual"
	script := filepath.Join(dir, "proj", "s.py")

	if got := core.FindPythonForScript(script, dir, false, cfg); got != venvPython {
		t.Fatalf("venv lookup = %q, want %q", got, venvPython)
	}

	// The default venv name must NOT resolve to the custom directory, so the
	// lookup falls back to the system interpreter.
	defaultCfg := core.LoadConfig(dir)
	wantFallback := "python.exe"
	if runtime.GOOS != "windows" {
		wantFallback = "python3"
	}
	if got := core.FindPythonForScript(script, dir, false, defaultCfg); got != wantFallback {
		t.Fatalf("default venv_dir lookup = %q, want fallback %q", got, wantFallback)
	}
}

// TestWalkStopsAtExeDir checks that the venv walk never looks above the
// application exe directory: a venv at the exe directory's parent must not be
// found for scripts living inside the exe directory tree.
func TestWalkStopsAtExeDir(t *testing.T) {
	base := t.TempDir()
	exeDir := filepath.Join(base, "app")
	scriptDir := filepath.Join(exeDir, "proj")
	if err := os.MkdirAll(scriptDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Venv lives at base (the exe directory's parent) — outside the boundary.
	sibling := filepath.Join(base, "virtual")
	if err := os.MkdirAll(sibling, 0755); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		if err := os.MkdirAll(filepath.Join(sibling, "Scripts"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sibling, "Scripts", "python.exe"), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	} else {
		if err := os.MkdirAll(filepath.Join(sibling, "bin"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sibling, "bin", "python3"), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := core.LoadConfig(exeDir)
	cfg.VenvDir = "virtual"
	script := filepath.Join(scriptDir, "s.py")

	want := "python.exe"
	if runtime.GOOS != "windows" {
		want = "python3"
	}
	if got := core.FindPythonForScript(script, exeDir, false, cfg); got != want {
		t.Fatalf("walk did not stop at exeDir boundary: got %q, want fallback %q", got, want)
	}
}

// TestPythonExecutableOverrideWins checks that a non-empty python_executable
// is used verbatim and disables venv/system detection.
func TestPythonExecutableOverrideWins(t *testing.T) {
	dir := t.TempDir()
	for _, parts := range [][]string{{"virtual", "Scripts", "python.exe"}, {"virtual", "bin", "python3"}} {
		full := filepath.Join(dir, filepath.Join(parts...))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := core.LoadConfig(dir)
	cfg.VenvDir = "virtual"
	cfg.PythonExecutable = "C:/tools/python.exe"

	// Matches nothing on disk: the override must be returned verbatim anyway.
	if got := core.FindPythonForScript(filepath.Join(dir, "s.py"), dir, false, cfg); got != "C:/tools/python.exe" {
		t.Fatalf("python_executable override not honored: %q", got)
	}
}

// TestFallbackToSystemPython checks that with no venv anywhere the system
// interpreter name is returned (platform-specific).
func TestFallbackToSystemPython(t *testing.T) {
	dir := t.TempDir()
	cfg := core.LoadConfig(dir)
	script := filepath.Join(dir, "s.py")

	wantDefault := "python.exe"
	wantPyw := "pythonw.exe"
	if runtime.GOOS != "windows" {
		wantDefault = "python3"
		wantPyw = "python3"
	}

	if got := core.FindPythonForScript(script, dir, false, cfg); got != wantDefault {
		t.Fatalf("fallback (foreground) = %q, want %q", got, wantDefault)
	}
	if got := core.FindPythonForScript(script, dir, true, cfg); got != wantPyw {
		t.Fatalf("fallback (usePythonW) = %q, want %q", got, wantPyw)
	}
}

// TestVenvPythonW checks that resolving a venv interpreter for .pyw scripts
// (usePythonW=true) resolves pythonw.exe on Windows and python3 on Unix.
func TestVenvPythonW(t *testing.T) {
	dir := t.TempDir()
	var venvPython string
	if runtime.GOOS == "windows" {
		venvPython = filepath.Join(dir, "virtual", "Scripts", "pythonw.exe")
	} else {
		venvPython = filepath.Join(dir, "virtual", "bin", "python3")
	}
	if err := os.MkdirAll(filepath.Dir(venvPython), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(venvPython, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := core.LoadConfig(dir)
	cfg.VenvDir = "virtual"
	script := filepath.Join(dir, "proj", "s.pyw")

	if got := core.FindPythonForScript(script, dir, true, cfg); got != venvPython {
		t.Fatalf("venv pythonw lookup = %q, want %q", got, venvPython)
	}
}

// TestFindScriptsNonExistentDirectory checks that scanning an invalid, missing,
// or empty root returns an empty slice without error or panic.
func TestFindScriptsNonExistentDirectory(t *testing.T) {
	dir := t.TempDir()
	cfg := core.LoadConfig(dir)

	nonExistent := filepath.Join(dir, "missing")
	if got := core.FindScripts(nonExistent, cfg); len(got) != 0 {
		t.Fatalf("FindScripts on missing dir returned %v, want empty", got)
	}
	if got := core.FindScripts("", cfg); len(got) != 0 {
		t.Fatalf("FindScripts on empty string returned %v, want empty", got)
	}
}

// TestFindScriptsDirectoryWithExtensionIgnored verifies that a directory whose
// name ends with a script extension (e.g. dir.py/) is recursed into but not
// returned as a script itself.
func TestFindScriptsDirectoryWithExtensionIgnored(t *testing.T) {
	dir := t.TempDir()
	scriptDir := filepath.Join(dir, "scripts")
	dirWithExt := filepath.Join(scriptDir, "pkg.py")
	if err := os.MkdirAll(dirWithExt, 0755); err != nil {
		t.Fatal(err)
	}
	nestedScript := filepath.Join(dirWithExt, "main.py")
	if err := os.WriteFile(nestedScript, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := core.LoadConfig(dir)
	cfg.SearchDirectory = scriptDir
	got := core.FindScripts(scriptDir, cfg)

	if len(got) != 1 || got[0] != nestedScript {
		t.Fatalf("FindScripts returned %v, want [%q]", got, nestedScript)
	}
}
