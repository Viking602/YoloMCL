// Command release builds and verifies tag-based, signed GitHub release assets.
package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Viking602/YoloMCL/internal/buildconfig"
	"github.com/Viking602/YoloMCL/internal/buildinfo"
	"github.com/Viking602/YoloMCL/internal/release"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "release:", err)
		os.Exit(1)
	}
}

func run() error {
	tag := flag.String("tag", "", "Existing v-prefixed SemVer tag at HEAD")
	goos := flag.String("goos", runtime.GOOS, "Target operating system")
	goarch := flag.String("goarch", runtime.GOARCH, "Target architecture")
	output := flag.String("output", "build/release", "Release assets directory")
	verify := flag.Bool("verify", false, "Verify all six downloaded target artifacts instead of building")
	flag.Parse()
	if flag.NArg() != 0 || !buildinfo.ValidTag(*tag) || !release.ValidTarget(*goos+"-"+*goarch) {
		return errors.New("a valid release tag and target are required")
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	metadata, err := buildconfig.Metadata(root)
	if err != nil {
		return err
	}
	resolved, err := exec.Command("git", "rev-parse", *tag+"^{commit}").Output()
	if err != nil || strings.TrimSpace(string(resolved)) != metadata.Commit || metadata.Dirty {
		return errors.New("release requires a clean checkout at the selected existing tag")
	}
	metadata.Version = *tag
	public, err := base64.StdEncoding.DecodeString(buildinfo.PublicKey())
	if err != nil || len(public) != ed25519.PublicKeySize {
		return errors.New("configure the update public key before tagging the release")
	}
	if *verify {
		if err := release.Verify(*output, *tag, metadata.Commit, public); err != nil {
			return err
		}
		fmt.Println("All six release archives, signatures and checksums verified.")
		return nil
	}
	private, err := base64.StdEncoding.DecodeString(strings.TrimSpace(os.Getenv("MYGO_UPDATER_PRIVATE_KEY")))
	if err != nil || len(private) != ed25519.PrivateKeySize {
		return errors.New("MYGO_UPDATER_PRIVATE_KEY must contain an Ed25519 private key")
	}
	if !ed25519.PrivateKey(private).Public().(ed25519.PublicKey).Equal(ed25519.PublicKey(public)) {
		return errors.New("signing key does not match this launcher's public key")
	}
	if *goos == "darwin" && runtime.GOOS != "darwin" {
		return errors.New("macOS release bundles must be built on macOS for ad-hoc code signing")
	}
	config, err := buildconfig.Load(root, "", os.Getenv)
	if err != nil {
		return err
	}
	work, err := os.MkdirTemp("", "yolomcl-release-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work) // This directory was created above, never supplied by a caller.
	exe := "YoloMCL"
	if *goos == "windows" {
		exe += ".exe"
	}
	binary := filepath.Join(work, exe)
	args, err := buildconfig.Arguments(config, *goos, os.Getenv("GOFLAGS"), binary, true)
	if err != nil {
		return err
	}
	args, err = buildconfig.MetadataArguments(args, metadata, *goos, *goarch, true)
	if err != nil {
		return err
	}
	cmd := exec.Command("go", args...)
	cmd.Env = append(os.Environ(), "GOOS="+*goos, "GOARCH="+*goarch, "CGO_ENABLED=0")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("compile release: %w", err)
	}
	files := []release.File{{Name: exe, Path: binary, Mode: 0o755}}
	for _, name := range []string{"LICENSE", "THIRD_PARTY_NOTICES.md"} {
		files = append(files, release.File{Name: name, Path: filepath.Join(root, name), Mode: 0o644})
	}
	if *goos == "darwin" {
		files, err = macBundle(root, work, binary, *tag, metadata.Commit)
		if err != nil {
			return err
		}
	}
	dateOutput, err := exec.Command("git", "show", "-s", "--format=%cI", "HEAD").Output()
	if err != nil {
		return errors.New("cannot read release commit date")
	}
	date, err := time.Parse(time.RFC3339, strings.TrimSpace(string(dateOutput)))
	if err != nil {
		return err
	}
	target := *goos + "-" + *goarch
	if err := release.Package(*output, *tag, metadata.Commit, target, files, private, date); err != nil {
		return err
	}
	fmt.Printf("Built and signed %s %s (%s)\n", *tag, target, metadata.ShortCommit())
	return nil
}
