package modjar

import (
	"context"
	"math"
	"testing"
)

func TestJDKGapBindings(t *testing.T) {
	v := newMachine(context.Background(), map[string][]byte{})
	h := &bindings{vm: v}
	v.host = h
	call := func(owner, name, desc string, receiver any, args ...any) any {
		t.Helper()
		x, ok := h.javaCall(reference{owner, name, desc}, receiver, args)
		if !ok {
			t.Fatalf("unbound %s.%s%s", owner, name, desc)
		}
		return x
	}
	if got := call("java/lang/Long", "numberOfLeadingZeros", "(J)I", nil, int64(1)); got != int32(63) {
		t.Fatal(got)
	}
	if got := call("java/lang/Integer", "numberOfTrailingZeros", "(I)I", nil, int32(0)); got != int32(32) {
		t.Fatal(got)
	}
	if got := call("java/lang/Float", "compare", "(FF)I", nil, float32(1), float32(2)); got != int32(-1) {
		t.Fatal(got)
	}
	if got := call("java/lang/Double", "compare", "(DD)I", nil, math.NaN(), 1.0); got != int32(1) {
		t.Fatal(got)
	}
	if got := call("java/lang/Double", "compare", "(DD)I", nil, 0.0, math.Copysign(0, -1)); got != int32(1) {
		t.Fatal(got)
	}
	// Arrays.hashCode semantics: 31*(31*1+1)+0 for {1-like, null}.
	if got := call("java/util/Objects", "hash", "([Ljava/lang/Object;)I", nil, &array{values: []any{nil, nil}}); got != int32(961) {
		t.Fatal(got)
	}
	sb := v.alloc("java/lang/StringBuilder", nil)
	call("java/lang/StringBuilder", "<init>", "()V", sb)
	call("java/lang/StringBuilder", "append", "(Ljava/lang/CharSequence;II)Ljava/lang/StringBuilder;", sb, "abcdef", int32(1), int32(4))
	if got := call("java/lang/StringBuilder", "toString", "()Ljava/lang/String;", sb); got != "bcd" {
		t.Fatal(got)
	}
	if _, ok := h.javaCall(reference{"java/lang/StringBuilder", "append", "(Ljava/lang/CharSequence;II)Ljava/lang/StringBuilder;"}, sb, []any{"ab", int32(1), int32(9)}); ok {
		t.Fatal("out-of-range append accepted")
	}
	st := v.alloc("java/util/Stack", nil)
	call("java/util/Stack", "<init>", "()V", st)
	if _, ok := objectData(st).(*javaList); !ok {
		t.Fatal("stack has no native list")
	}
}

func TestInMemoryReadersOnlyWrapResourceBytes(t *testing.T) {
	for _, name := range []string{"java/io/RandomAccessFile", "java/nio/channels/FileChannel", "java/lang/Runtime"} {
		if !forbiddenClass(name) {
			t.Fatal("host I/O allowed", name)
		}
	}
	for _, name := range []string{"java/io/File", "java/io/BufferedReader", "java/io/IOException"} {
		if forbiddenClass(name) {
			t.Fatal("in-memory type forbidden", name)
		}
	}
}
