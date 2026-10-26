package platform

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func init() {
	ReadVariable = readWindowsVariable
	ExpandVariableValue = expandWindows
	ReadSystemVariable = readWindowsSystemVariable
	EnvironmentNames = windowsEnvironmentNames
	WriteVariable = writeWindowsVariable
	SetAutostart = windowsAutostart
	ResolvedJava = windowsResolvedJava
}
func readWindowsVariable(name string) (string, bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, "Environment", registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	defer key.Close()
	v, _, err := key.GetStringValue(name)
	if errors.Is(err, registry.ErrNotExist) {
		return "", false, nil
	}
	return v, err == nil, err
}
func writeWindowsVariable(name, value string, exists bool) error {
	if !ValidVariable(name) {
		return errors.New("nombre de variable no válido")
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, "Environment", registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	if exists {
		err = key.SetExpandStringValue(name, value)
	} else {
		err = key.DeleteValue(name)
		if errors.Is(err, registry.ErrNotExist) {
			err = nil
		}
	}
	if err != nil {
		return err
	}
	if strings.EqualFold(name, "path") {
		effective := value
		machine, e := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`, registry.QUERY_VALUE)
		if e == nil {
			systemPath, _, readErr := machine.GetStringValue("Path")
			machine.Close()
			if readErr == nil && systemPath != "" {
				if effective != "" {
					effective = systemPath + ";" + effective
				} else {
					effective = systemPath
				}
			}
		}
		_ = os.Setenv("Path", expandWindows(effective))
	} else if exists {
		_ = os.Setenv(name, value)
	} else {
		_ = os.Unsetenv(name)
	}
	environment, _ := windows.UTF16PtrFromString("Environment")
	var result uintptr
	windows.NewLazySystemDLL("user32.dll").NewProc("SendMessageTimeoutW").Call(0xffff, 0x1a, 0, uintptr(unsafe.Pointer(environment)), 2, 2000, uintptr(unsafe.Pointer(&result)))
	return nil
}
func windowsAutostart(enabled bool) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	if !enabled {
		err = key.DeleteValue("tools-switcher")
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	return key.SetStringValue("tools-switcher", "\""+executable+"\" --background")
}
func expandWindows(value string) string {
	expanded, err := registry.ExpandString(value)
	if err != nil {
		return value
	}
	return expanded
}

func readWindowsSystemVariable(name string) (string, bool, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	defer key.Close()
	value, _, err := key.GetStringValue(name)
	if errors.Is(err, registry.ErrNotExist) {
		return "", false, nil
	}
	return value, err == nil, err
}

func windowsEnvironmentNames() []string {
	names := []string{}
	for _, location := range []struct {
		root registry.Key
		path string
	}{{registry.CURRENT_USER, "Environment"}, {registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`}} {
		key, err := registry.OpenKey(location.root, location.path, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		values, err := key.ReadValueNames(-1)
		key.Close()
		if err == nil {
			names = append(names, values...)
		}
	}
	return names
}
func windowsResolvedJava() string {
	var entries []string
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`, registry.QUERY_VALUE)
	if err == nil {
		value, _, e := key.GetStringValue("Path")
		key.Close()
		if e == nil {
			entries = append(entries, strings.Split(expandWindows(value), ";")...)
		}
	}
	value, _, err := readWindowsVariable("Path")
	if err == nil {
		entries = append(entries, strings.Split(expandWindows(value), ";")...)
	}
	for _, dir := range entries {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(strings.Trim(dir, "\""), "java.exe")
		if info, e := os.Stat(candidate); e == nil && info.Mode().IsRegular() {
			return candidate
		}
	}
	p, _ := exec.LookPath("java.exe")
	return p
}
