package structure3d

import (
	"archive/zip"
	"bytes"
)

func zipWith(parts ...any) []byte {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for i := 0; i+1 < len(parts); i += 2 {
		name := parts[i].(string)
		data := parts[i+1].([]byte)
		f, _ := w.Create(name)
		_, _ = f.Write(data)
	}
	_ = w.Close()
	return buf.Bytes()
}
