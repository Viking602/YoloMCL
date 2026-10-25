package nativeui

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"golang.org/x/image/draw"
)

const resourceIconEdge = 128 // sharp at 2x DPI for the 64-DIP project artwork

// Only oversized raster sources serialize; normal icon decodes stay parallel.
// A 4096-square PNG can otherwise allocate 64 MB in each worker before scaling.
var largeImageDecode = make(chan struct{}, 1)

func decodeResourceImage(b []byte) (ui.ImageSource, error) {
	return decodeThumbnail(b, resourceIconEdge)
}

func decodeLocalImage(key string, b []byte) (ui.ImageSource, error) {
	return decodeLocalImageContext(context.Background(), key, b)
}

func decodeLocalImageContext(ctx context.Context, key string, b []byte) (ui.ImageSource, error) {
	return decodeThumbnailContext(ctx, b, localImageEdge(key))
}

func localImageEdge(key string) int {
	if strings.HasPrefix(key, "screenshot:") {
		return 960
	}
	if strings.HasPrefix(key, "addon:") {
		// Add-on rows and the disable dialog use at most 40-DIP artwork.
		return 96
	}
	return resourceIconEdge
}

// Hash the compressed source before decoding: repeated artwork avoids the PNG
// decoder, resizer and mipmaps as well as their temporary allocations. Nothing
// is retained until the caller checks its request generation and publishes it.
func (a *App) decodeImageContent(ctx context.Context, b []byte, edge int) (ui.ImageSource, imageContentKey, error) {
	if err := ctx.Err(); err != nil {
		return nil, imageContentKey{}, err
	}
	content := imageContentKey{sum: sha256.Sum256(b), edge: edge}
	if bytes.HasPrefix(bytes.TrimSpace(b), []byte("<")) {
		content.edge = 0 // vector sources are independent of requested size
	}
	a.imageMu.Lock()
	data := a.imageContents[content]
	a.imageMu.Unlock()
	if data != nil {
		return data.source, content, nil
	}
	source, err := decodeThumbnailContext(ctx, b, edge)
	return source, content, err
}

func decodeThumbnail(b []byte, edge int) (ui.ImageSource, error) {
	return decodeThumbnailContext(context.Background(), b, edge)
}

