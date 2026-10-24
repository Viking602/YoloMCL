package launcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type updFixture struct {
	s      *Service
	routes map[string][]byte
	v1, v2 string // local pack paths
	root   Instance
}

func mrIndex(t *testing.T, version string, mods map[string][]byte, routes map[string][]byte) map[string]any {
	files := []map[string]any{}
	names := []string{}
	for n := range mods {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		sha, sha512 := hashes(mods[n])
		u := "https://fixture.test/" + version + "/" + n
		routes[u] = mods[n]
		files = append(files, map[string]any{"path": "mods/" + n, "hashes": map[string]string{"sha1": sha, "sha512": sha512}, "downloads": []string{u}, "fileSize": len(mods[n])})
	}
	return map[string]any{"formatVersion": 1, "game": "minecraft", "versionId": version, "name": "Update Fixture", "dependencies": map[string]string{"minecraft": "1.21.1"}, "files": files}
}

func newUpdFixture(t *testing.T) *updFixture {
	s := testService(t)
	f := &updFixture{s: s, routes: map[string][]byte{}}
	client := []byte("client-jar")
	csha, _ := hashes(client)
	version := jsonBytes(t, gameVersion{ID: "1.21.1", Downloads: map[string]artifact{"client": {URL: "https://fixture.test/client", SHA1: csha}}})
	vh, _ := hashes(version)
	f.routes[manifestURL] = jsonBytes(t, map[string]any{"versions": []map[string]string{{"id": "1.21.1", "url": "https://fixture.test/version", "sha1": vh}}})
	f.routes["https://fixture.test/version"] = version
	f.routes["https://fixture.test/client"] = client
	dir := t.TempDir()
	f.v1 = filepath.Join(dir, "fixture-1.0.0.mrpack")
	f.v2 = filepath.Join(dir, "fixture-1.1.0.mrpack")
	writeTestFile(t, f.v1, zipBytes(t, map[string][]byte{
		"modrinth.index.json":        jsonBytes(t, mrIndex(t, "1.0.0", map[string][]byte{"alpha.jar": []byte("alpha-1"), "beta.jar": []byte("beta-1"), "gamma.jar": []byte("gamma")}, f.routes)),
		"overrides/config/pack.toml": []byte("v=1"),
		"overrides/config/keep.toml": []byte("keep"),
	}))
	writeTestFile(t, f.v2, zipBytes(t, map[string][]byte{
		"modrinth.index.json":         jsonBytes(t, mrIndex(t, "1.1.0", map[string][]byte{"alpha.jar": []byte("alpha-2"), "gamma.jar": []byte("gamma"), "delta.jar": []byte("delta")}, f.routes)),
		"overrides/config/pack.toml":  []byte("v=2"),
		"overrides/config/keep.toml":  []byte("keep"),
		"overrides/config/extra.toml": []byte("pack-extra"),
	}))
	// Modrinth knows both archives by hash.
	for _, x := range []struct{ path, id, num string }{{f.v1, "ver1", "1.0.0"}, {f.v2, "ver2", "1.1.0"}} {
		b, _ := os.ReadFile(x.path)
		sha, sha512 := hashes(b)
		u := "https://fixture.test/pack-" + x.num + ".mrpack"
		f.routes[u] = b
		v := map[string]any{"id": x.id, "project_id": "proj", "version_number": x.num, "version_type": "release", "changelog": "Changes in " + x.num, "game_versions": []string{"1.21.1"}, "loaders": []string{"minecraft"},
			"files": []map[string]any{{"filename": filepath.Base(x.path), "url": u, "primary": true, "hashes": map[string]string{"sha1": sha, "sha512": sha512}}}}
		f.routes[modrinthAPI+"/version_file/"+sha+"?algorithm=sha1"] = jsonBytes(t, v)
		f.routes[modrinthAPI+"/version/"+x.id] = jsonBytes(t, v)
	}
	fakeHTTP(t, s, f.routes)
	id, e := s.ImportPack(ImportRequest{Path: f.v1, Parent: t.TempDir(), Name: "Fixture"})
	if e != nil {
		t.Fatal(e)
	}
	if j := waitJob(t, s, id); j.State != "completed" {
		t.Fatal(j.Message)
	}
	f.root = s.Snapshot().Instances[0]
	// Player edits: changes a pack config, adds own mod, adds a file the new pack will also ship, disables a pack mod.
	writeTestFile(t, filepath.Join(f.root.Path, "config/pack.toml"), []byte("player edit"))
	writeTestFile(t, filepath.Join(f.root.Path, "mods/user.jar"), []byte("user mod"))
	writeTestFile(t, filepath.Join(f.root.Path, "config/extra.toml"), []byte("player extra"))
	if e := os.Rename(filepath.Join(f.root.Path, "mods/alpha.jar"), filepath.Join(f.root.Path, "mods/alpha.jar.disabled")); e != nil {
		t.Fatal(e)
	}
	return f
}

