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

type KeyAction int

const (
	ActionUp KeyAction = iota
	ActionDown
	ActionEnter
	ActionEsc
	ActionBackspace
	ActionRefresh
	ActionChar
	ActionAutoRefresh
)

type refreshScriptsEvent struct {
	scripts []string
}

type KeyPress struct {
	Action  KeyAction
	Char    rune
	Scripts []string // non-nil for ActionAutoRefresh
}

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

func finiTerminal() {
	if screen != nil {
		screen.Fini()
		screen = nil
	}
}

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
		case *tcell.EventInterrupt:
			if data, ok := ev.Data().(refreshScriptsEvent); ok {
				return KeyPress{Action: ActionAutoRefresh, Scripts: data.scripts}
			}
		case *tcell.EventResize:
			screen.Sync()
		}
	}
}

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

func setTerminalTitle(title string) {
	_, err := fmt.Fprintf(os.Stdout, "\033]0;%s\007", title)
	if err != nil {
		return
	}
}

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

func getMaxVisibleItems() int {
	h := getTerminalHeight() - 8
	if h < 5 {
		return 5
	}
	return h
}

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
