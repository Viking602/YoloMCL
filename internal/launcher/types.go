package launcher

import (
	"context"
	"mcl/internal/distribution"
	"net/http"
	"sync"
	"time"
)

const appVersion = "0.2.0"

type VersionConfig struct {
	JavaPath string `json:"javaPath,omitempty"`
	MemoryMB int    `json:"memoryMB,omitempty"`
	JVMArgs  string `json:"jvmArgs,omitempty"`
	// MemoryAuto explicitly picks automatic memory (MemoryMB 0).
	MemoryAuto bool `json:"memoryAuto,omitempty"`
	// DisplayName is the name the user gave the version in MCL.
	DisplayName string `json:"displayName,omitempty"`
}
type Settings struct {
	Language string `json:"language,omitempty"`
	JavaPath string `json:"javaPath"`
	MemoryMB int    `json:"memoryMB"`
	// MemoryMode is "manual" to use MemoryMB; otherwise ("" / "auto") the
	// heap is computed automatically.
	MemoryMode      string `json:"memoryMode,omitempty"`
	JVMArgs         string `json:"jvmArgs,omitempty"`
	ShowSnapshots   bool   `json:"showSnapshots"`
	UseRootSettings bool   `json:"useRootSettings"`
	// Modpack update checks: on by default, releases only by default.
	PackAutoCheckOff bool `json:"packAutoCheckOff,omitempty"`
	PackIncludeBeta  bool `json:"packIncludeBeta,omitempty"`
}
type Root struct {
	ID              string                   `json:"id"`
	Name            string                   `json:"name"`
	Path            string                   `json:"path"`
	SelectedVersion string                   `json:"selectedVersion"`
	JavaPath        string                   `json:"javaPath"`
	MemoryMB        int                      `json:"memoryMB"`
	MemoryAuto      bool                     `json:"memoryAuto,omitempty"`
	JVMArgs         string                   `json:"jvmArgs,omitempty"`
	VersionConfigs  map[string]VersionConfig `json:"versionConfigs,omitempty"`
}
type Account struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	UUID     string `json:"uuid"`
	Kind     string `json:"kind"`
	Server   string `json:"server"`
	ClientID string `json:"clientId,omitempty"`
}
type config struct {
	Schema        int       `json:"schema"`
	Settings      Settings  `json:"settings"`
	Roots         []Root    `json:"roots"`
	Accounts      []Account `json:"accounts"`
	ActiveRoot    string    `json:"activeRoot"`
	ActiveAccount string    `json:"activeAccount"`
	// AuthServers are user-saved authlib-injector / Yggdrasil servers.
	AuthServers []AuthServer `json:"authServers,omitempty"`
}

