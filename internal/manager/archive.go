package manager

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

const maxArchiveBytes int64 = 8 << 30

func releaseValues(data []byte) map[string]string {
	v := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if ok {
			v[key] = strings.Trim(value, "\"")
		}
	}
	return v
}
func javaNames() (string, string) {
	if runtime.GOOS == "windows" {
		return "java.exe", "javac.exe"
	}
	return "java", "javac"
}
func inspectHome(home string) (Installation, error) {
	java, javac := javaNames()
	for _, name := range []string{java, javac} {
		info, err := os.Stat(filepath.Join(home, "bin", name))
		if err != nil || !info.Mode().IsRegular() {
			return Installation{}, errors.New("la carpeta no contiene bin/java y bin/javac")
		}
	}
	release, err := os.Open(filepath.Join(home, "release"))
	if err != nil {
		return Installation{}, err
	}
	defer release.Close()
	data, err := io.ReadAll(io.LimitReader(release, (1<<20)+1))
	if err != nil {
		return Installation{}, err
	}
	if len(data) > 1<<20 {
		return Installation{}, errors.New("metadatos demasiado grandes")
	}
	v := releaseValues(data)
	if v["JAVA_VERSION"] == "" {
		return Installation{}, errors.New("falta JAVA_VERSION en release")
	}
	vendor := v["IMPLEMENTOR"]
	if vendor == "" {
		vendor = "OpenJDK"
	}
	return Installation{Tool: "jdk", Name: vendor + " " + v["JAVA_VERSION"], Version: v["JAVA_VERSION"], Vendor: vendor, Path: home}, nil
}

func findHomes(root string) ([]Installation, error) {
	result := []Installation{}
	count := 0
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		count++
		if count > 100000 {
			return errors.New("demasiados archivos en la carpeta")
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		if len(strings.Split(rel, string(filepath.Separator))) > 6 {
			return filepath.SkipDir
		}
		if home, err := inspectHome(p); err == nil {
			result = append(result, home)
			return filepath.SkipDir
		}
		return nil
	})
	return result, err
}

func scanLocal(r Repository) ([]Package, error) {
	homes, err := findHomes(r.Source)
	if err != nil {
		return nil, err
	}
	result := []Package{}
	for _, h := range homes {
		if err := validateHomePlatform(h.Path, Package{OS: runtime.GOOS, Arch: runtime.GOARCH}); err != nil {
			continue
		}
		result = append(result, Package{ID: packageID(r.ID, h.Path), RepositoryID: r.ID, Name: h.Name, Version: h.Version, Vendor: h.Vendor, OS: runtime.GOOS, Arch: runtime.GOARCH, LocalPath: h.Path})
	}
	entries, err := os.ReadDir(r.Source)
	if err != nil {
		return nil, err
	}
	archives := 0
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".zip") && !strings.HasSuffix(name, ".tar.gz") {
			continue
		}
		archives++
		if archives > 100 {
			return nil, errors.New("la carpeta admite hasta 100 archivos de JDK")
		}
		file := filepath.Join(r.Source, name)
		v, err := archiveMetadata(file)
		if err != nil {
			continue
		}
		arch := v["OS_ARCH"]
		if arch == "x86_64" || arch == "x64" {
			arch = "amd64"
		}
		if arch == "aarch64" {
			arch = "arm64"
		}
		osName := strings.ToLower(v["OS_NAME"])
		if strings.Contains(osName, "windows") {
			osName = "windows"
		}
		if osName == "darwin" || osName == "mac os x" {
			osName = "darwin"
		}
		p := Package{ID: packageID(r.ID, file), RepositoryID: r.ID, Name: v["IMPLEMENTOR"] + " " + v["JAVA_VERSION"], Version: v["JAVA_VERSION"], Vendor: v["IMPLEMENTOR"], OS: osName, Arch: arch, Filename: name, LocalPath: file}
		if info, e := entry.Info(); e == nil {
			p.Size = info.Size()
		}
		if compatible(p) {
			result = append(result, p)
		}
	}
	return result, nil
}

