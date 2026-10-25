package manager

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jecortes2304/tools-switcher/internal/platform"
)

func canonicalVariable(name string) string {
	if strings.EqualFold(name, "path") {
		if runtime.GOOS == "windows" {
			return "Path"
		}
		return "PATH"
	}
	return name
}
func pathContains(value, entry string) bool {
	for _, p := range filepath.SplitList(value) {
		if samePath(p, entry) {
			return true
		}
	}
	return false
}
func removePath(value, entry string) string {
	result := []string{}
	for _, p := range filepath.SplitList(value) {
		if !samePath(p, entry) {
			result = append(result, p)
		}
	}
	return strings.Join(result, string(os.PathListSeparator))
}

func (m *Manager) Assign(installationID, name string) error {
	return m.assignVariables(installationID, []string{name})
}

func (m *Manager) Activate(installationID string) error {
	return m.assignVariables(installationID, []string{"JAVA_HOME", "PATH"})
}

func (m *Manager) assignVariables(installationID string, names []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var installation Installation
	for _, i := range m.state.Installations {
		if i.ID == installationID {
			installation = i
		}
	}
	if installation.ID == "" {
		return errors.New("JDK no encontrada")
	}
	if _, err := inspectHome(installation.Path); err != nil {
		return err
	}
	previous := m.cloneState()
	type original struct {
		name, value string
		exists      bool
	}
	originals := []original{}
	rollback := func(cause error) error {
		m.state = previous
		for j := len(originals) - 1; j >= 0; j-- {
			o := originals[j]
			if err := platform.WriteVariable(o.name, o.value, o.exists); err != nil {
				cause = errors.Join(cause, fmt.Errorf("restaurando %s: %w", o.name, err))
			}
		}
		return cause
	}
	for _, name := range names {
		name = canonicalVariable(strings.TrimSpace(name))
		if !platform.ValidVariable(name) {
			return rollback(errors.New("nombre de variable no válido"))
		}
		value, exists, err := platform.ReadVariable(name)
		if err != nil {
			return rollback(err)
		}
		binding := Binding{Name: name, Previous: value, Existed: exists, InstallationID: installationID}
		index := -1
		for i, b := range m.state.Bindings {
			if b.Name == name {
				if value != b.Applied || !exists {
					return rollback(fmt.Errorf("%s cambió fuera de tools-switcher; restaura el valor gestionado antes de continuar", name))
				}
				index = i
				binding.Previous = b.Previous
				binding.Existed = b.Existed
			}
		}
		applied := installation.Path
		if strings.EqualFold(name, "path") {
			if index >= 0 {
				value = removePath(value, filepath.Join(m.bindingHome(m.state.Bindings[index].InstallationID), "bin"))
			}
			entry := filepath.Join(installation.Path, "bin")
			if !pathContains(value, entry) {
				if value == "" {
					value = entry
				} else {
					value = entry + string(os.PathListSeparator) + value
				}
			}
			applied = value
		}
		originalValue, originalExists, err := platform.ReadVariable(name)
		if err != nil {
			return rollback(err)
		}
		if err = platform.WriteVariable(name, applied, true); err != nil {
			return rollback(err)
		}
		originals = append(originals, original{name, originalValue, originalExists})
		binding.Applied = applied
		if index < 0 {
			m.state.Bindings = append(m.state.Bindings, binding)
		} else {
			m.state.Bindings[index] = binding
		}
	}
	if err := m.saveLocked(); err != nil {
		return rollback(err)
	}
	return nil
}
func (m *Manager) bindingHome(installationID string) string {
	for _, i := range m.state.Installations {
		if i.ID == installationID {
			return i.Path
		}
	}
	return ""
}

