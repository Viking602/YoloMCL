package launcher

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveKindAndPreviews(t *testing.T) {
	t.Setenv("CURSEFORGE_API_KEY", "")
	dir := t.TempDir()
	s, e := New(dir)
	if e != nil {
		t.Fatal(e)
	}
	mmcPack, _ := json.Marshal(map[string]any{"components": []map[string]string{{"uid": "net.minecraft", "version": "1.20.1"}, {"uid": "net.fabricmc.fabric-loader", "version": "0.16.14"}}})
	cf, _ := json.Marshal(map[string]any{"manifestType": "minecraftModpack", "manifestVersion": 1, "name": "CF", "minecraft": map[string]any{"version": "1.21.1", "modLoaders": []map[string]any{{"id": "fabric-0.16.14", "primary": true}}}, "files": []map[string]any{{"projectID": 1, "fileID": 2, "required": true}}, "overrides": "overrides"})
	zips := map[string]map[string][]byte{
		"mmc.zip":    {"My Inst/instance.cfg": []byte("InstanceType=OneSix\nname=我的实例\n"), "My Inst/mmc-pack.json": mmcPack, "My Inst/.minecraft/mods/a.jar": []byte("x"), "My Inst/.minecraft/options.txt": []byte("lang:zh_cn")},
		"cf.zip":     {"manifest.json": cf},
		"rp.zip":     {"pack.mcmeta": []byte(`{"pack":{"pack_format":34}}`), "assets/minecraft/textures/a.png": []byte("x")},
		"rp2.zip":    {"Faithful/pack.mcmeta": []byte(`{}`)},
		"shader.zip": {"shaders/composite.fsh": []byte("x")},
		"random.zip": {"readme.txt": []byte("hi"), "pics/a.png": []byte("x")},
		"badmmc.zip": {"instance.cfg": []byte("name=x"), "mmc-pack.json": []byte("{")},
	}
	want := map[string]string{"mmc.zip": "modpack", "cf.zip": "modpack", "rp.zip": "resourcepack", "rp2.zip": "resourcepack", "shader.zip": "shader", "random.zip": "", "badmmc.zip": ""}
	for name, files := range zips {
		p := filepath.Join(dir, name)
		writeTestFile(t, p, zipBytes(t, files))
		kind, e := s.ArchiveKind(p)
		if kind != want[name] {
			t.Errorf("%s: kind %q err %v, want %q", name, kind, e, want[name])
		}
		if want[name] == "" && e == nil {
			t.Errorf("%s: unknown zips must explain themselves", name)
		}
	}
	if _, e := s.ArchiveKind(filepath.Join(dir, "random.zip")); e == nil || !strings.Contains(e.Error(), "无法识别") {
		t.Errorf("random zip error: %v", e)
	}
	mmc, e := s.InspectPack(filepath.Join(dir, "mmc.zip"))
	if e != nil || mmc.Source != "multimc" || mmc.Minecraft != "1.20.1" || mmc.Loader != "fabric" || mmc.LoaderVersion != "0.16.14" || mmc.Name != "我的实例" || mmc.Unavailable != "" || mmc.DownloadMB < 600 {
		t.Errorf("multimc preview: %+v %v", mmc, e)
	}
	cfp, e := s.InspectPack(filepath.Join(dir, "cf.zip"))
	if e != nil || cfp.Unavailable == "" {
		t.Errorf("CurseForge without API must be flagged: %+v %v", cfp, e)
	}
	s.distribution.CurseForgeAPIKey = "test-key"
	if cfp, _ := s.InspectPack(filepath.Join(dir, "cf.zip")); cfp.Unavailable != "" {
		t.Error("configured CurseForge must stay importable")
	}
}
