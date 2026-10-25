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
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jecortes2304/tools-switcher/internal/platform"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	m, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.state.Settings.Vault = filepath.Join(t.TempDir(), "vault")
	t.Cleanup(m.Close)
	return m
}
func jdkZip(t *testing.T) []byte {
	t.Helper()
	var data bytes.Buffer
	w := zip.NewWriter(&data)
	java, javac := javaNames()
	files := map[string]string{"jdk/release": "JAVA_VERSION=\"21.0.9\"\nIMPLEMENTOR=\"Test JDK\"\nOS_ARCH=\"" + runtime.GOARCH + "\"\nOS_NAME=\"" + runtime.GOOS + "\"\n", "jdk/bin/" + java: "java", "jdk/bin/" + javac: "javac"}
	for name, body := range files {
		header := &zip.FileHeader{Name: name}
		header.SetMode(0755)
		f, err := w.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}
func waitJob(t *testing.T, m *Manager, id string) Download {
	t.Helper()
	deadline := time.After(5 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			t.Fatal("job timed out")
		case <-ticker.C:
			for _, j := range m.Snapshot().Downloads {
				if j.ID == id && (j.Status == "completed" || j.Status == "error" || j.Status == "cancelled") {
					return j
				}
			}
		}
	}
}

func TestManifestDownloadVerificationAndRemoval(t *testing.T) {
	for _, valid := range []bool{true, false} {
		t.Run(map[bool]string{true: "verified", false: "tampered"}[valid], func(t *testing.T) {
			m := newTestManager(t)
			archive := jdkZip(t)
			sum := sha256.Sum256(archive)
			hash := hex.EncodeToString(sum[:])
			if !valid {
				hash = strings.Repeat("0", 64)
			}
			var server *httptest.Server
			server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/package.zip" {
					w.Header().Set("Content-Type", "application/zip")
					_, _ = w.Write(archive)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"schemaVersion": 1, "packages": []Package{{Name: "Test JDK", Version: "21.0.9", OS: runtime.GOOS, Arch: runtime.GOARCH, URL: server.URL + "/package.zip", SHA256: hash, Filename: "package.zip", Size: int64(len(archive))}}})
			}))
			defer server.Close()
			m.client = server.Client()
			repo, err := m.AddRepository("Test", server.URL+"/manifest.json", "remote")
			if err != nil {
				t.Fatal(err)
			}
			packages, err := m.Catalog(repo.ID, 21)
			if err != nil {
				t.Fatal(err)
			}
			if err = m.Queue([]string{packages[0].ID}); err != nil {
				t.Fatal(err)
			}
			job := waitJob(t, m, packages[0].ID)
			if !valid {
				if job.Status != "error" || !strings.Contains(job.Error, "checksum") {
					t.Fatalf("unexpected result: %+v", job)
				}
				if len(m.Snapshot().Installations) != 0 {
					t.Fatal("tampered package was registered")
				}
				return
			}
			if job.Status != "completed" {
				t.Fatalf("%+v", job)
			}
			installed := m.Snapshot().Installations
			if len(installed) != 1 {
				t.Fatal("installation missing")
			}
			if _, err = inspectHome(installed[0].Path); err != nil {
				t.Fatal(err)
			}
			reload, err := New(filepath.Dir(m.file))
			if err != nil {
				t.Fatal(err)
			}
			defer reload.Close()
			if len(reload.Snapshot().Installations) != 1 {
				t.Fatal("installation not persisted")
			}
			if err = m.RemoveInstallation(installed[0].ID, true); err != nil {
				t.Fatal(err)
			}
			if _, err = os.Stat(installed[0].ManagedRoot); !os.IsNotExist(err) {
				t.Fatal("managed root was not deleted")
			}
		})
	}
}

