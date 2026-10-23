//go:build windows

package startup

import (
	"context"
	"encoding/binary"
	"testing"
	"unsafe"
)

func TestWindowsNativeLayouts(t *testing.T) {
	// x64 and ARM64 share these Windows SDK ABI layouts. Incorrect sizes can
	// corrupt native memory before an HRESULT can be checked.
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("64-bit Windows ABI")
	}
	for _, v := range []struct {
		name      string
		got, want uintptr
	}{
		{"swap chain", unsafe.Sizeof(startupSwapChain{}), 72},
		{"input element", unsafe.Sizeof(element{}), 32},
		{"buffer", unsafe.Sizeof(bufferDesc{}), 24},
		{"resource", unsafe.Sizeof(resourceData{}), 16},
		{"target blend", unsafe.Sizeof(targetBlend{}), 32},
		{"blend", unsafe.Sizeof(blendDesc{}), 264},
		{"raster", unsafe.Sizeof(rasterDesc{}), 40},
	} {
		if v.got != v.want {
			t.Errorf("%s: %d != %d", v.name, v.got, v.want)
		}
	}
	if unsafe.Offsetof(startupSwapChain{}.Window) != 48 {
		t.Fatal("invalid HWND offset")
	}
	for _, code := range [][]byte{vsCode, psCode} {
		if len(code) < 32 || string(code[:4]) != "DXBC" || int(binary.LittleEndian.Uint32(code[24:28])) != len(code) {
			t.Fatal("invalid embedded shader")
		}
	}
}

func TestCancelledPreparationDoesNotInitializeDriver(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	prepareGraphics(ctx)
}
