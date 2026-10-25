package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/gdamore/tcell/v2"
	"golang.org/x/term"
)

var screen tcell.Screen

// KeyAction represents the type of key pressed
type KeyAction int

const (
	ActionUp KeyAction = iota
	ActionDown
	ActionEnter
	ActionEsc
	ActionBackspace
	ActionRefresh
	ActionChar
)

// KeyPress holds information about a single key press
type KeyPress struct {
	Action KeyAction
	Char   rune
}

// initTerminal creates and initializes the tcell screen.
func initTerminal() {
	if screen != nil {
		return
	}
	var err error
	screen, err = tcell.NewScreen()
	if err != nil {
		_, err := fmt.Fprintf(os.Stderr, "Failed to initialize terminal: %v\n", err)
		if err != nil {
			return
		}
		os.Exit(1)
	}
	if err := screen.Init(); err != nil {
		_, err := fmt.Fprintf(os.Stderr, "Failed to initialize screen: %v\n", err)
		if err != nil {
			return
		}
		os.Exit(1)
	}
}

// finiTerminal finalizes the screen and restores the terminal to normal mode.
func finiTerminal() {
	if screen != nil {
		screen.Fini()
		screen = nil
	}
}

// readKey blocks until the user presses a key and returns the KeyPress.
func readKey() KeyPress {
	for {
		ev := screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			switch ev.Key() {
			case tcell.KeyUp:
				return KeyPress{Action: ActionUp}
			case tcell.KeyDown:
				return KeyPress{Action: ActionDown}
			case tcell.KeyEnter:
				return KeyPress{Action: ActionEnter}
			case tcell.KeyEscape:
				return KeyPress{Action: ActionEsc}
			case tcell.KeyCtrlC:
				return KeyPress{Action: ActionEsc}
			case tcell.KeyBackspace, tcell.KeyBackspace2:
				return KeyPress{Action: ActionBackspace}
			case tcell.KeyF5:
				return KeyPress{Action: ActionRefresh}
			case tcell.KeyRune:
				return KeyPress{Action: ActionChar, Char: ev.Rune()}
			default:
			}
		case *tcell.EventResize:
			screen.Sync()
		}
	}
}

// clearTerminal clears the terminal screen.
func clearTerminal() {
	if screen != nil {
		screen.Clear()
		screen.Sync()
		return
	}
	// tcell is not active; use platform-specific command
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls")
	} else {
		cmd = exec.Command("clear")
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

// setTerminalTitle sets the console window title using ANSI escape sequences.
func setTerminalTitle(title string) {
	_, err := fmt.Fprintf(os.Stdout, "\033]0;%s\007", title)
	if err != nil {
		return
	}
}

// getTerminalWidth returns the current terminal width in columns.
func getTerminalWidth() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return w
	}
	if screen != nil {
		w, _ := screen.Size()
		if w > 0 {
			return w
		}
	}
	return 120
}

// getTerminalHeight returns the current terminal height in rows.
func getTerminalHeight() int {
	if _, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil && h > 0 {
		return h
	}
	if screen != nil {
		_, h := screen.Size()
		if h > 0 {
			return h
		}
	}
	return 40
}

// getMaxVisibleItems returns the maximum number of menu items that can be displayed.
func getMaxVisibleItems() int {
	h := getTerminalHeight() - 8
	if h < 5 {
		return 5
	}
	return h
}

// resizeTerminalWidth dynamically resizes the terminal window width.
func resizeTerminalWidth(cols int) {
	if runtime.GOOS != "windows" {
		return
	}
	rows := getTerminalHeight()
	if rows <= 0 {
		rows = 40
	}
	_, err := fmt.Fprintf(os.Stdout, "\033[8;%d;%dt", rows, cols)
	if err != nil {
		return
	}
}
