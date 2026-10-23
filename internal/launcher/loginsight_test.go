package launcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDiagnoseCrash(t *testing.T) {
	cases := []struct{ text, kind, mod string }{
		{"Exception in thread \"main\" java.lang.OutOfMemoryError: Java heap space", "memory", ""},
		{"java.lang.UnsupportedClassVersionError: x has been compiled by a more recent version of the Java Runtime (class file version 65.0)", "java", ""},
		{"Missing or unsupported mandatory dependencies:\n\tMod ID: 'geckolib', Requested by: 'alexsmobs', Expected range: '[4.0,)'", "dependency", "alexsmobs"},
		{"org.spongepowered.asm.mixin.transformer.throwables.MixinTransformerError: Mixin apply failed\n... from mod create_extra", "mixin", "create_extra"},
		{"Caught exception from Create (create)\njava.lang.NullPointerException", "mod", "Create"},
		{"Process exited with code 1", "unknown", ""},
	}
	for _, c := range cases {
		got := DiagnoseCrash(c.text)
		if got.Kind != c.kind || got.Mod != c.mod || got.Cause == "" || got.CauseEn == "" {
			t.Errorf("%q: got %+v", c.text[:20], got)
		}
	}
	if a := DiagnoseCrash("class file version 65.0 UnsupportedClassVersionError").Advice; a != "在版本设置里换用 Java 21 或更高" {
		t.Fatal(a)
	}
}

func TestFindCrashReport(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "crash-reports"), 0o755)
	old := filepath.Join(dir, "crash-reports", "crash-old.txt")
	os.WriteFile(old, []byte("x"), 0o644)
	os.Chtimes(old, time.Now().Add(-time.Hour), time.Now().Add(-time.Hour))
	start := time.Now()
	if findCrashReport(dir, start) != "" {
		t.Fatal("old report picked")
	}
	nw := filepath.Join(dir, "crash-reports", "crash-new.txt")
	os.WriteFile(nw, []byte("y"), 0o644)
	if findCrashReport(dir, start) != nw {
		t.Fatal("new report missed")
	}
}
