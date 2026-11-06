package modjar

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"reflect"
	"testing"
)

func legacyModClass(id, version, annotationName string) []byte {
	c := &fixtureClass{}
	data := words(1, c.utf(annotationName), 2, c.utf("modid"))
	data = append(data, 's')
	data = append(data, words(c.utf(id))...)
	data = append(data, words(c.utf("version"))...)
	data = append(data, 's')
	data = append(data, words(c.utf(version))...)
	attr := words(c.utf("RuntimeVisibleAnnotations"))
	attr = binary.BigEndian.AppendUint32(attr, uint32(len(data)))
	c.attrs = append(c.attrs, append(attr, data...))
	return c.bytes("sample/Main", "java/lang/Object")
}

func TestLegacyForgeDiscoversAnnotationsWithOptionalMetadata(t *testing.T) {
	for _, mc := range []string{"1.7.10", "1.12.2"} {
		p, _ := SelectLoader("forge", mc, "")
		annotationName := "Lnet/minecraftforge/fml/common/Mod;"
		if mc == "1.7.10" {
			annotationName = "Lcpw/mods/fml/common/Mod;"
		}
		for _, tc := range []struct{ name, annotationVersion, metadata, properties, want string }{
			{"annotation only", "2.1", "", "", "2.1"},
			{"optional malformed metadata", "2.1", "[broken", "", "2.1"},
			{"no version", "", "", "", "1.0"},
			{"lenient metadata", "", `[{modid:'sample',version:'3.0',authors:[first,second]},{modid:'ghost',version:99}]`, "", "3.0"},
			{"version properties before metadata", "", `[{"modid":"sample","version":"3.0"}]`, "sample.version = 4.\\\n  5\n", "4.5"},
			{"annotation before properties", "2.1", `[{"modid":"sample","version":"3.0"}]`, "sample.version=4.5", "2.1"},
		} {
			t.Run(mc+"/"+tc.name, func(t *testing.T) {
				files := map[string][]byte{"sample/Main.class": legacyModClass("sample", tc.annotationVersion, annotationName)}
				if tc.metadata != "" {
					files["mcmod.info"] = []byte(tc.metadata)
				}
				if tc.properties != "" {
					files["version.properties"] = []byte(tc.properties)
				}
				mods, err := LegacyMods(context.Background(), fixtureArchive(t, files), p)
				if err != nil || len(mods) != 1 || mods[0].ID != "sample" || mods[0].Version != tc.want {
					t.Fatalf("%+v %v", mods, err)
				}
			})
		}
		mods, err := LegacyMods(context.Background(), fixtureArchive(t, map[string][]byte{"mcmod.info": []byte(`[{"modid":"ghost","version":"1"}]`)}), p)
		if err != nil || len(mods) != 0 {
			t.Fatalf("metadata activated a nonexistent mod: %+v %v", mods, err)
		}
	}
}

func TestLegacyMetadataGsonGrammar(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{`{name:'quoted',authors:[one,two]}`, `{"authors":["one","two"],"name":"quoted"}`},
		{"[\"raw\nline\"," + `'quote\'s',TRUE,False,NULL,1.2e3]`, `["raw\nline","quote's",true,false,null,1.2e3]`},
		{"\xef\xbb\xbf)]}'\n{a=1; b=>[;2,] # comment\n}", `{"a":1,"b":[null,2,null]}`},
		{`{/*comment*/a:1,a:2}`, `{"a":2}`},
		{`[]`, `[]`},
	} {
		got, err := legacyJSON([]byte(tc.in))
		encoded, _ := json.Marshal(got)
		if err != nil || string(encoded) != tc.want {
			t.Errorf("%q => %s %v; want %s", tc.in, encoded, err, tc.want)
		}
	}
	for _, in := range []string{`{a:1,}`, `{a:"bad\x"}`, `[1] extra`, `/*`, `{"unterminated}`} {
		if _, err := legacyJSON([]byte(in)); err == nil {
			t.Errorf("accepted %q", in)
		}
	}
}

func TestJavaPropertiesVersionEncoding(t *testing.T) {
	got, err := readJavaProperties([]byte("# ignored\nkey\\ with\\ space : one\\\r\n   two\r\nlatin=\xe9\nescaped=\\u4e2d\\u6587\nemoji=\\ud83d\\ude00\nliteral=\\z\nkey\\ with\\ space=last\n"))
	want := map[string]string{"key with space": "last", "latin": "é", "escaped": "中文", "emoji": "😀", "literal": "z"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := readJavaProperties([]byte(`x=\uXX00`)); err == nil {
		t.Fatal("invalid Unicode escape accepted")
	}
}
