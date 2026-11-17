package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"runny/src/core"
	"runny/src/input"
	"runny/src/job"
	"runny/src/register"
)

var exeDir string

const defaultPrompt = "Press Enter to return to the launcher, R to restart, or Esc to exit..."
const directRunPrompt = "Press Enter to run again, R to restart, or Esc to exit..."

func main() {
	// Contain all spawned scripts in a job object so they cannot outlive Runny
	job.Init()

	exe, err := os.Executable()
	if err != nil {
		_, err := fmt.Fprintf(os.Stderr, "Failed to determine executable path: %v\n", err)
		if err != nil {
			return
		}
		os.Exit(1)
	}
	exeDir = filepath.Dir(exe)

	originalWidth := getTerminalWidth()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--register", "-register":
			if err := register.Register(); err != nil {
				_, err := fmt.Fprintf(os.Stderr, "Failed to register file associations: %v\n", err)
				if err != nil {
					return
				}
				os.Exit(1)
			}
			fmt.Println("Successfully registered Runny for .py and .pyw files.")
			return
		case "--unregister", "-unregister":
			if err := register.Unregister(); err != nil {
				_, err := fmt.Fprintf(os.Stderr, "Failed to unregister file associations: %v\n", err)
				if err != nil {
					return
				}
				os.Exit(1)
			}
			fmt.Println("Successfully unregistered Runny.")
			return
		}

		// Direct-run mode
		script, err := filepath.Abs(os.Args[1])
		if err == nil {
			if info, err := os.Stat(script); err == nil && !info.IsDir() {
				cfg := core.LoadConfig(exeDir)
				for {
					action := runScript(script, originalWidth, exeDir, directRunPrompt, cfg)
					if action == input.ActionEnter || action == input.ActionRestart {
						continue
					}
					break
				}
				return
			}
		}
	}

	mainLoop(originalWidth)
}

// mainLoop runs the interactive script launcher loop.
func mainLoop(originalWidth int) {
	cfg := core.LoadConfig(exeDir)
	searchRoots := cfg.SearchDirectory
	if len(searchRoots) == 0 {
		searchRoots = core.SearchPaths{exeDir}
	}

	for {
		scripts := core.FindAllScripts(searchRoots, cfg)

		if len(scripts) == 0 {
			clearTerminal()
			fmt.Printf("No Python scripts found in:\n  %s\n\nPlease select a directory to search.\n", strings.Join(searchRoots, "\n  "))

			newDir := core.PickDirectory()
			if newDir == "" {
				fmt.Println("Exiting launcher.")
				return
			}

			core.SaveSearchDirectory(exeDir, newDir)
			searchRoots = core.SearchPaths{newDir}
			continue
		}

		selected, ok := chooseScript(scripts, searchRoots, cfg)
		if !ok {
			fmt.Println("Exiting launcher.")
			return
		}

		for {
			action := runScript(selected, originalWidth, exeDir, defaultPrompt, cfg)
			if action == input.ActionRestart {
				continue
			}
			if action == input.ActionEnter {
				break
			}
			fmt.Println("Exiting launcher.")
			return
		}
	}
}
