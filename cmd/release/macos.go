package main

import (
	"encoding/binary"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Viking602/YoloMCL/internal/release"
)

func macBundle(root, work, executable, tag, commit string) ([]release.File, error) {
	bundle := filepath.Join(work, "YoloMCL.app")
	contents := filepath.Join(bundle, "Contents")
	for _, dir := range []string{"MacOS", "Resources"} {
		if err := os.MkdirAll(filepath.Join(contents, dir), 0o755); err != nil {
			return nil, err
		}
	}
	if err := os.Rename(executable, filepath.Join(contents, "MacOS", "YoloMCL")); err != nil {
		return nil, err
	}
	version := strings.SplitN(strings.TrimPrefix(tag, "v"), "-", 2)[0]
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleName</key><string>YoloMCL</string>
<key>CFBundleDisplayName</key><string>YoloMCL</string>
<key>CFBundleIdentifier</key><string>io.github.viking602.yolomcl</string>
<key>CFBundleExecutable</key><string>YoloMCL</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>CFBundleShortVersionString</key><string>%s</string>
<key>CFBundleVersion</key><string>%s</string>
<key>CFBundleIconFile</key><string>app.icns</string>
<key>NSHighResolutionCapable</key><true/>
<key>LSMinimumSystemVersion</key><string>12.0</string>
<key>YoloMCLCommit</key><string>%s</string>
</dict></plist>
`, version, version, commit)
	if err := os.WriteFile(filepath.Join(contents, "Info.plist"), []byte(plist), 0o644); err != nil {
		return nil, err
	}
	png, err := os.ReadFile(filepath.Join(root, "internal", "nativeui", "assets", "brand", "brand-256.png"))
	if err != nil {
		return nil, err
	}
	icns := append([]byte("icns"), binary.BigEndian.AppendUint32(nil, uint32(len(png)+16))...)
	icns = append(icns, []byte("ic08")...)
	icns = binary.BigEndian.AppendUint32(icns, uint32(len(png)+8))
	icns = append(icns, png...)
	if err := os.WriteFile(filepath.Join(contents, "Resources", "app.icns"), icns, 0o644); err != nil {
		return nil, err
	}
	for _, name := range []string{"LICENSE", "THIRD_PARTY_NOTICES.md"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(contents, "Resources", name), data, 0o644); err != nil {
			return nil, err
		}
	}
	cmd := exec.Command("codesign", "--force", "--sign", "-", "--timestamp=none", bundle)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ad-hoc sign app bundle: %w", err)
	}
	if err := exec.Command("codesign", "--verify", "--deep", "--strict", bundle).Run(); err != nil {
		return nil, fmt.Errorf("verify app bundle signature: %w", err)
	}
	var files []release.File
	// Walk only the freshly generated bundle, never game or user directories.
	err = filepath.WalkDir(bundle, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		relative, err := filepath.Rel(work, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		files = append(files, release.File{Name: filepath.ToSlash(relative), Path: path, Mode: int64(info.Mode().Perm())})
		return nil
	})
	return files, err
}
