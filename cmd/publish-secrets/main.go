// Command publish-secrets encrypts selected publisher settings into GitHub
// Actions Secrets. Values are passed over stdin, never shell arguments or logs.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Viking602/YoloMCL/internal/buildconfig"
	"github.com/Viking602/YoloMCL/internal/buildinfo"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "publish-secrets:", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 1 {
		return errors.New("run without arguments from the repository root")
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	config, err := buildconfig.Load(root, "", os.Getenv)
	if err != nil {
		return err
	}
	if !config.HasMicrosoft() || config.CurseForgeAPIKey == "" {
		return errors.New("MS_CLIENT_ID and CURSEFORGE_API_KEY are required")
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	keyPath := filepath.Join(configDir, "YoloMCL", "publisher", "update-private-key.txt")
	publicPath := filepath.Join(root, "internal", "buildinfo", "update-public-key.txt")
	publicData, err := os.ReadFile(publicPath)
	if err != nil {
		return err
	}
	key, err := os.ReadFile(keyPath)
	if errors.Is(err, os.ErrNotExist) {
		if strings.TrimSpace(string(publicData)) != "" {
			return errors.New("restore the existing private key backup; refusing to rotate the published update key")
		}
		_, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return err
		}
		key = []byte(base64.StdEncoding.EncodeToString(private))
		if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
			return err
		}
		f, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return err
		}
		_, writeErr := f.Write(key)
		if err := errors.Join(writeErr, f.Close()); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	private, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(key)))
	if err != nil || len(private) != ed25519.PrivateKeySize {
		return errors.New("invalid private key backup")
	}
	public := base64.StdEncoding.EncodeToString(ed25519.PrivateKey(private).Public().(ed25519.PublicKey))
	if existing := strings.TrimSpace(string(publicData)); existing != "" && existing != public {
		return errors.New("private key does not match the committed update public key")
	}
	if err := os.WriteFile(publicPath, []byte(public+"\n"), 0o644); err != nil {
		return err
	}
	for _, item := range []struct{ name, value string }{
		{"CURSEFORGE_API_KEY", config.CurseForgeAPIKey},
		{"MS_CLIENT_ID", config.MicrosoftClientID},
		{"MYGO_UPDATER_PRIVATE_KEY", strings.TrimSpace(string(key))},
	} {
		cmd := exec.Command("gh", "secret", "set", item.name, "--repo", buildinfo.Repository)
		cmd.Stdin = strings.NewReader(item.value)
		// Do not echo subprocess output: failure diagnostics must not include inputs.
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("could not set %s; check gh authentication and repository permissions", item.name)
		}
		fmt.Println("Configured Actions Secret:", item.name)
	}
	fmt.Println("Private signing-key backup:", keyPath)
	return nil
}
