package platform

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type Variable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

var ReadVariable = readUnixVariable
var WriteVariable = writeUnixVariable
var SetAutostart = unixAutostart
var ResolvedJava = resolvedJava

func Variables() []Variable {
	names := map[string]bool{}
	for _, entry := range os.Environ() {
		name, _, ok := strings.Cut(entry, "=")
		if ok && ValidVariable(name) {
			names[name] = true
		}
	}
	for _, name := range EnvironmentNames() {
		if ValidVariable(name) {
			names[name] = true
		}
	}
	for _, name := range []string{"JAVA_HOME", "JDK_HOME", "GRAALVM_HOME", "JRE_HOME"} {
		names[name] = true
	}
	result := []Variable{}
	for name := range names {
		result = append(result, Variable{Name: name, Value: InspectVariable(name).Value})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}
func ValidVariable(name string) bool {
	if name == "" {
		return false
	}
	for i, c := range name {
		if !(c == '_' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || i > 0 && c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}
func resolvedJava() string {
	p, _ := exec.LookPath("java")
	if p != "" {
		if real, err := filepath.EvalSymlinks(p); err == nil {
			return real
		}
	}
	return p
}
func OpenFolder(folder string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer.exe", folder)
	case "darwin":
		cmd = exec.Command("open", folder)
	default:
		cmd = exec.Command("xdg-open", folder)
	}
	return cmd.Run()
}
func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
func envFiles() (string, string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", "", err
	}
	dir := filepath.Join(base, "tools-switcher")
	if err = os.MkdirAll(dir, 0700); err != nil {
		return "", "", err
	}
	return filepath.Join(dir, "environment.json"), filepath.Join(dir, "environment.sh"), nil
}
func readUnixVariable(name string) (string, bool, error) {
	file, _, err := envFiles()
	if err != nil {
		return "", false, err
	}
	values := map[string]string{}
	data, err := os.ReadFile(file)
	if err == nil {
		if err = json.Unmarshal(data, &values); err != nil {
			return "", false, err
		}
		if v, ok := values[name]; ok {
			return v, true, nil
		}
	} else if !os.IsNotExist(err) {
		return "", false, err
	}
	v, ok := os.LookupEnv(name)
	return v, ok, nil
}
func writeUnixVariable(name, value string, exists bool) error {
	if !ValidVariable(name) {
		return errors.New("nombre de variable no válido")
	}
	file, script, err := envFiles()
	if err != nil {
		return err
	}
	values := map[string]string{}
	if data, e := os.ReadFile(file); e == nil {
		if err = json.Unmarshal(data, &values); err != nil {
			return err
		}
	}
	if exists {
		values[name] = value
	} else {
		delete(values, name)
	}
	data, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(file, data, 0600); err != nil {
		return err
	}
	keys := []string{}
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var out strings.Builder
	for _, key := range keys {
		fmt.Fprintf(&out, "export %s=%s\n", key, shellQuote(values[key]))
	}
	if err = os.WriteFile(script, []byte(out.String()), 0600); err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	profiles := []string{filepath.Join(home, ".profile"), filepath.Join(home, ".bashrc")}
	if runtime.GOOS == "darwin" {
		profiles = append(profiles, filepath.Join(home, ".zprofile"), filepath.Join(home, ".zshrc"))
	}
	line := "[ -f " + shellQuote(script) + " ] && . " + shellQuote(script)
	for _, profile := range profiles {
		content, e := os.ReadFile(profile)
		if e != nil && !os.IsNotExist(e) {
			return e
		}
		if !strings.Contains(string(content), line) {
			f, e := os.OpenFile(profile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
			if e != nil {
				return e
			}
			_, e = f.WriteString("\n" + line + "\n")
			closeErr := f.Close()
			if e != nil {
				return e
			}
			if closeErr != nil {
				return closeErr
			}
		}
	}
	if exists {
		return os.Setenv(name, value)
	}
	return os.Unsetenv(name)
}
func unixAutostart(enabled bool) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	var file, content string
	if runtime.GOOS == "darwin" {
		file = filepath.Join(home, "Library", "LaunchAgents", "com.cortestudios.tools-switcher.plist")
		content = `<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict><key>Label</key><string>com.cortestudios.tools-switcher</string><key>ProgramArguments</key><array><string>` + html.EscapeString(executable) + `</string><string>--background</string></array><key>RunAtLoad</key><true/></dict></plist>`
	} else {
		config, err := os.UserConfigDir()
		if err != nil {
			return err
		}
		file = filepath.Join(config, "autostart", "tools-switcher.desktop")
		quoted := strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "`", "\\`", "$", "\\$").Replace(executable)
		content = "[Desktop Entry]\nType=Application\nName=tools-switcher\nExec=\"" + quoted + "\" --background\nX-GNOME-Autostart-enabled=true\n"
	}
	if !enabled {
		err = os.Remove(file)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err = os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		return err
	}
	return os.WriteFile(file, []byte(content), 0600)
}

func RevealFile(file string) error {
	if _, err := os.Stat(file); err != nil {
		return err
	}
	switch runtime.GOOS {
	case "windows":
		command := exec.Command(filepath.Join(os.Getenv("SystemRoot"), "explorer.exe"), "/select,"+file)
		if err := command.Start(); err != nil {
			return err
		}
		go command.Wait()
		return nil
	case "darwin":
		return exec.Command("open", "-R", file).Run()
	default:
		return OpenFolder(filepath.Dir(file))
	}
}

func readUnixStoredVariable(name string) (string, bool, error) {
	file, _, err := envFiles()
	if err != nil {
		return "", false, err
	}
	data, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	values := map[string]string{}
	if err = json.Unmarshal(data, &values); err != nil {
		return "", false, err
	}
	value, ok := values[name]
	return value, ok, nil
}