func treeSnapshot(t *testing.T, root string) map[string]string {
	out := map[string]string{}
	_ = filepath.Walk(root, func(p string, info os.FileInfo, e error) error {
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if e != nil || info.IsDir() || strings.HasPrefix(rel, ".mcl/") || strings.HasPrefix(rel, "versions/") || strings.HasPrefix(rel, "libraries/") || strings.HasPrefix(rel, "assets/") {
			return nil
		}
		b, _ := os.ReadFile(p)
		out[rel] = string(b)
		return nil
	})
	return out
}

func paths(c []PackChange) string {
	s := []string{}
	for _, x := range c {
		s = append(s, x.Path+":"+x.Reason)
	}
	return strings.Join(s, ",")
}

func TestPackImportResolvesOriginAndSchema(t *testing.T) {
	f := newUpdFixture(t)
	lock := readPack(f.root.Path)
	if lock.Schema != 2 || lock.ProjectID != "proj" || lock.VersionID != "ver1" || lock.Origin != "file" || lock.PackFile == nil || lock.GameVersion != "1.21.1" {
		t.Fatalf("lock not stamped: %+v", lock)
	}
	for _, pf := range lock.Files {
		if pf.Kind == "" {
			t.Fatalf("file kind missing: %+v", pf)
		}
	}
	// old (schema 1) locks still load with sensible kinds
	var old PackLock
	_ = json.Unmarshal([]byte(`{"name":"x","files":[{"path":"mods/a.jar","source":"modrinth"},{"path":"config/a","source":"override"}]}`), &old)
	upgradeLock(&old)
	if old.Files[0].Kind != "remote" || old.Files[1].Kind != "override" || old.Origin != "file" {
		t.Fatalf("compat default wrong: %+v", old)
	}
}

func TestPackUpdateCheckFiltersByGameAndRelease(t *testing.T) {
	f := newUpdFixture(t)
	q := url.Values{}
	gv, _ := json.Marshal([]string{"1.21.1"})
	q.Set("game_versions", string(gv))
	beta := map[string]any{"id": "ver3", "project_id": "proj", "version_number": "1.2.0-beta", "version_type": "beta", "game_versions": []string{"1.21.1"}}
	other := map[string]any{"id": "ver9", "project_id": "proj", "version_number": "2.0.0", "version_type": "release", "game_versions": []string{"1.20.1"}}
	var v2, v1 map[string]any
	_ = json.Unmarshal(f.routes[modrinthAPI+"/version/ver2"], &v2)
	_ = json.Unmarshal(f.routes[modrinthAPI+"/version/ver1"], &v1)
	f.routes[modrinthAPI+"/project/proj/version?"+q.Encode()] = jsonBytes(t, []any{beta, other, v2, v1})
	info, e := f.s.CheckPackUpdate(context.Background(), f.root.ID)
	if e != nil {
		t.Fatal(e)
	}
	if !info.HasUpdate || info.Latest.ID != "ver2" || info.Latest.Changelog == "" {
		t.Fatalf("wrong latest: %+v", info)
	}
	if snap := f.s.Snapshot().Instances[0]; snap.PackUpdate == nil || !snap.PackUpdate.HasUpdate {
		t.Fatal("update badge state not cached")
	}
}