func archiveMetadata(file string) (map[string]string, error) {
	var data []byte
	hasJava, hasJavac := false, false
	java, javac := javaNames()
	visit := func(name string, size int64, reader io.Reader) error {
		name = strings.ReplaceAll(name, "\\", "/")
		if path.Base(name) == "release" && size < 1<<20 {
			var err error
			data, err = io.ReadAll(io.LimitReader(reader, 1<<20))
			return err
		}
		if strings.HasSuffix(name, "/bin/"+java) || name == "bin/"+java {
			hasJava = true
		}
		if strings.HasSuffix(name, "/bin/"+javac) || name == "bin/"+javac {
			hasJavac = true
		}
		return nil
	}
	if strings.HasSuffix(file, ".zip") {
		z, err := zip.OpenReader(file)
		if err != nil {
			return nil, err
		}
		defer z.Close()
		if len(z.File) > 100000 {
			return nil, errors.New("demasiados archivos")
		}
		for _, f := range z.File {
			if f.FileInfo().IsDir() {
				continue
			}
			if path.Base(f.Name) == "release" {
				reader, err := f.Open()
				if err != nil {
					return nil, err
				}
				err = visit(f.Name, int64(f.UncompressedSize64), reader)
				reader.Close()
				if err != nil {
					return nil, err
				}
			} else {
				_ = visit(f.Name, 0, strings.NewReader(""))
			}
		}
	} else {
		f, err := os.Open(file)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		g, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer g.Close()
		t := tar.NewReader(io.LimitReader(g, maxArchiveBytes+1))
		count := 0
		for {
			h, err := t.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			count++
			if count > 100000 {
				return nil, errors.New("demasiados archivos")
			}
			if err = visit(h.Name, h.Size, t); err != nil {
				return nil, err
			}
		}
	}
	v := releaseValues(data)
	if !hasJava || !hasJavac || v["JAVA_VERSION"] == "" {
		return nil, errors.New("el archivo no contiene una JDK")
	}
	return v, nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(b []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(b)
}
func inside(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
func archiveTarget(root, name string) (string, error) {
	if strings.ContainsAny(name, "\\:") || strings.HasPrefix(name, "/") {
		return "", errors.New("ruta no segura en archivo")
	}
	target := filepath.Join(root, filepath.FromSlash(name))
	if !inside(root, target) {
		return "", errors.New("el archivo intenta salir del directorio de destino")
	}
	return target, nil
}

func extract(ctx context.Context, file, dest string) error {
	var total int64
	count := 0
	write := func(name string, size int64, mode os.FileMode, reader io.Reader) error {
		count++
		total += size
		if count > 100000 || size < 0 || total > maxArchiveBytes {
			return errors.New("el archivo supera los límites de extracción")
		}
		target, err := archiveTarget(dest, name)
		if err != nil {
			return err
		}
		if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return err
		}
		f, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode.Perm()|0600)
		if err != nil {
			return err
		}
		written, copyErr := io.Copy(f, io.LimitReader(contextReader{ctx, reader}, size+1))
		if copyErr == nil && written != size {
			copyErr = errors.New("tamaño de archivo extraído incorrecto")
		}
		closeErr := f.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	}
	if strings.HasSuffix(file, ".zip") {
		z, err := zip.OpenReader(file)
		if err != nil {
			return err
		}
		defer z.Close()
		for _, f := range z.File {
			target, err := archiveTarget(dest, f.Name)
			if err != nil {
				return err
			}
			if f.FileInfo().IsDir() {
				if err = os.MkdirAll(target, 0700); err != nil {
					return err
				}
				continue
			}
			if f.Mode()&os.ModeSymlink != 0 {
				return errors.New("enlace simbólico ZIP no admitido")
			}
			if f.UncompressedSize64 > uint64(maxArchiveBytes) {
				return errors.New("archivo demasiado grande")
			}
			reader, err := f.Open()
			if err != nil {
				return err
			}
			err = write(f.Name, int64(f.UncompressedSize64), f.Mode(), reader)
			reader.Close()
			if err != nil {
				return err
			}
		}
		return nil
	}
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	g, err := gzip.NewReader(contextReader{ctx, f})
	if err != nil {
		return err
	}
	defer g.Close()
	t := tar.NewReader(g)
	type link struct {
		name, target string
		hard         bool
	}
	links := []link{}
	for {
		h, err := t.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		target, err := archiveTarget(dest, h.Name)
		if err != nil {
			return err
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err = os.MkdirAll(target, 0700); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err = write(h.Name, h.Size, os.FileMode(h.Mode), t); err != nil {
				return err
			}
		case tar.TypeSymlink, tar.TypeLink:
			if len(links) > 10000 {
				return errors.New("demasiados enlaces")
			}
			links = append(links, link{h.Name, h.Linkname, h.Typeflag == tar.TypeLink})
		default:
			return fmt.Errorf("tipo de archivo TAR no admitido: %d", h.Typeflag)
		}
	}
	for _, l := range links {
		name, err := archiveTarget(dest, l.name)
		if err != nil {
			return err
		}
		var target string
		if l.hard {
			target, err = archiveTarget(dest, l.target)
		} else {
			target, err = archiveTarget(dest, path.Join(path.Dir(l.name), l.target))
			if path.IsAbs(l.target) {
				return errors.New("enlace absoluto no permitido")
			}
		}
		if err != nil {
			return err
		}
		resolved, err := filepath.EvalSymlinks(target)
		if err != nil || !inside(dest, resolved) {
			return errors.New("destino de enlace no seguro")
		}
		parent, err := filepath.EvalSymlinks(filepath.Dir(name))
		if err != nil || !inside(dest, parent) {
			return errors.New("directorio de enlace no seguro")
		}
		if l.hard {
			err = os.Link(resolved, name)
		} else {
			err = os.Symlink(l.target, name)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func validateHomePlatform(home string, p Package) error {
	f, err := os.Open(filepath.Join(home, "release"))
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 1<<20))
	if err != nil {
		return err
	}
	values := releaseValues(data)
	arch := values["OS_ARCH"]
	switch arch {
	case "x86_64", "x64":
		arch = "amd64"
	case "aarch64":
		arch = "arm64"
	}
	operatingSystem := strings.ToLower(values["OS_NAME"])
	if strings.Contains(operatingSystem, "windows") {
		operatingSystem = "windows"
	}
	if operatingSystem == "mac os x" || operatingSystem == "macos" {
		operatingSystem = "darwin"
	}
	if arch != p.Arch || operatingSystem != p.OS {
		return errors.New("el contenido de la JDK no coincide con la plataforma declarada en el catálogo")
	}
	return nil
}
