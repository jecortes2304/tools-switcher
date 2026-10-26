package desktop

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/jecortes2304/tools-switcher/internal/manager"
	"github.com/jecortes2304/tools-switcher/internal/platform"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/endpoint"
)

type UpdateStatus struct {
	Status     string `json:"status"`
	Version    string `json:"version"`
	Error      string `json:"error"`
	Configured bool   `json:"configured"`
}
type Service struct {
	manager         *manager.Manager
	app             *application.App
	notifications   *notifications.NotificationService
	version         string
	key             string
	mu              sync.Mutex
	updateOperation sync.Mutex
	update          UpdateStatus
	ctx             context.Context
	cancel          context.CancelFunc
	workers         sync.WaitGroup
	lastNotified    string
}

func New(m *manager.Manager, version, key string) *Service {
	return &Service{manager: m, version: version, key: key, update: UpdateStatus{Status: "idle", Configured: key != ""}}
}
func Attach(s *Service, app *application.App, n *notifications.NotificationService) {
	s.app = app
	s.notifications = n
}
func (s *Service) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.ctx, s.cancel = context.WithCancel(ctx)
	if s.key != "" {
		feed, err := endpoint.New(endpoint.Config{URL: "https://github.com/jecortes2304/tools-switcher/releases/latest/download/manifest.json", Channel: "stable"})
		if err != nil {
			return err
		}
		if err = s.app.Updater.Init(updater.Config{CurrentVersion: s.version, PublicKey: []byte(s.key), Providers: []updater.Provider{signedProvider{feed}}}); err != nil {
			return err
		}
	}
	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		timer := time.NewTimer(8 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-timer.C:
				if s.manager.Settings().AutoCheckUpdates && s.key != "" {
					_, _ = s.CheckUpdates()
				}
				timer.Reset(6 * time.Hour)
			}
		}
	}()
	return nil
}
func (s *Service) ServiceShutdown() error {
	if s.cancel != nil {
		s.cancel()
	}
	s.workers.Wait()
	s.manager.Close()
	return nil
}
func (s *Service) Snapshot() manager.Snapshot {
	v := s.manager.Snapshot()
	v.Environment = platform.JavaEnvironment()
	if len(v.Environment) > 0 {
		v.ActivePath = v.Environment[0].Value
		v.ActiveSource = v.Environment[0].Source
	}
	v.ActiveName = s.manager.DescribeHome(v.ActivePath)
	v.PreferencesFile = s.manager.PreferencesFile()
	v.ResolvedJava = platform.ResolvedJava()
	return v
}
func (s *Service) Version() string          { return s.version }
func (s *Service) Versions() ([]int, error) { return s.manager.Versions() }
func (s *Service) AddRepository(name, source, kind string) (manager.Repository, error) {
	return s.manager.AddRepository(name, source, kind)
}
func (s *Service) Catalog(repositoryID string, major int) ([]manager.Package, error) {
	return s.manager.Catalog(repositoryID, major)
}
func (s *Service) ToggleRepository(repositoryID string, enabled bool) error {
	return s.manager.ToggleRepository(repositoryID, enabled)
}
func (s *Service) RemoveRepository(repositoryID string) error {
	return s.manager.RemoveRepository(repositoryID)
}
func (s *Service) Queue(ids []string) error                  { return s.manager.Queue(ids) }
func (s *Service) CancelDownload(id string)                  { s.manager.CancelDownload(id) }
func (s *Service) Import(path string) error                  { return s.manager.Import(path, "") }
func (s *Service) Discover() ([]manager.Installation, error) { return s.manager.Discover() }
func (s *Service) Assign(id, name string) error              { return s.manager.Assign(id, name) }
func (s *Service) Restore(id string) error                   { return s.manager.Restore(id) }
func (s *Service) RemoveInstallation(id string, deleteFiles bool) error {
	return s.manager.RemoveInstallation(id, deleteFiles)
}
func (s *Service) Variables() []platform.Variable { return platform.Variables() }
func (s *Service) OpenInstallation(id string) error {
	i, err := s.manager.Installation(id)
	if err != nil {
		return err
	}
	return platform.OpenFolder(i.Path)
}
func (s *Service) ChooseDirectory() (string, error) {
	return s.app.Dialog.OpenFile().SetTitle("Seleccionar carpeta").CanChooseDirectories(true).CanChooseFiles(false).PromptForSingleSelection()
}
func (s *Service) SaveSettings(settings manager.Settings) error {
	previous := s.manager.Settings()
	if settings.StartAtLogin != previous.StartAtLogin {
		if err := platform.SetAutostart(settings.StartAtLogin); err != nil {
			return err
		}
	}
	if err := s.manager.SaveSettings(settings); err != nil {
		if settings.StartAtLogin != previous.StartAtLogin {
			_ = platform.SetAutostart(previous.StartAtLogin)
		}
		return err
	}
	return nil
}
func (s *Service) UpdateStatus() UpdateStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := s.update
	if s.key != "" {
		result.Status = string(s.app.Updater.State())
	}
	return result
}
func (s *Service) CheckUpdates() (UpdateStatus, error) {
	if !s.updateOperation.TryLock() {
		return s.UpdateStatus(), errors.New("hay otra operación de actualización en curso")
	}
	defer s.updateOperation.Unlock()
	if s.key == "" {
		return s.UpdateStatus(), errors.New("las actualizaciones firmadas se activarán al configurar la clave pública de publicación")
	}
	state := s.app.Updater.State()
	if state == updater.StateReady {
		return s.UpdateStatus(), nil
	}
	ctx, cancel := context.WithTimeout(s.ctx, 45*time.Second)
	defer cancel()
	release, err := s.app.Updater.Check(ctx)
	s.mu.Lock()
	if err != nil {
		s.update.Error = err.Error()
		s.mu.Unlock()
		return s.UpdateStatus(), err
	}
	s.update.Error = ""
	notify := release != nil && s.lastNotified != release.Version
	if release != nil {
		s.update.Version = release.Version
		if notify {
			s.lastNotified = release.Version
		}
	}
	s.mu.Unlock()
	if notify {
		allowed, e := s.notifications.RequestNotificationAuthorization()
		if e == nil && allowed {
			_ = s.notifications.SendNotification(notifications.NotificationOptions{ID: "tools-switcher-update", Title: "Actualización disponible", Body: fmt.Sprintf("tools-switcher %s está listo para descargar. Abre Ajustes para actualizar.", release.Version)})
		}
	}
	return s.UpdateStatus(), nil
}