func decodeThumbnailContext(ctx context.Context, b []byte, edge int) (ui.ImageSource, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	trim := bytes.TrimSpace(b)
	if bytes.HasPrefix(trim, []byte("<")) {
		if len(trim) > 512<<10 {
			return nil, errors.New("resource icon data is too large")
		}
		svg, e := ui.ParseSVG(trim)
		if e != nil {
			return nil, e
		}
		return svg, nil
	}
	config, _, e := image.DecodeConfig(bytes.NewReader(b))
	if e != nil {
		return nil, e
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > 4096 || config.Height > 4096 || config.Width*config.Height > 16<<20 {
		return nil, errors.New("资源图标尺寸过大")
	}
	if max(config.Width, config.Height) <= edge {
		return ui.DecodeBitmap(b)
	}
	if config.Width*config.Height > 1024*1024 {
		select {
		case largeImageDecode <- struct{}{}:
			defer func() { <-largeImageDecode }()
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	source, format, err := image.Decode(cancelImageReader{ctx, bytes.NewReader(b)})
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	w, h := config.Width, config.Height
	longest := max(w, h)
	w, h = max(1, w*edge/longest), max(1, h*edge/longest)
	thumb := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.ApproxBiLinear.Scale(thumb, thumb.Bounds(), source, source.Bounds(), draw.Src, nil)
	if format == "jpeg" {
		thumb = orientThumbnail(thumb, jpegOrientation(b))
	}
	// NewBitmap takes this compact RGBA buffer directly. The source-sized
	// decoder buffer can be collected without also creating a full RGBA copy.
	return ui.NewBitmap(thumb), nil
}

type cancelImageReader struct {
	ctx    context.Context
	reader *bytes.Reader
}

func (r cancelImageReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func (a *App) remoteImage(raw string) ui.ImageSource {
	return a.remoteImageSize(raw, resourceIconEdge)
}

// Small feed tiles need fewer decoded pixels than project dialogs. Keep one
// bitmap per URL and upgrade from the disk source only when a larger view
// requests it; the existing image stays visible until that decode completes.
func (a *App) remoteImageSize(raw string, edge int) ui.ImageSource {
	if raw == "" {
		return nil
	}
	discover := a.page == "discover"
	a.imageMu.Lock()
	cached := a.cachedPageImageLocked(raw, discover)
	pending := a.imagePendingLocked(raw)
	retry := a.imageRetry[raw]
	decodedEdge := a.imageUses[raw].edge
	if (cached != nil && (decodedEdge == 0 || decodedEdge >= edge)) || pending || time.Now().Before(retry) {
		a.imageMu.Unlock()
		return cached
	}
	ctx := a.imageContextLocked()
	a.reserveImageLocked(raw, ctx)
	a.imageMu.Unlock()
	dataDir := a.snapshot.DataDir
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		defer a.clearImagePending(raw, ctx)
		select {
		case a.imageSlots <- struct{}{}:
			defer func() { <-a.imageSlots }()
		case <-ctx.Done():
			return
		}
		if ctx.Err() != nil {
			return
		}
		sum := sha256.Sum256([]byte(raw))
		file := filepath.Join(dataDir, "cache", "resource-icons", hex.EncodeToString(sum[:])+".img")
		var source ui.ImageSource
		var content imageContentKey
		var err error
		var encodedBytes int
		if b, e := readCachedImage(file); e == nil {
			source, content, _ = a.decodeImageContent(ctx, b, edge)
			encodedBytes = len(b)
		}
		if source == nil {
			var b []byte
			b, err = a.service.ResourceImage(ctx, raw)
			encodedBytes = len(b)
			if err == nil {
				source, content, err = a.decodeImageContent(ctx, b, edge)
			}
			if err == nil {
				if e := os.MkdirAll(filepath.Dir(file), 0700); e == nil {
					if tmp, e := os.CreateTemp(filepath.Dir(file), ".icon-*"); e == nil {
						_, writeErr := tmp.Write(b)
						closeErr := tmp.Close()
						if writeErr == nil && closeErr == nil {
							_ = os.Rename(tmp.Name(), file)
						}
						_ = os.Remove(tmp.Name())
					}
				}
			}
		}
		if ctx.Err() != nil {
			return
		}
		a.imageMu.Lock()
		if ctx.Err() != nil || a.imageRequests[raw] != ctx {
			a.imageMu.Unlock()
			return
		}
		if err != nil {
			a.failImageLocked(raw, 15*time.Second)
		} else {
			a.cacheImageContentLocked(raw, source, encodedBytes, content)
			if use, ok := a.imageUses[raw]; ok {
				use.discover = discover
				a.imageUses[raw] = use
			}
		}
		a.imageMu.Unlock()
		a.post(func() {})
	}()
	return cached
}

func readCachedImage(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, (4<<20)+1))
	if len(b) > 4<<20 {
		return nil, errors.New("cached resource icon is too large")
	}
	return b, err
}

func imageFromData(data string) image.Image {
	b, e := base64.StdEncoding.DecodeString(strings.TrimPrefix(data, "data:image/png;base64,"))
	if e != nil {
		return nil
	}
	img, _, _ := image.Decode(bytes.NewReader(b))
	return img
}

// Keep the same EXIF orientation as ui.DecodeBitmap when shrinking JPEGs.
func jpegOrientation(b []byte) int {
	for i := 2; i+4 <= len(b) && b[i] == 0xff; {
		marker, size := b[i+1], int(binary.BigEndian.Uint16(b[i+2:]))
		if marker == 0xda || size < 2 || size > len(b)-i-2 {
			break
		}
		segment := b[i+4 : i+2+size]
		i += 2 + size
		if marker != 0xe1 || len(segment) < 14 || string(segment[:6]) != "Exif\x00\x00" {
			continue
		}
		tiff := segment[6:]
		var order binary.ByteOrder
		switch string(tiff[:2]) {
		case "II":
			order = binary.LittleEndian
		case "MM":
			order = binary.BigEndian
		default:
			return 0
		}
		offset := int(order.Uint32(tiff[4:]))
		if offset < 8 || offset > len(tiff)-2 {
			return 0
		}
		count := int(order.Uint16(tiff[offset:]))
		for entry := offset + 2; count > 0 && entry+12 <= len(tiff); entry, count = entry+12, count-1 {
			if order.Uint16(tiff[entry:]) == 0x0112 && order.Uint16(tiff[entry+2:]) == 3 {
				return int(order.Uint16(tiff[entry+8:]))
			}
		}
	}
	return 0
}

func orientThumbnail(src *image.RGBA, orientation int) *image.RGBA {
	if orientation < 2 || orientation > 8 {
		return src
	}
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dw, dh := w, h
	if orientation >= 5 {
		dw, dh = h, w
	}
	out := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := range dh {
		for x := range dw {
			sx, sy := x, y
			switch orientation {
			case 2:
				sx = w - 1 - x
			case 3:
				sx, sy = w-1-x, h-1-y
			case 4:
				sy = h - 1 - y
			case 5:
				sx, sy = y, x
			case 6:
				sx, sy = y, h-1-x
			case 7:
				sx, sy = w-1-y, h-1-x
			case 8:
				sx, sy = w-1-y, x
			}
			out.SetRGBA(x, y, src.RGBAAt(sx, sy))
		}
	}
	return out
}
