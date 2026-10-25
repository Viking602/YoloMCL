package launcher

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestReadZipBytesValidatesSizeAndChecksum(t *testing.T) {
	for _, method := range []uint16{zip.Store, zip.Deflate} {
		for _, data := range [][]byte{nil, []byte("embedded artwork or nested jar")} {
			var buf bytes.Buffer
			writer := zip.NewWriter(&buf)
			entry, err := writer.CreateHeader(&zip.FileHeader{Name: "asset", Method: method})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := entry.Write(data); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
			if err != nil {
				t.Fatal(err)
			}
			file := reader.File[0]
			got, err := readZipBytes(file, 64)
			if err != nil || !bytes.Equal(got, data) {
				t.Fatalf("read: %q, %v", got, err)
			}
			file.CRC32 ^= 1
			if _, err := readZipBytes(file, 64); err == nil {
				t.Fatal("corrupt checksum was accepted")
			}
			file.CRC32 ^= 1
			file.UncompressedSize64++
			if _, err := readZipBytes(file, 64); err == nil {
				t.Fatal("truncated content was accepted")
			}
			file.UncompressedSize64 = 65
			if _, err := readZipBytes(file, 64); err == nil {
				t.Fatal("oversized entry was allocated")
			}
		}
	}
}

func TestModIconSelectsAdequateResolution(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{`"icon.png"`, "icon.png"},
		{`{"32":"small.png","128":"icon.png","2048":"large.png"}`, "icon.png"},
		{`{"16":"small.png","64":"icon.png"}`, "icon.png"},
		{`{"256":"icon.png","1024":"large.png"}`, "icon.png"},
	} {
		if got := iconField([]byte(tc.raw)); got != tc.want {
			t.Fatalf("%s: %s, want %s", tc.raw, got, tc.want)
		}
	}
}
