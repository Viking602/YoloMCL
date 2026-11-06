// Package modjar translates bounded world-generation registration bytecode
// into native engine inputs. Its metadata APIs inspect only supplied class
// files; it has no host filesystem, process, network or JVM bindings.
package modjar

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"unicode/utf16"
)

type fault struct{ message string }

func (f fault) Error() string { return f.message }

func fail(format string, args ...any) { panic(fault{fmt.Sprintf(format, args...)}) }

type reader struct {
	b []byte
	p int
}

func (r *reader) take(n int) []byte {
	if n < 0 || n > len(r.b)-r.p {
		fail("truncated class file")
	}
	b := r.b[r.p : r.p+n]
	r.p += n
	return b
}
func (r *reader) u1() int    { return int(r.take(1)[0]) }
func (r *reader) u2() int    { return int(binary.BigEndian.Uint16(r.take(2))) }
func (r *reader) u4() uint32 { return binary.BigEndian.Uint32(r.take(4)) }

type constant struct {
	tag   int
	a, b  int
	value any
}
type reference struct{ owner, name, desc string }
type field struct {
	name, desc  string
	access      int
	value       any
	annotations []annotation
}
type method struct {
	name, desc            string
	access, locals, stack int
	code                  []byte
	annotations           []annotation
	handlers              []exceptionHandler
	// origin resolves constants for code merged from a mixin class.
	origin  *class
	injects []injectHook
	// annotationDefault is an annotation element's AnnotationDefault value.
	annotationDefault any
	hasDefault        bool
}

type exceptionHandler struct {
	start, end, pc int
	catch          string
}
type bootstrap struct {
	method int
	args   []int
}
type class struct {
	symbols     *Symbols
	access      int
	name, super string
	interfaces  []string
	annotations []annotation
	pool        []constant
	fields      []field
	methods     []method
	boots       []bootstrap
	// signature is the class Signature attribute, used by getGenericSuperclass.
	signature string
	// mixinInits are merged static initialisers, run after <clinit>.
	mixinInits []method
}

func (c *class) cp(index int, tags ...int) constant {
	if index <= 0 || index >= len(c.pool) {
		fail("invalid constant pool index %d", index)
	}
	v := c.pool[index]
	for _, tag := range tags {
		if v.tag == tag {
			return v
		}
	}
	fail("unexpected constant pool tag %d at %d", v.tag, index)
	return constant{}
}
func (c *class) text(index int) string       { return c.cp(index, 1).value.(string) }
func (c *class) descriptor(index int) string { return c.symbols.descriptor(c.text(index)) }
func (c *class) className(index int) string  { return c.symbols.typeName(c.text(c.cp(index, 7).a)) }
func (c *class) ref(index int) reference {
	v := c.cp(index, 9, 10, 11)
	n := c.cp(v.b, 12)
	return c.symbols.ref(reference{c.className(v.a), c.text(n.a), c.text(n.b)})
}
func (c *class) literal(index int) any {
	v := c.cp(index, 3, 4, 5, 6, 8, 7)
	if v.tag == 8 {
		return c.text(v.a)
	}
	if v.tag == 7 {
		return classLiteral(c.className(index))
	}
	return v.value
}

type classLiteral string

func modifiedUTF8(b []byte) string {
	units := make([]uint16, 0, len(b))
	for p := 0; p < len(b); {
		a := b[p]
		p++
		switch {
		case a > 0 && a < 128:
			units = append(units, uint16(a))
		case a&0xe0 == 0xc0 && p < len(b) && b[p]&0xc0 == 0x80:
			v := uint16(a&31)<<6 | uint16(b[p]&63)
			p++
			if v != 0 && v < 128 {
				fail("invalid modified UTF-8")
			}
			units = append(units, v)
		case a&0xf0 == 0xe0 && p+1 < len(b) && b[p]&0xc0 == 0x80 && b[p+1]&0xc0 == 0x80:
			v := uint16(a&15)<<12 | uint16(b[p]&63)<<6 | uint16(b[p+1]&63)
			p += 2
			if v < 2048 {
				fail("invalid modified UTF-8")
			}
			units = append(units, v)
		default:
			fail("invalid modified UTF-8")
		}
	}
	return string(utf16.Decode(units))
}