// AuthServer is a saved third-party (Yggdrasil) login server.
type AuthServer struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	// Builtin marks LittleSkin / Ely.by; never stored in config.
	Builtin bool `json:"builtin,omitempty"`
}
type VersionSummary struct {
	ID        string `json:"id"`
	Minecraft string `json:"minecraft"`
	Loader    string `json:"loader"`
	// LoaderVersion is the loader build read from the version JSON
	// libraries; DisplayName the user-set name (VersionConfig).
	LoaderVersion string `json:"loaderVersion,omitempty"`
	DisplayName   string `json:"displayName,omitempty"`
	// Snapshot: the vanilla base is a snapshot / pre-release.
	Snapshot    bool   `json:"snapshot,omitempty"`
	JavaVersion int    `json:"javaVersion"`
	JavaPath    string `json:"javaPath,omitempty"`
	MemoryMB    int    `json:"memoryMB,omitempty"`
	JVMArgs     string `json:"jvmArgs,omitempty"`
	// MemoryAutoSet: the version explicitly chose 自动; MemoryAuto: the
	// effective mode is automatic; AutoMemoryMB: the computed heap.
	MemoryAutoSet bool `json:"memoryAutoSet,omitempty"`
	MemoryAuto    bool `json:"memoryAuto,omitempty"`
	AutoMemoryMB  int  `json:"autoMemoryMB,omitempty"`
}
type Instance struct {
	Root
	Versions  []VersionSummary `json:"versions"`
	ModCount  int              `json:"modCount"`
	Available bool             `json:"available"`
	Pack      *PackLock        `json:"pack,omitempty"`
	Running   bool             `json:"running"`
	// PackUpdate is the cached result of the last online update check.
	PackUpdate  *PackUpdateInfo `json:"packUpdate,omitempty"`
	PackBackups []PackBackup    `json:"packBackups,omitempty"`
}
type Snapshot struct {
	Settings      Settings            `json:"settings"`
	Services      ServiceAvailability `json:"services"`
	Instances     []Instance          `json:"instances"`
	Accounts      []Account           `json:"accounts"`
	ActiveRoot    string              `json:"activeRoot"`
	ActiveAccount string              `json:"activeAccount"`
	Jobs          []Job               `json:"jobs"`
	AuthServers   []AuthServer        `json:"authServers"`
	DataDir       string              `json:"dataDir"`
}
type ServiceAvailability struct {
	Microsoft  bool `json:"microsoft"`
	CurseForge bool `json:"curseForge"`
}
type Job struct {
	ID      string    `json:"id"`
	Title   string    `json:"title"`
	State   string    `json:"state"`
	Message string    `json:"message"`
	Items   []string  `json:"items,omitempty"`
	Done    int       `json:"done"`
	Total   int       `json:"total"`
	Started time.Time `json:"started"`
	// ResultRoot is the directory a finished import/install created.
	ResultRoot string `json:"resultRoot,omitempty"`
	// ResultRef identifies an artefact of the job, e.g. a pack update backup.
	ResultRef string     `json:"resultRef,omitempty"`
	Finished  *time.Time `json:"finished,omitempty"`
}
type jobState struct {
	Job
	cancel context.CancelFunc
}
type Service struct {
	mu           sync.Mutex
	authMu       sync.Mutex
	cfg          config
	distribution distribution.Config
	dir          string
	http         *http.Client
	jobs         map[string]*jobState
	running      map[string]*runningGame
	rootLocks    map[string]bool
	devices      map[string]*deviceSession
}
type Mod struct {
	File        string          `json:"file"`
	Name        string          `json:"name"`
	ModID       string          `json:"modId"`
	Version     string          `json:"version"`
	Loader      string          `json:"loader"`
	Enabled     bool            `json:"enabled"`
	Size        int64           `json:"size"`
	SHA1        string          `json:"sha1"`
	Fingerprint uint32          `json:"fingerprint"`
	Source      string          `json:"source"`
	ProjectID   string          `json:"projectId"`
	VersionID   string          `json:"versionId"`
	Latest      *ProjectVersion `json:"latest,omitempty"`
	Status      string          `json:"status"`
}
type AssetPack struct {
	File        string `json:"file"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Kind        string `json:"kind"`
	Enabled     bool   `json:"enabled"`
	Size        int64  `json:"size"`
}
type PackPreview struct {
	Path          string `json:"path"`
	Name          string `json:"name"`
	Version       string `json:"version"`
	Source        string `json:"source"`
	Minecraft     string `json:"minecraft"`
	Loader        string `json:"loader"`
	LoaderVersion string `json:"loaderVersion"`
	FileCount     int    `json:"fileCount"`
	// DownloadMB estimates the import download (pack files + game).
	DownloadMB int `json:"downloadMB,omitempty"`
	// Unavailable explains why this pack cannot be imported here.
	Unavailable string `json:"unavailable,omitempty"`
}
type Project struct {
	ID          string   `json:"id"`
	Source      string   `json:"source"`
	Type        string   `json:"type"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Icon        string   `json:"icon"`
	Downloads   int64    `json:"downloads"`
	Author      string   `json:"author"`
	Categories  []string `json:"categories"`
	URL         string   `json:"url"`
}
type Dependency struct {
	ProjectID string `json:"projectId"`
	VersionID string `json:"versionId"`
	Kind      string `json:"kind"`
}
type DownloadFile struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	SHA1    string `json:"sha1"`
	SHA512  string `json:"sha512"`
	SHA256  string `json:"sha256,omitempty"`
	Size    int64  `json:"size"`
	Primary bool   `json:"primary"`
}
type ProjectVersion struct {
	ID           string         `json:"id"`
	ProjectID    string         `json:"projectId"`
	Source       string         `json:"source"`
	Name         string         `json:"name"`
	Number       string         `json:"number"`
	Published    string         `json:"published"`
	GameVersions []string       `json:"gameVersions"`
	Loaders      []string       `json:"loaders"`
	Files        []DownloadFile `json:"files"`
	Dependencies []Dependency   `json:"dependencies"`
	Type         string         `json:"type,omitempty"`
	Changelog    string         `json:"changelog,omitempty"`
}
type PackFile struct {
	Kind      string   `json:"kind,omitempty"` // remote | override | client-override
	Size      int64    `json:"size,omitempty"`
	Path      string   `json:"path"`
	SHA1      string   `json:"sha1"`
	SHA512    string   `json:"sha512"`
	Source    string   `json:"source"`
	ProjectID string   `json:"projectId"`
	VersionID string   `json:"versionId"`
	URLs      []string `json:"urls,omitempty"`
}
type PackLock struct {
	Name          string           `json:"name"`
	Version       string           `json:"version"`
	Source        string           `json:"source"`
	ProjectID     string           `json:"projectId"`
	VersionID     string           `json:"versionId"`
	Minecraft     string           `json:"minecraft"`
	Loader        string           `json:"loader"`
	LoaderVersion string           `json:"loaderVersion"`
	Files         []PackFile       `json:"files"`
	Manual        []ManualDownload `json:"manual,omitempty"`
	// Schema 2 fields. Locks written before schema 2 read these as zero values
	// and are normalised by upgradeLock.
	Schema      int           `json:"schema,omitempty"`
	GameVersion string        `json:"gameVersion,omitempty"`
	PackFile    *PackArchive  `json:"packFile,omitempty"`
	Origin      string        `json:"origin,omitempty"` // modrinth | file
	ImportedAt  string        `json:"importedAt,omitempty"`
	UpdatedAt   string        `json:"updatedAt,omitempty"`
	History     []PackHistory `json:"history,omitempty"`
}
type PackArchive struct {
	Name   string `json:"name"`
	SHA1   string `json:"sha1"`
	SHA512 string `json:"sha512"`
	Size   int64  `json:"size"`
}
type PackHistory struct {
	Version string `json:"version"`
	At      string `json:"at"`
	Action  string `json:"action"` // import | update | rollback
}
type ManualDownload struct {
	ProjectID string `json:"projectId"`
	FileID    string `json:"fileId"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	SHA1      string `json:"sha1,omitempty"`
}
