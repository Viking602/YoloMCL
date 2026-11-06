package launcher

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	publisherconfig "github.com/Viking602/YoloMCL/internal/config"
)

type dependencyFixture struct {
	s        *Service
	root     Root
	source   string
	versions map[string]ProjectVersion
	jars     map[string][]byte
	mu       sync.Mutex
	requests map[string]int
}

func newDependencyFixture(t *testing.T, source string) *dependencyFixture {
	t.Helper()
	s := testService(t)
	s.publisher = publisherconfig.Config{CurseForgeAPIKey: "fixture-key"}
	r := newTestRoot(t, s, "Dependencies")
	installFabricFixture(t, s, r)
	r, err := s.getRoot(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	f := &dependencyFixture{s: s, root: r, source: source, versions: map[string]ProjectVersion{}, jars: map[string][]byte{}, requests: map[string]int{}}
	s.http = &http.Client{Transport: transportFunc(f.respond)}
	return f
}

func (f *dependencyFixture) add(t *testing.T, project, version string, deps ...Dependency) {
	t.Helper()
	jar := fabricJar(t, "mod"+project, version)
	hash, _ := hashes(jar)
	f.jars[version] = jar
	f.versions[version] = ProjectVersion{ID: version, ProjectID: project, Source: f.source, Name: "Mod " + project, Number: version, GameVersions: []string{"1.21.1"}, Loaders: []string{"fabric"}, Dependencies: deps, Files: []DownloadFile{{Name: version + ".jar", URL: "https://fixture.test/" + version + ".jar", SHA1: hash, Size: int64(len(jar)), Primary: true}}}
}

func (f *dependencyFixture) existing(t *testing.T, version string, enabled bool) {
	t.Helper()
	v := f.versions[version]
	name := version + ".jar"
	if !enabled {
		name += ".disabled"
	}
	writeTestFile(t, filepath.Join(f.root.VersionDir(), "mods", name), f.jars[version])
	if err := atomicJSON(filepath.Join(f.root.VersionDir(), ".mcl", "mods.json"), map[string]modRecord{version + ".jar": {SHA1: v.Files[0].SHA1, Source: f.source, ProjectID: v.ProjectID, VersionID: version}}); err != nil {
		t.Fatal(err)
	}
}

func (f *dependencyFixture) wire(v ProjectVersion) any {
	file := v.Files[0]
	if f.source == "curseforge" {
		id, _ := strconv.Atoi(v.ID)
		project, _ := strconv.Atoi(v.ProjectID)
		deps := []map[string]int{}
		for _, d := range v.Dependencies {
			dep, _ := strconv.Atoi(d.ProjectID)
			deps = append(deps, map[string]int{"modId": dep, "relationType": map[string]int{"required": 3, "optional": 2, "incompatible": 5}[d.Kind]})
		}
		return map[string]any{"id": id, "modId": project, "displayName": v.Number, "fileName": file.Name, "downloadUrl": file.URL, "fileLength": file.Size, "gameVersions": append(append([]string{}, v.GameVersions...), v.Loaders...), "hashes": []map[string]any{{"algo": 1, "value": file.SHA1}}, "dependencies": deps}
	}
	deps := []map[string]string{}
	for _, d := range v.Dependencies {
		deps = append(deps, map[string]string{"project_id": d.ProjectID, "version_id": d.VersionID, "dependency_type": d.Kind})
	}
	return map[string]any{"id": v.ID, "project_id": v.ProjectID, "name": v.Name, "version_number": v.Number, "game_versions": v.GameVersions, "loaders": v.Loaders, "dependencies": deps, "files": []map[string]any{{"filename": file.Name, "url": file.URL, "size": file.Size, "primary": true, "hashes": map[string]string{"sha1": file.SHA1}}}}
}

func (f *dependencyFixture) respond(r *http.Request) (*http.Response, error) {
	if err := r.Context().Err(); err != nil {
		return nil, err
	}
	path := r.URL.Path
	f.mu.Lock()
	f.requests[path]++
	f.mu.Unlock()
	var body any
	var raw []byte
	switch {
	case r.URL.Host == "fixture.test":
		raw = f.jars[strings.TrimSuffix(strings.TrimPrefix(path, "/"), ".jar")]
	case strings.HasSuffix(path, ".csv"):
		raw = []byte("id,curseforge,modrinth,name,english,abbr,modid\n")
	case path == "/v2/version_files":
		body = map[string]any{}
	case strings.HasPrefix(path, "/v2/version_file/"):
		for _, v := range f.versions {
			if strings.TrimPrefix(path, "/v2/version_file/") == v.Files[0].SHA1 {
				body = f.wire(v)
				break
			}
		}
	case path == "/v1/fingerprints":
		body = map[string]any{"data": map[string]any{"exactMatches": []any{}}}
	case path == "/v2/projects", path == "/v1/mods":
		projects := []map[string]any{}
		for _, v := range f.versions {
			var id any = v.ProjectID
			if f.source == "curseforge" {
				id, _ = strconv.Atoi(v.ProjectID)
			}
			projects = append(projects, map[string]any{"id": id, "title": v.Name, "name": v.Name, "slug": "mod" + v.ProjectID})
		}
		body = projects
		if f.source == "curseforge" {
			body = map[string]any{"data": projects}
		}
	case strings.HasPrefix(path, "/v2/version/"):
		v, ok := f.versions[strings.TrimPrefix(path, "/v2/version/")]
		if !ok {
			return nil, fmt.Errorf("unknown fixture version %s", path)
		}
		body = f.wire(v)
	case strings.HasPrefix(path, "/v2/project/"), strings.HasPrefix(path, "/v1/mods/"):
		parts := strings.Split(path, "/")
		project := parts[3]
		if len(parts) == 6 {
			v, ok := f.versions[parts[5]]
			if !ok {
				return nil, fmt.Errorf("unknown fixture version %s", path)
			}
			body = map[string]any{"data": f.wire(v)}
		} else {
			versions := []any{}
			for _, v := range f.versions {
				if v.ProjectID == project {
					versions = append(versions, f.wire(v))
				}
			}
			body = versions
			if f.source == "curseforge" {
				body = map[string]any{"data": versions}
			}
		}
	default:
		return nil, fmt.Errorf("unexpected dependency fixture request: %s", r.URL)
	}
	if raw == nil {
		var err error
		raw, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(raw)), Request: r}, nil
}