func (m *Manager) Restore(installationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	previous := m.cloneState()
	remaining := []Binding{}
	restored := []Binding{}
	for _, b := range m.state.Bindings {
		if b.InstallationID != installationID {
			remaining = append(remaining, b)
			continue
		}
		current, exists, err := platform.ReadVariable(b.Name)
		if err != nil {
			return err
		}
		if !exists || current != b.Applied {
			return fmt.Errorf("%s cambió fuera de tools-switcher; restaura su valor antes de continuar", b.Name)
		}
	}
	for _, b := range m.state.Bindings {
		if b.InstallationID != installationID {
			continue
		}
		if err := platform.WriteVariable(b.Name, b.Previous, b.Existed); err != nil {
			for _, done := range restored {
				_ = platform.WriteVariable(done.Name, done.Applied, true)
			}
			return err
		}
		restored = append(restored, b)
	}
	m.state.Bindings = remaining
	if err := m.persistLocked(previous); err != nil {
		for _, b := range restored {
			_ = platform.WriteVariable(b.Name, b.Applied, true)
		}
		return err
	}
	return nil
}

func (m *Manager) RemoveInstallation(installationID string, deleteFiles bool) error {
	if err := m.Restore(installationID); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	previous := m.cloneState()
	for index, i := range m.state.Installations {
		if i.ID != installationID {
			continue
		}
		var quarantine string
		if deleteFiles {
			if !i.Managed || i.ManagedRoot == "" {
				return errors.New("las instalaciones externas solo se desvinculan; sus archivos se conservan")
			}
			root, err := filepath.EvalSymlinks(i.ManagedRoot)
			if err != nil {
				return err
			}
			if !samePath(root, i.ManagedRoot) || !inside(root, i.Path) {
				return errors.New("directorio de instalación no seguro")
			}
			marker, err := os.ReadFile(filepath.Join(root, ".tools-switcher-id"))
			if err != nil || string(marker) != i.ID {
				return errors.New("no se puede verificar que esta instalación pertenezca a tools-switcher")
			}
			quarantine = root + ".removing-" + id()
			if err = os.Rename(root, quarantine); err != nil {
				return fmt.Errorf("no se puede retirar la JDK; comprueba si está en uso: %w", err)
			}
		}
		m.state.Installations = append(m.state.Installations[:index], m.state.Installations[index+1:]...)
		if err := m.persistLocked(previous); err != nil {
			if quarantine != "" {
				_ = os.Rename(quarantine, i.ManagedRoot)
			}
			return err
		}
		if quarantine != "" {
			return os.RemoveAll(quarantine)
		}
		return nil
	}
	return errors.New("JDK no encontrada")
}

func (m *Manager) Discover() ([]Installation, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	roots := []string{os.Getenv("JAVA_HOME"), filepath.Join(home, ".jdks"), filepath.Join(home, ".sdkman", "candidates", "java"), m.Settings().Vault}
	if runtime.GOOS == "windows" {
		roots = append(roots, filepath.Join(os.Getenv("ProgramFiles"), "Java"), filepath.Join(os.Getenv("ProgramFiles"), "Eclipse Adoptium"), filepath.Join(os.Getenv("ProgramFiles"), "Microsoft"), `C:\Java`)
	} else if runtime.GOOS == "darwin" {
		roots = append(roots, "/Library/Java/JavaVirtualMachines")
	} else {
		roots = append(roots, "/usr/lib/jvm", "/opt/java")
	}
	result := []Installation{}
	seen := map[string]bool{}
	for _, root := range roots {
		if root == "" {
			continue
		}
		if info, e := os.Stat(root); e != nil || !info.IsDir() {
			continue
		}
		homes, e := findHomes(root)
		if e != nil {
			continue
		}
		for _, i := range homes {
			real, e := filepath.EvalSymlinks(i.Path)
			if e != nil {
				continue
			}
			key := strings.ToLower(real)
			if seen[key] {
				continue
			}
			seen[key] = true
			i.Path = real
			i.ID = packageID("local", real)
			result = append(result, i)
		}
	}
	return result, nil
}

func samePath(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
