//go:build !windows

package register

import "fmt"

func Register() error {
	return fmt.Errorf("file association registration is only supported on Windows")
}

func Unregister() error {
	return fmt.Errorf("file association unregistration is only supported on Windows")
}
