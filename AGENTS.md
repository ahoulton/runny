# AGENTS.md

Portable project instructions for AI agents. Tracked in git and shared by every checkout.
Machine-specific environment details (toolchain paths, IDE `projectPath`, local git quirks) live in the gitignored `AGENTS.local.md` next to this file.

Read that file too before running commands.

**Don't overdo it.** Make the minimal change asked for. Do not touch code paths you were not asked to touch, do not add features/messages/prompts that were not requested, and keep messaging consistent — reuse existing strings (e.g. `defaultPrompt` / `directRunPrompt` / `"%s interrupted by user."`) instead of inventing new wording.

**Keep the agent docs accurate.** This file MUST be updated after making changes that make anything within it invalid or incorrect, and `AGENTS.local.md` likewise for environment details. `README.md` is user-facing and tracked: keep it in step when CLI flags, config fields, key bindings, or release assets change.

**Prefer IDE tooling; verify with the CLI.** If your environment exposes a GoLand/JetBrains MCP server (`lint_files`, `get_file_problems`, `build_project`), use it as the first source for code inspection, linting, problem analysis and building. Fall back to the CLI checks in this file when that tooling is unavailable, fails, or cannot answer the question — for example it exposes no test runner and no cross-compilation-with-env, so `go test` and `GOOS`/`GOARCH` builds are always CLI work, or `build_project` returns "limited build diagnostics functionality" with no compiler messages. Either way, state in your reply which checks you actually ran and, when you fall back, why. Silent substitution is a violation, and so is skipping verification because "the CLI also works".

Runny: a Go TUI launcher for Python scripts. Cross-platform: compiles and runs natively on Windows, Linux, and macOS (darwin). The directory picker uses `github.com/ncruces/zenity` (pure Go, cross-platform); if a graphical dialog is unavailable, it falls back to an interactive console text prompt.

## Build & verify

- **Go toolchain.** `go.mod` requires `go 1.27.1`, and CI resolves the same version through `setup-go`'s `go-version-file`, so local and released builds agree. Do not assume `go`/`gofmt` are on `PATH` — resolve the toolchain from the `GOROOT` your IDE is configured with (or `go env GOROOT`) and, if the bare command is missing, invoke the absolute path under that `GOROOT`'s `bin` directory. Where `$HOME` is unset (notably Git Bash on Windows), `"$HOME/..."` expands to a broken `/...` path; use `~` or `$USERPROFILE` instead. The exact install path for this machine is in `AGENTS.local.md`.
- When IDE tooling is available, run it against changed files to catch compilation errors and inspection warnings, passing the repository root as `projectPath`; otherwise rely on the CLI checks below.
- Package `main` (the thin TUI/process entry point) lives in `./src`; the testable core (config, discovery, Python resolution) lives in `./src/core` (`package core`). Use `go build -o runny.exe ./src` on Windows (or `go build -o runny ./src` on Linux/macOS) — plain `go build ./src` emits `src.exe` (named after the directory), which is why a stray `src.exe` can appear if flags are omitted (it is gitignored, so it cannot be committed).
- There is a **formal test suite in `./tests`** (package `tests`) that must always pass; run it with `go test ./tests` after every change. The only CI is `.github/workflows/release.yml`, which runs on a `v*` tag push (or manual dispatch with an existing tag): it gates on `gofmt`, `go vet ./src/...` and `go test ./tests`, cross-builds windows/amd64, windows/arm64, linux/amd64, linux/arm64, darwin/amd64 and darwin/arm64 (`-trimpath -ldflags "-s -w"`, `CGO_ENABLED=0`), and publishes raw `runny_<tag>_<os>_<arch>` binaries (`.exe` on Windows) to a GitHub release. Pushes to `main` without a tag trigger none of this repo's workflows. No lint config beyond `gofmt`. Verification is IDE inspection (when available) alongside `go test ./tests` + `go build ./src` + `go vet ./src/...`, then a manual run.
- Cross-compilation can be verified from any OS:
  - Linux: `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o /dev/null ./src`
  - Linux ARM64: `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o /dev/null ./src`
  - macOS: `GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -o /dev/null ./src`
  - Windows: `GOOS=windows GOARCH=amd64 go build -o /dev/null ./src`
  - Windows ARM64: `GOOS=windows GOARCH=arm64 go build -o /dev/null ./src`
