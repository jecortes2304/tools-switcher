package manager

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (m *Manager) Queue(packageIDs []string) error {
	if len(packageIDs) == 0 || len(packageIDs) > 50 {
		return errors.New("selecciona entre 1 y 50 paquetes")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ctx.Err() != nil {
		return errors.New("la aplicación se está cerrando")
	}
	selected := []Package{}
	for _, pid := range packageIDs {
		p, ok := m.packages[pid]
		if !ok {
			return errors.New("recarga el catálogo antes de instalar")
		}
		enabled := false
		for _, r := range m.state.Repositories {
			if r.ID == p.RepositoryID && r.Enabled {
				enabled = true
			}
		}
		if !enabled {
			return errors.New("el repositorio no está disponible")
		}
		if j, ok := m.jobs[pid]; ok && (j.Status == "queued" || j.Status == "downloading" || j.Status == "extracting") {
			continue
		}
		if !compatible(p) {
			return errors.New("paquete incompatible con el equipo")
		}
		selected = append(selected, p)
	}
	for _, p := range selected {
		if j, ok := m.jobs[p.ID]; ok && (j.Status == "queued" || j.Status == "downloading" || j.Status == "extracting") {
			continue
		}
		ctx, cancel := context.WithCancel(m.ctx)
		m.cancels[p.ID] = cancel
		m.jobs[p.ID] = &Download{ID: p.ID, Name: p.Name, Status: "queued", Total: p.Size}
		vault := m.state.Settings.Vault
		m.workers.Add(1)
		go func(p Package) { defer m.workers.Done(); defer cancel(); m.runInstall(ctx, p, vault) }(p)
	}
	return nil
}

func (m *Manager) CancelDownload(jobID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cancel, ok := m.cancels[jobID]; ok {
		cancel()
	}
}
func (m *Manager) jobStatus(jobID, status string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if j := m.jobs[jobID]; j != nil {
		j.Status = status
		if err != nil {
			j.Error = err.Error()
		}
	}
}

type progressWriter struct {
	m       *Manager
	id      string
	written int64
	last    time.Time
}

func (w *progressWriter) Write(p []byte) (int, error) {
	w.written += int64(len(p))
	if time.Since(w.last) > 150*time.Millisecond {
		w.m.mu.Lock()
		if j := w.m.jobs[w.id]; j != nil {
			j.Written = w.written
		}
		w.m.mu.Unlock()
		w.last = time.Now()
	}
	return len(p), nil
}

func (m *Manager) runInstall(ctx context.Context, p Package, vault string) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	select {
	case m.slots <- struct{}{}:
		defer func() { <-m.slots }()
	case <-ctx.Done():
		m.jobStatus(p.ID, "cancelled", nil)
		return
	}
	err := m.install(ctx, p, vault)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			m.jobStatus(p.ID, "cancelled", nil)
		} else {
			m.jobStatus(p.ID, "error", err)
		}
	} else {
		m.jobStatus(p.ID, "completed", nil)
	}
	m.mu.Lock()
	delete(m.cancels, p.ID)
	m.mu.Unlock()
}

func (m *Manager) install(ctx context.Context, p Package, vault string) error {
	if p.LocalPath != "" {
		if info, err := os.Stat(p.LocalPath); err == nil && info.IsDir() {
			return m.Import(p.LocalPath, p.RepositoryID)
		}
	}
	if err := os.MkdirAll(vault, 0700); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(vault, ".staging-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	archive := filepath.Join(staging, p.Filename)
	if p.Filename == "" {
		return errors.New("nombre de paquete vacío")
	}
	m.jobStatus(p.ID, "downloading", nil)
	if p.LocalPath != "" {
		source, err := os.Open(p.LocalPath)
		if err != nil {
			return err
		}
		defer source.Close()
		target, err := os.Create(archive)
		if err != nil {
			return err
		}
		_, err = io.Copy(target, io.LimitReader(contextReader{ctx, source}, maxArchiveBytes+1))
		closeErr := target.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	} else {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.URL, nil)
		if err != nil {
			return err
		}
		client := *m.client
		client.Timeout = 0
		res, err := client.Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			return fmt.Errorf("descarga: HTTP %d", res.StatusCode)
		}
		f, err := os.Create(archive)
		if err != nil {
			return err
		}
		hash := sha256.New()
		progress := &progressWriter{m: m, id: p.ID}
		written, err := io.Copy(io.MultiWriter(f, hash, progress), io.LimitReader(res.Body, maxArchiveBytes+1))
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		if written > maxArchiveBytes {
			return errors.New("el paquete supera 8 GiB")
		}
		if p.Size > 0 && written != p.Size {
			return errors.New("tamaño de descarga incorrecto")
		}
		if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), p.SHA256) {
			return errors.New("el checksum SHA-256 no coincide; se ha descartado el paquete")
		}
	}
	m.jobStatus(p.ID, "extracting", nil)
	content := filepath.Join(staging, "content")
	if err = os.Mkdir(content, 0700); err != nil {
		return err
	}
	if err = extract(ctx, archive, content); err != nil {
		return err
	}
	homes, err := findHomes(content)
	if err != nil {
		return err
	}
	if len(homes) != 1 {
		return errors.New("el paquete debe contener exactamente una JDK")
	}
	if err = validateHomePlatform(homes[0].Path, p); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	root := filepath.Join(vault, "jdk", p.ID)
	if err = os.MkdirAll(filepath.Dir(root), 0700); err != nil {
		return err
	}
	if _, err = os.Lstat(root); err == nil {
		return errors.New("esta JDK ya está instalada")
	}
	rel, err := filepath.Rel(content, homes[0].Path)
	if err != nil {
		return err
	}
	installation := homes[0]
	installation.ID = p.ID
	installation.RepositoryID = p.RepositoryID
	installation.Managed = true
	installation.ManagedRoot = root
	installation.Path = filepath.Join(root, rel)
	if err = os.WriteFile(filepath.Join(content, ".tools-switcher-id"), []byte(p.ID), 0600); err != nil {
		return err
	}
	if err = os.Rename(content, root); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	previous := m.cloneState()
	m.state.Installations = append(m.state.Installations, installation)
	if err = m.persistLocked(previous); err != nil {
		os.RemoveAll(root)
		return err
	}
	return nil
}

func (m *Manager) Import(home, repoID string) error {
	abs, err := filepath.Abs(home)
	if err != nil {
		return err
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return err
	}
	installation, err := inspectHome(abs)
	if err != nil {
		return err
	}
	installation.ID = packageID("local", abs)
	installation.RepositoryID = repoID
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.state.Installations {
		if samePath(existing.Path, abs) {
			return errors.New("la JDK ya está registrada")
		}
	}
	previous := m.cloneState()
	m.state.Installations = append(m.state.Installations, installation)
	return m.persistLocked(previous)
}

func (m *Manager) Installation(installationID string) (Installation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, i := range m.state.Installations {
		if i.ID == installationID {
			return i, nil
		}
	}
	return Installation{}, errors.New("JDK no encontrada")
}
