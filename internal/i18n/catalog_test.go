package i18n

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
)

func TestBundledCatalogsComplete(t *testing.T) {
	type source struct {
		Copy        map[string]string `json:"copy"`
		Messages    map[string]string `json:"messages"`
		JobMessages map[string]string `json:"jobMessages"`
		Phases      map[string]string `json:"phases"`
	}
	var sources []source
	for _, code := range []string{LangEN, LangZH} {
		path := "locales/" + code + ".json"
		b, err := localeFiles.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(code, func(t *testing.T) {
			checkJSONKeys(t, json.NewDecoder(strings.NewReader(string(b))))
			if _, err := loadCatalog(localeFiles, path, code, nil); err != nil {
				t.Fatal(err)
			}
		})
		var s source
		if err := json.Unmarshal(b, &s); err != nil {
			t.Fatal(err)
		}
		typ := reflect.TypeFor[Copy]()
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if field.Tag.Get("json") == "-" {
				continue
			}
			if _, ok := s.Copy[field.Name]; !ok {
				t.Errorf("%s: missing copy.%s", code, field.Name)
			}
		}
		sources = append(sources, s)
	}
	// Compare raw JSON, before fallback can hide an incomplete translation.
	checkTranslationPairs(t, "copy", sources[0].Copy, sources[1].Copy)
	checkTranslationPairs(t, "messages", sources[0].Messages, sources[1].Messages)
	checkTranslationPairs(t, "jobMessages", sources[0].JobMessages, sources[1].JobMessages)
	checkTranslationPairs(t, "phases", sources[0].Phases, sources[1].Phases)
}

// Duplicate object keys are otherwise silently accepted by encoding/json.
func checkJSONKeys(t *testing.T, d *json.Decoder) {
	t.Helper()
	value, err := d.Token()
	if err != nil {
		t.Fatal(err)
	}
	switch value {
	case json.Delim('{'):
		seen := map[string]bool{}
		for d.More() {
			token, err := d.Token()
			if err != nil {
				t.Fatal(err)
			}
			key := token.(string)
			if seen[key] {
				t.Errorf("duplicate JSON key %q", key)
			}
			seen[key] = true
			checkJSONKeys(t, d)
		}
		if _, err := d.Token(); err != nil {
			t.Fatal(err)
		}
	case json.Delim('['):
		for d.More() {
			checkJSONKeys(t, d)
		}
		if _, err := d.Token(); err != nil {
			t.Fatal(err)
		}
	}
}

func checkTranslationPairs(t *testing.T, section string, en, zh map[string]string) {
	t.Helper()
	// All bundled formats use sequential arguments. Check types, precision and
	// literal %% so a translated template cannot corrupt a formatted UI value.
	format := regexp.MustCompile(`%(?:\[[0-9]+\])?[+\-#0]*[0-9]*(?:\.[0-9]+)?[a-zA-Z%]`)
	for key, text := range en {
		translated, ok := zh[key]
		if !ok {
			t.Errorf("zh-CN missing %s.%s", section, key)
			continue
		}
		if !slices.Equal(format.FindAllString(text, -1), format.FindAllString(translated, -1)) {
			t.Errorf("%s.%s: incompatible placeholders in %q / %q", section, key, text, translated)
		}
	}
	for key := range zh {
		if _, ok := en[key]; !ok {
			t.Errorf("en-US missing %s.%s", section, key)
		}
	}
}

func TestLoadCatalogFallbackAndExplicitEmpty(t *testing.T) {
	files := fstest.MapFS{"custom.json": {Data: []byte(`{
		"code":"zh-CN",
		"copy":{"AppTitle":"Custom title","AppSubtitle":"","DiscoverSortDownloads":"Custom downloads"},
		"messages":{"count.mods":"","custom.message":"Custom message"}
	}`)}}
	c, err := loadCatalog(files, "custom.json", LangZH, &english)
	if err != nil {
		t.Fatal(err)
	}
	if c.Copy.Code != LangZH || c.Copy.AppTitle != "Custom title" || c.Copy.AppSubtitle != "" || c.Copy.NavHome != english.Copy.NavHome {
		t.Fatal("copy overrides, explicit empties or English fallback were lost")
	}
	if c.Messages["count.mods"] != "" || c.Messages["custom.message"] != "Custom message" || c.Messages["job.state.running"] != english.Messages["job.state.running"] {
		t.Fatal("message overrides, explicit empties or English fallback were lost")
	}
	if c.labels["sort"]["custom downloads"] != "downloads" {
		t.Fatal("configured selector label was not recognized")
	}
	if english.Messages["count.mods"] == "" || english.Copy.AppTitle == c.Copy.AppTitle {
		t.Fatal("loading another language mutated the fallback catalog")
	}
	c.Categories["test"] = "test"
	if _, ok := english.Categories["test"]; ok {
		t.Fatal("category fallback aliases the original catalog")
	}
}

