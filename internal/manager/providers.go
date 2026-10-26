package manager

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

type providerDefinition struct{ ID, Name, Kind, Source string }

var builtInProviders = []providerDefinition{
	{"temurin", "Eclipse Temurin", "adoptium", "https://api.adoptium.net/v3"},
	{"corretto", "Amazon Corretto", "corretto", "https://corretto.aws/downloads"},
	{"oracle", "Oracle JDK", "foojay", "https://api.foojay.io/disco/v3.0"},
	{"oracle_open_jdk", "Oracle OpenJDK", "foojay", "https://api.foojay.io/disco/v3.0"},
	{"graalvm_community", "GraalVM Community", "foojay", "https://api.foojay.io/disco/v3.0"},
	{"graalvm", "Oracle GraalVM", "foojay", "https://api.foojay.io/disco/v3.0"},
	{"zulu", "Azul Zulu", "foojay", "https://api.foojay.io/disco/v3.0"},
	{"liberica", "BellSoft Liberica", "foojay", "https://api.foojay.io/disco/v3.0"},
	{"microsoft", "Microsoft OpenJDK", "foojay", "https://api.foojay.io/disco/v3.0"},
	{"jetbrains", "JetBrains Runtime", "foojay", "https://api.foojay.io/disco/v3.0"},
}

func (m *Manager) seedProviders() bool {
	changed := false
	for _, definition := range builtInProviders {
		found := false
		for i := range m.state.Repositories {
			r := &m.state.Repositories[i]
			if r.ProviderID == definition.ID || definition.ID == "temurin" && r.Kind == "adoptium" {
				found = true
				if r.ProviderID == "" {
					r.ProviderID = definition.ID
					changed = true
				}
				break
			}
		}
		if !found {
			m.state.Repositories = append(m.state.Repositories, Repository{ID: "provider-" + definition.ID, ProviderID: definition.ID, Name: definition.Name, Kind: definition.Kind, Source: definition.Source, Tool: "jdk", Enabled: definition.ID == "temurin"})
			changed = true
		}
	}
	for i := range m.state.Installations {
		item := &m.state.Installations[i]
		if inspected, err := inspectHome(item.Path); err == nil && item.Name != inspected.Name {
			item.Name = inspected.Name
			changed = true
		}
	}
	return changed
}

func (m *Manager) RepositoryVersions(repositoryID string) ([]int, error) {
	r, err := m.repository(repositoryID)
	if err != nil {
		return nil, err
	}
	if r.Kind == "adoptium" {
		return m.Versions()
	}
	if r.Kind != "foojay" && r.Kind != "corretto" {
		return []int{}, nil
	}
	var data struct {
		Result []struct {
			Versions []string `json:"versions"`
		} `json:"result"`
	}
	if err = m.getJSON(m.ctx, m.discoBase+"/distributions/"+url.PathEscape(r.ProviderID)+"?include_versions=true", &data); err != nil {
		return nil, err
	}
	majors := map[int]bool{}
	for _, d := range data.Result {
		for _, version := range d.Versions {
			if strings.Contains(strings.ToLower(version), "ea") {
				continue
			}
			value := strings.SplitN(version, ".", 2)[0]
			value = strings.SplitN(value, "+", 2)[0]
			major, e := strconv.Atoi(value)
			if e == nil && major >= 8 && major <= 100 {
				majors[major] = true
			}
		}
	}
	versions := []int{}
	for major := range majors {
		versions = append(versions, major)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(versions)))
	return versions, nil
}

