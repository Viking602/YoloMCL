package launcher

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"runtime"
	"slices"
	"strings"
	"time"
)

const (
	horizonRepo              = "horizon-rust/horizon"
	horizonGitHubReleasesURL = "https://api.github.com/repos/horizon-rust/horizon/releases?per_page=30"
	horizonMirrorReleasesURL = "https://mcp.playhorizon.dev/github/public/repos/horizon-rust/horizon/releases?per_page=30"
	horizonDownloadBaseURL   = "https://github.com/horizon-rust/horizon/releases/download"
	horizonDefaultMinecraft  = "26.3"
	horizonDefaultJavaMajor  = 25
	horizonReleasesCacheTTL  = 5 * time.Minute
	horizonSumsFile          = "SHA256SUMS"
	horizonSigFile           = "SHA256SUMS.sig"
	horizonSigV2File         = "SHA256SUMS.v2.sig"
	horizonInstalledMarker   = ".installed"
)

// Public release signing keys from horizon-rust/horizon
// (crates/horizon-release-sign/release-keys.txt).
var horizonReleaseKeyBase64 = []string{
	"MU4oiIQcCxkZvbwSjjXs7SySCuM6ywRQ3nGaJYZG0gw=",
}

type horizonVersionMeta struct {
	BuildID     string `json:"buildId"`
	ReleaseTag  string `json:"releaseTag,omitempty"`
	ReleaseName string `json:"releaseName,omitempty"`
	Minecraft   string `json:"minecraft"`
	JavaMajor   int    `json:"javaMajor,omitempty"`
	Asset       string `json:"asset,omitempty"`
}

// HorizonRelease describes one downloadable build published by horizon-rust/horizon.
type HorizonRelease struct {
	BuildID     string `json:"buildId"`
	ReleaseTag  string `json:"releaseTag"`
	Name        string `json:"name"`
	Minecraft   string `json:"minecraft"`
	AssetName   string `json:"assetName"`
	AssetSize   int64  `json:"assetSize"`
	AssetURL    string `json:"assetUrl"`
	PublishedAt string `json:"publishedAt"`
	Nightly     bool   `json:"nightly"`
	Prerelease  bool   `json:"prerelease"`
	HasSDKJSON  bool   `json:"-"`
}

type horizonReleaseCache struct {
	at       time.Time
	releases []HorizonRelease
}

type horizonTestConfig struct {
	apiURL      string
	mirrorURL   string
	downloadURL string
	manifestURL string
	modIndexURL string
	keys        []ed25519.PublicKey
	goos        string
	goarch      string
}

type horizonLocalizedError struct {
	zh, en string
}

func (e *horizonLocalizedError) Error() string { return e.zh }

func (e *horizonLocalizedError) LocalizedMessage(en bool) string {
	if en && e.en != "" {
		return e.en
	}
	return e.zh
}

func horizonErr(zh, en string) error {
	return &horizonLocalizedError{zh: zh, en: en}
}

type horizonAPIOption struct {
	Name  string  `json:"name"`
	Value *string `json:"value"`
}

type horizonPrepareAPI struct {
	LaunchAPI     string             `json:"launch_api"`
	Horizon       string             `json:"horizon"`
	Minecraft     string             `json:"minecraft"`
	JavaMajor     int                `json:"java_major"`
	Schema        int                `json:"schema"`
	OutputVersion int                `json:"output_version"`
	Options       []horizonAPIOption `json:"options"`
	ExitCodes     []int              `json:"exit_codes"`
}

func (a horizonPrepareAPI) hasOption(name string) bool {
	for _, opt := range a.Options {
		if opt.Name == name {
			return true
		}
	}
	return false
}

type horizonClientAPI struct {
	LaunchAPI       string             `json:"launch_api"`
	Horizon         string             `json:"horizon"`
	Minecraft       string             `json:"minecraft"`
	CrashReportLine string             `json:"crash_report_line"`
	Options         []horizonAPIOption `json:"options"`
	ExitCodes       []int              `json:"exit_codes"`
}