func TestRejectRepositoryWithoutPackages(t *testing.T) {
	m := newTestManager(t)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"schemaVersion":1,"packages":[]}`))
	}))
	defer server.Close()
	m.client = server.Client()
	if _, err := m.AddRepository("Empty", server.URL, "remote"); err == nil {
		t.Fatal("empty repository accepted")
	}
	if _, err := m.AddRepository("HTTP", "http://example.com", "remote"); err == nil {
		t.Fatal("HTTP repository accepted")
	}
	if len(m.Snapshot().Repositories) != 0 {
		t.Fatal("invalid repository persisted")
	}
}

func TestArchiveTraversalRejected(t *testing.T) {
	for _, name := range []string{"../escape", "/absolute", "jdk/../../escape", "jdk\\..\\escape", "C:/escape"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			file := filepath.Join(root, "bad.zip")
			f, err := os.Create(file)
			if err != nil {
				t.Fatal(err)
			}
			z := zip.NewWriter(f)
			entry, err := z.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = entry.Write([]byte("danger"))
			_ = z.Close()
			_ = f.Close()
			dest := filepath.Join(root, "dest")
			_ = os.Mkdir(dest, 0700)
			if err = extract(context.Background(), file, dest); err == nil {
				t.Fatal("unsafe path accepted")
			}
			if _, err = os.Stat(filepath.Join(root, "escape")); !os.IsNotExist(err) {
				t.Fatal("archive escaped destination")
			}
		})
	}
}

func TestTarLinkEscapeRejected(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "bad.tar.gz")
	f, _ := os.Create(file)
	g := gzip.NewWriter(f)
	w := tar.NewWriter(g)
	_ = w.WriteHeader(&tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "../outside"})
	_ = w.Close()
	_ = g.Close()
	_ = f.Close()
	dest := filepath.Join(root, "dest")
	_ = os.Mkdir(dest, 0700)
	if err := extract(context.Background(), file, dest); err == nil {
		t.Fatal("escaping symlink accepted")
	}
}

func TestCancellationDiscardsStaging(t *testing.T) {
	m := newTestManager(t)
	started := make(chan struct{})
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done() }))
	defer server.Close()
	m.client = server.Client()
	p := Package{ID: "cancel-test", RepositoryID: "test", Name: "Cancel", Filename: "jdk.zip", URL: server.URL, OS: runtime.GOOS, Arch: runtime.GOARCH, SHA256: strings.Repeat("0", 64)}
	m.packages[p.ID] = p
	m.state.Repositories = append(m.state.Repositories, Repository{ID: "test", Enabled: true})
	if err := m.Queue([]string{p.ID}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("download never started")
	}
	m.CancelDownload(p.ID)
	if job := waitJob(t, m, p.ID); job.Status != "cancelled" {
		t.Fatalf("%+v", job)
	}
	entries, err := os.ReadDir(m.Settings().Vault)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("staging was not cleaned")
	}
}

func fakeEnvironment(t *testing.T, values map[string]string) {
	t.Helper()
	read, write := platform.ReadVariable, platform.WriteVariable
	t.Cleanup(func() { platform.ReadVariable = read; platform.WriteVariable = write })
	platform.ReadVariable = func(name string) (string, bool, error) { v, ok := values[name]; return v, ok, nil }
	platform.WriteVariable = func(name, value string, exists bool) error {
		if exists {
			values[name] = value
		} else {
			delete(values, name)
		}
		return nil
	}
}
func importFixture(t *testing.T, m *Manager) Installation {
	t.Helper()
	dir := t.TempDir()
	archive := filepath.Join(dir, "jdk.zip")
	if err := os.WriteFile(archive, jdkZip(t), 0600); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "content")
	_ = os.Mkdir(dest, 0700)
	if err := extract(context.Background(), archive, dest); err != nil {
		t.Fatal(err)
	}
	if err := m.Import(filepath.Join(dest, "jdk"), ""); err != nil {
		t.Fatal(err)
	}
	return m.Snapshot().Installations[len(m.Snapshot().Installations)-1]
}

func TestSwitchAndRestoreEnvironment(t *testing.T) {
	m := newTestManager(t)
	first := importFixture(t, m)
	second := importFixture(t, m)
	pathKey := canonicalVariable("PATH")
	values := map[string]string{"JAVA_HOME": "original-java", pathKey: "original-path"}
	fakeEnvironment(t, values)
	if err := m.Activate(first.ID); err != nil {
		t.Fatal(err)
	}
	if values["JAVA_HOME"] != first.Path || !pathContains(values[pathKey], filepath.Join(first.Path, "bin")) {
		t.Fatal("activation not applied")
	}
	if err := m.Activate(second.ID); err != nil {
		t.Fatal(err)
	}
	if pathContains(values[pathKey], filepath.Join(first.Path, "bin")) {
		t.Fatal("previous path entry left behind")
	}
	if err := m.Restore(second.ID); err != nil {
		t.Fatal(err)
	}
	if values["JAVA_HOME"] != "original-java" || values[pathKey] != "original-path" {
		t.Fatalf("baseline lost: %+v", values)
	}
}
func TestEnvironmentFailureRollsBackActivation(t *testing.T) {
	m := newTestManager(t)
	i := importFixture(t, m)
	values := map[string]string{"JAVA_HOME": "original"}
	fakeEnvironment(t, values)
	write := platform.WriteVariable
	platform.WriteVariable = func(name, value string, exists bool) error {
		if name == canonicalVariable("PATH") {
			return errors.New("access denied")
		}
		return write(name, value, exists)
	}
	if err := m.Activate(i.ID); err == nil {
		t.Fatal("activation unexpectedly succeeded")
	}
	if values["JAVA_HOME"] != "original" || len(m.state.Bindings) != 0 {
		t.Fatal("partial activation left behind")
	}
}
func TestExternalChangesAndExternalFilesPreserved(t *testing.T) {
	m := newTestManager(t)
	i := importFixture(t, m)
	values := map[string]string{}
	fakeEnvironment(t, values)
	if err := m.Assign(i.ID, "JDK_TEST_HOME"); err != nil {
		t.Fatal(err)
	}
	values["JDK_TEST_HOME"] = "changed-elsewhere"
	if err := m.Restore(i.ID); err == nil {
		t.Fatal("external edit was overwritten")
	}
	if values["JDK_TEST_HOME"] != "changed-elsewhere" {
		t.Fatal("external variable changed")
	}
	values["JDK_TEST_HOME"] = i.Path
	if err := m.RemoveInstallation(i.ID, true); err == nil {
		t.Fatal("external files were allowed to be deleted")
	}
	if _, err := os.Stat(i.Path); err != nil {
		t.Fatal("external home removed")
	}
	if err := m.RemoveInstallation(i.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(i.Path); err != nil {
		t.Fatal("unregister deleted external home")
	}
}
func TestCorruptStatePreserved(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "state.json")
	data := []byte(`{"schemaVersion":`)
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(dir); err == nil {
		t.Fatal("corrupt state accepted")
	}
	after, _ := os.ReadFile(file)
	if !bytes.Equal(data, after) {
		t.Fatal("corrupt state was overwritten")
	}
}

func TestManagedInstallThroughVaultAlias(t *testing.T) {
	m := newTestManager(t)
	root := t.TempDir()
	real := filepath.Join(root, "real-vault")
	alias := filepath.Join(root, "alias-vault")
	if err := os.Mkdir(real, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, alias); err != nil {
		t.Skip("directory aliases unavailable: " + err.Error())
	}
	file := filepath.Join(root, "jdk.zip")
	if err := os.WriteFile(file, jdkZip(t), 0600); err != nil {
		t.Fatal(err)
	}
	p := Package{ID: "alias-install", Name: "Test JDK", Filename: "jdk.zip", LocalPath: file, OS: runtime.GOOS, Arch: runtime.GOARCH}
	if err := m.install(context.Background(), p, alias); err != nil {
		t.Fatal(err)
	}
	installation := m.Snapshot().Installations[0]
	canonical, err := filepath.EvalSymlinks(installation.ManagedRoot)
	if err != nil {
		t.Fatal(err)
	}
	if !samePath(canonical, installation.ManagedRoot) {
		t.Fatal("managed root was not canonicalized")
	}
	if err = m.RemoveInstallation(installation.ID, true); err != nil {
		t.Fatal(err)
	}
}
