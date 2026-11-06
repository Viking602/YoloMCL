package modjar

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// SurfaceRuleSet is one TerraBlender SurfaceRuleManager.addSurfaceRules call,
// statically traced to vanilla surface_rule codec JSON.
type SurfaceRuleSet struct {
	Category  string // TerraBlender RuleCategory: OVERWORLD or NETHER
	Namespace string
	Rule      json.RawMessage
}

const surfaceManager = "terrablender/api/SurfaceRuleManager"

// ReadSurfaceRules finds straight-line addSurfaceRules registrations in the
// archive and folds their rule builders (SurfaceRules factories, static
// helpers, static fields) into data. Any control flow or call the tracer does
// not model is an error, never a guessed rule.
func ReadSurfaceRules(ctx context.Context, z *zip.Reader, options Options) (sets []SurfaceRuleSet, err error) {
	defer func() {
		if p := recover(); p != nil {
			if f, ok := p.(fault); ok {
				err = fmt.Errorf("surface rules: %s", f.message)
			} else {
				err = fmt.Errorf("surface rules: %v", p)
			}
			sets = nil
		}
	}()
	v, names, err := archiveMachine(ctx, z, options)
	if err != nil {
		return nil, err
	}
	t := &surfaceTracer{v: v, symbols: options.Symbols, modID: options.ModID, statics: map[string]map[string]any{}, classes: map[string]*class{}}
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		data := v.inspectClass(name)
		if !bytes.Contains(data, []byte("addSurfaceRules")) {
			continue
		}
		c := t.class(name)
		for _, m := range c.methods {
			if !calls(c, m, surfaceManager, "addSurfaceRules") {
				continue
			}
			t.record = func(category, namespace string, rule any) {
				raw, e := json.Marshal(rule)
				if e != nil {
					fail("surface rule encoding: %v", e)
				}
				sets = append(sets, SurfaceRuleSet{category, namespace, raw})
			}
			t.run(c, m, nil, 0)
		}
	}
	return sets, nil
}

type surfaceTracer struct {
	v       *machine
	symbols *Symbols
	modID   string
	statics map[string]map[string]any
	classes map[string]*class
	steps   int
	record  func(category, namespace string, rule any)
	keys    map[string]map[string]string
}

// keyPath reads the registry path a holder class assigns to a static key
// field: in its <clinit>, the first string constant since the previous putstatic
// (ResourceKey.create / Noises.createKey / register("path", ...)). It falls
// back to the lowercased field name when the class is unavailable.
func (t *surfaceTracer) keyPath(owner, name string) string {
	if t.keys == nil {
		t.keys = map[string]map[string]string{}
	}
	fields, ok := t.keys[owner]
	if !ok {
		fields = map[string]string{}
		t.keys[owner] = fields
		var data []byte
		if t.v.resolveClass != nil {
			var e error
			if data, e = t.v.resolveClass(owner); e != nil {
				fail("surface rule key holder %s: %v", owner, e)
			}
		}
		if data != nil {
			c := parseClass(data, t.symbols)
			for _, m := range c.methods {
				if m.name != "<clinit>" {
					continue
				}
				last := ""
				r := reader{b: m.code}
				for r.p < len(r.b) {
					op := r.u1()
					switch {
					case op == 0x12:
						if s, ok := c.literal(r.u1()).(string); ok && last == "" {
							last = s
						}
					case op == 0x13 || op == 0x14:
						if s, ok := c.literal(r.u2()).(string); ok && last == "" {
							last = s
						}
					case op == 0xb3:
						ref := c.ref(r.u2())
						if ref.owner == owner && last != "" && !strings.ContainsAny(last, " /") {
							if _, seen := fields[ref.name]; !seen {
								fields[ref.name] = strings.TrimPrefix(last, "minecraft:")
							}
						}
						last = ""
					default:
						if !skipOperands(&r, op) {
							r.p = len(r.b) // unknown layout: stop, keep what was read
						}
					}
				}
			}
		}
	}
	if path, ok := fields[name]; ok {
		return path
	}
	if path, ok := vanillaKeyPaths[owner+"."+name]; ok {
		return path // holder class not on the mod classpath
	}
	return registryPath(name)
}

