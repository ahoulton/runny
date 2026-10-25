//go:build !windows

package job

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// Init sets up process containment on Unix platforms.
// It traps SIGTERM and SIGHUP to ensure the process group is killed on exit.
func Init() {
	pgid, err := syscall.Getpgid(0)
	if err != nil || pgid != os.Getpid() {
		if err := syscall.Setpgid(0, 0); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to enable child process containment: %v\n", err)
			return
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-sigCh
		_ = syscall.Kill(-os.Getpid(), syscall.SIGKILL)
	}()
}
