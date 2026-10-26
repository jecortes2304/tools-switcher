package manager

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFreshPreferencesAndProviderMigration(t *testing.T) {
	dir := t.TempDir()
	m, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Snapshot().Repositories) != 10 {
		t.Fatal("default providers missing")
	}
	if _, err = os.Stat(m.PreferencesFile()); err != nil {
		t.Fatal("fresh preferences not persisted")
	}
	if err = m.ToggleRepository("provider-oracle", false); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(m.PreferencesFile()); err != nil {
		t.Fatal(err)
	}
	if err = m.EnsurePreferencesFile(); err != nil {
		t.Fatal(err)
	}
	m.Close()
	reloaded, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reloaded.Close()
	if len(reloaded.Snapshot().Repositories) != 10 {
		t.Fatal("providers duplicated after restart")
	}
	legacy := State{SchemaVersion: 1, Repositories: []Repository{{ID: "existing-temurin", Name: "Mi Temurin", Kind: "adoptium", Source: "https://api.adoptium.net/v3", Enabled: false}}}
	data, _ := json.Marshal(legacy)
	legacyDir := t.TempDir()
	if err = os.WriteFile(filepath.Join(legacyDir, "state.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	migrated, err := New(legacyDir)
	if err != nil {
		t.Fatal(err)
	}
	defer migrated.Close()
	repos := migrated.Snapshot().Repositories
	if len(repos) != 10 || repos[0].ID != "existing-temurin" || repos[0].Name != "Mi Temurin" || repos[0].ProviderID != "temurin" || repos[0].Enabled {
		t.Fatal("legacy provider settings lost")
	}
}

func TestProviderPackageResolutionAndInstallation(t *testing.T) {
	m := newTestManager(t)
	archive, filename := providerArchive(t)
	digest := ""
	id := strings.Repeat("a", 32)
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/distributions/"):
			_ = json.NewEncoder(w).Encode(map[string]any{"result": []any{map[string]any{"versions": []string{"26-ea", "25.0.1", "21.0.1", "21", "8"}}}})
		case r.URL.Path == "/packages":
			if r.URL.Query().Get("distribution") != "oracle" || r.URL.Query().Get("directly_downloadable") != "true" {
				t.Error("provider filters missing")
			}
			osName, arch := runtime.GOOS, runtime.GOARCH
			if osName == "darwin" {
				osName = "macos"
			}
			if arch == "amd64" {
				arch = "x64"
			} else if arch == "arm64" {
				arch = "aarch64"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"result": []any{map[string]any{"id": id, "distribution": "oracle", "java_version": "21.0.9", "filename": filename, "operating_system": osName, "architecture": arch, "package_type": "jdk", "directly_downloadable": true, "size": len(archive)}, map[string]any{"id": strings.Repeat("b", 32), "distribution": "zulu", "directly_downloadable": true}}})
		case r.URL.Path == "/ids/"+id:
			_ = json.NewEncoder(w).Encode(map[string]any{"result": []any{map[string]any{"filename": filename, "direct_download_uri": server.URL + "/" + filename, "checksum_type": "sha256", "checksum_uri": server.URL + "/checksum"}}})
		case r.URL.Path == "/checksum":
			_, _ = fmt.Fprint(w, digest)
		case r.URL.Path == "/"+filename:
			_, _ = w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	m.client = server.Client()
	m.discoBase = server.URL
	sum := sha256Bytes(archive)
	digest = sum
	repo := Repository{ID: "test-provider", ProviderID: "oracle", Name: "Oracle JDK", Kind: "foojay", Source: server.URL}
	m.state.Repositories = append(m.state.Repositories, repo)
	versions, err := m.RepositoryVersions(repo.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 3 || versions[0] != 25 || versions[1] != 21 {
		t.Fatal("version filtering failed")
	}
	if err = m.ToggleRepository(repo.ID, true); err != nil {
		t.Fatal(err)
	}
	packages, err := m.Catalog(repo.ID, 21)
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 1 {
		t.Fatal("foreign provider package accepted")
	}
	resolved, err := m.resolvePackage(context.Background(), packages[0])
	if err != nil {
		t.Fatal(err)
	}
	if resolved.SHA256 != digest {
		t.Fatal("checksum not resolved")
	}
	if err = m.Queue([]string{packages[0].ID}); err != nil {
		t.Fatal(err)
	}
	if job := waitJob(t, m, packages[0].ID); job.Status != "completed" {
		t.Fatal(job.Error)
	}
	if err = m.RemoveRepository(repo.ID); err != nil {
		t.Fatal(err)
	}
	r, _ := m.repository(repo.ID)
	if r.Enabled {
		t.Fatal("built-in provider was not disabled")
	}
	if len(m.Snapshot().Installations) != 1 {
		t.Fatal("disabling removed installation")
	}
}

func TestRejectInvalidProviderChecksums(t *testing.T) {
	m := newTestManager(t)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, "<html>"+strings.Repeat("a", 64)+"</html>")
	}))
	defer server.Close()
	m.client = server.Client()
	if _, err := m.readChecksum(context.Background(), server.URL, "sha256"); err == nil {
		t.Fatal("HTML accepted as checksum")
	}
}

