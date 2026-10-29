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
	cfg.SearchDirectory = core.SearchPaths{scriptDir}
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
	cfg.SearchDirectory = core.SearchPaths{scriptDir}
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
	cfg.SearchDirectory = core.SearchPaths{scriptDir}
	got := core.FindScripts(scriptDir, cfg)

	if len(got) != 1 || got[0] != nestedScript {
		t.Fatalf("FindScripts returned %v, want [%q]", got, nestedScript)
	}
}

// TestFindAllScriptsMultipleRoots checks that FindAllScripts discovers scripts across
// multiple search roots and deduplicates any overlaps.
func TestFindAllScriptsMultipleRoots(t *testing.T) {
	dir := t.TempDir()
	root1 := filepath.Join(dir, "root1")
	root2 := filepath.Join(dir, "root2")
	if err := os.MkdirAll(root1, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root2, 0755); err != nil {
		t.Fatal(err)
	}

	s1 := filepath.Join(root1, "alpha.py")
	s2 := filepath.Join(root2, "beta.py")
	if err := os.WriteFile(s1, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s2, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := core.LoadConfig(dir)
	got := core.FindAllScripts([]string{root1, root2}, cfg)
	want := []string{s1, s2}
	if len(got) != 2 || got[0] != s1 || got[1] != s2 {
		t.Fatalf("FindAllScripts = %v, want %v", got, want)
	}

	// Overlapping roots check (deduplication)
	gotOverlap := core.FindAllScripts([]string{root1, root1, root2}, cfg)
	if len(gotOverlap) != 2 || gotOverlap[0] != s1 || gotOverlap[1] != s2 {
		t.Fatalf("FindAllScripts deduplication failed: %v", gotOverlap)
	}
}

// TestGetRelPathDisambiguation checks that GetRelPath prefixes colliding relative
// paths with root identity, while keeping unique relative paths clean.
func TestGetRelPathDisambiguation(t *testing.T) {
	dir := t.TempDir()
	root1 := filepath.Join(dir, "Python")
	root2 := filepath.Join(dir, "example-secondary")
	if err := os.MkdirAll(filepath.Join(root1, "tools"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root2, "tools"), 0755); err != nil {
		t.Fatal(err)
	}

	s1 := filepath.Join(root1, "tools", "build.py")
	s2 := filepath.Join(root2, "tools", "build.py")
	s3 := filepath.Join(root1, "unique.py")

	if err := os.WriteFile(s1, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s2, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s3, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	roots := []string{root1, root2}

	// Colliding paths should include root identity
	got1 := core.GetRelPath(s1, roots)
	want1 := "[Python] tools/build.py"
	if got1 != want1 {
		t.Fatalf("GetRelPath(s1) = %q, want %q", got1, want1)
	}

	got2 := core.GetRelPath(s2, roots)
	want2 := "[example-secondary] tools/build.py"
	if got2 != want2 {
		t.Fatalf("GetRelPath(s2) = %q, want %q", got2, want2)
	}

	// Unique paths stay clean without prefix
	got3 := core.GetRelPath(s3, roots)
	want3 := "unique.py"
	if got3 != want3 {
		t.Fatalf("GetRelPath(s3) = %q, want %q", got3, want3)
	}

	// Single root has zero prefix even if named build.py
	gotSingle := core.GetRelPath(s1, []string{root1})
	if gotSingle != "tools/build.py" {
		t.Fatalf("GetRelPath single root = %q, want %q", gotSingle, "tools/build.py")
	}

	// Colliding base names fall back to full root path
	rootA := filepath.Join(dir, "repoA", "scripts")
	rootB := filepath.Join(dir, "repoB", "scripts")
	if err := os.MkdirAll(rootA, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(rootB, 0755); err != nil {
		t.Fatal(err)
	}
	sa := filepath.Join(rootA, "run.py")
	sb := filepath.Join(rootB, "run.py")
	if err := os.WriteFile(sa, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sb, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	rootsDup := []string{rootA, rootB}
	gotA := core.GetRelPath(sa, rootsDup)
	wantA := "[" + filepath.ToSlash(filepath.Clean(rootA)) + "] run.py"
	if gotA != wantA {
		t.Fatalf("GetRelPath(sa) = %q, want %q", gotA, wantA)
	}
	gotB := core.GetRelPath(sb, rootsDup)
	wantB := "[" + filepath.ToSlash(filepath.Clean(rootB)) + "] run.py"
	if gotB != wantB {
		t.Fatalf("GetRelPath(sb) = %q, want %q", gotB, wantB)
	}
}
