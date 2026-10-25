package manager

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

func (m *Manager) getJSON(ctx context.Context, address string, dst any) error {
	if err := httpsURL(address); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "tools-switcher")
	res, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("catálogo no accesible: HTTP %d", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, 8*1024*1024+1))
	if err != nil {
		return err
	}
	if len(data) > 8*1024*1024 {
		return errors.New("el catálogo supera 8 MB")
	}
	if err = json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("el origen no devuelve un catálogo JSON compatible: %w", err)
	}
	return nil
}

func httpsURL(address string) error {
	u, err := url.Parse(address)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
		return errors.New("introduce una URL HTTPS sin credenciales")
	}
	return nil
}
func packageID(repo, source string) string {
	sum := sha256.Sum256([]byte(repo + "\x00" + source))
	return hex.EncodeToString(sum[:12])
}
func compatible(p Package) bool { return p.OS == runtime.GOOS && p.Arch == runtime.GOARCH }
func (m *Manager) Versions() ([]int, error) {
	var info struct {
		Available []int `json:"available_releases"`
	}
	err := m.getJSON(m.ctx, "https://api.adoptium.net/v3/info/available_releases", &info)
	if err != nil {
		return nil, err
	}
	sort.Sort(sort.Reverse(sort.IntSlice(info.Available)))
	return info.Available, nil
}

func (m *Manager) AddRepository(name, source, kind string) (Repository, error) {
	source = strings.TrimSpace(source)
	name = strings.TrimSpace(name)
	r := Repository{ID: id(), Name: name, Source: source, Kind: kind, Tool: "jdk", Enabled: true}
	switch kind {
	case "local":
		abs, err := filepath.Abs(source)
		if err != nil {
			return r, err
		}
		abs, err = filepath.EvalSymlinks(abs)
		if err != nil {
			return r, err
		}
		info, err := os.Stat(abs)
		if err != nil || !info.IsDir() {
			return r, errors.New("selecciona una carpeta existente")
		}
		r.Source = abs
	case "remote":
		if err := httpsURL(source); err != nil {
			return r, err
		}
		u, _ := url.Parse(source)
		if strings.EqualFold(u.Hostname(), "api.adoptium.net") {
			r.Kind = "adoptium"
			r.Source = "https://api.adoptium.net/v3"
			if r.Name == "" {
				r.Name = "Eclipse Temurin"
			}
		} else {
			r.Kind = "manifest"
		}
	default:
		return r, errors.New("tipo de repositorio desconocido")
	}
	if r.Name == "" {
		if r.Kind == "local" {
			r.Name = filepath.Base(r.Source)
		} else {
			r.Name = "Catálogo JDK"
		}
	}
	packages, err := m.loadCatalog(r, 21)
	if err != nil {
		return r, err
	}
	if len(packages) == 0 {
		return r, errors.New("el origen no contiene ninguna JDK compatible con este equipo")
	}
	if r.Kind != "local" {
		if err = m.probe(packages[0].URL); err != nil {
			return r, fmt.Errorf("el paquete de muestra no es descargable: %w", err)
		}
	}
	r.Count = len(packages)
	r.LastSync = now()
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.state.Repositories {
		if strings.EqualFold(existing.Source, r.Source) {
			return r, errors.New("este repositorio ya está registrado")
		}
	}
	previous := m.cloneState()
	m.state.Repositories = append(m.state.Repositories, r)
	if err = m.persistLocked(previous); err != nil {
		return r, err
	}
	for _, p := range packages {
		m.packages[p.ID] = p
	}
	return r, nil
}

