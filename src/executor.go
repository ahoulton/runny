package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"runny/src/core"
	"runny/src/input"
)

// runScript executes a Python script and returns the user's prompt input.Action.
func runScript(script string, originalWidth int, appExeDir, promptText string, cfg core.Config) input.Action {
	filename := filepath.Base(script)
	ext := strings.ToLower(filepath.Ext(script))
	isPyw := ext == ".pyw"

	// Determine if we need pythonw (Windows only, .pyw files)
	usePythonW := isPyw && runtime.GOOS == "windows"

	// Find the Python interpreter
	pythonExe := core.FindPythonForScript(script, appExeDir, usePythonW, cfg)

	// Finalize tcell before running the script
	finiTerminal()

	// Reset terminal width to original
	resizeTerminalWidth(originalWidth)

	// Set terminal title
	setTerminalTitle("Runny (" + filename + ")")

	// Clear the terminal
	clearTerminal()

	if isPyw {
		return runBackground(pythonExe, script, filename, promptText)
	}
	return runForeground(pythonExe, script, filename, promptText)
}

// runForeground executes a .py script in the foreground with stdin/stdout/stderr connected.
func runForeground(pythonExe, script, filename, promptText string) input.Action {
	cmd := exec.Command(pythonExe, script)
	cmd.Dir = filepath.Dir(script)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Ignore SIGINT in the parent so Ctrl+C passes through to the child
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT)

	err := cmd.Run()

	// Restore default signal handling
	signal.Stop(sigCh)
	signal.Reset(syscall.SIGINT)

	// Check exit status
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			exitCode := exitErr.ExitCode()
			// Exit code -1 on Unix (signal), 0xC000013A on Windows (STATUS_CONTROL_C_EXIT)
			if exitCode == -1 || (runtime.GOOS == "windows" && exitCode == 3221225786) {
				fmt.Printf("%s interrupted by user.\n", filename)
			} else {
				fmt.Printf("%s exited with an error code: %d\n", filename, exitCode)
			}
		}
	}

	fmt.Println()
	fmt.Println(promptText)

	return input.WaitPromptKey(nil)
}

// sigInterrupt converts a SIGINT notification channel into a cancel channel
// for waitPromptKey: the returned channel is closed when Ctrl+C is pressed.
// Call the returned stop func once the prompt finishes so the goroutine exits.
func sigInterrupt(sigCh <-chan os.Signal) (cancel <-chan struct{}, stop func()) {
	c := make(chan struct{})
	done := make(chan struct{})
	go func() {
		select {
		case <-sigCh:
			close(c)
		case <-done:
		}
	}()
	return c, func() { close(done) }
}

// runBackground starts a .pyw script as a managed background process.
// The script stays a child of Runny: pressing Ctrl+C at the prompt kills it.
func runBackground(pythonExe, script, filename, promptText string) input.Action {
	cmd := exec.Command(pythonExe, script)
	cmd.Dir = filepath.Dir(script)
	// Disconnect stdout/stderr for background execution
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	err := cmd.Start()
	if err != nil {
		fmt.Printf("Failed to start %s: %v\n", filename, err)
		return input.WaitPromptKey(nil)
	}

	// Reap the child process in a goroutine to prevent zombies on Unix
	go func() {
		err := cmd.Wait()
		if err != nil {
		}
	}()
	fmt.Printf("%s started as a background task.\n", filename)

	// Catch Ctrl+C. On Windows it arrives as a console control event rather
	// than console input, so route it through a cancel channel that
	// waitPromptKey can observe. Enter/R/Esc leave the script running.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT)
	cancel, stop := sigInterrupt(sigCh)
	action := input.WaitPromptKey(cancel)
	stop()
	signal.Stop(sigCh)
	signal.Reset(syscall.SIGINT)

	if action != input.ActionInterrupt {
		return action
	}

	// Ctrl+C: kill the managed script, then show the same prompt as a
	// foreground .py (Enter returns, R restarts, Esc exits).
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	fmt.Println()
	fmt.Println(promptText)
	return input.WaitPromptKey(nil)
}