func (m *Manager) loadDiscoCatalog(ctx context.Context, r Repository, major int) ([]Package, error) {
	osName, arch := runtime.GOOS, runtime.GOARCH
	if osName == "darwin" {
		osName = "macos"
	}
	if arch == "amd64" {
		arch = "x64"
	} else if arch == "arm64" {
		arch = "aarch64"
	}
	archive := "tar.gz"
	if osName == "windows" {
		archive = "zip"
	}
	params := url.Values{"distribution": {r.ProviderID}, "version": {strconv.Itoa(major)}, "operating_system": {osName}, "architecture": {arch}, "archive_type": {archive}, "package_type": {"jdk"}, "release_status": {"ga"}, "latest": {"available"}, "directly_downloadable": {"true"}}
	if osName == "linux" {
		params.Set("libc_type", "glibc")
	}
	var data struct {
		Result []struct {
			ID           string `json:"id"`
			Distribution string `json:"distribution"`
			Version      string `json:"java_version"`
			Filename     string `json:"filename"`
			OS           string `json:"operating_system"`
			Arch         string `json:"architecture"`
			PackageType  string `json:"package_type"`
			Direct       bool   `json:"directly_downloadable"`
			Size         int64  `json:"size"`
		} `json:"result"`
	}
	if err := m.getJSON(ctx, m.discoBase+"/packages?"+params.Encode(), &data); err != nil {
		return nil, err
	}
	result := []Package{}
	for _, p := range data.Result {
		if p.Distribution != r.ProviderID || p.OS != osName || normalizeArchitecture(p.Arch) != normalizeArchitecture(arch) || p.PackageType != "jdk" || !p.Direct {
			continue
		}
		if !regexp.MustCompile(`^[a-fA-F0-9]{32}$`).MatchString(p.ID) {
			return nil, errors.New("el proveedor devolvió un identificador de paquete inválido")
		}
		if !strings.HasSuffix(p.Filename, "."+archive) {
			continue
		}
		result = append(result, Package{ID: packageID(r.ID, p.ID), DiscoveryID: p.ID, RepositoryID: r.ID, Name: r.Name + " " + p.Version, Version: p.Version, Vendor: r.Name, OS: runtime.GOOS, Arch: runtime.GOARCH, Filename: p.Filename, Size: p.Size})
	}
	return result, nil
}

func (m *Manager) resolvePackage(ctx context.Context, p Package) (Package, error) {
	if p.DiscoveryID == "" {
		return p, nil
	}
	var data struct {
		Result []struct {
			Filename    string `json:"filename"`
			Download    string `json:"direct_download_uri"`
			Checksum    string `json:"checksum"`
			ChecksumURL string `json:"checksum_uri"`
			Algorithm   string `json:"checksum_type"`
		} `json:"result"`
	}
	if err := m.getJSON(ctx, m.discoBase+"/ids/"+p.DiscoveryID, &data); err != nil {
		return p, err
	}
	if len(data.Result) != 1 {
		return p, errors.New("no se ha podido resolver la descarga del paquete")
	}
	info := data.Result[0]
	if info.Filename != p.Filename {
		return p, errors.New("el nombre del paquete cambió; actualiza el catálogo")
	}
	if err := httpsURL(info.Download); err != nil {
		return p, err
	}
	p.URL = info.Download
	digest := strings.TrimSpace(info.Checksum)
	algorithm := strings.ToLower(strings.ReplaceAll(info.Algorithm, "-", ""))
	if digest == "" && info.ChecksumURL != "" && (algorithm == "sha256" || algorithm == "sha512") {
		var err error
		digest, err = m.readChecksum(ctx, info.ChecksumURL, algorithm)
		if err != nil {
			return p, err
		}
	}

	if algorithm != "sha256" && algorithm != "sha512" || digest == "" {
		fallback, kind, err := m.githubDigest(ctx, p.URL, p.Filename)
		if err != nil {
			return p, err
		}
		digest = fallback
		algorithm = kind
	}
	switch algorithm {
	case "sha256":
		p.SHA256 = digest
	case "sha512":
		p.SHA512 = digest
	default:
		return p, errors.New("el proveedor no ofrece un checksum SHA-256 o SHA-512 verificable para este paquete")
	}
	if err := validatePackage(p); err != nil {
		return p, err
	}
	return p, nil
}

