package launcher

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadIdentityResumeAndReuse(t *testing.T) {
	s := testService(t)
	var archive bytes.Buffer
	z := zip.NewWriter(&archive)
	f, _ := z.Create("fabric.mod.json")
	_, _ = io.WriteString(f, `{"id":"large","name":"Large","version":"1"}`)
	f, _ = z.CreateHeader(&zip.FileHeader{Name: "payload", Method: zip.Store})
	_, _ = f.Write(bytes.Repeat([]byte("abc \n\t\rdef"), 100000))
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	payload := archive.Bytes()
	sha, sha512 := hashes(payload)
	dest := filepath.Join(t.TempDir(), "large.jar")
	part := len(payload) / 3
	writeTestFile(t, dest+".mcl-part", payload[:part])
	requests := 0
	s.http = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		if r.Header.Get("Range") != fmt.Sprintf("bytes=%d-", part) {
			t.Error("missing resume range")
		}
		return &http.Response{StatusCode: http.StatusPartialContent, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(payload[part:])), Request: r}, nil
	})}
	want := DownloadFile{URL: "https://fixture.test/large.jar", Name: "large.jar", SHA1: sha, SHA512: sha512}
	var identity downloadIdentity
	ctx := context.Background()
	if err := s.downloadWithIdentity(ctx, want, dest, &identity); err != nil {
		t.Fatal(err)
	}
	if identity.SHA1 != sha || identity.Size != int64(len(payload)) || identity.Filtered != uint32(fingerprintLength(payload)) {
		t.Fatal("wrong resumed identity", identity)
	}
	w := beginModScan()
	var mod Mod
	gotSHA, fp, hasFP, err := w.readJarCached(ctx, dest, identity.Size, &mod, nil, &identity, nil)
	endModScan()
	if err != nil || gotSHA != sha || fp != curseFingerprint(payload) || !hasFP || mod.ModID != "large" {
		t.Fatal("identity changed metadata/fingerprint", gotSHA, fp, mod, err)
	}
	var reused downloadIdentity
	if err := s.downloadWithIdentity(ctx, want, dest, &reused); err != nil || reused != identity || requests != 1 {
		t.Fatal("complete staging file redownloaded", reused, requests, err)
	}
	if _, err := os.Stat(dest + ".mcl-part"); !os.IsNotExist(err) {
		t.Fatal("download part retained")
	}
}