func required(project, version string) Dependency {
	return Dependency{ProjectID: project, VersionID: version, Kind: "required"}
}

func (f *dependencyFixture) chain(t *testing.T, count int) {
	t.Helper()
	for i := 1; i <= count; i++ {
		deps := []Dependency{}
		if i < count {
			deps = append(deps, required(strconv.Itoa(i+1), strconv.Itoa(i+101)))
		}
		f.add(t, strconv.Itoa(i), strconv.Itoa(i+100), deps...)
	}
}

func (f *dependencyFixture) resolve() ([]PlanItem, error) {
	return f.s.ResolveInstall(context.Background(), f.root.ID, f.source, "1", "101", func([]PlanItem) {})
}

func TestResolveInstallDeepDependencies(t *testing.T) {
	for _, source := range []string{"modrinth", "curseforge"} {
		t.Run(source, func(t *testing.T) {
			f := newDependencyFixture(t, source)
			f.chain(t, 5)
			items, err := f.resolve()
			if err != nil || len(items) != 5 {
				t.Fatalf("incomplete chain: %+v, %v", items, err)
			}
			for i, item := range items {
				if item.ProjectID != strconv.Itoa(i+1) || item.Depth != i || item.Status != "install" {
					t.Fatalf("incorrect depth/version in chain: %+v", item)
				}
			}
		})
	}
}