func parseClass(data []byte, symbols ...*Symbols) *class {
	r := reader{b: data}
	if r.u4() != 0xcafebabe {
		fail("invalid class header")
	}
	r.u2()
	major := r.u2()
	if major < 45 || major > 70 {
		fail("unsupported class version %d", major)
	}
	c := &class{pool: make([]constant, r.u2())}
	if len(symbols) != 0 {
		c.symbols = symbols[0]
	}
	for i := 1; i < len(c.pool); i++ {
		v := constant{tag: r.u1()}
		switch v.tag {
		case 1:
			v.value = modifiedUTF8(r.take(r.u2()))
		case 3:
			v.value = int32(r.u4())
		case 4:
			v.value = math.Float32frombits(r.u4())
		case 5:
			v.value = int64(binary.BigEndian.Uint64(r.take(8)))
		case 6:
			v.value = math.Float64frombits(binary.BigEndian.Uint64(r.take(8)))
		case 7, 8, 16, 19, 20:
			v.a = r.u2()
		case 9, 10, 11, 12, 17, 18:
			v.a, v.b = r.u2(), r.u2()
		case 15:
			v.a, v.b = r.u1(), r.u2()
		default:
			fail("unsupported constant pool tag %d", v.tag)
		}
		c.pool[i] = v
		if v.tag == 5 || v.tag == 6 {
			i++
			if i >= len(c.pool) {
				fail("truncated wide constant")
			}
		}
	}
	c.access = r.u2()
	c.name = c.className(r.u2())
	if super := r.u2(); super != 0 {
		c.super = c.className(super)
	}
	for n := r.u2(); n > 0; n-- {
		c.interfaces = append(c.interfaces, c.className(r.u2()))
	}
	attributes := func(input *reader, visit func(string, []byte)) {
		for n := input.u2(); n > 0; n-- {
			name := c.text(input.u2())
			data := input.take(int(input.u4()))
			visit(name, data)
		}
	}
	for n := r.u2(); n > 0; n-- {
		f := field{access: r.u2(), name: c.text(r.u2()), desc: c.text(r.u2())}
		ref := c.symbols.ref(reference{c.name, f.name, f.desc})
		f.name, f.desc = ref.name, ref.desc
		attributes(&r, func(name string, b []byte) {
			if name == "RuntimeVisibleAnnotations" || name == "RuntimeInvisibleAnnotations" {
				f.annotations = append(f.annotations, markInvisible(c.readAnnotations(b), name)...)
			}
			if name == "ConstantValue" {
				a := reader{b: b}
				f.value = c.literal(a.u2())
			}
		})
		c.fields = append(c.fields, f)
	}
	for n := r.u2(); n > 0; n-- {
		m := method{access: r.u2(), name: c.text(r.u2()), desc: c.text(r.u2())}
		ref := c.symbols.ref(reference{c.name, m.name, m.desc})
		m.name, m.desc = ref.name, ref.desc
		attributes(&r, func(name string, b []byte) {
			if name == "RuntimeVisibleAnnotations" || name == "RuntimeInvisibleAnnotations" {
				m.annotations = append(m.annotations, markInvisible(c.readAnnotations(b), name)...)
			}
			if name == "AnnotationDefault" {
				m.annotationDefault, m.hasDefault = c.readAnnotationDefault(b), true
			}
			if name != "Code" {
				return
			}
			a := reader{b: b}
			m.stack, m.locals = a.u2(), a.u2()
			length := int(a.u4())
			if length > 65535 {
				fail("oversized method")
			}
			m.code = a.take(length)
			for handlers := a.u2(); handlers > 0; handlers-- {
				h := exceptionHandler{start: a.u2(), end: a.u2(), pc: a.u2()}
				if kind := a.u2(); kind != 0 {
					h.catch = c.className(kind)
				}
				if h.start >= h.end || h.end > len(m.code) || h.pc >= len(m.code) {
					fail("invalid exception handler range")
				}
				m.handlers = append(m.handlers, h)
			}
			attributes(&a, func(string, []byte) {})
		})
		c.methods = append(c.methods, m)
	}
	attributes(&r, func(name string, b []byte) {
		if name == "RuntimeVisibleAnnotations" || name == "RuntimeInvisibleAnnotations" {
			c.annotations = append(c.annotations, markInvisible(c.readAnnotations(b), name)...)
		}
		if name == "Signature" && len(b) >= 2 {
			c.signature = c.text(int(binary.BigEndian.Uint16(b[:2])))
		}
		if name != "BootstrapMethods" {
			return
		}
		a := reader{b: b}
		for n := a.u2(); n > 0; n-- {
			v := bootstrap{method: a.u2()}
			for k := a.u2(); k > 0; k-- {
				v.args = append(v.args, a.u2())
			}
			c.boots = append(c.boots, v)
		}
	})
	if r.p != len(data) {
		fail("trailing class data")
	}
	return c
}

func descriptor(desc string) (args []string, result string) {
	if len(desc) < 3 || len(desc) > 4096 || desc[0] != '(' {
		fail("invalid method descriptor")
	}
	p := 1
	readType := func() string {
		start := p
		for p < len(desc) && desc[p] == '[' {
			p++
		}
		if p >= len(desc) {
			fail("invalid method descriptor")
		}
		if desc[p] == 'L' {
			end := strings.IndexByte(desc[p:], ';')
			if end < 0 {
				fail("invalid method descriptor")
			}
			p += end + 1
		} else {
			if !strings.ContainsRune("BCDFIJSZV", rune(desc[p])) {
				fail("invalid method descriptor")
			}
			p++
		}
		return desc[start:p]
	}
	for p < len(desc) && desc[p] != ')' {
		args = append(args, readType())
		if len(args) > 255 {
			fail("too many arguments")
		}
	}
	if p >= len(desc) {
		fail("invalid method descriptor")
	}
	p++
	result = readType()
	if p != len(desc) {
		fail("trailing method descriptor")
	}
	return
}
