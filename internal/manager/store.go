package manager

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

type Manager struct {
	mu       sync.Mutex
	state    State
	file     string
	client   *http.Client
	jobs     map[string]*Download
	cancels  map[string]context.CancelFunc
	packages map[string]Package
	slots    chan struct{}
	ctx      context.Context
	cancel   context.CancelFunc
	workers  sync.WaitGroup
}

func New(configDir string) (*Manager, error) {
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{file: filepath.Join(configDir, "state.json"), jobs: map[string]*Download{}, cancels: map[string]context.CancelFunc{}, packages: map[string]Package{}, slots: make(chan struct{}, 3), ctx: ctx, cancel: cancel, client: &http.Client{Timeout: 30 * time.Second, CheckRedirect: secureRedirect}}
	m.state = State{SchemaVersion: 1, Settings: Settings{Theme: "dark", Vault: filepath.Join(home, ".tools-switcher", "vault"), CloseToTray: true, AutoCheckUpdates: true}, Repositories: []Repository{}, Installations: []Installation{}, Bindings: []Binding{}}
	data, err := os.ReadFile(m.file)
	if err == nil {
		if err = json.Unmarshal(data, &m.state); err != nil {
			cancel()
			return nil, fmt.Errorf("configuración inválida: %w", err)
		}
		if m.state.SchemaVersion != 1 {
			cancel()
			return nil, errors.New("versión de configuración no compatible")
		}
	} else if !os.IsNotExist(err) {
		cancel()
		return nil, err
	}
	return m, nil
}

func id() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

func (m *Manager) saveLocked() error {
	data, err := json.MarshalIndent(m.state, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(m.file), "state-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(name, m.file)
}

func (m *Manager) persistLocked(previous State) error {
	if err := m.saveLocked(); err != nil {
		m.state = previous
		return err
	}
	return nil
}

func (m *Manager) Settings() Settings { m.mu.Lock(); defer m.mu.Unlock(); return m.state.Settings }

func (m *Manager) SaveSettings(s Settings) error {
	if s.Theme != "dark" && s.Theme != "light" && s.Theme != "system" {
		return errors.New("tema no válido")
	}
	if !filepath.IsAbs(s.Vault) {
		return errors.New("selecciona una ruta absoluta para el vault")
	}
	s.Vault = filepath.Clean(s.Vault)
	if err := os.MkdirAll(s.Vault, 0700); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if s.Vault != m.state.Settings.Vault {
		for _, j := range m.jobs {
			if j.Status == "queued" || j.Status == "downloading" || j.Status == "extracting" {
				return errors.New("espera a que terminen las instalaciones antes de cambiar el vault")
			}
		}
	}
	previous := m.cloneState()
	m.state.Settings = s
	return m.persistLocked(previous)
}

func (m *Manager) Snapshot() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := Snapshot{Settings: m.state.Settings, Repositories: append([]Repository{}, m.state.Repositories...), Installations: append([]Installation{}, m.state.Installations...), Downloads: []Download{}, Platform: runtime.GOOS, Architecture: runtime.GOARCH}
	for _, j := range m.jobs {
		s.Downloads = append(s.Downloads, *j)
	}
	return s
}

func (m *Manager) Close() { m.mu.Lock(); m.cancel(); m.mu.Unlock(); m.workers.Wait() }

func secureRedirect(req *http.Request, via []*http.Request) error {
	if len(via) > 8 {
		return errors.New("demasiadas redirecciones")
	}
	if req.URL.Scheme != "https" {
		return errors.New("se requiere HTTPS")
	}
	return nil
}
