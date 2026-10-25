//go:build windows

package register

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

func notifyShell() {
	shell32 := syscall.NewLazyDLL("shell32.dll")
	procSHChangeNotify := shell32.NewProc("SHChangeNotify")
	// SHCNE_ASSOCCHANGED = 0x08000000, SHCNF_IDLIST = 0
	_, _, err := procSHChangeNotify.Call(0x08000000, 0, 0, 0)
	if err != nil {
		return
	}
}

func Register() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exePath, err := filepath.Abs(exe)
	if err != nil {
		return err
	}
	exeName := filepath.Base(exePath)
	openCmd := fmt.Sprintf(`"%s" "%%1"`, exePath)

	// 1. HKCU\Software\Classes\Applications\<exeName>\shell\open\command
	appCmdKey, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		`Software\Classes\Applications\`+exeName+`\shell\open\command`,
		registry.SET_VALUE,
	)
	if err != nil {
		return err
	}
	defer func(appCmdKey registry.Key) {
		err := appCmdKey.Close()
		if err != nil {
		}
	}(appCmdKey)
	if err := appCmdKey.SetStringValue("", openCmd); err != nil {
		return err
	}

	// 2. HKCU\Software\Classes\Applications\<exeName>\SupportedTypes
	suppKey, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		`Software\Classes\Applications\`+exeName+`\SupportedTypes`,
		registry.SET_VALUE,
	)
	if err != nil {
		return err
	}
	defer func(suppKey registry.Key) {
		err := suppKey.Close()
		if err != nil {
		}
	}(suppKey)
	_ = suppKey.SetStringValue(".py", "")
	_ = suppKey.SetStringValue(".pyw", "")

	// 3. HKCU\Software\Classes\Runny.PythonScript\shell\open\command
	progKey, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		`Software\Classes\Runny.PythonScript\shell\open\command`,
		registry.SET_VALUE,
	)
	if err != nil {
		return err
	}
	defer func(progKey registry.Key) {
		err := progKey.Close()
		if err != nil {
		}
	}(progKey)
	if err := progKey.SetStringValue("", openCmd); err != nil {
		return err
	}

	// Set ProgID display name
	progBaseKey, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		`Software\Classes\Runny.PythonScript`,
		registry.SET_VALUE,
	)
	if err == nil {
		_ = progBaseKey.SetStringValue("", "Python Script (Runny)")
		err := progBaseKey.Close()
		if err != nil {
			return err
		}
	}

	// 4. HKCU\Software\Classes\.py\OpenWithProgids and .pyw\OpenWithProgids
	for _, ext := range []string{".py", ".pyw"} {
		owKey, _, err := registry.CreateKey(
			registry.CURRENT_USER,
			`Software\Classes\`+ext+`\OpenWithProgids`,
			registry.SET_VALUE,
		)
		if err == nil {
			_ = owKey.SetStringValue("Runny.PythonScript", "")
			err := owKey.Close()
			if err != nil {
				return err
			}
		}
	}

	notifyShell()
	return nil
}

func Unregister() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exeName := filepath.Base(exe)

	// Remove OpenWithProgids entries
	for _, ext := range []string{".py", ".pyw"} {
		owKey, err := registry.OpenKey(
			registry.CURRENT_USER,
			`Software\Classes\`+ext+`\OpenWithProgids`,
			registry.SET_VALUE,
		)
		if err == nil {
			_ = owKey.DeleteValue("Runny.PythonScript")
			err := owKey.Close()
			if err != nil {
				return err
			}
		}
	}

	// Delete subkeys under Applications\<exeName>
	_ = registry.DeleteKey(registry.CURRENT_USER, `Software\Classes\Applications\`+exeName+`\shell\open\command`)
	_ = registry.DeleteKey(registry.CURRENT_USER, `Software\Classes\Applications\`+exeName+`\shell\open`)
	_ = registry.DeleteKey(registry.CURRENT_USER, `Software\Classes\Applications\`+exeName+`\shell`)
	_ = registry.DeleteKey(registry.CURRENT_USER, `Software\Classes\Applications\`+exeName+`\SupportedTypes`)
	_ = registry.DeleteKey(registry.CURRENT_USER, `Software\Classes\Applications\`+exeName)

	// Delete Runny.PythonScript ProgID
	_ = registry.DeleteKey(registry.CURRENT_USER, `Software\Classes\Runny.PythonScript\shell\open\command`)
	_ = registry.DeleteKey(registry.CURRENT_USER, `Software\Classes\Runny.PythonScript\shell\open`)
	_ = registry.DeleteKey(registry.CURRENT_USER, `Software\Classes\Runny.PythonScript\shell`)
	_ = registry.DeleteKey(registry.CURRENT_USER, `Software\Classes\Runny.PythonScript`)

	notifyShell()
	return nil
}
