package launcher

import (
	"archive/zip"
	"compress/flate"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func openPackExportFile(dir *os.Root, f packExportFile) (*os.File, error) {
	in, err := dir.Open(f.path)
	if err != nil {
		return nil, err
	}
	if err := checkPackExportFile(in, f); err != nil {
		in.Close()
		return nil, err
	}
	return in, nil
}

func checkPackExportFile(in *os.File, f packExportFile) error {
	st, err := in.Stat()
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() || st.Size() != f.size || !st.ModTime().Equal(f.mtime) {
		return fmt.Errorf("导出期间文件发生变化，请重试：%s", f.path)
	}
	return nil
}

func packExportDestination(root, target string, overwrite bool) (string, os.FileInfo, error) {
	parent, err := filepath.EvalSymlinks(filepath.Dir(target))
	if err != nil {
		return "", nil, err
	}
	dest, err := safeJoin(parent, filepath.Base(target))
	if err != nil {
		return "", nil, err
	}
	source, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", nil, err
	}
	rel, err := filepath.Rel(source, dest)
	if err == nil && filepath.IsLocal(rel) {
		return "", nil, errors.New("请把整合包导出到游戏目录之外")
	}
	st, err := os.Lstat(dest)
	if err != nil && !os.IsNotExist(err) {
		return "", nil, err
	}
	if st != nil && (!st.Mode().IsRegular() || !overwrite) {
		return "", nil, errors.New("导出文件已存在，请选择其他名称或确认覆盖")
	}
	return dest, st, nil
}

func (s *Service) exportPack(ctx context.Context, r Root, req ExportPackRequest, p progress) error {
	preview, err := packExportVersion(r)
	if err != nil {
		return err
	}
	dest, previous, err := packExportDestination(r.Path, req.Path, req.Overwrite)
	if err != nil {
		return err
	}
	dir, err := os.OpenRoot(r.VersionDir())
	if err != nil {
		return err
	}
	defer dir.Close()
	p("正在整理导出内容", 0, 0)
	files, err := collectPackExportFiles(ctx, dir, r.SelectedVersion, req.Entries)
	if err != nil {
		return err
	}
	if err := s.resolvePackExportSources(ctx, r, dir, files, req.Format, p); err != nil {
		return err
	}

	// A unique sibling file prevents concurrent exports and cancelled retries
	// from damaging each other. No instance data is ever modified or deleted.
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".mcl-export-*.tmp")
	if err != nil {
		return err
	}
	defer func() { tmp.Close(); _ = os.Remove(tmp.Name()) }()
	if err := writePackExportArchive(ctx, dir, tmp, req, preview, files, p); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// Do not replace a different file created while the export was running.
	current, err := os.Lstat(dest)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if previous == nil && current != nil || previous != nil && (current == nil || !os.SameFile(previous, current) || previous.Size() != current.Size() || !previous.ModTime().Equal(current.ModTime())) {
		return errors.New("导出目标文件已变化，请重新选择保存位置")
	}
	if err := os.Rename(tmp.Name(), dest); err != nil {
		return err
	}
	s.setJobResultRef(ctx, dest)
	p("导出完成："+dest, len(files), len(files))
	return nil
}

func writePackExportArchive(ctx context.Context, dir *os.Root, out io.Writer, req ExportPackRequest, preview PackExportPreview, files []packExportFile, p progress) error {
	zw := zip.NewWriter(out)
	h := &zip.FileHeader{Name: "overrides/", Method: zip.Store}
	h.SetMode(os.ModeDir | 0755)
	if _, err := zw.CreateHeader(h); err != nil {
		return err
	}
	// Reuse a single compressor for text/config files instead of allocating a
	// new deflater per file. Already-compressed assets are stored directly.
	var compressor *flate.Writer
	zw.RegisterCompressor(zip.Deflate, func(w io.Writer) (io.WriteCloser, error) {
		if compressor == nil {
			var err error
			compressor, err = flate.NewWriter(w, flate.BestSpeed)
			return compressor, err
		}
		compressor.Reset(w)
		return compressor, nil
	})
	buf := make([]byte, 64<<10)
	references := 0
	for i, f := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		in, err := openPackExportFile(dir, f)
		if err != nil {
			return err
		}
		if f.mr != nil || f.cf != nil {
			in.Close()
			references++
		} else {
			err = writePackExportOverride(ctx, zw, in, f, buf)
			in.Close()
			if err != nil {
				return err
			}
		}
		if i%16 == 0 || i+1 == len(files) {
			p("正在导出："+f.path, i+1, len(files))
		}
	}
	if err := writePackExportIndex(ctx, zw, req, preview, files); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	p(fmt.Sprintf("导出文件引用 / 随包文件：%d / %d", references, len(files)-references), len(files), len(files))
	return ctx.Err()
}