func (a horizonClientAPI) hasOption(name string) bool {
	for _, opt := range a.Options {
		if opt.Name == name {
			return true
		}
	}
	return false
}

func compatibleHorizonLaunchAPI(version string) bool {
	parts := strings.Split(strings.TrimSpace(version), ".")
	return len(parts) >= 2 && parts[0] == "0" && parts[1] == "1"
}

func horizonTrustedKeys(override []ed25519.PublicKey) ([]ed25519.PublicKey, error) {
	if len(override) > 0 {
		return override, nil
	}
	keys := make([]ed25519.PublicKey, 0, len(horizonReleaseKeyBase64))
	for _, raw := range horizonReleaseKeyBase64 {
		b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
		if err != nil || len(b) != ed25519.PublicKeySize {
			return nil, horizonErr("内置 Horizon 签名公钥无效", "Built-in Horizon signing key is invalid")
		}
		keys = append(keys, ed25519.PublicKey(b))
	}
	return keys, nil
}

// horizonPlatformAssets returns the primary (and optional fallback) archive name
// for the given Go OS/architecture pair, matching Horizon's release matrix.
func horizonPlatformAssets(goos, goarch string) (primary, fallback string, ok bool) {
	switch goos {
	case "windows":
		switch goarch {
		case "amd64":
			return "horizon-windows-x64.zip", "", true
		case "arm64":
			return "horizon-windows-arm64.zip", "horizon-windows-x64.zip", true
		}
	case "linux":
		if goarch == "amd64" {
			return "horizon-linux-x86_64.tar.gz", "", true
		}
	case "darwin":
		switch goarch {
		case "arm64":
			return "horizon-macos-arm64.zip", "", true
		case "amd64":
			return "horizon-macos-x64.zip", "", true
		}
	}
	return "", "", false
}

