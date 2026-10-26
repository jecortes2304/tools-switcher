package manager

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"runtime"
	"strconv"
	"strings"
)

func (m *Manager) loadCorrettoCatalog(ctx context.Context, r Repository, major int) ([]Package, error) {
	operatingSystem, arch := runtime.GOOS, runtime.GOARCH
	if operatingSystem == "darwin" {
		operatingSystem = "macos"
	}
	if arch == "amd64" {
		arch = "x64"
	} else if arch == "arm64" {
		arch = "aarch64"
	}
	extension := "tar.gz"
	if operatingSystem == "windows" {
		extension = "zip"
	}
	filename := fmt.Sprintf("amazon-corretto-%d-%s-%s-jdk.%s", major, arch, operatingSystem, extension)
	latest := m.correttoBase + "/latest/" + filename
	fetchTarget := func() (string, int64, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, latest, nil)
		if err != nil {
			return "", 0, err
		}
		req.Header.Set("Range", "bytes=0-0")
		res, err := m.client.Do(req)
		if err != nil {
			return "", 0, err
		}
		defer res.Body.Close()
		if res.StatusCode == 404 {
			return "", 0, nil
		}
		if res.StatusCode != 200 && res.StatusCode != 206 {
			return "", 0, fmt.Errorf("Amazon Corretto: HTTP %d para Java %d/%s/%s", res.StatusCode, major, operatingSystem, arch)
		}
		if strings.Contains(res.Header.Get("Content-Type"), "text/html") {
			return "", 0, errors.New("Amazon devolvió una página web en lugar del paquete")
		}
		var byteBuffer [1]byte
		if _, err = io.ReadFull(res.Body, byteBuffer[:]); err != nil {
			return "", 0, err
		}
		size := res.ContentLength
		if res.StatusCode == 206 {
			if _, total, ok := strings.Cut(res.Header.Get("Content-Range"), "/"); ok {
				size, _ = strconv.ParseInt(total, 10, 64)
			}
		}
		return res.Request.URL.String(), size, nil
	}
	target, size, err := fetchTarget()
	if err != nil {
		return nil, err
	}
	if target == "" {
		return []Package{}, nil
	}
	digest, err := m.readChecksum(ctx, m.correttoBase+"/latest_sha256/"+filename, "sha256")
	if err != nil {
		return nil, err
	}
	confirmation, _, err := fetchTarget()
	if err != nil {
		return nil, err
	}
	if confirmation != target {
		return nil, errors.New("la versión de Corretto cambió durante la consulta; vuelve a sincronizar")
	}
	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	version := strconv.Itoa(major)
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i, part := range parts {
		if part == "resources" && i+1 < len(parts) {
			version = parts[i+1]
			break
		}
	}
	p := Package{ID: packageID(r.ID, target), RepositoryID: r.ID, Name: r.Name + " " + version, Version: version, Vendor: "Amazon", OS: runtime.GOOS, Arch: runtime.GOARCH, URL: target, Filename: path.Base(u.Path), Size: size, SHA256: digest}
	if err = validatePackage(p); err != nil {
		return nil, err
	}
	return []Package{p}, nil
}
