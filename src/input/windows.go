//go:build windows

package input

import (
	"syscall"
	"unsafe"
)

var (
	kernel32                    = syscall.NewLazyDLL("kernel32.dll")
	procGetStdHandle            = kernel32.NewProc("GetStdHandle")
	procReadConsoleInputW       = kernel32.NewProc("ReadConsoleInputW")
	procFlushConsoleInputBuffer = kernel32.NewProc("FlushConsoleInputBuffer")
	procWaitForSingleObject     = kernel32.NewProc("WaitForSingleObject")
)

const (
	stdInputHandle = uint32(0xFFFFFFF6) // -10 (STD_INPUT_HANDLE)
	keyEventRecord = 0x0001
	vkReturn       = 0x0D
	vkEscape       = 0x1B
	vkR            = 0x52
	waitObject0    = 0x00000000
	waitFailed     = 0xFFFFFFFF
)

type inputRecord struct {
	eventType uint16
	pad       uint16
	event     [16]byte
}

type keyEventRecordStruct struct {
	bKeyDown          int32
	wRepeatCount      uint16
	wVirtualKeyCode   uint16
	wVirtualScanCode  uint16
	unicodeChar       uint16
	dwControlKeyState uint32
}

// Action represents an action chosen by the user at the post-script prompt.
type Action int

const (
	ActionEnter Action = iota
	ActionEsc
	ActionRestart
	ActionInterrupt
)

// WaitPromptKey waits for a key press after script execution on Windows.
// If cancel is closed while waiting, it returns ActionInterrupt.
func WaitPromptKey(cancel <-chan struct{}) Action {
	hIn, _, _ := procGetStdHandle.Call(uintptr(stdInputHandle))
	if hIn == 0 || hIn == uintptr(syscall.InvalidHandle) {
		return ActionEnter
	}

	// Flush any pending input in the console buffer
	_, _, _ = procFlushConsoleInputBuffer.Call(hIn)

	var rec inputRecord
	var eventsRead uint32

	for {
		// A closed cancel channel means the caller caught Ctrl+C via a signal.
		if cancel != nil {
			select {
			case <-cancel:
				return ActionInterrupt
			default:
			}
		}

		// Wait briefly for console input so the cancel channel can be polled.
		// (Ctrl+C is a console control event, not console input, so it would
		// never show up in ReadConsoleInputW on its own.)
		r, _, _ := procWaitForSingleObject.Call(hIn, 50)
		if r == waitFailed {
			return ActionEsc
		}
		if r != waitObject0 {
			continue
		}

		r, _, _ = procReadConsoleInputW.Call(
			hIn,
			uintptr(unsafe.Pointer(&rec)),
			1,
			uintptr(unsafe.Pointer(&eventsRead)),
		)
		if r == 0 || eventsRead == 0 {
			return ActionEsc
		}

		if rec.eventType == keyEventRecord {
			ke := (*keyEventRecordStruct)(unsafe.Pointer(&rec.event[0]))
			if ke.bKeyDown != 0 {
				switch ke.wVirtualKeyCode {
				case vkReturn:
					return ActionEnter
				case vkEscape:
					return ActionEsc
				case vkR:
					return ActionRestart
				}
				switch ke.unicodeChar {
				case '\r', '\n':
					return ActionEnter
				case 27: // Esc
					return ActionEsc
				case 3: // Ctrl+C (only delivered if processed input is off)
					return ActionInterrupt
				case 'r', 'R':
					return ActionRestart
				}
			}
		}
	}
}