func TestPackUpdateApplyAndRollback(t *testing.T) {
	f := newUpdFixture(t)
	before := treeSnapshot(t, f.root.Path)
	plan, e := f.s.PlanPackUpdate(context.Background(), f.root.ID, "", "ver2")
	if e != nil {
		t.Fatal(e)
	}
	if got := paths(plan.Added); got != "mods/delta.jar:" {
		t.Fatalf("added %s", got)
	}
	if got := paths(plan.Removed); got != "mods/beta.jar:" {
		t.Fatalf("removed %s", got)
	}
	if got := paths(plan.Updated); got != "mods/alpha.jar:" {
		t.Fatalf("updated %s", got)
	}
	if got := paths(plan.Overwritten); got != "config/extra.toml:user,config/pack.toml:modified" {
		t.Fatalf("overwritten %s", got)
	}
	if plan.Changelog != "Changes in 1.1.0" || plan.OldVersion != "1.0.0" || plan.NewVersion != "1.1.0" {
		t.Fatalf("plan meta %+v", plan)
	}
	if treeSnapshot(t, f.root.Path)["config/pack.toml"] != "player edit" {
		t.Fatal("planning touched the game dir")
	}
	id, e := f.s.ApplyPackUpdate(plan.ID)
	if e != nil {
		t.Fatal(e)
	}
	j := waitJob(t, f.s, id)
	if j.State != "completed" || j.ResultRef == "" {
		t.Fatalf("apply: %+v", j)
	}
	after := treeSnapshot(t, f.root.Path)
	want := map[string]string{"mods/alpha.jar.disabled": "alpha-2", "mods/gamma.jar": "gamma", "mods/delta.jar": "delta", "mods/user.jar": "user mod", "config/pack.toml": "v=2", "config/keep.toml": "keep", "config/extra.toml": "pack-extra"}
	for k, v := range want {
		if after[k] != v {
			t.Fatalf("%s = %q, want %q (tree %v)", k, after[k], v, after)
		}
	}
	for _, gone := range []string{"mods/beta.jar", "mods/alpha.jar"} {
		if _, ok := after[gone]; ok {
			t.Fatalf("%s still present", gone)
		}
	}
	if lock := readPack(f.root.Path); lock.Version != "1.1.0" || lock.VersionID != "ver2" {
		t.Fatalf("lock not updated: %+v", lock)
	}
	if _, e := os.Stat(filepath.Join(f.root.Path, ".mcl", "update-staging")); !os.IsNotExist(e) {
		t.Fatal("staging not cleaned")
	}
	snap := f.s.Snapshot().Instances[0]
	if len(snap.PackBackups) != 1 {
		t.Fatalf("backups %+v", snap.PackBackups)
	}
	rep, e := f.s.PackUpdateReport(f.root.ID, j.ResultRef)
	if e != nil || len(rep.Overwritten) != 2 {
		t.Fatalf("report %v %+v", e, rep)
	}
	id, e = f.s.RollbackPack(f.root.ID, j.ResultRef)
	if e != nil {
		t.Fatal(e)
	}
	if j := waitJob(t, f.s, id); j.State != "completed" {
		t.Fatal(j.Message)
	}
	restored := treeSnapshot(t, f.root.Path)
	if len(restored) != len(before) {
		t.Fatalf("rollback tree differs:\n%v\n%v", before, restored)
	}
	for k, v := range before {
		if restored[k] != v {
			t.Fatalf("rollback %s = %q want %q", k, restored[k], v)
		}
	}
	if lock := readPack(f.root.Path); lock.Version != "1.0.0" {
		t.Fatal("lock not rolled back")
	}
}