func TestLoadCatalogRejectsInvalidData(t *testing.T) {
	for _, data := range []string{
		`{`, `null`, `[]`, `{"code":"en-US"}`, `{"code":"zh-CN"} {}`,
		`{"code":"zh-CN","unknown":true}`,
		`{"code":"zh-CN","copy":{"MisspelledTitle":"x"}}`,
		`{"code":"zh-CN","messages":{"key":1}}`,
		`{"code":"zh-CN","messages":null}`,
		`{"code":"zh-CN","jobMessages":{"":"catch all"}}`,
		`{"code":"zh-CN","numberUnits":[{"size":0,"name":"X"}]}`,
		`{"code":"zh-CN","numberUnits":[{"size":100,"name":"X"},{"size":1000,"name":"Y"}]}`,
	} {
		t.Run(data, func(t *testing.T) {
			files := fstest.MapFS{"broken.json": {Data: []byte(data)}}
			if _, err := loadCatalog(files, "broken.json", LangZH, &english); err == nil || !strings.Contains(err.Error(), "broken.json") {
				t.Fatalf("expected a contextual configuration error, got %v", err)
			}
		})
	}
	if _, err := loadCatalog(fstest.MapFS{}, "missing.json", LangEN, nil); err == nil {
		t.Fatal("missing JSON did not report an error")
	}
}

func TestMessageLookupAndProgress(t *testing.T) {
	if Text(true, "missing.key") != "missing.key" {
		t.Fatal("unknown key must stay visible")
	}
	if text, ok := Lookup(true, "worlds_ui.text"); !ok || text != "" {
		t.Fatalf("intentional empty English prefix was lost: %q, %v", text, ok)
	}
	if got := fmt.Sprintf(Plural(true, "time.ago.minute", 1), 1); got != "1 minute ago" {
		t.Fatal(got)
	}
	if got := fmt.Sprintf(Plural(true, "time.ago.minute", 2), 2); got != "2 minutes ago" {
		t.Fatal(got)
	}
	for _, tc := range []struct{ input, want string }{
		{"下载整合包文件 (15/71): 中文.jar", "Downloading pack files (15/71): 中文.jar"},
		{"安装 项目 #42", "Install project #42"},
		{"来源查询不可用，未识别文件将随包打包", "Provider lookup unavailable; unidentified files will be bundled"},
		{"unknown status: file.jar", "unknown status: file.jar"},
	} {
		if got := JobMessage(true, tc.input); got != tc.want {
			t.Errorf("JobMessage(%q) = %q, want %q", tc.input, got, tc.want)
		}
		if got := JobMessage(false, tc.input); got != tc.input {
			t.Errorf("Chinese progress changed: %q", got)
		}
	}
	if Phase(true, "扫描已停止") != "Scan stopped" || Phase(true, "unknown phase") != "unknown phase" {
		t.Fatal("scan phase lookup or fallback failed")
	}
	if TranslateCategory(false, " Technology ") != "科技工业" || TranslateCategory(false, "API and Library") != "前置库" || TranslateCategory(true, " Technology ") != "Technology" || TranslateCategory(false, "New Category") != "New Category" {
		t.Fatal("category translation precedence or fallback changed")
	}
}

// A misspelled literal key should fail CI instead of appearing in the UI.
func TestReferencedMessageKeysExist(t *testing.T) {
	files, err := filepath.Glob("../nativeui/*.go")
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, "../launcher/loginsight.go")
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			index, plural, crash := -1, false, false
			switch fun := call.Fun.(type) {
			case *ast.SelectorExpr:
				if fun.Sel.Name == "L" {
					index = 0
				} else if pkg, ok := fun.X.(*ast.Ident); ok && pkg.Name == "i18n" {
					switch fun.Sel.Name {
					case "Text", "Lookup":
						index = 1
					case "Plural":
						index, plural = 1, true
					}
				}
			case *ast.Ident:
				switch fun.Name {
				case "L", "crashText":
					index = 0
				case "newCrashInsight":
					index, crash = 0, true
				}
			}
			if index < 0 || index >= len(call.Args) {
				return true
			}
			literal, ok := call.Args[index].(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true // Dynamic families are covered by their feature tests.
			}
			key, err := strconv.Unquote(literal.Value)
			if err != nil {
				t.Fatal(err)
			}
			keys := []string{key}
			if plural {
				keys = []string{key + ".one", key + ".other"}
			} else if crash {
				keys = []string{"crash." + key + ".cause", "crash." + key + ".advice"}
			}
			for _, key := range keys {
				if _, ok := Lookup(true, key); !ok {
					t.Errorf("%s: unknown message key %q", fset.Position(call.Pos()), key)
				}
			}
			return true
		})
	}
}

func TestCatalogIsIndependentOfWorkingDirectory(t *testing.T) {
	t.Chdir(t.TempDir())
	if got := Text(true, "job.state.running"); got != "In Progress" {
		t.Fatal(got)
	}
	f, err := localeFiles.Open("locales/zh-CN.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := io.Copy(io.Discard, f); err != nil {
		t.Fatal(err)
	}
}
