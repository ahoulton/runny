# Runny

A terminal launcher for Python scripts. Runny scans a directory tree for `.py`/`.pyw` files, lets you pick one with an arrow-key menu and type-to-filter search, then runs it with the right interpreter (project `.venv` first, system Python as fallback).

Cross-platform: Windows, Linux, macOS.

## Download

Grab the binary for your platform from the [latest release](https://github.com/ahoulton/runny/releases/latest):

| Platform | Asset |
|---|---|
| Windows x64 | `runny_<version>_windows_amd64.exe` |
| Windows ARM64 | `runny_<version>_windows_arm64.exe` |
| Linux x64 | `runny_<version>_linux_amd64` |
| Linux ARM64 | `runny_<version>_linux_arm64` |
| macOS Intel | `runny_<version>_darwin_amd64` |
| macOS Apple Silicon | `runny_<version>_darwin_arm64` |

**Unix (Linux/macOS): the downloaded file is not executable.** Mark it before running:

```sh
chmod +x runny_<version>_linux_amd64
./runny_<version>_linux_amd64
```

macOS additionally quarantines downloaded files; if Gatekeeper blocks it, run `xattr -d com.apple.quarantine <file>`.

## Usage

Run the binary with no arguments to start the launcher.

| Key | Action |
|---|---|
| `↑` / `↓` | Move selection |
| `Enter` | Run the selected script |
| `F5` | Refresh — re-scan for scripts created or deleted while the menu is open |
| type | Filter by filename |
| `Backspace` | Delete a filter character |
| `Esc` / `Ctrl+C` | Exit |

After a script finishes you get a prompt: `Enter` returns to the launcher, `R` restarts the script, `Esc` exits.

`.pyw` scripts on Windows run in the background via `pythonw.exe`; Runny stays alive until you press `Esc`, and `Ctrl+C` at the prompt kills the script.

If no scripts are found, Runny asks you to pick a directory (native dialog, or a text prompt when no GUI is available). The choice is saved.

### Command line

```sh
runny                       # interactive launcher
runny path/to/script.py     # run one script directly, then offer to run it again
runny --register            # associate .py/.pyw with Runny (Windows only)
runny --unregister          # remove those associations (Windows only)
```

## Configuration

`runny_config.json` is read from **next to the executable** and created the first time you pick a search directory. Every field is optional.

```json
{
    "search_directory": "C:/Users/me/scripts",
    "excluded_dirs": [".venv", "venv", "env", ".env", "Lib", "lib", "site-packages", "__pycache__", ".git"],
    "included_extensions": [".py", ".pyw"],
    "venv_dir": ".venv",
    "python_executable": ""
}
```

| Field | Meaning |
|---|---|
| `search_directory` | Root directory scanned for scripts. |
| `excluded_dirs` | Directories skipped during discovery: bare names match at any depth, paths such as `redirect/core` match relative to `search_directory`; absolute and drive-qualified paths are ignored. Replaces the built-in list entirely; matching is case-insensitive. |
| `included_extensions` | Extensions discovered (`.py`/`.pyw` by default). |
| `venv_dir` | Virtual environment directory name searched for when resolving Python. |
| `python_executable` | When set, used verbatim for every script and disables venv/system detection. |

Interpreter resolution walks upward from the script's directory (up to the executable's directory) looking for `<venv_dir>`, then falls back to `python`/`python3`.

## Build from source

Requires Go 1.27.1 or newer.

```sh
go build -o runny ./src          # ./runny.exe on Windows
go test ./tests
```

Cross-compilation (from any OS):

```sh
GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -o runny_linux ./src
GOOS=linux   GOARCH=arm64 CGO_ENABLED=0 go build -o runny_linux ./src
GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -o runny_mac   ./src
GOOS=windows GOARCH=amd64                go build -o runny.exe   ./src
GOOS=windows GOARCH=arm64                go build -o runny.exe   ./src
```

Pushing a `v*` tag triggers `.github/workflows/release.yml`, which verifies, cross-builds all six targets, and publishes them as release assets.

Note: Runny takes over the terminal while it runs, so it needs a real console — not a piped shell.