func TestPackUpdateFaultMidApplyRestores(t *testing.T) {
	f := newUpdFixture(t)
	before := treeSnapshot(t, f.root.Path)
	plan, e := f.s.PlanPackUpdate(context.Background(), f.root.ID, f.v2, "")
	if e != nil {
		t.Fatal(e)
	}
	applyFaultHook = func(n int) error {
		if n == 2 {
			return errors.New("injected disk failure")
		}
		return nil
	}
	defer func() { applyFaultHook = nil }()
	id, e := f.s.ApplyPackUpdate(plan.ID)
	if e != nil {
		t.Fatal(e)
	}
	j := waitJob(t, f.s, id)
	if j.State != "failed" || !strings.Contains(j.Message, "已恢复原状") {
		t.Fatalf("expected restore: %+v", j)
	}
	got := treeSnapshot(t, f.root.Path)
	if len(got) != len(before) {
		t.Fatalf("tree differs after restore:\n%v\n%v", before, got)
	}
	for k, v := range before {
		if got[k] != v {
			t.Fatalf("%s = %q want %q", k, got[k], v)
		}
	}
	if lock := readPack(f.root.Path); lock.Version != "1.0.0" {
		t.Fatal("lock changed")
	}
	if len(listPackBackups(f.root.Path)) != 0 {
		t.Fatal("failed update left a backup")
	}
}