func (m *Manager) probe(address string) error {
	req, err := http.NewRequestWithContext(m.ctx, http.MethodGet, address, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Range", "bytes=0-0")
	res, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 && res.StatusCode != 206 {
		return fmt.Errorf("HTTP %d", res.StatusCode)
	}
	content := res.Header.Get("Content-Type")
	if strings.Contains(content, "text/html") {
		return errors.New("el enlace devuelve una página HTML")
	}
	var one [1]byte
	_, err = io.ReadFull(res.Body, one[:])
	return err
}

func (m *Manager) repository(repoID string) (Repository, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.state.Repositories {
		if r.ID == repoID {
			return r, nil
		}
	}
	return Repository{}, errors.New("repositorio no encontrado")
}

func (m *Manager) Catalog(repoID string, major int) ([]Package, error) {
	r, err := m.repository(repoID)
	if err != nil {
		return nil, err
	}
	if !r.Enabled {
		return nil, errors.New("el repositorio está desactivado")
	}
	if major < 8 || major > 100 {
		return nil, errors.New("versión de Java no válida")
	}
	packages, loadErr := m.loadCatalog(r, major)
	m.mu.Lock()
	defer m.mu.Unlock()
	previous := m.cloneState()
	for i := range m.state.Repositories {
		if m.state.Repositories[i].ID == repoID {
			if loadErr != nil {
				m.state.Repositories[i].Error = loadErr.Error()
			} else {
				m.state.Repositories[i].Count = len(packages)
				m.state.Repositories[i].LastSync = now()
				m.state.Repositories[i].Error = ""
			}
		}
	}
	if err = m.persistLocked(previous); err != nil {
		return nil, err
	}
	if loadErr != nil {
		return nil, loadErr
	}
	for _, p := range packages {
		m.packages[p.ID] = p
	}
	return packages, nil
}

func (m *Manager) ToggleRepository(repoID string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	previous := m.cloneState()
	for i := range m.state.Repositories {
		if m.state.Repositories[i].ID == repoID {
			m.state.Repositories[i].Enabled = enabled
			return m.persistLocked(previous)
		}
	}
	return errors.New("repositorio no encontrado")
}
func (m *Manager) RemoveRepository(repoID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	previous := m.cloneState()
	for i, r := range m.state.Repositories {
		if r.ID == repoID {
			m.state.Repositories = append(m.state.Repositories[:i], m.state.Repositories[i+1:]...)
			return m.persistLocked(previous)
		}
	}
	return errors.New("repositorio no encontrado")
}

func (m *Manager) loadCatalog(r Repository, major int) ([]Package, error) {
	ctx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
	defer cancel()
	switch r.Kind {
	case "adoptium":
		osName := runtime.GOOS
		if osName == "darwin" {
			osName = "mac"
		}
		arch := runtime.GOARCH
		if arch == "amd64" {
			arch = "x64"
		} else if arch == "arm64" {
			arch = "aarch64"
		}
		endpoint := fmt.Sprintf("%s/assets/latest/%d/hotspot?architecture=%s&image_type=jdk&os=%s&vendor=eclipse", r.Source, major, arch, osName)
		var assets []struct {
			Version struct {
				Semver string `json:"semver"`
			} `json:"version"`
			Binary struct {
				Package struct {
					Name     string `json:"name"`
					Link     string `json:"link"`
					Checksum string `json:"checksum"`
					Size     int64  `json:"size"`
				} `json:"package"`
			} `json:"binary"`
		}
		if err := m.getJSON(ctx, endpoint, &assets); err != nil {
			return nil, err
		}
		result := []Package{}
		for _, a := range assets {
			p := Package{RepositoryID: r.ID, Name: "Eclipse Temurin " + a.Version.Semver, Version: a.Version.Semver, Vendor: "Eclipse Adoptium", OS: runtime.GOOS, Arch: runtime.GOARCH, URL: a.Binary.Package.Link, SHA256: a.Binary.Package.Checksum, Filename: a.Binary.Package.Name, Size: a.Binary.Package.Size}
			p.ID = packageID(r.ID, p.URL)
			if err := validatePackage(p); err != nil {
				return nil, err
			}
			result = append(result, p)
		}
		return result, nil
	case "manifest":
		var manifest struct {
			SchemaVersion int       `json:"schemaVersion"`
			Packages      []Package `json:"packages"`
		}
		if err := m.getJSON(ctx, r.Source, &manifest); err != nil {
			return nil, err
		}
		if manifest.SchemaVersion != 1 {
			return nil, errors.New("el manifiesto debe usar schemaVersion 1")
		}
		result := []Package{}
		for _, p := range manifest.Packages {
			p.RepositoryID = r.ID
			p.LocalPath = ""
			p.ID = packageID(r.ID, p.URL)
			if err := validatePackage(p); err != nil {
				return nil, err
			}
			if compatible(p) {
				result = append(result, p)
			}
		}
		return result, nil
	case "local":
		return scanLocal(r)
	}
	return nil, errors.New("proveedor desconocido")
}

func validatePackage(p Package) error {
	if err := httpsURL(p.URL); err != nil {
		return err
	}
	hash, err := hex.DecodeString(p.SHA256)
	if err != nil || len(hash) != 32 {
		return errors.New("cada paquete necesita un checksum SHA-256 válido")
	}
	if p.Name == "" || p.Version == "" || p.OS == "" || p.Arch == "" {
		return errors.New("faltan metadatos de la JDK")
	}
	if filepath.Base(p.Filename) != p.Filename || strings.ContainsAny(p.Filename, "\\/") || (!strings.HasSuffix(p.Filename, ".zip") && !strings.HasSuffix(p.Filename, ".tar.gz")) {
		return errors.New("solo se admiten paquetes .zip y .tar.gz")
	}
	return nil
}
