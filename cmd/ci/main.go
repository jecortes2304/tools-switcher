package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var version string

func run(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		panic(fmt.Errorf("%s: %w", name, err))
	}
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}
func copyFile(source, dest string) {
	in, err := os.Open(source)
	must(err)
	defer in.Close()
	must(os.MkdirAll(filepath.Dir(dest), 0755))
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	must(err)
	_, err = io.Copy(out, in)
	must(err)
	must(out.Close())
}
func isProduction() bool { return os.Getenv("GITHUB_REF") == "refs/heads/main" }
func main() {
	data, err := os.ReadFile("VERSION")
	must(err)
	version = strings.TrimSpace(string(data))
	if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(version) {
		panic("VERSION debe tener formato X.Y.Z")
	}
	if len(os.Args) != 2 {
		panic("uso: go run ./cmd/ci preflight|build|release")
	}
	switch os.Args[1] {
	case "preflight":
		preflight()
	case "build":
		build()
	case "release":
		release()
	default:
		panic("operación desconocida")
	}
}
func preflight() {
	if isProduction() {
		for _, name := range []string{"UPDATER_PUBLIC_KEY_BASE64", "UPDATER_PRIVATE_KEY_BASE64", "WINDOWS_CERTIFICATE_BASE64", "WINDOWS_CERTIFICATE_PASSWORD"} {
			if os.Getenv(name) == "" {
				panic("Configura " + name + " en GitHub Settings > Secrets and variables > Actions. Consulta docs/releases.md")
			}
		}
		public, err := base64.StdEncoding.DecodeString(os.Getenv("UPDATER_PUBLIC_KEY_BASE64"))
		must(err)
		block, _ := pem.Decode(public)
		if block == nil {
			panic("clave pública PEM inválida")
		}
		parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
		must(err)
		pub, ok := parsed.(ed25519.PublicKey)
		if !ok {
			panic("se requiere clave Ed25519")
		}
		private, err := base64.StdEncoding.DecodeString(os.Getenv("UPDATER_PRIVATE_KEY_BASE64"))
		must(err)
		block, _ = pem.Decode(private)
		if block == nil {
			panic("clave privada PEM inválida")
		}
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		must(err)
		priv, ok := key.(ed25519.PrivateKey)
		if !ok || !priv.Public().(ed25519.PublicKey).Equal(pub) {
			panic("las claves del actualizador no coinciden")
		}
	}
	if output := os.Getenv("GITHUB_OUTPUT"); output != "" {
		f, err := os.OpenFile(output, os.O_APPEND|os.O_WRONLY, 0600)
		must(err)
		_, err = fmt.Fprintf(f, "version=%s\n", version)
		must(err)
		must(f.Close())
	}
}
func build() {
	syncMetadata()
	public := os.Getenv("UPDATER_PUBLIC_KEY_BASE64")
	if public != "" {
		_, err := base64.StdEncoding.DecodeString(public)
		must(err)
	}
	flags := "-X main.version=" + version + " -X main.updatePublicKeyBase64=" + public
	run("wails3", "task", "build", "EXTRA_LDFLAGS="+flags)
	must(os.MkdirAll("artifacts/updates", 0755))
	must(os.MkdirAll("artifacts/packages", 0755))
	prefix := "tools-switcher-" + version + "-" + runtime.GOOS + "-" + runtime.GOARCH
	switch runtime.GOOS {
	case "windows":
		if isProduction() {
			signWindows("bin/tools-switcher.exe")
		}
		copyFile("bin/tools-switcher.exe", "artifacts/updates/"+prefix+".exe")
		if isProduction() {
			run("wails3", "generate", "webview2bootstrapper", "-dir", "build/windows/nsis")
			executable, err := filepath.Abs("bin/tools-switcher.exe")
			must(err)
			cmd := exec.Command("makensis", "-DWAILS_INSTALL_SCOPE=user", "-DREQUEST_EXECUTION_LEVEL=user", "-DARG_WAILS_AMD64_BINARY="+executable, "project.nsi")
			cmd.Dir = "build/windows/nsis"
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			must(cmd.Run())
			installer := "bin/tools-switcher-amd64-installer.exe"
			signWindows(installer)
			copyFile(installer, "artifacts/packages/"+prefix+"-setup.exe")
		}
	case "darwin":
		run("wails3", "task", "darwin:create:app:bundle")
		run("codesign", "--verify", "--deep", "--strict", "bin/tools-switcher.app")
		run("ditto", "-c", "-k", "--sequesterRsrc", "--keepParent", "bin/tools-switcher.app", "artifacts/updates/"+prefix+".zip")
		if isProduction() {
			run("hdiutil", "create", "-volname", "tools-switcher", "-srcfolder", "bin/tools-switcher.app", "-ov", "-format", "UDZO", "artifacts/packages/"+prefix+".dmg")
		}
	case "linux":
		run("tar", "-czf", "artifacts/updates/"+prefix+".tar.gz", "-C", "bin", "tools-switcher")
		if isProduction() {
			run("wails3", "task", "linux:create:appimage", "EXTRA_LDFLAGS="+flags)
			run("wails3", "task", "linux:create:deb", "EXTRA_LDFLAGS="+flags)
			run("wails3", "task", "linux:create:rpm", "EXTRA_LDFLAGS="+flags)
			for _, pattern := range []string{"bin/*.AppImage", "bin/*.deb", "bin/*.rpm"} {
				files, err := filepath.Glob(pattern)
				must(err)
				if len(files) == 0 {
					panic("falta paquete: " + pattern)
				}
				for _, file := range files {
					copyFile(file, filepath.Join("artifacts/packages", filepath.Base(file)))
				}
			}
		}
	}
}
func signWindows(file string) {
	encoded := os.Getenv("WINDOWS_CERTIFICATE_BASE64")
	if encoded == "" {
		panic("falta WINDOWS_CERTIFICATE_BASE64")
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	must(err)
	temp, err := os.CreateTemp("", "tools-switcher-*.pfx")
	must(err)
	name := temp.Name()
	defer os.Remove(name)
	_, err = temp.Write(data)
	must(err)
	must(temp.Close())
	must(os.Setenv("WAILS_WINDOWS_CERT_PASSWORD", os.Getenv("WINDOWS_CERTIFICATE_PASSWORD")))
	run("wails3", "tool", "sign", "--input", file, "--certificate", name, "--timestamp", "http://timestamp.digicert.com")
}
func release() {
	key, err := base64.StdEncoding.DecodeString(os.Getenv("UPDATER_PRIVATE_KEY_BASE64"))
	must(err)
	if len(key) == 0 {
		panic("falta clave privada de actualizaciones")
	}
	temp, err := os.CreateTemp("", "tools-switcher-updater-*.key")
	must(err)
	name := temp.Name()
	defer os.Remove(name)
	_, err = temp.Write(key)
	must(err)
	must(temp.Close())
	run("wails3", "updater", "manifest", "-version", version, "-channel", "stable", "-key", name, "-name", "tools-switcher "+version, "-url-prefix", "https://github.com/jecortes2304/tools-switcher/releases/download/v"+version, "-output", "artifacts/manifest.json", "artifacts/updates")
	public, err := base64.StdEncoding.DecodeString(os.Getenv("UPDATER_PUBLIC_KEY_BASE64"))
	must(err)
	pub, err := os.CreateTemp("", "tools-switcher-*.pub")
	must(err)
	pubname := pub.Name()
	defer os.Remove(pubname)
	_, err = pub.Write(public)
	must(err)
	must(pub.Close())
	run("wails3", "updater", "verify", "-manifest", "artifacts/manifest.json", "-publickey", pubname, "-dir", "artifacts/updates")
	for _, name := range []string{"tools-switcher-" + version + "-windows-amd64.exe", "tools-switcher-" + version + "-darwin-arm64.zip", "tools-switcher-" + version + "-linux-amd64.tar.gz"} {
		info, err := os.Stat(filepath.Join("artifacts/updates", name))
		must(err)
		if info.Size() == 0 {
			panic("artefacto vacío: " + name)
		}
	}
	for _, pattern := range []string{"artifacts/packages/*windows-amd64-setup.exe", "artifacts/packages/*darwin-arm64.dmg", "artifacts/packages/*.AppImage", "artifacts/packages/*.deb", "artifacts/packages/*.rpm"} {
		matches, err := filepath.Glob(pattern)
		must(err)
		if len(matches) != 1 {
			panic("debe existir exactamente un paquete para " + pattern)
		}
	}
	files := []string{}
	for _, pattern := range []string{"artifacts/updates/*", "artifacts/packages/*"} {
		matches, err := filepath.Glob(pattern)
		must(err)
		files = append(files, matches...)
	}
	if len(files) < 8 {
		panic("faltan artefactos de alguna plataforma")
	}
	var sums strings.Builder
	for _, file := range files {
		f, err := os.Open(file)
		must(err)
		hash := sha256.New()
		_, err = io.Copy(hash, f)
		_ = f.Close()
		must(err)
		fmt.Fprintf(&sums, "%s  %s\n", hex.EncodeToString(hash.Sum(nil)), filepath.Base(file))
	}
	must(os.WriteFile("artifacts/SHA256SUMS", []byte(sums.String()), 0644))
	if len(files) == 0 {
		panic(errors.New("no hay artefactos"))
	}
}

func syncMetadata() {
	for _, file := range []string{"build/config.yml", "build/linux/nfpm/nfpm.yaml"} {
		data, err := os.ReadFile(file)
		must(err)
		changed := regexp.MustCompile(`(?m)^(\s*version:\s*)"[^"]+"`).ReplaceAllString(string(data), `${1}"`+version+`"`)
		must(os.WriteFile(file, []byte(changed), 0644))
	}
	data, err := os.ReadFile("build/windows/info.json")
	must(err)
	changed := regexp.MustCompile(`("(?:file_version|ProductVersion)"\s*:\s*)"[^"]+"`).ReplaceAllString(string(data), `${1}"`+version+`"`)
	must(os.WriteFile("build/windows/info.json", []byte(changed), 0644))
	data, err = os.ReadFile("build/windows/nsis/wails_tools.nsh")
	must(err)
	changed = regexp.MustCompile(`(!define INFO_PRODUCTVERSION )"[^"]+"`).ReplaceAllString(string(data), `${1}"`+version+`"`)
	must(os.WriteFile("build/windows/nsis/wails_tools.nsh", []byte(changed), 0644))
	for _, file := range []string{"build/darwin/Info.plist", "build/darwin/Info.dev.plist"} {
		data, err = os.ReadFile(file)
		must(err)
		changed = regexp.MustCompile(`(?s)(<key>CFBundle(?:ShortVersionString|Version)</key>\s*<string>)[^<]+(</string>)`).ReplaceAllString(string(data), `${1}`+version+`${2}`)
		must(os.WriteFile(file, []byte(changed), 0644))
	}
}