func isValidISODate(s string) bool {
	if len(s) != 10 {
		return false
	}
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

func isLowerHex(s string) bool {
	if len(s) < 7 || len(s) > 40 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// parseHorizonNightlyTitle strictly parses Horizon's signed nightly title
// ("Nightly YYYY-MM-DD · <commit>") into "nightly-YYYY-MM-DD-<commit>".
func parseHorizonNightlyTitle(title string) (string, bool) {
	rest, ok := strings.CutPrefix(title, "Nightly ")
	if !ok {
		return "", false
	}
	date, commit, ok := strings.Cut(rest, " · ")
	if !ok || !isValidISODate(date) || !isLowerHex(commit) {
		return "", false
	}
	return "nightly-" + date + "-" + commit, true
}

func isHorizonNightlyTag(tag string) bool {
	if tag == "nightly" {
		return true
	}
	rest, ok := strings.CutPrefix(tag, "nightly-")
	return ok && isValidISODate(rest)
}

func isHorizonGameTag(tag string) bool {
	if len(tag) < 2 || tag[0] != 'v' {
		return false
	}
	if strings.HasPrefix(tag, "launcher@") || strings.HasPrefix(tag, "launcher-v") || strings.HasPrefix(tag, "sdk-") {
		return false
	}
	return tag[1] >= '0' && tag[1] <= '9'
}

func validHorizonBuildID(id string) bool {
	if id == "" || len(id) > 96 || id == "." || id == ".." {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.') {
			return false
		}
	}
	return true
}

func horizonEscapePathComponent(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	for i := 0; i < len(text); i++ {
		c := text[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == '@' || c == '+' {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func horizonAssetURL(downloadBase, tag, name string) string {
	return strings.TrimRight(downloadBase, "/") + "/" + horizonEscapePathComponent(tag) + "/" + horizonEscapePathComponent(name)
}

type horizonGitHubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Draft       bool   `json:"draft"`
	Prerelease  bool   `json:"prerelease"`
	PublishedAt string `json:"published_at"`
	Assets      []struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
	} `json:"assets"`
}

func parseHorizonReleases(raw []horizonGitHubRelease, primaryAsset, fallbackAsset, downloadBase string) []HorizonRelease {
	byBuild := map[string]HorizonRelease{}
	for _, r := range raw {
		if r.Draft {
			continue
		}
		nightly := isHorizonNightlyTag(r.TagName)
		if !nightly && !isHorizonGameTag(r.TagName) {
			continue
		}
		buildID := r.TagName
		if nightly {
			parsed, ok := parseHorizonNightlyTitle(r.Name)
			if !ok {
				continue
			}
			buildID = parsed
		}
		if !validHorizonBuildID(buildID) {
			continue
		}
		var assetName string
		var assetSize int64
		var hasSDK bool
		for _, a := range r.Assets {
			if a.Name == "sdk.json" {
				hasSDK = true
			}
			if a.Name == primaryAsset {
				assetName = a.Name
				assetSize = a.Size
			}
		}
		if assetName == "" && fallbackAsset != "" {
			for _, a := range r.Assets {
				if a.Name == fallbackAsset {
					assetName = a.Name
					assetSize = a.Size
					break
				}
			}
		}
		if assetName == "" {
			continue
		}
		rel := HorizonRelease{
			BuildID:     buildID,
			ReleaseTag:  r.TagName,
			Name:        r.Name,
			Minecraft:   horizonDefaultMinecraft,
			AssetName:   assetName,
			AssetSize:   assetSize,
			AssetURL:    horizonAssetURL(downloadBase, r.TagName, assetName),
			PublishedAt: r.PublishedAt,
			Nightly:     nightly,
			Prerelease:  r.Prerelease && r.TagName != "nightly",
			HasSDKJSON:  hasSDK,
		}
		// Prefer the canonical "nightly" tag when both "nightly" and "nightly-YYYY-MM-DD"
		// point to the same build.
		if prev, exists := byBuild[buildID]; exists {
			if prev.ReleaseTag == "nightly" && rel.ReleaseTag != "nightly" {
				continue
			}
		}
		byBuild[buildID] = rel
	}
	out := make([]HorizonRelease, 0, len(byBuild))
	for _, rel := range byBuild {
		out = append(out, rel)
	}
	slices.SortFunc(out, func(a, b HorizonRelease) int {
		aLatest := a.ReleaseTag == "nightly"
		bLatest := b.ReleaseTag == "nightly"
		if aLatest != bLatest {
			if aLatest {
				return -1
			}
			return 1
		}
		if a.PublishedAt != b.PublishedAt {
			return strings.Compare(b.PublishedAt, a.PublishedAt)
		}
		return strings.Compare(b.BuildID, a.BuildID)
	})
	return out
}

// parseHorizonSHA256SUMS parses standard sha256sum lines ("<hex>  <file>" or "<hex> *<file>").
func parseHorizonSHA256SUMS(text string) (map[string]string, error) {
	out := map[string]string{}
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimRight(raw, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		hexSum, rest, ok := strings.Cut(line, " ")
		if !ok {
			return nil, horizonErr("Horizon SHA256SUMS 格式无效", "Invalid Horizon SHA256SUMS format")
		}
		if len(rest) > 0 && (rest[0] == ' ' || rest[0] == '*') {
			rest = rest[1:]
		}
		if len(hexSum) != 64 || rest == "" {
			return nil, horizonErr("Horizon SHA256SUMS 格式无效", "Invalid Horizon SHA256SUMS format")
		}
		lower := strings.ToLower(hexSum)
		for i := 0; i < len(lower); i++ {
			c := lower[i]
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				return nil, horizonErr("Horizon SHA256SUMS 包含无效哈希", "Invalid checksum in Horizon SHA256SUMS")
			}
		}
		out[rest] = lower
	}
	return out, nil
}

// verifyHorizonReleaseSignature verifies an Ed25519 signature over
// "horizon release <signedTag>\n" + sums against the built-in Horizon release keys.
func verifyHorizonReleaseSignature(signedTag string, sums []byte, sigText string, keys []ed25519.PublicKey) error {
	if len(keys) == 0 {
		return horizonErr("缺少 Horizon 发布签名公钥，拒绝安装", "No Horizon release public key available; installation refused")
	}
	msg := make([]byte, 0, len("horizon release \n")+len(signedTag)+len(sums))
	msg = append(msg, "horizon release "...)
	msg = append(msg, signedTag...)
	msg = append(msg, '\n')
	msg = append(msg, sums...)
	for _, line := range strings.Split(sigText, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		sig, err := base64.StdEncoding.DecodeString(line)
		if err != nil || len(sig) != ed25519.SignatureSize {
			continue
		}
		for _, pub := range keys {
			if ed25519.Verify(pub, msg, sig) {
				return nil
			}
		}
	}
	return horizonErr(
		fmt.Sprintf("Horizon %s 的数字签名校验未通过，已中止安装", signedTag),
		fmt.Sprintf("Signature verification failed for Horizon %s; installation aborted", signedTag),
	)
}

func (s *Service) horizonConfig() (apiURL, mirrorURL, downloadBase, goos, goarch string, keys []ed25519.PublicKey) {
	apiURL = horizonGitHubReleasesURL
	mirrorURL = horizonMirrorReleasesURL
	downloadBase = horizonDownloadBaseURL
	goos = runtime.GOOS
	goarch = runtime.GOARCH
	if cfg := s.horizonTest; cfg != nil {
		if cfg.apiURL != "" {
			apiURL = cfg.apiURL
		}
		if cfg.mirrorURL != "" {
			mirrorURL = cfg.mirrorURL
		}
		if cfg.downloadURL != "" {
			downloadBase = cfg.downloadURL
		}
		if cfg.goos != "" {
			goos = cfg.goos
		}
		if cfg.goarch != "" {
			goarch = cfg.goarch
		}
		keys = cfg.keys
	}
	return apiURL, mirrorURL, downloadBase, goos, goarch, keys
}

func (s *Service) fetchSmallBytes(ctx context.Context, rawURL string, maxBytes int64) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	resp, err := s.request(ctx, "GET", rawURL, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > maxBytes {
		return nil, horizonErr("远程校验文件过大", "Remote verification file is too large")
	}
	return b, nil
}

func (s *Service) horizonReleases(ctx context.Context, forceNetwork bool) ([]HorizonRelease, error) {
	apiURL, mirrorURL, downloadBase, goos, goarch, _ := s.horizonConfig()
	primaryAsset, fallbackAsset, ok := horizonPlatformAssets(goos, goarch)
	if !ok {
		return nil, horizonErr(
			fmt.Sprintf("Horizon 暂不提供当前平台 (%s/%s) 的预编译版本", goos, goarch),
			fmt.Sprintf("Horizon does not provide prebuilt binaries for this platform (%s/%s)", goos, goarch),
		)
	}
	if !forceNetwork {
		s.horizonMu.Lock()
		if s.horizonCache != nil && time.Since(s.horizonCache.at) < horizonReleasesCacheTTL {
			cached := slices.Clone(s.horizonCache.releases)
			s.horizonMu.Unlock()
			return cached, nil
		}
		s.horizonMu.Unlock()
	}
	var raw []horizonGitHubRelease
	headers := map[string]string{"Accept": "application/vnd.github+json"}
	err := s.jsonRequest(ctx, "GET", apiURL, nil, headers, &raw)
	if err != nil && mirrorURL != "" && mirrorURL != apiURL {
		if mirrorErr := s.jsonRequest(ctx, "GET", mirrorURL, nil, headers, &raw); mirrorErr == nil {
			err = nil
		}
	}
	if err != nil {
		s.horizonMu.Lock()
		if s.horizonCache != nil && len(s.horizonCache.releases) > 0 {
			cached := slices.Clone(s.horizonCache.releases)
			s.horizonMu.Unlock()
			return cached, nil
		}
		s.horizonMu.Unlock()
		return nil, err
	}
	releases := parseHorizonReleases(raw, primaryAsset, fallbackAsset, downloadBase)
	if len(releases) > 0 && releases[0].HasSDKJSON {
		if game := s.fetchHorizonSDKGame(ctx, downloadBase, releases[0].ReleaseTag); game != "" {
			for i := range releases {
				releases[i].Minecraft = game
			}
		}
	}
	s.horizonMu.Lock()
	s.horizonCache = &horizonReleaseCache{at: time.Now(), releases: slices.Clone(releases)}
	s.horizonMu.Unlock()
	return releases, nil
}

func (s *Service) fetchHorizonSDKGame(ctx context.Context, downloadBase, tag string) string {
	sdkCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	b, err := s.fetchSmallBytes(sdkCtx, horizonAssetURL(downloadBase, tag, "sdk.json"), 1<<20)
	if err != nil {
		return ""
	}
	var sdk struct {
		Build struct {
			Game string `json:"game"`
		} `json:"build"`
	}
	if json.Unmarshal(b, &sdk) != nil {
		return ""
	}
	return strings.TrimSpace(sdk.Build.Game)
}

// HorizonMinecraftVersion returns the Minecraft version targeted by current Horizon releases.
func (s *Service) HorizonMinecraftVersion(ctx context.Context) string {
	s.horizonMu.Lock()
	if s.horizonCache != nil && len(s.horizonCache.releases) > 0 && s.horizonCache.releases[0].Minecraft != "" {
		mc := s.horizonCache.releases[0].Minecraft
		s.horizonMu.Unlock()
		return mc
	}
	s.horizonMu.Unlock()
	if releases, err := s.horizonReleases(ctx, false); err == nil && len(releases) > 0 && releases[0].Minecraft != "" {
		return releases[0].Minecraft
	}
	return horizonDefaultMinecraft
}

func (s *Service) horizonSignedDigest(ctx context.Context, rel HorizonRelease) (string, error) {
	_, _, downloadBase, _, _, overrideKeys := s.horizonConfig()
	keys, err := horizonTrustedKeys(overrideKeys)
	if err != nil {
		return "", err
	}
	signedTag := rel.ReleaseTag
	sigFile := horizonSigFile
	if rel.Nightly {
		parsed, ok := parseHorizonNightlyTitle(rel.Name)
		if !ok || parsed != rel.BuildID {
			return "", horizonErr(
				"Horizon Nightly 版本标题格式无效，无法验证签名",
				"Horizon Nightly release title does not match its signed build identity",
			)
		}
		signedTag = parsed
		sigFile = horizonSigV2File
	}
	sumsBytes, err := s.fetchSmallBytes(ctx, horizonAssetURL(downloadBase, rel.ReleaseTag, horizonSumsFile), 1<<20)
	if err != nil {
		return "", horizonErr(
			fmt.Sprintf("无法获取 Horizon %s 的校验清单 (SHA256SUMS)", rel.BuildID),
			fmt.Sprintf("Failed to fetch SHA256SUMS for Horizon %s", rel.BuildID),
		)
	}
	sigBytes, err := s.fetchSmallBytes(ctx, horizonAssetURL(downloadBase, rel.ReleaseTag, sigFile), 64<<10)
	if err != nil {
		return "", horizonErr(
			fmt.Sprintf("Horizon %s 缺少发布签名 (%s)，已中止安装", rel.BuildID, sigFile),
			fmt.Sprintf("Horizon %s is missing its release signature (%s); installation aborted", rel.BuildID, sigFile),
		)
	}
	if err := verifyHorizonReleaseSignature(signedTag, sumsBytes, string(sigBytes), keys); err != nil {
		return "", err
	}
	sums, err := parseHorizonSHA256SUMS(string(sumsBytes))
	if err != nil {
		return "", err
	}
	digest, ok := sums[rel.AssetName]
	if !ok {
		return "", horizonErr(
			fmt.Sprintf("Horizon 校验清单中未包含 %s", rel.AssetName),
			fmt.Sprintf("Horizon SHA256SUMS does not list %s", rel.AssetName),
		)
	}
	return digest, nil
}
