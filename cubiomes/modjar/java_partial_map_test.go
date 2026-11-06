package modjar

import (
	"context"
	"testing"
)

func TestNativeMapWritesCannotTurnMissingBaselineIntoNull(t *testing.T) {
	c := &fixtureClass{}
	for _, name := range []string{"discard", "observe", "overwrite"} {
		code := []byte{42, 43, 44}
		code = append(code, c.instruction(182, "java/util/HashMap", "put", "(Ljava/lang/Object;Ljava/lang/Object;)Ljava/lang/Object;")...)
		if name == "observe" {
			code = append(code, 176)
		} else {
			code = append(code, 87)
			if name == "overwrite" {
				code = append(code, 42, 43, 44)
				code = append(code, c.instruction(182, "java/util/HashMap", "put", "(Ljava/lang/Object;Ljava/lang/Object;)Ljava/lang/Object;")...)
				code = append(code, 176)
			} else {
				code = append(code, 1, 176)
			}
		}
		c.method(name, "(Ljava/util/HashMap;Ljava/lang/Object;Ljava/lang/Object;)Ljava/lang/Object;", 9, code)
	}
	v := newMachine(context.Background(), map[string][]byte{"sample/MapWrites": c.bytes("sample/MapWrites", "java/lang/Object")})
	v.host = &bindings{vm: v}
	call := func(name string) any {
		return v.method(reference{"sample/MapWrites", name, "(Ljava/util/HashMap;Ljava/lang/Object;Ljava/lang/Object;)Ljava/lang/Object;"}, nil, false, []any{v.alloc("java/util/HashMap", &javaMap{baseline: "external table"}), "key", "new"})
	}
	if call("discard") != nil {
		t.Fatal("unexpected return")
	}
	expectFault(t, "baseline", func() { call("observe") })
	if got := call("overwrite"); got != "new" {
		t.Fatal("written value was not retained", got)
	}
}
