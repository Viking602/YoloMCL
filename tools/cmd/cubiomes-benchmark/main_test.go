package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReferenceHashAgreement(t *testing.T) {
	want := []byte(`{"overworld":"0123","nether":"abcd"}`)
	if err := verifyHashes([]byte("\xef\xbb\xbf"+string(want)), want); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{`{"overworld":"0123"}`, `{"overworld":"0123","nether":"wrong"}`, `null`, `broken`} {
		if err := verifyHashes([]byte(bad), want); err == nil {
			t.Fatalf("accepted incorrect hashes: %s", bad)
		}
	}
}

func TestFailedFetchLeavesReferencesUntouched(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, referenceFiles[0])
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if !strings.HasPrefix(r.URL.Path, "/"+revision+"/") {
			t.Error("download is not pinned to the reference revision")
		}
		if requests == 2 {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		if _, err := w.Write([]byte("downloaded")); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	if err := fetchReference(server.Client(), server.URL+"/"+revision+"/", dir); err == nil {
		t.Fatal("fetch failure was ignored")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "original" {
		t.Fatalf("reference overwritten on partial fetch: %q %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "reference-revision.txt")); !os.IsNotExist(err) {
		t.Fatal("marked an incomplete download as complete")
	}
}

func TestBaselineUsesSameBenchmarkSelection(t *testing.T) {
	result := jobs("c-reference", "before.test.exe", 3)
	if result[0].name != "before.txt" {
		t.Fatal("baseline missing")
	}
	for _, job := range result {
		if job.name != "before.txt" && job.name != "go.txt" {
			continue
		}
		args := strings.Join(job.command, " ")
		for _, want := range []string{benchmarkFilter, "benchtime=500ms", "count=3", "benchmem"} {
			if !strings.Contains(args, want) {
				t.Fatalf("%s missing %s", job.name, want)
			}
		}
	}
}
