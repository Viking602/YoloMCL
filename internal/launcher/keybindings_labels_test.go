package launcher

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func bindingRow(t *testing.T, view KeyBindingsView, id string) KeyBindingRow {
	t.Helper()
	for _, row := range view.Rows {
		if row.ID == id {
			return row
		}
	}
	t.Fatalf("missing action %q", id)
	return KeyBindingRow{}
}

func readBindingLabels(t *testing.T, s *Service, r Root) KeyBindingsView {
	t.Helper()
	v, err := s.ReadKeyBindings(context.Background(), r.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestKeyBindingLabelsModernLegacyNestedAndPersistence(t *testing.T) {
	s := testService(t)
	r := newTestRoot(t, s, "translated")
	options := "key_key.forward:key.keyboard.w\nkey_key.jei.toggleOverlay:key.keyboard.o\nkey_quest.open:key.keyboard.g\nkey_key.deep.open:key.keyboard.d\nkey_key.old.open:key.keyboard.m\nkey_key.escape.open:key.keyboard.l\nkey_key.unknown:key.keyboard.u\nkey_key.disabled:key.keyboard.b\n"
	writeTestFile(t, filepath.Join(r.Path, "options.txt"), []byte(options))
	deep := zipBytes(t, map[string][]byte{
		"fabric.mod.json":             []byte(`{"id":"deep","name":"Nested Controls"}`),
		"assets/deep/lang/zh_cn.json": []byte(`{"key.deep.open":"嵌套菜单"}`),
	})
	child := zipBytes(t, map[string][]byte{"META-INF/jarjar/deep.jar": deep})
	writeTestFile(t, filepath.Join(r.Path, "mods", "jei.jar"), zipBytes(t, map[string][]byte{
		"fabric.mod.json":            []byte(`{"id":"jei","name":"Just Enough Items","jars":[{"file":"bundled/child.jar"}]}`),
		"bundled/child.jar":          child,
		"assets/jei/lang/zh_cn.json": []byte("\ufeff" + `{"key.jei.toggleOverlay":"§a显示配方\n列表","unrelated.item":"忽略"}`),
		"assets/jei/lang/en_us.json": []byte(`{"key.jei.toggleOverlay":"Toggle Overlay"}`),
		"assets/jei/lang/zh_cn.lang": []byte("key.jei.toggleOverlay=旧格式应被 JSON 覆盖"),
	}))
	writeTestFile(t, filepath.Join(r.Path, "mods", "old.jar"), zipBytes(t, map[string][]byte{
		"mcmod.info":                 []byte(`{"modList":[{"modid":"old","name":"Old Controls"}]}`),
		"assets/old/lang/zh_CN.lang": []byte("# comment\nkey.old.open=旧版=菜单\nquest.open=原样保留 \\u4e2d\n"),
		"assets/old/lang/en_US.lang": []byte("key.old.open=Legacy Menu"),
		"assets/old/lang/zh_TW.lang": []byte("#PARSE_ESCAPES\nkey.escape.open=\\u6253\\u5f00\\u5730\\u56fe"),
	}))
	writeTestFile(t, filepath.Join(r.Path, "mods", "off.jar.disabled"), zipBytes(t, map[string][]byte{"assets/off/lang/zh_cn.json": []byte(`{"key.disabled":"不应出现"}`)}))
	v := readBindingLabels(t, s, r)
	if v.LabelsIncomplete {
		t.Fatal("complete fixtures reported as incomplete")
	}
	for id, want := range map[string]string{"key_key.jei.toggleOverlay": "显示配方 列表", "key_quest.open": `原样保留 \u4e2d`, "key_key.deep.open": "嵌套菜单", "key_key.old.open": "旧版=菜单"} {
		if row := bindingRow(t, v, id); row.Names["zh_cn"] != want {
			t.Errorf("%s: %+v, want %q", id, row, want)
		}
	}
	if row := bindingRow(t, v, "key_key.jei.toggleOverlay"); row.Names["en_us"] != "Toggle Overlay" || row.ModName != "Just Enough Items" || row.Namespace != "jei" {
		t.Fatal(row)
	}
	if row := bindingRow(t, v, "key_key.deep.open"); row.ModName != "Nested Controls" {
		t.Fatal(row)
	}
	if row := bindingRow(t, v, "key_key.escape.open"); row.Names["zh_tw"] != "打开地图" {
		t.Fatal(row)
	}
	for _, id := range []string{"key_key.unknown", "key_key.disabled", "key_key.forward"} {
		if row := bindingRow(t, v, id); len(row.Names) != 0 {
			t.Fatal("unexpected translation", row)
		}
	}
	if got := zzr(filepath.Join(r.Path, "options.txt")); got != options {
		t.Fatal("label lookup modified options", got)
	}
	saveBindings(t, s, r.ID, map[string]string{"key_key.jei.toggleOverlay": "key.keyboard.j"})
	profile := zzr(filepath.Join(r.Path, ".mcl", "keybindings.json"))
	if strings.Contains(profile, "显示配方") || !strings.Contains(profile, "key_key.jei.toggleOverlay") {
		t.Fatal("translations leaked into profile identifiers", profile)
	}
	applyBindings(t, s, r.ID)
	if got := zzr(filepath.Join(r.Path, "options.txt")); got != strings.Replace(options, "key_key.jei.toggleOverlay:key.keyboard.o", "key_key.jei.toggleOverlay:key.keyboard.j", 1) {
		t.Fatal("translated edit changed unrelated controls", got)
	}
}

func TestKeyBindingLabelsActiveResourcePackOrderAndCache(t *testing.T) {
	s := testService(t)
	r := newTestRoot(t, s, "pack-order")
	writeTestFile(t, filepath.Join(r.Path, "mods", "jei.jar"), zipBytes(t, map[string][]byte{
		"fabric.mod.json":            []byte(`{"id":"jei","name":"JEI"}`),
		"assets/jei/lang/zh_cn.json": []byte(`{"key.jei.open":"模组原文"}`),
		"assets/jei/lang/en_us.json": []byte(`{"key.jei.open":"Open JEI"}`),
	}))
	for _, name := range []string{"low.zip", "inactive.zip", "off.zip.disabled"} {
		writeTestFile(t, filepath.Join(r.Path, "resourcepacks", name), zipBytes(t, map[string][]byte{"assets/translation/lang/zh_cn.json": jsonBytes(t, map[string]string{"key.jei.open": name})}))
	}
	folder := filepath.Join(r.Path, "resourcepacks", "high", "assets", "translation", "lang")
	writeTestFile(t, filepath.Join(folder, "zh_CN.LANG"), []byte("key.jei.open=旧版翻译"))
	writeTestFile(t, filepath.Join(folder, "zh_cn.json"), []byte(`{"key.jei.open":"高优先级"}`))
	writeTestFile(t, filepath.Join(folder, "en_us.json"), []byte(`{"key.jei.open":"Translated JEI"}`))
	options := "key_key.jei.open:key.keyboard.o\nresourcePacks:[\"vanilla\",\"mod_resources\",\"fabric\",\"programmer_art\",\"high_contrast\",\"Fabric Mods\",\"mod:extra\",\"custom:resource\",\"resource/custom\",\"Runtime resource pack\",\"file/low.zip\",\"file/high\"]\n"
	writeTestFile(t, filepath.Join(r.Path, "options.txt"), []byte(options))
	v := readBindingLabels(t, s, r)
	row := bindingRow(t, v, "key_key.jei.open")
	if v.LabelsIncomplete || row.Names["zh_cn"] != "高优先级" || row.Names["en_us"] != "Translated JEI" || row.ModName != "JEI" || row.Namespace != "jei" {
		t.Fatal(v, row)
	}
	row.Names["zh_cn"] = "caller mutation"
	if row = bindingRow(t, readBindingLabels(t, s, r), row.ID); row.Names["zh_cn"] != "高优先级" {
		t.Fatal("caller mutated the cache", row)
	}
	writeTestFile(t, filepath.Join(folder, "zh_cn.json"), []byte(`{"key.jei.open":"更新后的翻译"}`))
	if row = bindingRow(t, readBindingLabels(t, s, r), row.ID); row.Names["zh_cn"] != "更新后的翻译" {
		t.Fatal("stale directory-pack translation", row)
	}
	writeTestFile(t, filepath.Join(r.Path, "options.txt"), []byte(strings.Replace(options, "file/low.zip\",\"file/high", "file/high\",\"file/low.zip", 1)))
	if row = bindingRow(t, readBindingLabels(t, s, r), row.ID); row.Names["zh_cn"] != "low.zip" || row.Names["en_us"] != "Translated JEI" {
		t.Fatal("pack priority or per-language fallback changed", row)
	}
	// Legacy bare filenames and resource-pack disablement also invalidate.
	writeTestFile(t, filepath.Join(r.Path, "options.txt"), []byte("key_key.jei.open:key.keyboard.o\nresourcePacks:[\"low.zip\"]\n"))
	if row = bindingRow(t, readBindingLabels(t, s, r), row.ID); row.Names["zh_cn"] != "low.zip" || row.Names["en_us"] != "Open JEI" {
		t.Fatal(row)
	}
	writeTestFile(t, filepath.Join(r.Path, "options.txt"), []byte("key_key.jei.open:key.keyboard.o\n"))
	if row = bindingRow(t, readBindingLabels(t, s, r), row.ID); row.Names["zh_cn"] != "模组原文" {
		t.Fatal("disabled packs still translated", row)
	}
	writeTestFile(t, filepath.Join(r.Path, "resourcepacks", "vanilla", "assets", "jei", "lang", "zh_cn.json"), []byte(`{"key.jei.open":"外部资源包"}`))
	writeTestFile(t, filepath.Join(r.Path, "options.txt"), []byte("key_key.jei.open:key.keyboard.o\nresourcePacks:[\"vanilla\",\"file/vanilla\"]\n"))
	if row = bindingRow(t, readBindingLabels(t, s, r), row.ID); row.Names["zh_cn"] != "外部资源包" {
		t.Fatal("external pack confused with built-in pack", row)
	}
}

func TestKeyBindingLabelsModCacheInvalidationAndIsolation(t *testing.T) {
	s := testService(t)
	r := newTestRoot(t, s, "cached")
	path := filepath.Join(r.Path, "mods", "mod.jar")
	write := func(name string) {
		writeTestFile(t, path, zipBytes(t, map[string][]byte{"assets/mod/lang/zh_cn.json": jsonBytes(t, map[string]string{"key.mod.a": name, "key.mod.b": "第二个"})}))
	}
	write("第一个")
	options := "key_key.mod.a:key.keyboard.a\n"
	writeTestFile(t, filepath.Join(r.Path, "options.txt"), []byte(options))
	readBindingLabels(t, s, r)
	write("新名称")
	stamp := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if row := bindingRow(t, readBindingLabels(t, s, r), "key_key.mod.a"); row.Names["zh_cn"] != "新名称" {
		t.Fatal("stale changed JAR", row)
	}
	writeTestFile(t, filepath.Join(r.Path, "options.txt"), []byte(options+"key_key.mod.b:key.keyboard.b\n"))
	if row := bindingRow(t, readBindingLabels(t, s, r), "key_key.mod.b"); row.Names["zh_cn"] != "第二个" {
		t.Fatal("new action omitted by cache", row)
	}
	other := newTestRoot(t, s, "untranslated")
	writeTestFile(t, filepath.Join(other.Path, "options.txt"), []byte(options))
	if row := bindingRow(t, readBindingLabels(t, s, other), "key_key.mod.a"); len(row.Names) != 0 {
		t.Fatal("translations leaked to another pack", row)
	}
	readBindingLabels(t, s, r)
	if err := os.Rename(path, path+".disabled"); err != nil {
		t.Fatal(err)
	}
	if row := bindingRow(t, readBindingLabels(t, s, r), "key_key.mod.a"); len(row.Names) != 0 {
		t.Fatal("disabled mod still translated", row)
	}
}

func TestKeyBindingLabelsLoaderMetadataAndStoredNestedJar(t *testing.T) {
	s := testService(t)
	r := newTestRoot(t, s, "loaders")
	var options strings.Builder
	for _, fixture := range []struct{ id, manifest, body, want string }{
		{"quilt", "quilt.mod.json", `{"quilt_loader":{"id":"quilt","metadata":{"name":"Quilt Mod"}}}`, "Quilt Mod"},
		{"forge", "META-INF/mods.toml", "[[mods]]\nmodId='forge'\ndisplayName='Forge Mod'", "Forge Mod"},
		{"neo", "META-INF/neoforge.mods.toml", "[[mods]]\nmodId='neo'\ndisplayName='NeoForge Mod'", "NeoForge Mod"},
	} {
		fmt.Fprintf(&options, "key_key.%s.open:key.keyboard.o\n", fixture.id)
		writeTestFile(t, filepath.Join(r.Path, "mods", fixture.id+".jar"), zipBytes(t, map[string][]byte{fixture.manifest: []byte(fixture.body), "assets/" + fixture.id + "/lang/en_us.json": jsonBytes(t, map[string]string{"key." + fixture.id + ".open": "Open"})}))
	}
	inner := zipBytes(t, map[string][]byte{"assets/stored/lang/zh_cn.json": []byte(`{"key.stored":"存储的嵌套模组"}`)})
	var buffer bytes.Buffer
	w := zip.NewWriter(&buffer)
	f, err := w.CreateHeader(&zip.FileHeader{Name: "META-INF/jars/stored.jar", Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Write(inner); err != nil {
		t.Fatal(err)
	}
	if err = w.Close(); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(r.Path, "mods", "stored.jar"), buffer.Bytes())
	options.WriteString("key_key.stored:key.keyboard.s\n")
	writeTestFile(t, filepath.Join(r.Path, "options.txt"), []byte(options.String()))
	v := readBindingLabels(t, s, r)
	if v.LabelsIncomplete {
		t.Fatal(v)
	}
	for id, want := range map[string]string{"quilt": "Quilt Mod", "forge": "Forge Mod", "neo": "NeoForge Mod"} {
		if row := bindingRow(t, v, "key_key."+id+".open"); row.ModName != want {
			t.Fatal(row)
		}
	}
	if row := bindingRow(t, v, "key_key.stored"); row.Names["zh_cn"] != "存储的嵌套模组" {
		t.Fatal(row)
	}
}

func TestKeyBindingLabelsPartialResourcesAndCancellation(t *testing.T) {
	s := testService(t)
	r := newTestRoot(t, s, "partial")
	writeTestFile(t, filepath.Join(r.Path, "options.txt"), []byte("key_key.valid:key.keyboard.a\nkey_key.deep:key.keyboard.d\nresourcePacks:[\"file/../outside.zip\",\"file/missing.zip\"]\n"))
	writeTestFile(t, filepath.Join(r.Path, "mods", "broken.jar"), []byte("broken zip"))
	inner := zipBytes(t, map[string][]byte{"assets/deep/lang/zh_cn.json": []byte(`{"key.deep":"太深"}`)})
	for range 6 {
		inner = zipBytes(t, map[string][]byte{"META-INF/jars/child.jar": inner})
	}
	writeTestFile(t, filepath.Join(r.Path, "mods", "deep.jar"), inner)
	writeTestFile(t, filepath.Join(r.Path, "mods", "valid.jar"), zipBytes(t, map[string][]byte{
		"assets/valid/lang/en_us.json":    []byte(`{"key.valid":"Valid Action"}`),
		"assets/valid/lang/zh_cn.json":    []byte("{broken"),
		"assets/valid/lang/zh_tw.lang":    bytes.Repeat([]byte{'x'}, maxBindingLanguageBytes+1),
		"../assets/valid/lang/zh_cn.json": []byte(`{"key.valid":"非法路径"}`),
	}))
	v := readBindingLabels(t, s, r)
	if !v.LabelsIncomplete || bindingRow(t, v, "key_key.valid").Names["en_us"] != "Valid Action" || len(bindingRow(t, v, "key_key.deep").Names) != 0 {
		t.Fatal(v)
	}
	saveBindings(t, s, r.ID, map[string]string{"key_key.deep": "key.keyboard.unknown"})
	applyBindings(t, s, r.ID)
	if !strings.Contains(zzr(filepath.Join(r.Path, "options.txt")), "key_key.deep:key.keyboard.unknown") {
		t.Fatal("missing labels blocked editing")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.ReadKeyBindings(ctx, r.ID, ""); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation lost", err)
	}
}

func TestKeyBindingLabelsVersionedModsDirectory(t *testing.T) {
	dir := t.TempDir()
	for _, version := range []string{"1.12.2", "1.20.1"} {
		writeTestFile(t, filepath.Join(dir, "mods", version, "mod.jar"), []byte("fixture"))
	}
	sources, incomplete := bindingLabelSources(context.Background(), dir, "1.12.2", nil)
	if incomplete || len(sources) != 1 || sources[0].path != filepath.Join(dir, "mods", "1.12.2", "mod.jar") {
		t.Fatal(sources, incomplete)
	}
	sources, _ = bindingLabelSources(context.Background(), dir, "../outside", nil)
	if len(sources) != 0 {
		t.Fatal("unsafe version traversed", sources)
	}
}
