package core

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// FindPythonForScript searches for a virtual environment Python interpreter.
// It walks upward from the script's directory to exeDir, looking for the
// configured venv directory. If the config specifies an explicit
// python_executable, it is returned verbatim and no detection happens.
func FindPythonForScript(script, appExeDir string, usePythonW bool, cfg Config) string {
	// Explicit interpreter override from the config takes precedence.
	if cfg.PythonExecutable != "" {
		return cfg.PythonExecutable
	}

	scriptDir := filepath.Dir(script)
	currentDir := scriptDir

	// Normalize exeDir for comparison
	normExeDir, _ := filepath.Abs(appExeDir)

	for {
		var venvPython string
		if runtime.GOOS == "windows" {
			if usePythonW {
				venvPython = filepath.Join(currentDir, cfg.VenvDir, "Scripts", "pythonw.exe")
			} else {
				venvPython = filepath.Join(currentDir, cfg.VenvDir, "Scripts", "python.exe")
			}
		} else {
			venvPython = filepath.Join(currentDir, cfg.VenvDir, "bin", "python3")
		}

		// Check if the venv Python exists
		if info, err := os.Stat(venvPython); err == nil && !info.IsDir() {
			return venvPython
		}

		// Check if we've reached exeDir (boundary)
		normCurrent, _ := filepath.Abs(currentDir)
		if strings.EqualFold(normCurrent, normExeDir) {
			break
		}

		// Move to parent directory
		parent := filepath.Dir(currentDir)
		if parent == currentDir {
			// Reached filesystem root
			break
		}
		currentDir = parent
	}

	// Fall back to system Python
	if runtime.GOOS == "windows" {
		if usePythonW {
			return "pythonw.exe"
		}
		return "python.exe"
	}
	return "python3"
}
