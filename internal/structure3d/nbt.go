package structure3d

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// DecodeNBT reads a root compound from gzipped or raw big-endian NBT.
// Structure templates in jars are gzip-compressed.
func DecodeNBT(data []byte) (map[string]any, error) {
	r := bytes.NewReader(data)
	var in io.Reader = r
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		gz, err := gzip.NewReader(r)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		in = gz
	}
	n := &nbtDec{r: in}
	tag := n.u8()
	if tag != 10 {
		return nil, fmt.Errorf("structure3d: root is not a compound (tag %d)", tag)
	}
	_ = n.str()
	v := n.val(10)
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("structure3d: root compound missing")
	}
	return m, n.err
}

type nbtDec struct {
	r   io.Reader
	err error
	buf [8]byte
}

func (n *nbtDec) read(p []byte) {
	if n.err != nil {
		return
	}
	_, n.err = io.ReadFull(n.r, p)
}

func (n *nbtDec) u8() byte {
	n.read(n.buf[:1])
	return n.buf[0]
}

func (n *nbtDec) i16() int16 {
	n.read(n.buf[:2])
	return int16(binary.BigEndian.Uint16(n.buf[:2]))
}

func (n *nbtDec) i32() int32 {
	n.read(n.buf[:4])
	return int32(binary.BigEndian.Uint32(n.buf[:4]))
}

func (n *nbtDec) i64() int64 {
	n.read(n.buf[:8])
	return int64(binary.BigEndian.Uint64(n.buf[:8]))
}

func (n *nbtDec) str() string {
	l := int(uint16(n.i16()))
	if l < 0 || l > 1<<20 {
		n.err = fmt.Errorf("structure3d: bad nbt string length")
		return ""
	}
	b := make([]byte, l)
	n.read(b)
	return string(b)
}

func (n *nbtDec) val(tag byte) any {
	if n.err != nil {
		return nil
	}
	switch tag {
	case 1:
		return int32(int8(n.u8()))
	case 2:
		return int32(n.i16())
	case 3:
		return n.i32()
	case 4:
		return n.i64()
	case 5:
		n.read(n.buf[:4])
		return mathFloat32(n.buf[:4])
	case 6:
		n.read(n.buf[:8])
		return mathFloat64(n.buf[:8])
	case 7:
		l := int(n.i32())
		b := make([]byte, max(0, l))
		n.read(b)
		return b
	case 8:
		return n.str()
	case 9:
		et := n.u8()
		l := int(n.i32())
		out := make([]any, max(0, l))
		for i := range out {
			out[i] = n.val(et)
		}
		return out
	case 10:
		m := map[string]any{}
		for {
			t := n.u8()
			if t == 0 || n.err != nil {
				return m
			}
			k := n.str()
			m[k] = n.val(t)
		}
	case 11:
		l := int(n.i32())
		out := make([]int32, max(0, l))
		for i := range out {
			out[i] = n.i32()
		}
		return out
	case 12:
		l := int(n.i32())
		out := make([]int64, max(0, l))
		for i := range out {
			out[i] = n.i64()
		}
		return out
	default:
		n.err = fmt.Errorf("structure3d: unknown nbt tag %d", tag)
		return nil
	}
}

func mathFloat32(b []byte) float32 {
	return math.Float32frombits(binary.BigEndian.Uint32(b))
}

func mathFloat64(b []byte) float64 {
	return math.Float64frombits(binary.BigEndian.Uint64(b))
}