func (s *Service) InstallUpdate() error {
	if !s.updateOperation.TryLock() {
		return errors.New("hay otra operación de actualización en curso")
	}
	defer s.updateOperation.Unlock()
	if runtime.GOOS == "linux" {
		executable, _ := os.Executable()
		if os.Getenv("APPIMAGE") != "" || strings.HasPrefix(executable, "/usr/") {
			return errors.New("actualiza esta instalación con su gestor de paquetes o reemplazando el AppImage completo")
		}
	}
	for _, j := range s.manager.Snapshot().Downloads {
		if j.Status == "queued" || j.Status == "downloading" || j.Status == "extracting" {
			return errors.New("espera a que terminen las instalaciones de JDKs")
		}
	}
	if s.key == "" {
		return errors.New("actualizaciones sin configurar")
	}
	return s.app.Updater.DownloadAndInstall(s.ctx)
}
func (s *Service) Restart() error {
	for _, j := range s.manager.Snapshot().Downloads {
		if j.Status == "queued" || j.Status == "downloading" || j.Status == "extracting" {
			return errors.New("hay instalaciones en curso")
		}
	}
	return s.app.Updater.Restart(s.ctx)
}
func ConfigDirectory() (string, error) {
	if override := os.Getenv("TOOLS_SWITCHER_CONFIG_DIR"); override != "" {
		if !filepath.IsAbs(override) {
			return "", errors.New("TOOLS_SWITCHER_CONFIG_DIR debe ser una ruta absoluta")
		}
		return override, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "tools-switcher"), nil
}

type signedProvider struct{ inner updater.Provider }

func (p signedProvider) Name() string { return p.inner.Name() }
func (p signedProvider) Check(ctx context.Context, req updater.CheckRequest) (*updater.Release, error) {
	release, err := p.inner.Check(ctx, req)
	if err != nil || release == nil {
		return release, err
	}
	if release.Verification == nil || len(release.Verification.Signature) == 0 {
		return nil, errors.New("la actualización no está firmada")
	}
	return release, nil
}
func (p signedProvider) Download(ctx context.Context, release *updater.Release, dst io.Writer, onProgress func(int64, int64)) error {
	return p.inner.Download(ctx, release, dst, onProgress)
}

func (s *Service) Activate(id string) error { return s.manager.Activate(id) }

func (s *Service) ShowPreferencesFile() error {
	if err := s.manager.EnsurePreferencesFile(); err != nil {
		return err
	}
	return platform.RevealFile(s.manager.PreferencesFile())
}

func (s *Service) RepositoryVersions(id string) ([]int, error) {
	return s.manager.RepositoryVersions(id)
}
func (s *Service) SyncRepository(id string) error { return s.manager.SyncRepository(id) }