func TestPackUpdateConfigFailureRestoresFiles(t *testing.T) {
	f := newUpdFixture(t)
	if err := f.s.Select(f.root.ID, "", "user-selection"); err != nil {
		t.Fatal(err)
	}
	before := treeSnapshot(t, f.root.Path)
	plan, err := f.s.PlanPackUpdate(context.Background(), f.root.ID, f.v2, "")
	if err != nil {
		t.Fatal(err)
	}
	blockConfigWrite(t, f.s)
	id, err := f.s.ApplyPackUpdate(plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	j := waitJob(t, f.s, id)
	if j.State != "failed" || !strings.Contains(j.Message, "已恢复原状") {
		t.Fatalf("expected rollback after config failure: %+v", j)
	}
	if !reflect.DeepEqual(before, treeSnapshot(t, f.root.Path)) {
		t.Fatal("failed config save left updated game files")
	}
	r, err := f.s.getRoot(f.root.ID)
	if err != nil || r.SelectedVersion != "user-selection" || readPack(f.root.Path).Version != "1.0.0" {
		t.Fatalf("failed update changed version: %+v, %v", r, err)
	}
}

func TestPackRollbackConfigFailureRetainsBackupForRetry(t *testing.T) {
	f := newUpdFixture(t)
	plan, err := f.s.PlanPackUpdate(context.Background(), f.root.ID, f.v2, "")
	if err != nil {
		t.Fatal(err)
	}
	id, err := f.s.ApplyPackUpdate(plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	updated := waitJob(t, f.s, id)
	if updated.State != "completed" {
		t.Fatalf("update: %+v", updated)
	}
	blockConfigWrite(t, f.s)
	id, err = f.s.RollbackPack(f.root.ID, updated.ResultRef)
	if err != nil {
		t.Fatal(err)
	}
	if j := waitJob(t, f.s, id); j.State != "failed" || !strings.Contains(j.Message, "备份已保留") {
		t.Fatalf("rollback should report persistence error: %+v", j)
	}
	if len(listPackBackups(f.root.Path)) == 0 {
		t.Fatal("backup deleted before configuration was saved")
	}
	if err := os.Remove(filepath.Join(f.s.dir, "config.json")); err != nil {
		t.Fatal(err)
	}
	id, err = f.s.RollbackPack(f.root.ID, updated.ResultRef)
	if err != nil {
		t.Fatal(err)
	}
	if j := waitJob(t, f.s, id); j.State != "completed" {
		t.Fatalf("retry: %+v", j)
	}
}

func blockConfigWrite(t *testing.T, s *Service) {
	t.Helper()
	path := filepath.Join(s.dir, "config.json")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
}

func TestPackUpdateRejectsSamePackAndOtherPack(t *testing.T) {
	f := newUpdFixture(t)
	if _, e := f.s.PlanPackUpdate(context.Background(), f.root.ID, f.v1, ""); e == nil {
		t.Fatal("same archive accepted")
	}
	other := filepath.Join(t.TempDir(), "other.mrpack")
	writeTestFile(t, other, zipBytes(t, map[string][]byte{"modrinth.index.json": jsonBytes(t, map[string]any{"formatVersion": 1, "game": "minecraft", "versionId": "9", "name": "Other", "dependencies": map[string]string{"minecraft": "1.21.1"}, "files": []any{}})}))
	if _, e := f.s.PlanPackUpdate(context.Background(), f.root.ID, other, ""); e == nil {
		t.Fatal("different pack accepted")
	}
}

func TestPackBackupsKeepThree(t *testing.T) {
	root := t.TempDir()
	for _, n := range []string{"20260101-000001-a", "20260101-000002-b", "20260101-000003-c", "20260101-000004-d"} {
		_ = os.MkdirAll(filepath.Join(backupsDir(root), n), 0755)
	}
	prunePackBackups(root)
	e, _ := os.ReadDir(backupsDir(root))
	if len(e) != 3 || e[0].Name() != "20260101-000002-b" {
		t.Fatalf("prune kept %v", e)
	}
}

func importLocal(t *testing.T, f *updFixture, path, name string) Instance {
	id, e := f.s.ImportPack(ImportRequest{Path: path, Parent: t.TempDir(), Name: name})
	if e != nil {
		t.Fatal(e)
	}
	if j := waitJob(t, f.s, id); j.State != "completed" {
		t.Fatal(j.Message)
	}
	for _, in := range f.s.Snapshot().Instances {
		if in.Name == name {
			return in
		}
	}
	t.Fatal("root missing")
	return Instance{}
}

func applyPlan(t *testing.T, s *Service, plan PackUpdatePlan) Job {
	id, e := s.ApplyPackUpdate(plan.ID)
	if e != nil {
		t.Fatal(e)
	}
	return waitJob(t, s, id)
}

func TestPackUpdateRenamedModKeepsDisabled(t *testing.T) {
	f := newUpdFixture(t)
	mk := func(ver, file string, body []byte) string {
		sha, sha512 := hashes(body)
		u := "https://cdn.modrinth.com/data/AAAA1111/versions/" + ver + "/" + file
		f.routes[u] = body
		idx := map[string]any{"formatVersion": 1, "game": "minecraft", "versionId": ver, "name": "Rename Pack", "dependencies": map[string]string{"minecraft": "1.21.1"},
			"files": []map[string]any{{"path": "mods/" + file, "hashes": map[string]string{"sha1": sha, "sha512": sha512}, "downloads": []string{u}}}}
		p := filepath.Join(t.TempDir(), ver+".mrpack")
		writeTestFile(t, p, zipBytes(t, map[string][]byte{"modrinth.index.json": jsonBytes(t, idx)}))
		return p
	}
	p1, p2 := mk("1.0", "cool-1.0.jar", []byte("cool one")), mk("2.0", "cool-2.0.jar", []byte("cool two"))
	r := importLocal(t, f, p1, "Rename")
	if e := os.Rename(filepath.Join(r.Path, "mods/cool-1.0.jar"), filepath.Join(r.Path, "mods/cool-1.0.jar.disabled")); e != nil {
		t.Fatal(e)
	}
	plan, e := f.s.PlanPackUpdate(context.Background(), r.ID, p2, "")
	if e != nil {
		t.Fatal(e)
	}
	if len(plan.Updated) != 1 || plan.Updated[0].From != "mods/cool-1.0.jar" || !plan.Updated[0].Disabled || len(plan.Removed) != 0 || len(plan.Added) != 0 {
		t.Fatalf("rename not matched by project: %+v", plan)
	}
	if j := applyPlan(t, f.s, plan); j.State != "completed" {
		t.Fatal(j.Message)
	}
	if zzr(filepath.Join(r.Path, "mods/cool-2.0.jar.disabled")) != "cool two" {
		t.Fatal("new file not written disabled")
	}
	for _, gone := range []string{"mods/cool-1.0.jar.disabled", "mods/cool-2.0.jar"} {
		if _, e := os.Stat(filepath.Join(r.Path, gone)); e == nil {
			t.Fatalf("%s present", gone)
		}
	}
}

func zzr(p string) string { b, _ := os.ReadFile(p); return string(b) }

func TestMultiMCUpdateFromFile(t *testing.T) {
	f := newUpdFixture(t)
	mmc := func(name string, files map[string]string) string {
		m := map[string][]byte{"Inst/instance.cfg": []byte("name=My MMC\n"), "Inst/mmc-pack.json": []byte(`{"components":[{"uid":"net.minecraft","version":"1.21.1"}]}`)}
		for k, v := range files {
			m["Inst/.minecraft/"+k] = []byte(v)
		}
		p := filepath.Join(t.TempDir(), name)
		writeTestFile(t, p, zipBytes(t, m))
		return p
	}
	v1 := mmc("v1.zip", map[string]string{"config/a.txt": "a1", "mods/x.jar": "x", "saves/W/level.dat": "pack world"})
	v2 := mmc("v2.zip", map[string]string{"config/a.txt": "a2", "mods/y.jar": "y", "saves/W/level.dat": "pack world v2"})
	r := importLocal(t, f, v1, "MMC")
	writeTestFile(t, filepath.Join(r.Path, "config/a.txt"), []byte("player"))
	writeTestFile(t, filepath.Join(r.Path, "saves/W/level.dat"), []byte("my progress"))
	writeTestFile(t, filepath.Join(r.Path, "mods/mine.jar"), []byte("mine"))
	if _, e := f.s.PlanPackUpdate(context.Background(), r.ID, "", "x"); e == nil {
		t.Fatal("MultiMC must not offer online update")
	}
	plan, e := f.s.PlanPackUpdate(context.Background(), r.ID, v2, "")
	if e != nil {
		t.Fatal(e)
	}
	if j := applyPlan(t, f.s, plan); j.State != "completed" {
		t.Fatal(j.Message)
	}
	got := map[string]string{"config/a.txt": "a2", "mods/y.jar": "y", "saves/W/level.dat": "my progress", "mods/mine.jar": "mine"}
	for k, v := range got {
		if zzr(filepath.Join(r.Path, k)) != v {
			t.Errorf("%s = %q want %q", k, zzr(filepath.Join(r.Path, k)), v)
		}
	}
	if _, e := os.Stat(filepath.Join(r.Path, "mods/x.jar")); e == nil {
		t.Error("removed pack mod still present")
	}
	if paths(plan.Overwritten) != "config/a.txt:modified" {
		t.Errorf("overwritten %s", paths(plan.Overwritten))
	}
}

func TestCurseForge403IsExplained(t *testing.T) {
	e := cfError("/mods/search?x", errors.New("api.curseforge.com 返回 HTTP 403"))
	if e == nil || !strings.Contains(e.Error(), "没有搜索权限") {
		t.Fatal(e)
	}
	if cfError("/mods/1", nil) != nil {
		t.Fatal("nil must stay nil")
	}
}

func TestCurseForgeManualKeepsOldFile(t *testing.T) {
	f := newUpdFixture(t)
	f.s.publisher.CurseForgeAPIKey = "test-key"
	cf := func(pid, fid int, name, body string, open bool) map[string]any {
		sha, _ := hashes([]byte(body))
		u := ""
		if open {
			u = fmt.Sprintf("https://edge.forgecdn.net/files/%d/%s", fid, name)
			f.routes[u] = []byte(body)
		}
		return map[string]any{"id": fid, "modId": pid, "displayName": name, "fileName": name, "downloadUrl": u, "releaseType": 1, "hashes": []map[string]any{{"value": sha, "algo": 1}}}
	}
	a1, b1 := cf(10, 100, "a-1.jar", "a1", true), cf(20, 200, "b-1.jar", "b1", true)
	a2, b2 := cf(10, 101, "a-2.jar", "a2", true), cf(20, 201, "b-2.jar", "b2", false)
	for _, x := range []map[string]any{a1, b1} {
		f.routes[fmt.Sprintf("%s/mods/%d/files/%d", curseAPI, x["modId"], x["id"])] = jsonBytes(t, map[string]any{"data": x})
	}
	f.routes[curseAPI+"/mods/files"] = jsonBytes(t, map[string]any{"data": []any{a2, b2}})
	man := func(ver string, files ...[2]int) string {
		fl := []map[string]any{}
		for _, x := range files {
			fl = append(fl, map[string]any{"projectID": x[0], "fileID": x[1], "required": true})
		}
		m := map[string]any{"manifestType": "minecraftModpack", "manifestVersion": 1, "name": "CF Pack", "version": ver, "overrides": "overrides", "minecraft": map[string]any{"version": "1.21.1", "modLoaders": []any{}}, "files": fl}
		p := filepath.Join(t.TempDir(), ver+".zip")
		writeTestFile(t, p, zipBytes(t, map[string][]byte{"manifest.json": jsonBytes(t, m), "overrides/config/c.txt": []byte(ver)}))
		return p
	}
	r := importLocal(t, f, man("1", [2]int{10, 100}, [2]int{20, 200}), "CF")
	plan, e := f.s.PlanPackUpdate(context.Background(), r.ID, man("2", [2]int{10, 101}, [2]int{20, 201}), "")
	if e != nil {
		t.Fatal(e)
	}
	if len(plan.Manual) != 1 || plan.Manual[0].Reason != "kept-old" || plan.Manual[0].From != "mods/b-1.jar" {
		t.Fatalf("manual: %+v", plan.Manual)
	}
	if j := applyPlan(t, f.s, plan); j.State != "completed" {
		t.Fatal(j.Message)
	}
	if zzr(filepath.Join(r.Path, "mods/b-1.jar")) != "b1" || zzr(filepath.Join(r.Path, "mods/a-2.jar")) != "a2" {
		t.Fatal("old manual-only mod must stay until the new file is provided")
	}
	// player drops the manual file in: the audit retires the old copy
	writeTestFile(t, filepath.Join(r.Path, "mods/b-2.jar"), []byte("b2"))
	if _, e := f.s.CheckPack(context.Background(), r.ID); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(filepath.Join(r.Path, "mods/b-1.jar")); e == nil {
		t.Fatal("old copy not retired")
	}
	if l := readPack(r.Path); len(l.Manual) != 0 {
		t.Fatalf("manual flag not cleared: %+v", l.Manual)
	}
}

// Cancelling while the update is being applied restores the pack exactly.
func TestPackUpdateCancelRestores(t *testing.T) {
	f := newUpdFixture(t)
	before := treeSnapshot(t, f.root.Path)
	plan, e := f.s.PlanPackUpdate(context.Background(), f.root.ID, f.v2, "")
	if e != nil {
		t.Fatal(e)
	}
	idc := make(chan string, 1)
	applyFaultHook = func(n int) error {
		if n == 1 {
			id := <-idc
			_ = f.s.cancelJob(id)
		}
		return nil
	}
	defer func() { applyFaultHook = nil }()
	id, e := f.s.ApplyPackUpdate(plan.ID)
	if e != nil {
		t.Fatal(e)
	}
	idc <- id
	if j := waitJob(t, f.s, id); j.State != "cancelled" {
		t.Fatalf("want cancelled: %+v", j)
	}
	got := treeSnapshot(t, f.root.Path)
	for k, v := range before {
		if got[k] != v {
			t.Fatalf("%s = %q want %q after cancel", k, got[k], v)
		}
	}
	if len(got) != len(before) {
		t.Fatalf("tree differs after cancel restore")
	}
	if _, e := os.Stat(filepath.Join(f.root.Path, ".mcl", "update-staging")); !os.IsNotExist(e) {
		t.Fatal("staging left after cancel")
	}
}