// vanillaKeyPaths are the 1.20.1 vanilla key fields whose name is not the
// registry path (Noises.<clinit>); every other vanilla Blocks/Biomes/Noises
// field is its lowercased path. TestReadSurfaceRulesBOP checks the clinit read.
var vanillaKeyPaths = map[string]string{
	"net/minecraft/world/level/levelgen/Noises.SHIFT": "offset",
	"net/minecraft/world/level/levelgen/Noises.SWAMP": "surface_swamp",
}

// skipOperands advances past op's operands for straight-line clinit scans.
func skipOperands(r *reader, op int) bool {
	switch {
	case op == 0x10 || op == 0x15 || op == 0x16 || op == 0x17 || op == 0x18 || op == 0x19 || op == 0x36 || op == 0x37 || op == 0x38 || op == 0x39 || op == 0x3a || op == 0xbc || op == 0xa9:
		r.p++
	case op == 0x11 || (op >= 0x99 && op <= 0xa8) || op == 0xb2 || op == 0xb4 || op == 0xb5 || (op >= 0xb6 && op <= 0xb8) || op == 0xbb || op == 0xbd || op == 0xc0 || op == 0xc1 || op == 0xc6 || op == 0xc7 || op == 0x84:
		r.p += 2
	case op == 0xb9 || op == 0xba || op == 0xc8 || op == 0xc9:
		r.p += 4
	case op == 0xc5:
		r.p += 3
	case op == 0xaa || op == 0xab || op == 0xc4:
		return false
	}
	return r.p <= len(r.b)
}

type traceKind struct{ kind, value string } // block, biome, noise, surface, category, anchor-less tokens
type traceArray struct{ values []any }
type traceBuilder struct{ values []any }
type traceLambda struct{}

func (t *surfaceTracer) class(name string) *class {
	if c := t.classes[name]; c != nil {
		return c
	}
	data := t.v.inspectClass(name)
	if data == nil {
		fail("surface rule class %s not in archive", name)
	}
	c := parseClass(data, t.symbols)
	t.classes[name] = c
	return c
}

func (t *surfaceTracer) static(owner, name string) any {
	fields, ok := t.statics[owner]
	if !ok {
		fields = map[string]any{}
		t.statics[owner] = fields
		c := t.class(owner)
		for _, m := range c.methods {
			if m.name == "<clinit>" {
				t.run(c, m, nil, 0)
			}
		}
	}
	value, ok := fields[name]
	if !ok {
		fail("surface rule static %s.%s is not a traced constant", owner, name)
	}
	return value
}

func (t *surfaceTracer) local(owner string) bool {
	return t.v.inspectClass(owner) != nil && !strings.HasPrefix(owner, "net/minecraft/") && !strings.HasPrefix(owner, "terrablender/")
}

func registryPath(name string) string { return strings.ToLower(name) }

// field resolves a getstatic outside the traced class's own constants.
func (t *surfaceTracer) field(r reference) any {
	switch {
	case r.owner == "net/minecraft/world/level/levelgen/SurfaceRules":
		switch r.name {
		case "ON_FLOOR":
			return stoneDepth(0, false, 0, "floor")
		case "UNDER_FLOOR":
			return stoneDepth(0, true, 0, "floor")
		case "DEEP_UNDER_FLOOR":
			return stoneDepth(0, true, 6, "floor")
		case "VERY_DEEP_UNDER_FLOOR":
			return stoneDepth(0, true, 30, "floor")
		case "ON_CEILING":
			return stoneDepth(0, false, 0, "ceiling")
		case "UNDER_CEILING":
			return stoneDepth(0, true, 0, "ceiling")
		}
	case r.owner == "net/minecraft/world/level/levelgen/placement/CaveSurface":
		switch r.name {
		case "FLOOR", "CEILING":
			return traceKind{"surface", strings.ToLower(r.name)}
		}
	case r.owner == "terrablender/api/SurfaceRuleManager$RuleCategory":
		return traceKind{"category", r.name}
	case r.owner == "net/minecraft/world/level/block/Blocks" && r.desc == "Lnet/minecraft/world/level/block/Block;":
		return traceKind{"block", "minecraft:" + t.keyPath(r.owner, r.name)}
	case r.owner == "net/minecraft/world/level/biome/Biomes" && r.desc == "Lnet/minecraft/resources/ResourceKey;":
		return traceKind{"biome", "minecraft:" + t.keyPath(r.owner, r.name)}
	case r.owner == "net/minecraft/world/level/levelgen/Noises" && r.desc == "Lnet/minecraft/resources/ResourceKey;":
		return traceKind{"noise", "minecraft:" + t.keyPath(r.owner, r.name)}
	case t.modID != "" && t.local(r.owner) && r.desc == "Lnet/minecraft/world/level/block/Block;" && strings.HasSuffix(r.owner, "Blocks"):
		// Mod block holders name fields after registry paths (checked by tests).
		return traceKind{"block", t.modID + ":" + t.keyPath(r.owner, r.name)}
	case t.modID != "" && t.local(r.owner) && r.desc == "Lnet/minecraft/resources/ResourceKey;" && strings.HasSuffix(r.owner, "Biomes"):
		return traceKind{"biome", t.modID + ":" + t.keyPath(r.owner, r.name)}
	case t.local(r.owner):
		return t.static(r.owner, r.name)
	}
	fail("surface rule field %s.%s", r.owner, r.name)
	return nil
}