func TestResolveInstallVersionOnlyDependency(t *testing.T) {
	f := newDependencyFixture(t, "modrinth")
	f.chain(t, 4)
	f.add(t, "1", "101", required("", "102"))
	f.add(t, "3", "103", required("", "104"))
	items, err := f.resolve()
	if err != nil || len(items) != 4 {
		t.Fatalf("version-only edge lost its transitive dependencies: %+v, %v", items, err)
	}
}

func TestResolveInstallExpandsInstalledDependencies(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		t.Run(strconv.FormatBool(enabled), func(t *testing.T) {
			f := newDependencyFixture(t, "modrinth")
			f.chain(t, 4)
			f.existing(t, "102", enabled)
			items, err := f.resolve()
			if err != nil || len(items) != 4 {
				t.Fatalf("installed middle node hid missing dependencies: %+v, %v", items, err)
			}
			status := "disabled"
			if enabled {
				status = "installed"
			}
			if items[1].Status != status || items[2].Status != "install" || items[3].Status != "install" {
				t.Fatalf("installed node must be retained while descendants are installed: %+v", items)
			}
		})
	}
}

func TestResolveInstallSharedDependenciesAndCycles(t *testing.T) {
	f := newDependencyFixture(t, "modrinth")
	f.add(t, "1", "101", required("2", "102"), required("3", "103"))
	f.add(t, "2", "102", required("4", "104"))
	f.add(t, "3", "103", required("4", "104"))
	f.add(t, "4", "104", required("2", "102"))
	items, err := f.resolve()
	if err != nil || len(items) != 4 {
		t.Fatalf("cycle/shared dependency duplicated or lost nodes: %+v, %v", items, err)
	}
	if len(items[3].RequiredBy) != 2 || !contains(items[3].RequiredBy, "modrinth:2") || !contains(items[3].RequiredBy, "modrinth:3") {
		t.Fatalf("shared dependency lost one of its parents: %+v", items[3])
	}
	for i := 1; i <= 4; i++ {
		if got := f.requests["/v2/version/"+strconv.Itoa(i+100)]; got != 1 {
			t.Errorf("version %d fetched %d times", i+100, got)
		}
	}
}

func TestResolveInstallDoesNotSilentlyTruncate(t *testing.T) {
	for _, wide := range []bool{false, true} {
		t.Run(strconv.FormatBool(wide), func(t *testing.T) {
			f := newDependencyFixture(t, "modrinth")
			f.chain(t, 129)
			if wide {
				var deps []Dependency
				for i := 2; i <= 129; i++ {
					deps = append(deps, required(strconv.Itoa(i), strconv.Itoa(i+100)))
				}
				f.add(t, "1", "101", deps...)
			}
			items, err := f.resolve()
			if !errors.Is(err, errDependencyLimit) || len(items) > 128 {
				t.Fatalf("truncated dependency graph reported success: rows=%d, err=%v", len(items), err)
			}
		})
	}
}

func TestInstallModExpandsInstalledDependencies(t *testing.T) {
	f := newDependencyFixture(t, "modrinth")
	f.chain(t, 4)
	f.existing(t, "102", true)
	job, err := f.s.InstallMod(f.root.ID, f.source, "1", "101", "")
	if err != nil {
		t.Fatal(err)
	}
	if result := waitJob(t, f.s, job); result.State != "completed" {
		t.Fatal(result.Message)
	} else if result.TargetRootID != f.root.ID {
		t.Fatal("mod installation did not identify the instance to refresh")
	}
	for i := 1; i <= 4; i++ {
		name := strconv.Itoa(i+100) + ".jar"
		if _, err := os.Stat(filepath.Join(f.root.VersionDir(), "mods", name)); err != nil {
			t.Errorf("transitive dependency %s not installed: %v", name, err)
		}
	}
	if f.requests["/102.jar"] != 0 {
		t.Fatal("existing dependency was unnecessarily downloaded again")
	}
}

