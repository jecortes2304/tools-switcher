package manager

import (
	"github.com/jecortes2304/tools-switcher/internal/platform"
	"time"
)

type Settings struct {
	Theme            string `json:"theme"`
	Vault            string `json:"vault"`
	StartAtLogin     bool   `json:"startAtLogin"`
	CloseToTray      bool   `json:"closeToTray"`
	AutoCheckUpdates bool   `json:"autoCheckUpdates"`
}

type Repository struct {
	ProviderID string `json:"providerId"`
	ID         string `json:"id"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Source     string `json:"source"`
	Tool       string `json:"tool"`
	Enabled    bool   `json:"enabled"`
	Count      int    `json:"count"`
	LastSync   string `json:"lastSync"`
	Error      string `json:"error"`
}

type Package struct {
	DiscoveryID  string `json:"discoveryId"`
	ID           string `json:"id"`
	RepositoryID string `json:"repositoryId"`
	Name         string `json:"name"`
	Version      string `json:"version"`
	Vendor       string `json:"vendor"`
	OS           string `json:"os"`
	Arch         string `json:"arch"`
	URL          string `json:"url"`
	SHA256       string `json:"sha256"`
	SHA512       string `json:"sha512"`
	Filename     string `json:"filename"`
	Size         int64  `json:"size"`
	LocalPath    string `json:"localPath"`
}

type Installation struct {
	ID           string `json:"id"`
	Tool         string `json:"tool"`
	Name         string `json:"name"`
	Version      string `json:"version"`
	Vendor       string `json:"vendor"`
	RepositoryID string `json:"repositoryId"`
	Path         string `json:"path"`
	Managed      bool   `json:"managed"`
	ManagedRoot  string `json:"managedRoot"`
}

type Download struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Written int64  `json:"written"`
	Total   int64  `json:"total"`
	Error   string `json:"error"`
}

type Binding struct {
	Name           string `json:"name"`
	Previous       string `json:"previous"`
	Existed        bool   `json:"existed"`
	InstallationID string `json:"installationId"`
	Applied        string `json:"applied"`
}

type State struct {
	SchemaVersion int            `json:"schemaVersion"`
	Settings      Settings       `json:"settings"`
	Repositories  []Repository   `json:"repositories"`
	Installations []Installation `json:"installations"`
	Bindings      []Binding      `json:"bindings"`
}

type Snapshot struct {
	ActiveSource    string                         `json:"activeSource"`
	ActiveName      string                         `json:"activeName"`
	PreferencesFile string                         `json:"preferencesFile"`
	Environment     []platform.EnvironmentVariable `json:"environment"`
	Settings        Settings                       `json:"settings"`
	Repositories    []Repository                   `json:"repositories"`
	Installations   []Installation                 `json:"installations"`
	Downloads       []Download                     `json:"downloads"`
	ActivePath      string                         `json:"activePath"`
	ResolvedJava    string                         `json:"resolvedJava"`
	Platform        string                         `json:"platform"`
	Architecture    string                         `json:"architecture"`
}

var now = func() string { return time.Now().UTC().Format(time.RFC3339) }