func stoneDepth(offset int, addSurface bool, secondary int, surface string) map[string]any {
	return map[string]any{"type": "minecraft:stone_depth", "offset": offset, "add_surface_depth": addSurface, "secondary_depth_range": secondary, "surface_type": surface}
}

func traceInt(x any) int {
	switch n := x.(type) {
	case int32:
		return int(n)
	case int:
		return n
	}
	fail("surface rule expected int, got %T", x)
	return 0
}

func traceDouble(x any) float64 {
	if d, ok := x.(float64); ok {
		return d
	}
	fail("surface rule expected double, got %T", x)
	return 0
}

func traceToken(x any, kind string) string {
	if k, ok := x.(traceKind); ok && k.kind == kind {
		return k.value
	}
	fail("surface rule expected %s, got %v", kind, x)
	return ""
}

func traceNode(x any) map[string]any {
	if m, ok := x.(map[string]any); ok {
		return m
	}
	fail("surface rule expected rule/condition, got %T", x)
	return nil
}

func (t *surfaceTracer) invoke(r reference, args []any, depth int) (any, bool) {
	sr := r.owner == "net/minecraft/world/level/levelgen/SurfaceRules"
	switch {
	case sr && r.name == "ifTrue":
		return map[string]any{"type": "minecraft:condition", "if_true": traceNode(args[0]), "then_run": traceNode(args[1])}, true
	case sr && r.name == "sequence":
		a, ok := args[0].(*traceArray)
		if !ok {
			fail("sequence expects an array")
		}
		seq := make([]any, len(a.values))
		for i, x := range a.values {
			seq[i] = traceNode(x)
		}
		return map[string]any{"type": "minecraft:sequence", "sequence": seq}, true
	case sr && r.name == "state":
		return map[string]any{"type": "minecraft:block", "result_state": map[string]any{"Name": traceToken(args[0], "state")}}, true
	case sr && r.name == "not":
		return map[string]any{"type": "minecraft:not", "invert": traceNode(args[0])}, true
	case sr && (r.name == "yBlockCheck" || r.name == "yStartCheck"):
		return map[string]any{"type": "minecraft:y_above", "anchor": traceNode(args[0]), "surface_depth_multiplier": traceInt(args[1]), "add_stone_depth": r.name == "yStartCheck"}, true
	case sr && (r.name == "waterBlockCheck" || r.name == "waterStartCheck"):
		return map[string]any{"type": "minecraft:water", "offset": traceInt(args[0]), "surface_depth_multiplier": traceInt(args[1]), "add_stone_depth": r.name == "waterStartCheck"}, true
	case sr && r.name == "isBiome":
		a, ok := args[0].(*traceArray)
		if !ok {
			fail("isBiome expects an array")
		}
		var ids []any
		for _, x := range a.values {
			ids = append(ids, traceToken(x, "biome"))
		}
		return map[string]any{"type": "minecraft:biome", "biome_is": ids}, true
	case sr && r.name == "noiseCondition" && r.desc == "(Lnet/minecraft/resources/ResourceKey;D)Lnet/minecraft/world/level/levelgen/SurfaceRules$ConditionSource;":
		return map[string]any{"type": "minecraft:noise_threshold", "noise": traceToken(args[0], "noise"), "min_threshold": traceDouble(args[1]), "max_threshold": math.MaxFloat64}, true
	case sr && r.name == "noiseCondition" && r.desc == "(Lnet/minecraft/resources/ResourceKey;DD)Lnet/minecraft/world/level/levelgen/SurfaceRules$ConditionSource;":
		return map[string]any{"type": "minecraft:noise_threshold", "noise": traceToken(args[0], "noise"), "min_threshold": traceDouble(args[1]), "max_threshold": traceDouble(args[2])}, true
	case sr && r.name == "verticalGradient":
		s, ok := args[0].(string)
		if !ok {
			fail("verticalGradient expects a name")
		}
		return map[string]any{"type": "minecraft:vertical_gradient", "random_name": s, "true_at_and_below": traceNode(args[1]), "false_at_and_above": traceNode(args[2])}, true
	case sr && r.name == "steep":
		return map[string]any{"type": "minecraft:steep"}, true
	case sr && r.name == "hole":
		return map[string]any{"type": "minecraft:hole"}, true
	case sr && r.name == "abovePreliminarySurface":
		return map[string]any{"type": "minecraft:above_preliminary_surface"}, true
	case sr && r.name == "stoneDepthCheck" && len(args) == 3:
		return stoneDepth(traceInt(args[0]), traceInt(args[1]) != 0, 0, traceToken(args[2], "surface")), true
	case sr && r.name == "stoneDepthCheck" && len(args) == 4:
		return stoneDepth(traceInt(args[0]), traceInt(args[1]) != 0, traceInt(args[2]), traceToken(args[3], "surface")), true
	case r.owner == "net/minecraft/world/level/levelgen/VerticalAnchor":
		switch r.name {
		case "absolute":
			return map[string]any{"absolute": traceInt(args[0])}, true
		case "aboveBottom":
			return map[string]any{"above_bottom": traceInt(args[0])}, true
		case "belowTop":
			return map[string]any{"below_top": traceInt(args[0])}, true
		case "bottom":
			return map[string]any{"above_bottom": 0}, true
		case "top":
			return map[string]any{"below_top": 0}, true
		}
	case (r.name == "defaultBlockState" || r.name == "m_49966_") && r.desc == "()Lnet/minecraft/world/level/block/state/BlockState;":
		return traceKind{"state", traceToken(args[0], "block")}, true
	case r.owner == "com/google/common/collect/ImmutableList" && r.name == "builder":
		return &traceBuilder{}, true
	case r.owner == "com/google/common/collect/ImmutableList$Builder" && r.name == "add" && len(args) == 2:
		b, ok := args[0].(*traceBuilder)
		if !ok {
			fail("builder add receiver")
		}
		b.values = append(b.values, args[1])
		return b, true
	case r.owner == "com/google/common/collect/ImmutableList$Builder" && r.name == "build":
		b, ok := args[0].(*traceBuilder)
		if !ok {
			fail("builder build receiver")
		}
		return &traceArray{append([]any(nil), b.values...)}, true
	case r.owner == "com/google/common/collect/ImmutableList" && r.name == "toArray" && len(args) == 2:
		a, ok := args[0].(*traceArray)
		if !ok {
			fail("toArray receiver")
		}
		return &traceArray{append([]any(nil), a.values...)}, true
	case r.owner == surfaceManager && r.name == "addSurfaceRules" && len(args) == 3:
		ns, ok := args[1].(string)
		if !ok {
			fail("addSurfaceRules namespace")
		}
		t.record(traceToken(args[0], "category"), ns, traceNode(args[2]))
		return nil, false
	case t.local(r.owner):
		c := t.class(r.owner)
		for _, m := range c.methods {
			if m.name == r.name && m.desc == r.desc {
				return t.run(c, m, args, depth+1), !strings.HasSuffix(r.desc, ")V")
			}
		}
	}
	fail("surface rule call %s.%s%s", r.owner, r.name, r.desc)
	return nil, false
}