func TestResolveInstallUsesExactInstalledVersion(t *testing.T) {
	f := newDependencyFixture(t, "modrinth")
	f.chain(t, 4)
	f.add(t, "1", "101", required("2", ""))
	f.add(t, "2", "202", required("5", "105"))
	f.add(t, "5", "105")
	f.existing(t, "102", true)
	items, err := f.resolve()
	if err != nil || len(items) != 4 || items[1].VersionID != "102" || items[3].ProjectID != "4" {
		t.Fatalf("resolved a newer dependency instead of the installed file: %+v, %v", items, err)
	}
	if f.requests["/v2/project/2/version"] != 0 {
		t.Fatal("an unpinned installed dependency should not select a newer release")
	}
}

func TestResolveInstallIdentifiesImportedDependency(t *testing.T) {
	f := newDependencyFixture(t, "modrinth")
	f.chain(t, 4)
	f.add(t, "1", "101", required("2", ""))
	writeTestFile(t, filepath.Join(f.root.VersionDir(), "mods", "imported.jar"), f.jars["102"])
	items, err := f.resolve()
	if err != nil || len(items) != 4 || items[1].Status != "installed" || items[1].VersionID != "102" {
		t.Fatalf("imported jar's transitive dependencies were not identified: %+v, %v", items, err)
	}
	if f.requests["/v2/version_file/"+f.versions["102"].Files[0].SHA1] != 1 {
		t.Fatal("imported dependency was not identified by its actual file hash")
	}
}

func TestResolveInstallExpandsDependencyProvidedByAnotherJar(t *testing.T) {
	f := newDependencyFixture(t, "modrinth")
	f.chain(t, 4)
	f.add(t, "1", "101", required("2", ""))
	f.add(t, "9", "109", required("3", "103"))
	jar := zipBytes(t, map[string][]byte{"fabric.mod.json": jsonBytes(t, map[string]any{"id": "mod9", "version": "109", "provides": []string{"mod2"}})})
	v := f.versions["109"]
	v.Files[0].SHA1, _ = hashes(jar)
	v.Files[0].Size = int64(len(jar))
	f.versions["109"], f.jars["109"] = v, jar
	f.existing(t, "109", true)
	items, err := f.resolve()
	if err != nil || len(items) != 4 || items[1].Status != "installed" || items[1].VersionID != "109" || items[3].ProjectID != "4" {
		t.Fatalf("bundled dependency was reinstalled or hid its descendants: %+v, %v", items, err)
	}
}

func TestResolveInstallPinnedDependencyReplacesDifferentVersion(t *testing.T) {
	f := newDependencyFixture(t, "modrinth")
	f.chain(t, 4)
	f.add(t, "2", "202")
	f.existing(t, "202", true)
	items, err := f.resolve()
	if err != nil || len(items) != 4 || items[1].Status != "install" || items[1].VersionID != "102" {
		t.Fatalf("wrong installed version was treated as satisfied: %+v, %v", items, err)
	}
}

func TestResolveInstallRejectsInvalidPinnedVersion(t *testing.T) {
	for _, mismatch := range []string{"project", "minecraft", "loader"} {
		t.Run(mismatch, func(t *testing.T) {
			f := newDependencyFixture(t, "modrinth")
			f.chain(t, 3)
			v := f.versions["102"]
			switch mismatch {
			case "project":
				v.ProjectID = "99"
			case "minecraft":
				v.GameVersions = []string{"1.20.1"}
			case "loader":
				v.Loaders = []string{"neoforge"}
			}
			f.versions["102"] = v
			items, err := f.resolve()
			if err != nil || len(items) != 2 || items[1].Status != "none" {
				t.Fatalf("incompatible or wrong-project dependency reported installable: %+v, %v", items, err)
			}
		})
	}
}