- The app is a TUI (tcell) that takes over the terminal — automated/manual runs need a real console, not a piped shell.

## Layout conventions

- Platform-specific code is organized into subpackages under `src/`, each containing a `windows.go` and `other.go` pair:
  - `src/job/` (`windows.go` / `other.go`): Process containment (Job Object on Windows, process group on Unix).
  - `src/register/` (`windows.go` / `other.go`): OS file associations (HKCU registry on Windows, unsupported stub on Unix).
  - `src/input/` (`windows.go` / `other.go`): Post-execution prompt key polling (`WaitPromptKey`) and `input.Action` type (`ActionEnter`, `ActionEsc`, `ActionRestart`, `ActionInterrupt`).
  - `src/core/` (`core.go`, `config.go`, `discovery.go`, `python.go`): Platform-independent, unit-tested core — config load/save/auto-upgrade, `FindScripts`, `FindPythonForScript`. No `windows.go`/`other.go` pair here.
- If you change function signatures in any of these subpackages, update both files in the pair or the other platform stops compiling.

## Runtime facts that matter when editing

- `runny_config.json` is read/written **next to the executable** (`exeDir`), not the working directory. It is gitignored and is not committed: it appears next to the exe on the first directory pick (so building into the repo root creates one there, untracked).
- Config schema (all fields optional; absent/empty fields fall back to built-in defaults via `applyDefaults` in `src/core/config.go`):
  - `search_directory` — the launcher's root scan directory (slash-normalized on disk).
  - `excluded_dirs` — directory *names* skipped during script discovery. **Full replacement semantics**: when present it replaces the built-in list entirely; when absent the defaults apply (`.venv`, `venv`, `env`, `.env`, `Lib`, `lib`, `site-packages`, `__pycache__`, `.git`). Matching is case-insensitive (see `buildIgnoredDirs` in `src/core/discovery.go`).
  - `included_extensions` — file extensions discovered (default `.py`/`.pyw`); normalized to lowercase with a leading dot. Valid for discovery filtering only — Windows file associations in `src/register/` stay `.py`/`.pyw` regardless.
  - `venv_dir` — venv directory name used by `findPythonForScript` (default `.venv`).
  - `python_executable` — when non-empty, used verbatim for every script and disables venv/system auto-detection. Empty = auto-detect.
- **Config auto-upgrade:** `loadConfig` writes back the completed option set (with defaults) whenever the on-disk file lacks any of the newer option keys, so old configs gain the new fields on first run and stay user-editable. The on-disk `search_directory` value is preserved verbatim by this write-back — even when the directory no longer exists — so an unavailable drive/documents-USB scenario survives. No config file is created if none exists (that still happens only on directory pick via `saveSearchDirectory`).
- `SaveSearchDirectory` must load the existing config first and update only `search_directory` — never write a fresh `Config`, or user options get wiped every time a directory is picked. `tests/config_test.go` locks this in.
- CLI modes: no args = interactive launcher; a path argument = direct-run loop for that script; `--register`/`--unregister` = registry ops then exit. All arg handling is in `main.go`.
- The tcell screen must be finalized (`finiTerminal()`) before spawning the child Python process, and terminal width is resized via ANSI escape `\033[8;rows;colst` (`terminal.go`) on Windows (`runtime.GOOS == "windows"`) — both are easy to break when touching execution flow in `executor.go`.
- Python resolution (`core.FindPythonForScript` in `src/core/python.go`): walks upward from the script's directory up to `exeDir` looking for the configured `venv_dir` (default `.venv`); falls back to system `python.exe`/`python3`. A non-empty `python_executable` config value bypasses detection entirely.
- `.pyw` files run via `pythonw.exe` on Windows (and `python3` on non-Windows) as **managed** background children (`runBackground` in `executor.go`): after launch, Runny waits for Enter/R/Esc (leaves the script running until Runny exits — process containment kills it, see traps below) or Ctrl+C (kills the child). The prompt (`promptText`) is shown **only on Ctrl+C**, not on execution. On Windows Ctrl+C is a console control event, not console input — that is why Runny catches it via `signal.Notify` -> cancel channel, and why `input.WaitPromptKey` polls `WaitForSingleObject` instead of blocking in `ReadConsoleInputW`.
- **Ctrl+C consistency rule:** Ctrl+C must behave identically for `.py` (foreground) and `.pyw` (background) runs — the script is interrupted/killed, then the user sees the same prompt (Enter = return/re-run, R = restart, Esc = exit). If you touch `runForeground`, `runBackground`, `input.WaitPromptKey`, or the signal handling in `executor.go`, keep the two paths in lockstep.
- Script discovery skips directories listed in the config's `excluded_dirs` (defaults: venv/`.git`/`__pycache__` etc.; see `buildIgnoredDirs` in `src/core/discovery.go`) and only accepts the config's `included_extensions` (defaults `.py`/`.pyw`).