// argCount counts JVM descriptor parameters (receiver excluded).
func argCount(desc string) (n int) {
	for i := 1; i < len(desc) && desc[i] != ')'; i++ {
		for desc[i] == '[' {
			i++
		}
		if desc[i] == 'L' {
			i = strings.IndexByte(desc[i:], ';') + i
		}
		n++
	}
	return n
}

func (t *surfaceTracer) run(c *class, m method, args []any, depth int) any {
	if depth > 32 {
		fail("surface rule call depth")
	}
	if len(m.handlers) != 0 {
		fail("surface rule method %s.%s has exception handlers", c.name, m.name)
	}
	locals := map[int]any{}
	slot := 0
	if m.access&0x0008 == 0 {
		fail("surface rule method %s.%s is not static", c.name, m.name)
	}
	for i, a := range args {
		locals[slot] = a
		slot++
		_ = i
		if _, wide := a.(float64); wide {
			slot++
		}
	}
	var stack []any
	push := func(x any) { stack = append(stack, x) }
	pop := func() any {
		if len(stack) == 0 {
			fail("surface rule stack underflow")
		}
		x := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		return x
	}
	popN := func(n int) []any {
		out := make([]any, n)
		for i := n - 1; i >= 0; i-- {
			out[i] = pop()
		}
		return out
	}
	r := reader{b: m.code}
	for r.p < len(r.b) {
		t.steps++
		if t.steps > 2_000_000 {
			fail("surface rule instruction budget")
		}
		op := r.u1()
		switch {
		case op == 0x01:
			push(nil)
		case op >= 0x02 && op <= 0x08:
			push(int32(op - 3))
		case op == 0x0e || op == 0x0f:
			push(float64(op - 0x0e))
		case op == 0x10:
			push(int32(int8(r.u1())))
		case op == 0x11:
			push(int32(int16(r.u2())))
		case op == 0x12:
			push(c.literal(r.u1()))
		case op == 0x13 || op == 0x14:
			push(c.literal(r.u2()))
		case op == 0x15 || op == 0x18 || op == 0x19:
			push(locals[r.u1()])
		case op >= 0x1a && op <= 0x1d:
			push(locals[op-0x1a])
		case op >= 0x26 && op <= 0x29:
			push(locals[op-0x26])
		case op >= 0x2a && op <= 0x2d:
			push(locals[op-0x2a])
		case op == 0x36 || op == 0x39 || op == 0x3a:
			locals[r.u1()] = pop()
		case op >= 0x3b && op <= 0x3e:
			locals[op-0x3b] = pop()
		case op >= 0x47 && op <= 0x4a:
			locals[op-0x47] = pop()
		case op >= 0x4b && op <= 0x4e:
			locals[op-0x4b] = pop()
		case op == 0x53:
			x, i, a := pop(), traceInt(pop()), pop()
			arr, ok := a.(*traceArray)
			if !ok || i < 0 || i >= len(arr.values) {
				fail("surface rule aastore")
			}
			arr.values[i] = x
		case op == 0x57:
			pop()
		case op == 0x59:
			x := pop()
			push(x)
			push(x)
		case op == 0x63 || op == 0x67 || op == 0x6b || op == 0x6f:
			b, a := traceDouble(pop()), traceDouble(pop())
			switch op {
			case 0x63:
				push(a + b)
			case 0x67:
				push(a - b)
			case 0x6b:
				push(a * b)
			default:
				push(a / b)
			}
		case op == 0x77:
			push(-traceDouble(pop()))
		case op == 0x87: // i2d
			push(float64(traceInt(pop())))
		case op == 0xb0:
			return pop()
		case op == 0xb1:
			return nil
		case op == 0xb2:
			ref := c.ref(r.u2())
			if ref.owner == c.name {
				if fields := t.statics[c.name]; fields != nil {
					if x, ok := fields[ref.name]; ok {
						push(x)
						continue
					}
				}
			}
			push(t.field(ref))
		case op == 0xb3:
			ref := c.ref(r.u2())
			if t.statics[ref.owner] == nil {
				t.statics[ref.owner] = map[string]any{}
			}
			t.statics[ref.owner][ref.name] = pop()
		case op == 0xb6 || op == 0xb8 || op == 0xb9:
			ref := c.ref(r.u2())
			if op == 0xb9 {
				r.take(2)
			}
			n := argCount(ref.desc)
			if op != 0xb8 {
				n++
			}
			result, returns := t.invoke(ref, popN(n), depth)
			if returns {
				push(result)
			}
		case op == 0xb7:
			ref := c.ref(r.u2())
			if ref.owner != "java/lang/Object" || ref.name != "<init>" {
				fail("surface rule invokespecial %s.%s", ref.owner, ref.name)
			}
			pop()
		case op == 0xba:
			r.take(4)
			push(traceLambda{})
		case op == 0xbd:
			r.u2()
			n := traceInt(pop())
			if n < 0 || n > 65536 {
				fail("surface rule array size")
			}
			push(&traceArray{make([]any, n)})
		case op == 0xc0:
			r.u2()
		default:
			fail("surface rule opcode 0x%02x in %s.%s", op, c.name, m.name)
		}
	}
	fail("surface rule method %s.%s fell off its code", c.name, m.name)
	return nil
}
