package modjar

import (
	"context"
	"testing"
)

func TestDistributionSuppliersKeepInactiveSideUnloaded(t *testing.T) {
	for _, side := range []string{"CLIENT", "DEDICATED_SERVER"} {
		for _, method := range []string{"runForDist", "unsafeRunForDist", "safeRunForDist"} {
			t.Run(side+"/"+method, func(t *testing.T) {
				c := &fixtureClass{}
				c.method("get", "()Ljava/lang/Object;", 1, append(c.string("selected"), 176))
				outer := &fixtureClass{}
				outer.field("target", "Ljava/util/function/Supplier;", 1)
				code := append([]byte{42}, outer.instruction(180, "sample/Outer", "target", "Ljava/util/function/Supplier;")...)
				outer.method("get", "()Ljava/lang/Object;", 1, append(code, 176))
				v := newMachine(context.Background(), map[string][]byte{"sample/Selected": c.bytes("sample/Selected", "java/lang/Object"), "sample/Outer": outer.bytes("sample/Outer", "java/lang/Object")})
				v.host = &bindings{vm: v, options: Options{Side: side}}
				selected := v.alloc("sample/Outer", nil)
				selected.fields[fieldKey(reference{"sample/Outer", "target", "Ljava/util/function/Supplier;"})] = v.alloc("sample/Selected", nil)
				args := []any{selected, &lambda{kind: 6, target: reference{"missing/Inactive", "load", "()Ljava/lang/Object;"}}}
				if side == "DEDICATED_SERVER" {
					args[0], args[1] = args[1], args[0]
				}
				got := v.method(reference{"net/minecraftforge/fml/DistExecutor", method, "(Ljava/util/function/Supplier;Ljava/util/function/Supplier;)Ljava/lang/Object;"}, nil, false, args)
				if got != "selected" {
					t.Fatal(got)
				}
			})
		}
	}
}

func TestWhenOnSkipsInactiveDist(t *testing.T) {
	for _, side := range []string{"CLIENT", "DEDICATED_SERVER"} {
		for _, method := range []string{"runWhenOn", "safeRunWhenOn", "unsafeRunWhenOn"} {
			t.Run(side+"/"+method, func(t *testing.T) {
				v := newMachine(context.Background(), nil)
				v.host = &bindings{vm: v, options: Options{Side: side}}
				active, _ := v.host.static(reference{"net/minecraftforge/api/distmarker/Dist", side, "Lnet/minecraftforge/api/distmarker/Dist;"})
				inactiveName := "DEDICATED_SERVER"
				if side == "DEDICATED_SERVER" {
					inactiveName = "CLIENT"
				}
				inactive, _ := v.host.static(reference{"net/minecraftforge/api/distmarker/Dist", inactiveName, "Lnet/minecraftforge/api/distmarker/Dist;"})
				bad := &lambda{kind: 6, target: reference{"missing/Inactive", "load", "()Ljava/lang/Object;"}}
				ref := reference{"net/minecraftforge/fml/DistExecutor", method, "(Lnet/minecraftforge/api/distmarker/Dist;Ljava/util/function/Supplier;)V"}
				v.method(ref, nil, false, []any{inactive, bad})
				expectFault(t, "unbound", func() { v.method(ref, nil, false, []any{active, bad}) })
			})
		}
	}
}

func TestFabricEnvironmentReportsConfiguredSide(t *testing.T) {
	for _, item := range []struct {
		side string
		env  string
	}{{"CLIENT", "CLIENT"}, {"DEDICATED_SERVER", "SERVER"}, {"", "CLIENT"}} {
		t.Run(item.side+"/"+item.env, func(t *testing.T) {
			v := newMachine(context.Background(), nil)
			v.host = &bindings{vm: v, options: Options{Side: item.side}}
			loader := v.method(reference{"net/fabricmc/loader/api/FabricLoader", "getInstance", "()Lnet/fabricmc/loader/api/FabricLoader;"}, nil, false, nil)
			got := v.method(reference{"net/fabricmc/loader/api/FabricLoader", "getEnvironmentType", "()Lnet/fabricmc/api/EnvType;"}, loader, true, nil)
			if objectData(got) != item.env {
				t.Fatalf("EnvType %v, want %s", objectData(got), item.env)
			}
			dist, _ := v.host.static(reference{"net/minecraftforge/fml/loading/FMLEnvironment", "dist", "Lnet/minecraftforge/api/distmarker/Dist;"})
			wantDist := item.side
			if wantDist == "" {
				wantDist = "CLIENT"
			}
			if objectData(dist) != wantDist {
				t.Fatalf("Dist %v, want %s", objectData(dist), wantDist)
			}
			isClient := integer(v.method(reference{"net/minecraftforge/api/distmarker/Dist", "isClient", "()Z"}, dist, true, nil))
			isServer := integer(v.method(reference{"net/minecraftforge/api/distmarker/Dist", "isDedicatedServer", "()Z"}, dist, true, nil))
			if wantDist == "CLIENT" && (isClient != 1 || isServer != 0) || wantDist == "DEDICATED_SERVER" && (isClient != 0 || isServer != 1) {
				t.Fatalf("isClient=%d isDedicatedServer=%d for %s", isClient, isServer, wantDist)
			}
		})
	}
}
