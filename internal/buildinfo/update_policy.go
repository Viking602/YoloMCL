package buildinfo

import (
	"cmp"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// MaxUpdatePolicySize bounds both downloaded manifests and the local policy cache.
const MaxUpdatePolicySize = 64 << 10

// UpdatePolicy is the signed version floor in a platform's release manifest.
// It is independent of whether that installation supports in-place updates.
type UpdatePolicy struct {
	Version        string `json:"version"`
	MinimumVersion string `json:"minimumVersion"`
	Signature      string `json:"policySignature"`
}

// DecodeUpdatePolicy authenticates a policy before it can disable any version.
func DecodeUpdatePolicy(data []byte, target, key string) (*UpdatePolicy, error) {
	if len(data) > MaxUpdatePolicySize {
		return nil, errors.New("update policy exceeds size limit")
	}
	var p UpdatePolicy
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("decode update policy: %w", err)
	}
	if len(p.Version) > 128 || len(p.MinimumVersion) > 128 || !ValidTag("v"+p.Version) ||
		!ValidTag("v"+p.MinimumVersion) || strings.Contains(p.MinimumVersion, "-") ||
		CompareVersions(p.MinimumVersion, p.Version) > 0 {
		return nil, errors.New("invalid update policy versions")
	}
	pub, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return nil, errors.New("invalid update policy public key")
	}
	sig, err := base64.StdEncoding.DecodeString(p.Signature)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return nil, errors.New("invalid update policy signature")
	}
	// Domain separation prevents an archive signature being reused as a policy.
	// Repository and target binding prevents cross-project/platform replay.
	payload := "YoloMCL update policy v1\n" + Repository + "\n" + target + "\n" + p.Version + "\n" + p.MinimumVersion + "\n"
	if !ed25519.Verify(pub, []byte(payload), sig) {
		return nil, errors.New("update policy signature verification failed")
	}
	return &p, nil
}

// RequiresUpdate reports whether a tagged build is below the supported floor.
// Development builds have no release identity and are not remotely disabled.
func (p *UpdatePolicy) RequiresUpdate(version string) bool {
	return p != nil && ValidTag("v"+strings.TrimPrefix(version, "v")) && CompareVersions(version, p.MinimumVersion) < 0
}

// ReadUpdatePolicy re-verifies the small cached policy on every launcher start.
func ReadUpdatePolicy(path, target, key string) (*UpdatePolicy, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxUpdatePolicySize+1))
	if err != nil {
		return nil, err
	}
	return DecodeUpdatePolicy(data, target, key)
}

// SaveUpdatePolicy atomically replaces a single already-verified policy. The
// cache belongs beside launcher settings, not in the clearable mod cache.
func SaveUpdatePolicy(path string, p *UpdatePolicy) error {
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".update-policy-*.tmp")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }() // absent after a successful rename
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

// CompareVersions compares validated semantic versions, with optional v prefixes.
// Numeric components are compared without integer conversion or overflow.
func CompareVersions(a, b string) int {
	a, b = strings.TrimPrefix(a, "v"), strings.TrimPrefix(b, "v")
	aCore, aPre, _ := strings.Cut(a, "-")
	bCore, bPre, _ := strings.Cut(b, "-")
	aParts, bParts := strings.Split(aCore, "."), strings.Split(bCore, ".")
	for i := range min(len(aParts), len(bParts)) {
		if order := compareVersionNumber(aParts[i], bParts[i]); order != 0 {
			return order
		}
	}
	if aPre == bPre {
		return 0
	}
	if aPre == "" {
		return 1
	}
	if bPre == "" {
		return -1
	}
	aParts, bParts = strings.Split(aPre, "."), strings.Split(bPre, ".")
	for i := range min(len(aParts), len(bParts)) {
		x, y := aParts[i], bParts[i]
		xNum, yNum := strings.Trim(x, "0123456789") == "", strings.Trim(y, "0123456789") == ""
		var order int
		switch {
		case xNum && yNum:
			order = compareVersionNumber(x, y)
		case xNum:
			order = -1
		case yNum:
			order = 1
		default:
			order = strings.Compare(x, y)
		}
		if order != 0 {
			return order
		}
	}
	return cmp.Compare(len(aParts), len(bParts))
}

func compareVersionNumber(a, b string) int {
	if order := cmp.Compare(len(a), len(b)); order != 0 {
		return order
	}
	return strings.Compare(a, b)
}
