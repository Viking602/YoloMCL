// Package release creates the signed archives consumed by MyGo's public updater.
// Only explicit launcher files are included; configuration and credentials are
// never discovered by walking the repository.
package release

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Viking602/YoloMCL/internal/buildinfo"
)

// Manifest is compatible with MyGo's signed update manifest format.
type Manifest struct {
	Version   string `json:"version"`
	Notes     string `json:"notes"`
	Date      string `json:"date"`
	URL       string `json:"url"`
	Size      int64  `json:"size"`
	Signature string `json:"signature"`
	Commit    string `json:"commit"`
}

// File is one explicitly selected regular file in the release archive.
type File struct {
	Name string
	Path string
	Mode int64
}

// Package signs a tar.gz and emits its manifest, SHA-256 checksums and (on
// Windows) a ZIP for Explorer. Paths in files must be portable relative names.
func Package(output, tag, commit, target string, files []File, private ed25519.PrivateKey, date time.Time) error {
	if !buildinfo.ValidTag(tag) || !ValidTarget(target) || len(private) != ed25519.PrivateKeySize {
		return errors.New("invalid release identity, target or signing key")
	}
	seen := map[string]bool{}
	for _, file := range files {
		if file.Name == "" || strings.ContainsAny(file.Name, "\\:") || strings.HasPrefix(file.Name, "/") ||
			strings.Contains("/"+file.Name+"/", "/../") || strings.Contains("/"+file.Name+"/", "/./") || seen[file.Name] {
			return errors.New("unsafe or duplicate archive entry")
		}
		seen[file.Name] = true
	}
	if len(files) == 0 {
		return errors.New("empty release archive")
	}
	if err := os.MkdirAll(output, 0o755); err != nil {
		return err
	}
	base := "YoloMCL-" + strings.TrimPrefix(tag, "v") + "-" + target
	archive := base + ".tar.gz"
	if err := writeTar(filepath.Join(output, archive), files, date); err != nil {
		return err
	}
	digest, size, err := checksum(filepath.Join(output, archive))
	if err != nil {
		return err
	}
	manifest := Manifest{
		Version: strings.TrimPrefix(tag, "v"), Commit: commit,
		Notes: "Release notes: " + buildinfo.ReleasesURL + "/tag/" + tag,
		Date:  date.UTC().Format(time.RFC3339),
		URL:   buildinfo.ReleasesURL + "/download/" + tag + "/" + archive,
		Size:  size, Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(private, digest)),
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	manifestName := "update-" + target + ".json"
	if err := os.WriteFile(filepath.Join(output, manifestName), append(data, '\n'), 0o644); err != nil {
		return err
	}
	names := []string{archive, manifestName}
	if strings.HasPrefix(target, "windows-") {
		name := base + ".zip"
		if err := writeZip(filepath.Join(output, name), files, date); err != nil {
			return err
		}
		names = append(names, name)
	}
	sort.Strings(names)
	var sums strings.Builder
	for _, name := range names {
		digest, _, err := checksum(filepath.Join(output, name))
		if err != nil {
			return err
		}
		fmt.Fprintf(&sums, "%x  %s\n", digest, name)
	}
	return os.WriteFile(filepath.Join(output, "checksums-"+target+".txt"), []byte(sums.String()), 0o644)
}

// ValidTarget reports whether a platform has an official release archive.
func ValidTarget(target string) bool {
	for _, os := range []string{"windows", "darwin", "linux"} {
		if target == os+"-amd64" || target == os+"-arm64" {
			return true
		}
	}
	return false
}

func checksum(path string) ([]byte, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	return h.Sum(nil), n, err
}

func writeTar(path string, files []File, date time.Time) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	gz := gzip.NewWriter(f)
	defer func() { err = errors.Join(err, gz.Close()) }()
	tw := tar.NewWriter(gz)
	defer func() { err = errors.Join(err, tw.Close()) }()
	for _, file := range files {
		data, err := os.ReadFile(file.Path)
		if err != nil {
			return err
		}
		if err := tw.WriteHeader(&tar.Header{Name: file.Name, Mode: file.Mode, Size: int64(len(data)), ModTime: date, Typeflag: tar.TypeReg}); err != nil {
			return err
		}
		if _, err := tw.Write(data); err != nil {
			return err
		}
	}
	return nil
}

func writeZip(path string, files []File, date time.Time) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	zw := zip.NewWriter(f)
	defer func() { err = errors.Join(err, zw.Close()) }()
	for _, file := range files {
		header := &zip.FileHeader{Name: "YoloMCL/" + file.Name, Method: zip.Deflate, Modified: date}
		header.SetMode(os.FileMode(file.Mode))
		w, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(file.Path)
		if err != nil {
			return err
		}
		if _, err := w.Write(data); err != nil {
			return err
		}
	}
	return nil
}

// Verify checks every target before a draft release can be published.
func Verify(dir, tag, commit string, public ed25519.PublicKey) error {
	if !buildinfo.ValidTag(tag) || len(public) != ed25519.PublicKeySize {
		return errors.New("invalid release identity or public key")
	}
	for _, goos := range []string{"windows", "darwin", "linux"} {
		for _, arch := range []string{"amd64", "arm64"} {
			target := goos + "-" + arch
			if err := verifyTarget(dir, tag, commit, target, public); err != nil {
				return fmt.Errorf("%s: %w", target, err)
			}
		}
	}
	return nil
}

func verifyTarget(dir, tag, commit, target string, public ed25519.PublicKey) error {
	data, err := os.ReadFile(filepath.Join(dir, "update-"+target+".json"))
	if err != nil {
		return err
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return err
	}
	base := "YoloMCL-" + strings.TrimPrefix(tag, "v") + "-" + target
	if manifest.Version != strings.TrimPrefix(tag, "v") || manifest.Commit != commit ||
		manifest.URL != buildinfo.ReleasesURL+"/download/"+tag+"/"+base+".tar.gz" {
		return errors.New("manifest identity does not match the release tag")
	}
	digest, size, err := checksum(filepath.Join(dir, base+".tar.gz"))
	if err != nil {
		return err
	}
	signature, err := base64.StdEncoding.DecodeString(manifest.Signature)
	if err != nil || size != manifest.Size || !ed25519.Verify(public, digest, signature) {
		return errors.New("update signature or size verification failed")
	}
	sums, err := os.ReadFile(filepath.Join(dir, "checksums-"+target+".txt"))
	if err != nil {
		return err
	}
	names := []string{base + ".tar.gz", "update-" + target + ".json"}
	if strings.HasPrefix(target, "windows-") {
		names = append(names, base+".zip")
	}
	sort.Strings(names)
	var expected strings.Builder
	for _, name := range names {
		digest, _, err := checksum(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		fmt.Fprintf(&expected, "%x  %s\n", digest, name)
	}
	if string(sums) != expected.String() {
		return errors.New("release checksum verification failed")
	}
	return nil
}