## Other traps that bite agents

- **It IS a git repo.** `origin` points at the GitHub repository, the default branch is `main`, and releases are cut from `v*` tags. `git diff`/`git status` are available — but do not treat a commit or push as verification: no repo workflow runs on a plain push (see the CI note above), so the checks below still have to be run locally. `.gitignore` ignores `AGENTS.local.md` (the machine-specific half of these instructions), `.idea/`, `build/`, `runny_config.json` and `*.exe`; never force-add those. If git reports "dubious ownership" (common when a checkout is owned by a different user or SID than the one running git), add the path with `git config --global --add safe.directory <path>` rather than deleting `.git`. Line-ending warnings on commit ("LF will be replaced by CRLF") come from `core.autocrlf` and are expected, not a defect.
- **Hardcoded constants you must not "clean up":** the Win32 exit code `3221225786` (`0xC000013A` = STATUS_CONTROL_C_EXIT) in `executor.go` is how a Ctrl+C-killed `.py` child is detected — do not replace it with a "nicer" constant or remove the magic number. Same for the ANSI escape `\033[8;rows;colst` and the raw `syscall.NewLazyDLL` console calls in `src/input/windows.go`.
- **`signal.Notify` lifetime matters.** SIGINT is only captured while a script runs or while a `.pyw` prompt waits for the kill decision; it is deliberately stopped/reset afterward. If you move the `signal.Stop`/`signal.Reset` calls, Ctrl+C either leaks into the parent or stops being caught.
- **Process containment (`src/job/`).** `job.Init()` (called first in `main`) assigns Runny to an unnamed Windows job with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` (on Windows) or sets up process group signal traps (on Unix), so every spawned script — and its own children — dies whenever Runny exits, including a crash or kill signal. Do **not** remove containment; assignment is deliberately best-effort (one stderr warning, then continue without containment). Keep both `windows.go` and `other.go` in sync.
- **Directory picker dependency.** `github.com/ncruces/zenity` is pure Go and provides native file dialogs without cgo across all platforms. On headless systems or missing GUI utilities on Linux, it gracefully falls back to console prompt in `src/core/config.go`.
- **IDE unhandled-error inspection vs `syscall.Proc.Call`:** In Go on Windows, `proc.Call(...)` returns `(r1, r2 uintptr, err error)` where `err` is *always non-nil* (it returns an `error` interface containing `syscall.Errno(0)` even on successful Win32 calls). Never check `if err != nil` on `proc.Call` return values — that condition will unconditionally trigger. If an IDE inspection flags an unhandled error on a Win32 `proc.Call`, suppress it using blank assignments `_, _, _ = proc.Call(...)` or inspect the primary return value `r1`, never `if err != nil`.