func (m *Manager) readChecksum(ctx context.Context, address, algorithm string) (string, error) {
	if err := httpsURL(address); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return "", err
	}
	res, err := m.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return "", fmt.Errorf("checksum no accesible: HTTP %d", res.StatusCode)
	}
	if strings.Contains(res.Header.Get("Content-Type"), "text/html") {
		return "", errors.New("el enlace de checksum devuelve una página HTML")
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, 16385))
	if err != nil {
		return "", err
	}
	if len(data) > 16384 {
		return "", errors.New("el checksum supera el tamaño permitido")
	}
	size := 64
	if algorithm == "sha512" {
		size = 128
	} else if algorithm != "sha256" {
		return "", errors.New("algoritmo de checksum no admitido")
	}
	digest := regexp.MustCompile(fmt.Sprintf(`(?i)\b[a-f0-9]{%d}\b`, size)).FindString(string(data))
	if digest == "" {
		return "", errors.New("el archivo no contiene un checksum válido")
	}
	return strings.ToLower(digest), nil
}

func (m *Manager) SyncRepository(repositoryID string) error {
	r, err := m.repository(repositoryID)
	if err != nil {
		return err
	}
	if !r.Enabled {
		return errors.New("el repositorio está desactivado")
	}
	return m.checkRepository(r)
}

func (m *Manager) checkRepository(r Repository) error {
	major := 21
	if r.Kind == "foojay" || r.Kind == "corretto" {
		versions, err := m.RepositoryVersions(r.ID)
		if err != nil {
			return err
		}
		if len(versions) == 0 {
			return errors.New("el proveedor no anuncia versiones estables")
		}
		major = versions[0]
		for _, candidate := range []int{25, 21} {
			for _, version := range versions {
				if version == candidate {
					major = candidate
					break
				}
			}
			if major == candidate {
				break
			}
		}
	}
	packages, err := m.loadCatalog(r, major)
	if err == nil && len(packages) == 0 {
		err = fmt.Errorf("%s no ofrece una JDK Java %d compatible con %s/%s", r.Name, major, runtime.GOOS, runtime.GOARCH)
	}
	if err == nil && r.Kind != "local" {
		sample, resolveErr := m.resolvePackage(m.ctx, packages[0])
		if resolveErr != nil {
			err = resolveErr
		} else {
			err = m.probe(sample.URL)
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	previous := m.cloneState()
	for i := range m.state.Repositories {
		if m.state.Repositories[i].ID == r.ID {
			m.state.Repositories[i].Error = ""
			if err != nil {
				m.state.Repositories[i].Error = err.Error()
			} else {
				m.state.Repositories[i].Count = len(packages)
				m.state.Repositories[i].LastSync = now()
			}
			break
		}
	}
	if saveErr := m.persistLocked(previous); saveErr != nil {
		return saveErr
	}
	if err != nil {
		return err
	}
	for _, p := range packages {
		m.packages[p.ID] = p
	}
	return nil
}

func normalizeArchitecture(arch string) string {
	switch arch {
	case "amd64", "x64", "x86_64", "x86-64":
		return "amd64"
	case "arm64", "aarch64":
		return "arm64"
	}
	return arch
}

func (m *Manager) githubDigest(ctx context.Context, address, filename string) (string, string, error) {
	u, err := url.Parse(address)
	if err != nil || !strings.EqualFold(u.Hostname(), "github.com") {
		return "", "", errors.New("el proveedor no ofrece un checksum SHA-256 o SHA-512 verificable para este paquete")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 6 || parts[2] != "releases" || parts[3] != "download" {
		return "", "", errors.New("el enlace no identifica un artefacto oficial de GitHub")
	}
	var release struct {
		Assets []struct {
			Name   string `json:"name"`
			Digest string `json:"digest"`
		} `json:"assets"`
	}
	endpoint := m.githubBase + "/repos/" + url.PathEscape(parts[0]) + "/" + url.PathEscape(parts[1]) + "/releases/tags/" + url.PathEscape(parts[4])
	if err = m.getJSON(ctx, endpoint, &release); err != nil {
		return "", "", err
	}
	for _, asset := range release.Assets {
		if asset.Name == filename {
			algorithm, digest, ok := strings.Cut(asset.Digest, ":")
			if ok && (algorithm == "sha256" || algorithm == "sha512") {
				return digest, algorithm, nil
			}
		}
	}
	return "", "", errors.New("el artefacto oficial no publica un checksum fuerte verificable")
}
