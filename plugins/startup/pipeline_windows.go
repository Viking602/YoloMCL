//go:build windows

package startup

import (
	_ "embed"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

//go:embed assets/mygo-0.5.5/vertex.dxbc
var vsCode []byte

//go:embed assets/mygo-0.5.5/pixel.dxbc
var psCode []byte

type element struct {
	Name                                     *byte
	Index, Format, Slot, Offset, Class, Rate uint32
}
type bufferDesc struct{ Size, Usage, Bind, Access, Misc, Stride uint32 }
type resourceData struct {
	Data         unsafe.Pointer
	Pitch, Slice uint32
}
type targetBlend struct {
	Enable, Src, Dest, Op, SrcA, DestA, OpA uint32
	Mask                                    uint8
	_                                       [3]byte
}
type blendDesc struct {
	Coverage, Independent uint32
	Targets               [8]targetBlend
}
type rasterDesc struct {
	Fill, Cull, CCW, Bias       uint32
	Clamp, Slope                float32
	Clip, Scissor, Multi, Lines uint32
}

func warmPipeline(device, ctx, swap unsafe.Pointer) error {
	r := objects{device: device}
	defer func() {
		startupCOM(ctx, 110)
		for i := len(r.resources) - 1; i >= 0; i-- {
			startupCOM(r.resources[i], 2)
		}
	}()
	vs, e := r.create(12, uintptr(unsafe.Pointer(&vsCode[0])), uintptr(len(vsCode)), 0)
	if e != nil {
		return e
	}
	ps, e := r.create(15, uintptr(unsafe.Pointer(&psCode[0])), uintptr(len(psCode)), 0)
	if e != nil {
		return e
	}
	names := []string{"RECT", "RADII", "INNER", "COLOR", "COLOR", "COLOR", "GRAD", "UV", "CLIP", "CLIPR", "PARAMS", "TRANSFORM", "TRANSFORM", "TRANSFORM"}
	indexes := []uint32{0, 0, 0, 0, 1, 2, 0, 0, 0, 0, 0, 0, 1, 2}
	elems := make([]element, len(names))
	for i, n := range names {
		p, _ := syscall.BytePtrFromString(n)
		elems[i] = element{p, indexes[i], 2, 0, uint32(i * 16), 1, 1}
	}
	layout, e := r.create(11, uintptr(unsafe.Pointer(&elems[0])), uintptr(len(elems)), uintptr(unsafe.Pointer(&vsCode[0])), uintptr(len(vsCode)))
	if e != nil {
		return e
	}
	bd := blendDesc{}
	bd.Targets[0] = targetBlend{Enable: 1, Src: 2, Dest: 17, Op: 1, SrcA: 2, DestA: 19, OpA: 1, Mask: 15}
	blend, e := r.create(20, uintptr(unsafe.Pointer(&bd)))
	if e != nil {
		return e
	}
	rd := rasterDesc{Fill: 3, Cull: 1, Clip: 1, Scissor: 1}
	raster, e := r.create(22, uintptr(unsafe.Pointer(&rd)))
	if e != nil {
		return e
	}
	var tex unsafe.Pointer
	iid := [16]byte{0xf2, 0xaa, 0x15, 0x6f, 0x08, 0xd2, 0x89, 0x4e, 0x9a, 0xb4, 0x48, 0x95, 0x35, 0xd3, 0x4f, 0x9c}
	if hr := startupCOM(swap, 9, 0, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&tex))); int32(uint32(hr)) < 0 || tex == nil {
		startupRelease(tex)
		return fmt.Errorf("GetBuffer: %x", hr)
	}
	r.resources = append(r.resources, tex)
	rtv, e := r.create(9, uintptr(tex), 0)
	if e != nil {
		return e
	}
	// The pinned shader's 14 float4 attributes: rect, radii, inner, three
	// colors, gradient, UV, clip, clip radii, params and three transform rows.
	// Draw a solid quad with an identity transform; no textures are needed.
	inst := [56]float32{}
	inst[2], inst[3] = 16, 16
	inst[12], inst[15] = 1, 1
	inst[34], inst[35] = 16, 16
	inst[43] = 1
	inst[44], inst[49] = 1, 1
	data := resourceData{Data: unsafe.Pointer(&inst[0])}
	buf := bufferDesc{Size: 224, Bind: 1}
	vertex, e := r.create(3, uintptr(unsafe.Pointer(&buf)), uintptr(unsafe.Pointer(&data)))
	if e != nil {
		return e
	}
	globals := [4]float32{16, 16, 0, 0}
	data = resourceData{Data: unsafe.Pointer(&globals[0])}
	buf = bufferDesc{Size: 16, Bind: 4}
	constant, e := r.create(3, uintptr(unsafe.Pointer(&buf)), uintptr(unsafe.Pointer(&data)))
	if e != nil {
		return e
	}
	startupCOM(ctx, 33, 1, uintptr(unsafe.Pointer(&rtv)), 0)
	vp := [6]float32{0, 0, 16, 16, 0, 1}
	sc := [4]int32{0, 0, 16, 16}
	clear := [4]float32{}
	startupCOM(ctx, 50, uintptr(rtv), uintptr(unsafe.Pointer(&clear[0])))
	startupCOM(ctx, 43, uintptr(raster))
	startupCOM(ctx, 44, 1, uintptr(unsafe.Pointer(&vp[0])))
	startupCOM(ctx, 45, 1, uintptr(unsafe.Pointer(&sc[0])))
	startupCOM(ctx, 35, uintptr(blend), 0, 0xffffffff)
	startupCOM(ctx, 17, uintptr(layout))
	startupCOM(ctx, 24, 5)
	stride, offset := uint32(224), uint32(0)
	startupCOM(ctx, 18, 0, 1, uintptr(unsafe.Pointer(&vertex)), uintptr(unsafe.Pointer(&stride)), uintptr(unsafe.Pointer(&offset)))
	startupCOM(ctx, 11, uintptr(vs), 0, 0)
	startupCOM(ctx, 9, uintptr(ps), 0, 0)
	startupCOM(ctx, 7, 0, 1, uintptr(unsafe.Pointer(&constant)))
	startupCOM(ctx, 21, 4, 1, 0, 0)
	startupCOM(ctx, 111)
	startupCOM(swap, 8, 0, 0)
	runtime.KeepAlive(elems)
	return nil
}

type objects struct {
	device    unsafe.Pointer
	resources []unsafe.Pointer
}

// The output pointer must remain heap-allocated across SyscallN and a possible
// Go stack growth. Appending its uintptr to a variadic slice alone is not an
// escape guarantee; noinline keeps this allocation out of the caller's stack.
//
//go:noinline
func slot() *unsafe.Pointer { return new(unsafe.Pointer) }

//go:uintptrescapes
func (r *objects) create(method int, args ...uintptr) (unsafe.Pointer, error) {
	p := slot()
	hr := startupCOM(r.device, method, append(args, uintptr(unsafe.Pointer(p)))...)
	if *p != nil {
		r.resources = append(r.resources, *p)
	}
	if int32(uint32(hr)) < 0 || *p == nil {
		return nil, fmt.Errorf("D3D method %d: %x", method, hr)
	}
	return *p, nil
}
