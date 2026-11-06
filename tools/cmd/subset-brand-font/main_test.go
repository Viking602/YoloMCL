package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/go-text/typesetting/font"
	"golang.org/x/image/font/sfnt"

	"github.com/Viking602/YoloMCL/tools/internal/toolutil"
)

func TestWordmarkMatchesBundledFont(t *testing.T) {
	root, err := toolutil.Root()
	if err != nil {
		t.Fatal(err)
	}
	bundled, err := os.ReadFile(filepath.Join(root, "internal/nativeui/assets/pixel.otf"))
	if err != nil {
		t.Fatal(err)
	}
	checkWordmark(t, bundled, bundled)
}

// The full upstream font stays outside the checkout. Set MCL_WORDMARK_SOURCE
// when updating it to also compare the complete source with the bundled face.
func TestUpstreamWordmark(t *testing.T) {
	path := os.Getenv("MCL_WORDMARK_SOURCE")
	if path == "" {
		t.Skip("set MCL_WORDMARK_SOURCE to the upstream Fusion Pixel OTF")
	}
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	root, err := toolutil.Root()
	if err != nil {
		t.Fatal(err)
	}
	bundled, err := os.ReadFile(filepath.Join(root, "internal/nativeui/assets/pixel.otf"))
	if err != nil {
		t.Fatal(err)
	}
	checkWordmark(t, source, bundled)
}

func checkWordmark(t *testing.T, source, bundled []byte) {
	t.Helper()
	result, err := subset(source)
	if err != nil {
		t.Fatal(err)
	}
	again, err := subset(source)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(result, again) {
		t.Fatal("font output is not deterministic")
	}
	if len(result) > 8<<10 {
		t.Fatalf("wordmark should stay small: %d bytes", len(result))
	}
	if checksum(result) != 0xb1b0afba {
		t.Fatal("invalid OpenType checksum")
	}
	parsed, err := sfnt.Parse(result)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.NumGlyphs() != 8 {
		t.Fatalf("got %d glyphs, want .notdef and seven characters", parsed.NumGlyphs())
	}
	var buffer sfnt.Buffer
	for _, r := range "A汉" {
		id, err := parsed.GlyphIndex(&buffer, r)
		if err != nil || id != 0 {
			t.Fatalf("unexpected glyph %q: %d %v", r, id, err)
		}
	}
	got, err := font.ParseTTF(bytes.NewReader(result))
	if err != nil {
		t.Fatal(err)
	}
	for _, reference := range [][]byte{source, bundled} {
		want, err := font.ParseTTF(bytes.NewReader(reference))
		if err != nil {
			t.Fatal(err)
		}
		if got.Upem() != want.Upem() {
			t.Fatal("units per em changed")
		}
		for _, r := range characters {
			id, ok := got.NominalGlyph(r)
			if !ok {
				t.Fatalf("missing %q", r)
			}
			original, ok := want.NominalGlyph(r)
			if !ok {
				t.Fatalf("reference missing %q", r)
			}
			if got.HorizontalAdvance(id) != want.HorizontalAdvance(original) {
				t.Fatalf("advance changed for %q", r)
			}
			a, ok := got.GlyphDataOutline(id)
			if !ok {
				t.Fatalf("unreadable output %q", r)
			}
			b, ok := want.GlyphDataOutline(original)
			if !ok {
				t.Fatalf("unreadable reference %q", r)
			}
			if len(a.Segments) != len(b.Segments) {
				t.Fatalf("outline length changed for %q: %d != %d", r, len(a.Segments), len(b.Segments))
			}
			for i := range a.Segments {
				if a.Segments[i].Op != b.Segments[i].Op || !reflect.DeepEqual(a.Segments[i].ArgsSlice(), b.Segments[i].ArgsSlice()) {
					t.Fatalf("outline changed for %q at segment %d", r, i)
				}
			}
		}
	}
	before, err := readTables(source)
	if err != nil {
		t.Fatal(err)
	}
	after, err := readTables(result)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before["head"][20:36], after["head"][20:36]) {
		t.Fatal("timestamps changed")
	}
	if !bytes.Equal(before["hhea"][:34], after["hhea"][:34]) {
		t.Fatal("line metrics changed")
	}
	if !bytes.Equal(before["OS/2"][8:10], after["OS/2"][8:10]) {
		t.Fatal("embedding permissions changed")
	}
	// Compare every unrenamed multilingual name byte-for-byte, including the
	// copyright, license and attribution. Names 1/4/6/16 are checked separately.
	oldNames, newNames := nameRecords(t, before["name"]), nameRecords(t, after["name"])
	for key, value := range oldNames {
		switch be.Uint16([]byte(key)[6:8]) {
		case 1, 2, 3, 4, 6, 16, 17:
			continue
		}
		if !bytes.Equal(value, newNames[key]) {
			t.Fatalf("name metadata changed: %x", key)
		}
	}
	for id, want := range map[sfnt.NameID]string{sfnt.NameIDFamily: familyName, sfnt.NameIDFull: familyName, sfnt.NameIDPostScript: postScriptName} {
		name, err := parsed.Name(&buffer, id)
		if err != nil || name != want {
			t.Fatalf("name %d = %q, %v", id, name, err)
		}
	}
}

func nameRecords(t *testing.T, table []byte) map[string][]byte {
	t.Helper()
	count, base := int(be.Uint16(table[2:4])), int(be.Uint16(table[4:6]))
	result := map[string][]byte{}
	for i := 0; i < count; i++ {
		r := table[6+i*12 : 6+(i+1)*12]
		offset, size := base+int(be.Uint16(r[10:12])), int(be.Uint16(r[8:10]))
		result[string(r[:8])] = table[offset : offset+size]
	}
	return result
}

func TestInvalidFontDoesNotReplaceOutput(t *testing.T) {
	dir := t.TempDir()
	source, out := filepath.Join(dir, "bad.otf"), filepath.Join(dir, "result.otf")
	if err := os.WriteFile(source, []byte("broken font"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, []byte("previous output"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--output", out, source}); err == nil {
		t.Fatal("accepted invalid font")
	}
	got, err := os.ReadFile(out)
	if err != nil || string(got) != "previous output" {
		t.Fatalf("existing output changed: %q %v", got, err)
	}
}
