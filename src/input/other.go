//go:build !windows

package input

import (
	"os"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

type Action int

const (
	ActionEnter Action = iota
	ActionEsc
	ActionRestart
	ActionInterrupt
)

func WaitPromptKey(cancel <-chan struct{}) Action {
	fd := int(os.Stdin.Fd())

	// Raw mode delivers single key presses (including Ctrl+C as byte 3)
	// immediately. If it fails, fall back to line-buffered reads.
	oldState, err := term.MakeRaw(fd)
	if err == nil {
		defer term.Restore(fd, oldState)
	}

	var buf [1]byte
	for {
		if cancel != nil {
			select {
			case <-cancel:
				return ActionInterrupt
			default:
			}
		}

		// Poll with a timeout so the cancel channel can be observed even
		// while stdin has no pending input.
		fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		n, err := unix.Poll(fds, 50)
		if err != nil || n == 0 {
			continue
		}

		n, err = os.Stdin.Read(buf[:])
		if err != nil || n == 0 {
			return ActionEsc
		}
		switch buf[0] {
		case '\r', '\n':
			return ActionEnter
		case 27:
			return ActionEsc
		case 3:
			return ActionInterrupt
		case 'r', 'R':
			return ActionRestart
		}
	}
}