func TestResolveInstallReportsTransitiveVersionConflict(t *testing.T) {
	f := newDependencyFixture(t, "modrinth")
	f.add(t, "1", "101", required("2", "102"), required("3", "103"))
	f.add(t, "2", "102")
	f.add(t, "2", "202")
	f.add(t, "3", "103", required("2", "202"))
	items, err := f.resolve()
	if err != nil || len(items) != 3 || items[1].Status != "conflict" {
		t.Fatalf("conflicting transitive pin was silently deduplicated: %+v, %v", items, err)
	}
	job, err := f.s.InstallMod(f.root.ID, f.source, "1", "101", "")
	if err != nil {
		t.Fatal(err)
	}
	if result := waitJob(t, f.s, job); result.State != "failed" {
		t.Fatalf("installed conflicting versions: %+v", result)
	}
	for path := range f.requests {
		if strings.HasSuffix(path, ".jar") {
			t.Fatalf("download started before dependency conflict was checked: %s", path)
		}
	}
}

func TestResolveInstallCancellationDuringDeepDependency(t *testing.T) {
	f := newDependencyFixture(t, "modrinth")
	f.chain(t, 4)
	reached := make(chan struct{})
	f.s.http.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/v2/version/104" {
			close(reached)
			<-r.Context().Done()
			return nil, r.Context().Err()
		}
		return f.respond(r)
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := f.s.ResolveInstall(ctx, f.root.ID, f.source, "1", "101", func([]PlanItem) {})
		done <- err
	}()
	select {
	case <-reached:
	case <-time.After(5 * time.Second):
		t.Fatal("resolver never reached the deep dependency")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation was swallowed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("dependency resolver did not stop after cancellation")
	}
}

func TestInstallModDeepSharedDependenciesAndCycle(t *testing.T) {
	for _, source := range []string{"modrinth", "curseforge"} {
		t.Run(source, func(t *testing.T) {
			f := newDependencyFixture(t, source)
			f.chain(t, 5)
			f.add(t, "1", "101", required("2", "102"), required("4", "104"))
			f.add(t, "5", "105", required("2", "102"))
			job, err := f.s.InstallMod(f.root.ID, f.source, "1", "101", "")
			if err != nil {
				t.Fatal(err)
			}
			if result := waitJob(t, f.s, job); result.State != "completed" {
				t.Fatal(result.Message)
			}
			for i := 1; i <= 5; i++ {
				v := f.versions[strconv.Itoa(i+100)]
				if !checkHashes(filepath.Join(f.root.VersionDir(), "mods", v.Files[0].Name), v.Files[0].SHA1, "") {
					t.Errorf("deep dependency %s not installed", v.ProjectID)
				}
				if got := f.requests["/"+v.Files[0].Name]; got != 1 {
					t.Errorf("dependency %s downloaded %d times", v.ProjectID, got)
				}
			}
		})
	}
}

func TestInstallPlannedIncludesDeepDependenciesThroughDisabledMod(t *testing.T) {
	f := newDependencyFixture(t, "modrinth")
	f.chain(t, 5)
	f.existing(t, "102", false)
	plan, err := f.resolve()
	if err != nil {
		t.Fatal(err)
	}
	var items []PlanInstall
	var enable, skip []string
	for _, it := range plan {
		if it.Status == "disabled" {
			enable = append(enable, it.File)
			skip = append(skip, it.Key)
		} else if it.Status == "install" {
			items = append(items, PlanInstall{Key: it.Key, Source: it.Source, ProjectID: it.ProjectID, VersionID: it.VersionID, Title: it.Title, Depth: it.Depth})
		}
	}
	job, err := f.s.InstallPlanned(f.root.ID, items, enable, skip)
	if err != nil {
		t.Fatal(err)
	}
	if result := waitJob(t, f.s, job); result.State != "completed" {
		t.Fatal(result.Message)
	}
	for i := 1; i <= 5; i++ {
		v := f.versions[strconv.Itoa(i+100)]
		if !checkHashes(filepath.Join(f.root.VersionDir(), "mods", v.Files[0].Name), v.Files[0].SHA1, "") {
			t.Errorf("planned dependency %s not installed/enabled", v.ProjectID)
		}
	}
	if f.requests["/102.jar"] != 0 {
		t.Fatal("disabled dependency was downloaded instead of enabled")
	}
}
