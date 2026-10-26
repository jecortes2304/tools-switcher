package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestReleaseManifestAndChecksums(t *testing.T) {
	if _, err := exec.LookPath("wails3"); err != nil {
		t.Skip("Wails CLI not installed")
	}
	t.Chdir(t.TempDir())
	version = "1.2.3"
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("UPDATER_PRIVATE_KEY_BASE64", base64.StdEncoding.EncodeToString(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})))
	t.Setenv("UPDATER_PUBLIC_KEY_BASE64", base64.StdEncoding.EncodeToString(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})))
	files := []string{"updates/tools-switcher-1.2.3-windows-amd64.exe", "updates/tools-switcher-1.2.3-darwin-arm64.zip", "updates/tools-switcher-1.2.3-linux-amd64.tar.gz", "packages/tools-switcher-1.2.3-windows-amd64-setup.exe", "packages/tools-switcher-1.2.3-darwin-arm64.dmg", "packages/tools-switcher.AppImage", "packages/tools-switcher.deb", "packages/tools-switcher.rpm"}
	for _, file := range files {
		dest := filepath.Join("artifacts", file)
		if err = os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(dest, []byte("fixture:"+file), 0600); err != nil {
			t.Fatal(err)
		}
	}
	release()
	data, err := os.ReadFile("artifacts/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Version   string `json:"version"`
		Artifacts []struct {
			Signature string `json:"signature"`
			Platform  string `json:"platform"`
			Arch      string `json:"arch"`
		} `json:"artifacts"`
	}
	if err = json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Version != "1.2.3" || len(manifest.Artifacts) != 3 {
		t.Fatal("wrong update manifest")
	}
	for _, artifact := range manifest.Artifacts {
		if artifact.Signature == "" || artifact.Platform == "" || artifact.Arch == "" {
			t.Fatal("unsigned or unscoped update artifact")
		}
	}
	sums, err := os.ReadFile("artifacts/SHA256SUMS")
	if err != nil || len(sums) == 0 {
		t.Fatal("checksums missing")
	}
}