func TestDescribeGraalVM(t *testing.T) {
	m := newTestManager(t)
	i := importFixture(t, m)
	release := []byte("JAVA_VERSION=\"25.0.4\"\nIMPLEMENTOR=\"Oracle Corporation\"\nIMPLEMENTOR_VERSION=\"GraalVM 25.0.4\"\n")
	if err := os.WriteFile(filepath.Join(i.Path, "release"), release, 0600); err != nil {
		t.Fatal(err)
	}
	if got := m.DescribeHome(i.Path); got != "GraalVM 25.0.4" {
		t.Fatal(got)
	}
}

func TestLiveProviders(t *testing.T) {
	if os.Getenv("TOOLS_SWITCHER_PROVIDER_TEST") != "1" {
		t.Skip("enable TOOLS_SWITCHER_PROVIDER_TEST for public API checks")
	}
	m, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	for _, r := range m.Snapshot().Repositories {
		t.Run(r.ProviderID, func(t *testing.T) {
			if err := m.ToggleRepository(r.ID, true); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func sha256Bytes(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

func TestGitHubStrongDigestFallback(t *testing.T) {
	m := newTestManager(t)
	digest := strings.Repeat("a", 64)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/vendor/jdks/releases/tags/25.0.1+1" {
			t.Error(r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"assets": []any{map[string]string{"name": "jdk.zip", "digest": "sha256:" + digest}}})
	}))
	defer server.Close()
	m.client = server.Client()
	m.githubBase = server.URL
	hash, algorithm, err := m.githubDigest(context.Background(), "https://github.com/vendor/jdks/releases/download/25.0.1%2B1/jdk.zip", "jdk.zip")
	if err != nil || hash != digest || algorithm != "sha256" {
		t.Fatalf("%s %s %v", hash, algorithm, err)
	}
	if _, _, err = m.githubDigest(context.Background(), "https://example.com/jdk.zip", "jdk.zip"); err == nil {
		t.Fatal("untrusted checksum fallback accepted")
	}
}

func TestCorrettoUsesOfficialLatestAndDetectsReleaseRace(t *testing.T) {
	m := newTestManager(t)
	data := []byte("archive fixture")
	digest := sha256Bytes(data)
	change := false
	count := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/latest_sha256/") {
			_, _ = fmt.Fprint(w, digest)
			return
		}
		if strings.Contains(r.URL.Path, "/latest/") {
			count++
			version := "25.0.5.0.9"
			if change && count%2 == 0 {
				version = "25.0.6.0.1"
			}
			filename := "jdk.tar.gz"
			if runtime.GOOS == "windows" {
				filename = "jdk.zip"
			}
			http.Redirect(w, r, "/resources/"+version+"/"+filename, http.StatusFound)
			return
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-0/%d", len(data)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(data[:1])
	}))
	defer server.Close()
	m.client = server.Client()
	m.correttoBase = server.URL
	repo := Repository{ID: "corretto", ProviderID: "corretto", Name: "Amazon Corretto", Kind: "corretto"}
	packages, err := m.loadCorrettoCatalog(context.Background(), repo, 25)
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 1 || packages[0].Version != "25.0.5.0.9" || packages[0].SHA256 != digest || packages[0].Size != int64(len(data)) {
		t.Fatal("incorrect official package metadata")
	}
	change = true
	count = 0
	if _, err = m.loadCorrettoCatalog(context.Background(), repo, 25); err == nil {
		t.Fatal("release changed while binding its checksum")
	}
}

func providerArchive(t *testing.T) ([]byte, string) {
	data := jdkZip(t)
	if runtime.GOOS == "windows" {
		return data, "jdk.zip"
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	writer := tar.NewWriter(gzipWriter)
	for _, file := range reader.File {
		source, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		if err = writer.WriteHeader(&tar.Header{Name: file.Name, Size: int64(file.UncompressedSize64), Mode: 0755}); err != nil {
			t.Fatal(err)
		}
		_, err = io.Copy(writer, source)
		source.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err = gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes(), "jdk.tar.gz"
}