func writePackExportOverride(ctx context.Context, zw *zip.Writer, in *os.File, f packExportFile, buf []byte) error {
	h := &zip.FileHeader{Name: "overrides/" + f.path, Method: zip.Deflate, Modified: f.mtime}
	h.SetMode(f.mode.Perm())
	switch strings.ToLower(filepath.Ext(f.path)) {
	case ".jar", ".zip", ".disabled", ".png", ".jpg", ".jpeg", ".webp", ".ogg", ".mp3", ".mp4", ".gz", ".7z":
		h.Method = zip.Store
	}
	w, err := zw.CreateHeader(h)
	if err != nil {
		return err
	}
	n, err := io.CopyBuffer(w, modReader{ctx: ctx, r: io.LimitReader(in, f.size+1)}, buf)
	if err != nil {
		return err
	}
	if n != f.size {
		return fmt.Errorf("导出期间文件发生变化，请重试：%s", f.path)
	}
	return checkPackExportFile(in, f)
}

func writePackExportIndex(ctx context.Context, zw *zip.Writer, req ExportPackRequest, preview PackExportPreview, files []packExportFile) error {
	if req.Format == "modrinth" {
		deps := map[string]string{"minecraft": preview.Minecraft}
		if preview.Loader != "vanilla" {
			key := preview.Loader
			if key == "fabric" || key == "quilt" {
				key += "-loader"
			}
			deps[key] = preview.LoaderVersion
		}
		header := map[string]any{"formatVersion": 1, "game": "minecraft", "versionId": req.PackVersion, "name": req.Name, "dependencies": deps}
		var refs []*mrPackFile
		for i := range files {
			if files[i].mr != nil {
				refs = append(refs, files[i].mr)
			}
		}
		return writePackExportManifest(ctx, zw, "modrinth.index.json", header, refs)
	}
	loaders := []cfPackLoader{}
	if preview.Loader != "vanilla" {
		loaders = append(loaders, cfPackLoader{ID: preview.Loader + "-" + preview.LoaderVersion, Primary: true})
	}
	header := map[string]any{"manifestType": "minecraftModpack", "manifestVersion": 1, "name": req.Name, "version": req.PackVersion, "author": req.Author, "overrides": "overrides", "minecraft": map[string]any{"version": preview.Minecraft, "modLoaders": loaders}}
	refs := make([]*cfPackFile, 0)
	seen := map[int]bool{}
	for i := range files {
		if ref := files[i].cf; ref != nil && !seen[ref.FileID] {
			refs = append(refs, ref)
			seen[ref.FileID] = true
		}
	}
	return writePackExportManifest(ctx, zw, "manifest.json", header, refs)
}

// Encode entries separately so a very large manifest does not require another
// full-size JSON buffer. The importer bounds its manifest at 16 MiB as well.
func writePackExportManifest[T any](ctx context.Context, zw *zip.Writer, name string, header any, refs []T) error {
	out, err := zw.Create(name)
	if err != nil {
		return err
	}
	w := &packManifestWriter{Writer: out, left: 16 << 20}
	b, err := json.Marshal(header)
	if err != nil {
		return err
	}
	if _, err := w.Write(b[:len(b)-1]); err != nil {
		return err
	}
	if _, err := io.WriteString(w, ",\"files\":["); err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	for i, ref := range refs {
		if err := ctx.Err(); err != nil {
			return err
		}
		if i > 0 {
			if _, err := io.WriteString(w, ","); err != nil {
				return err
			}
		}
		if err := enc.Encode(ref); err != nil {
			return err
		}
	}
	_, err = io.WriteString(w, "]}\n")
	return err
}

type packManifestWriter struct {
	io.Writer
	left int
}

func (w *packManifestWriter) Write(b []byte) (int, error) {
	if len(b) > w.left {
		return 0, errors.New("整合包清单超过 16 MiB 限制")
	}
	n, err := w.Writer.Write(b)
	w.left -= n
	return n, err
}
