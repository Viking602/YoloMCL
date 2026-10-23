package launcher

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"image/png"
	"testing"
)

func TestVanillaSkinsAvailableWithoutFilesOrNetwork(t *testing.T) {
	// An uninitialized service has neither configured roots nor an HTTP client.
	s := &Service{}
	for _, tc := range []struct{ model, sha string }{
		{"default", "06628f9cef88520da742ab280a01677ce17c99717f964ed89ed791673121094c"},
		{"slim", "99391ed9dff9581ec88daf23b9b028adc9ac91f52889edcff1c982d0923c0cff"},
	} {
		t.Run(tc.model, func(t *testing.T) {
			data, err := s.VanillaSkin(context.Background(), tc.model)
			if err != nil {
				t.Fatal(err)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != tc.sha {
				t.Fatal("bundled skin no longer matches the original Minecraft texture")
			}
			img, err := png.Decode(bytes.NewReader(data))
			if err != nil || img.Bounds().Dx() != 64 || img.Bounds().Dy() != 64 {
				t.Fatalf("invalid player texture: %v", err)
			}
			data[0] ^= 0xff
			if got := fmt.Sprintf("%x", sha256.Sum256(VanillaSkinPNG(tc.model))); got != tc.sha {
				t.Fatal("a caller can modify the shared default skin")
			}
		})
	}
	if !bytes.Equal(VanillaSkinPNG(""), VanillaSkinPNG("default")) {
		t.Fatal("unspecified model must use Steve")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.VanillaSkin(ctx, "slim"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled appearance request returned %v", err)
	}
}
