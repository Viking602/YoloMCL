package launcher

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParsePropsJavaEscapes(t *testing.T) {
	b := []byte("# Iris\r\nshaderPack=[\\u8D85\\u7F8E\\!\\u661F\\u7A7A]\\ Pack\\:v2.zip\r\nenableShaders=true\nlong=a\\\n  b\n")
	p := parseProps(b)
	if got := p["shaderPack"]; got != "[超美!星空] Pack:v2.zip" {
		t.Fatalf("shaderPack = %q", got)
	}
	if p["enableShaders"] != "true" || p["long"] != "ab" {
		t.Fatalf("props %q", p)
	}
	if got := propUnescape(`C:\\x\u00e9`); got != `C:\xé` {
		t.Fatalf("%q", got)
	}
}

func TestReadGameOptions(t *testing.T) {
	dir := t.TempDir()
	st, err := readGameOptions(context.Background(), dir)
	if err != nil || st.HasOptions || st.ShaderConfig != "" {
		t.Fatalf("missing options: %+v, %v", st, err)
	}
	writeTestFile(t, filepath.Join(dir, "config", "iris.properties"), []byte("shaderPack=Iris.zip\nenableShaders=true"))
	writeTestFile(t, filepath.Join(dir, "optionsshaders.txt"), []byte("shaderPack=OptiFine.zip"))
	writeTestFile(t, filepath.Join(dir, "options.txt"), []byte("resourcePacks:[\"vanilla\",\"file/base.zip\",\"file/top.zip\"]"))
	st, err = readGameOptions(context.Background(), dir)
	if err != nil || st.ShaderConfig != "iris" || st.ShaderPack != "Iris.zip" || !st.ShadersOn || !st.HasOptions {
		t.Fatalf("options: %+v, %v", st, err)
	}
	if !reflect.DeepEqual(st.ResourcePackOrder, []string{"top.zip", "base.zip"}) {
		t.Fatalf("priority order: %v", st.ResourcePackOrder)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := readGameOptions(ctx, dir); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestReadGameOptionsRejectsMalformedAndOversizedFiles(t *testing.T) {
	for _, data := range [][]byte{[]byte("resourcePacks:invalid JSON"), make([]byte, maxGameOptionsSize+1)} {
		dir := t.TempDir()
		writeTestFile(t, filepath.Join(dir, "options.txt"), data)
		if _, err := readGameOptions(context.Background(), dir); err == nil {
			t.Fatal("invalid options accepted")
		}
	}
}

func TestResourcePackFileEntry(t *testing.T) {
	cases := []struct {
		id   string
		want string
		ok   bool
	}{
		{"file/top.zip", "top.zip", true},
		{"file/", "", false},
		{"vanilla", "", false},
		{"mod_resources", "", false},
		{"fabric", "", false},
		{"builtin/paster_vanilla_ui", "", false},
		{"netherexp:jne_retextures", "", false},
		{"Better-Leaves-9.0.zip", "Better-Leaves-9.0.zip", true},
		{"My Folder Pack", "My Folder Pack", true},
		{"Moonlight Mods Dynamic Assets", "Moonlight Mods Dynamic Assets", true},
	}
	for _, c := range cases {
		got, ok := resourcePackFileEntry(c.id)
		if ok != c.ok || got != c.want {
			t.Fatalf("%q: got %q,%v want %q,%v", c.id, got, ok, c.want, c.ok)
		}
	}
}

func TestReadGameOptionsBarePackNames(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "options.txt"), []byte(`resourcePacks:["vanilla","mod_resources","base.zip","file/top.zip"]`))
	st, err := readGameOptions(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(st.ResourcePackOrder, []string{"top.zip", "base.zip"}) {
		t.Fatalf("order %v", st.ResourcePackOrder)
	}
}

func TestResourcePackEmbeddedEntry(t *testing.T) {
	cases := []struct {
		id   string
		want string
		ok   bool
	}{
		{"netherexp:jne_retextures", "netherexp:jne_retextures", true},
		{"file/top.zip", "", false},
		{"vanilla", "", false},
		{"modid:", "", false},
		{":pack", "", false},
		{"a:b/c", "", false},
		{"builtin/x", "", false},
	}
	for _, c := range cases {
		got, ok := resourcePackEmbeddedEntry(c.id)
		if ok != c.ok || got != c.want {
			t.Fatalf("%q: got %q,%v want %q,%v", c.id, got, ok, c.want, c.ok)
		}
	}
}

func TestReadGameOptionsEmbeddedPacks(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "options.txt"), []byte(
		`resourcePacks:["vanilla","file/base.zip","netherexp:jne_retextures","file/top.zip"]`,
	))
	st, err := readGameOptions(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"top.zip", "netherexp:jne_retextures", "base.zip"}
	if !reflect.DeepEqual(st.ResourcePackOrder, want) {
		t.Fatalf("order %v want %v", st.ResourcePackOrder, want)
	}
}
