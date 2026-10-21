package launcher

import (
	"archive/zip"
	"crypto/sha1"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func safeJoin(root, name string) (string, error) {
	if name == "" || strings.ContainsAny(name, ":\x00") || filepath.IsAbs(name) || strings.HasPrefix(name, "\\") || strings.HasPrefix(name, "/") {
		return "", errors.New("不安全的文件路径")
	}
	name = strings.ReplaceAll(name, "\\", "/")
	for _, part := range strings.Split(name, "/") {
		if part == ".." || strings.TrimRight(part, " .") != part {
			return "", errors.New("文件路径不能包含上级目录或尾随空格")
		}
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || (len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '0' && base[3] <= '9') {
			return "", errors.New("文件路径包含保留设备名")
		}
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	target := filepath.Join(root, filepath.FromSlash(name))
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("文件路径越界")
	}
	current := root
	if st, e := os.Lstat(root); e == nil && st.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("拒绝写入符号链接目录")
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		st, e := os.Lstat(current)
		if e != nil && !os.IsNotExist(e) {
			return "", e
		}
		if e == nil && st.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("文件路径包含符号链接")
		}
	}
	return target, nil
}
func fileHashes(path string) (string, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	a, b := sha1.New(), sha512.New()
	if _, err = io.Copy(io.MultiWriter(a, b), f); err != nil {
		return "", "", err
	}
	return hex.EncodeToString(a.Sum(nil)), hex.EncodeToString(b.Sum(nil)), nil
}
func checkHashes(path, sha, sha512value string) bool {
	if sha == "" && sha512value == "" {
		return false
	}
	a, b, e := fileHashes(path)
	return e == nil && (sha == "" || strings.EqualFold(sha, a)) && (sha512value == "" || strings.EqualFold(sha512value, b))
}
func copyFile(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	if err = os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, f)
	closeErr := out.Close()
	if err != nil {
		return err
	}
	return closeErr
}
func readZipJSON(f *zip.File, v any) error {
	r, err := f.Open()
	if err != nil {
		return err
	}
	defer r.Close()
	return decodeJSON(io.LimitReader(r, 16<<20), v)
}
func extractZipEntry(f *zip.File, dest string) error {
	if f.Mode()&os.ModeSymlink != 0 {
		return errors.New("压缩包包含符号链接")
	}
	if f.UncompressedSize64 > 2<<30 {
		return errors.New("压缩包单文件过大")
	}
	if f.FileInfo().IsDir() {
		return os.MkdirAll(dest, 0755)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	r, err := f.Open()
	if err != nil {
		return err
	}
	defer r.Close()
	w, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	n, err := io.Copy(w, io.LimitReader(r, int64(f.UncompressedSize64)+1))
	ce := w.Close()
	if err != nil {
		return err
	}
	if n != int64(f.UncompressedSize64) {
		return fmt.Errorf("解压文件长度不匹配: %s", f.Name)
	}
	return ce
}
