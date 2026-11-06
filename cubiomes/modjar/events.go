package modjar

import (
	"cmp"
	"slices"
	"strings"
)

type eventListener struct {
	callback *lambda
	event    string
	priority int
	modID    string
	native   func(any)
}

type queuedWork struct {
	callback *lambda
	modID    string
}

func (h *bindings) withMod(id string, run func()) {
	previous := h.options.ModID
	defer func() { h.options.ModID = previous }()
	if id != "" {
		h.options.ModID = id
	}
	run()
}

func (h *bindings) drainWork() {
	h.inWork = true
	defer func() { h.inWork = false }()
	for i := 0; i < len(h.work); i++ {
		work := h.work[i]
		h.withMod(work.modID, func() { h.runWork(work) })
	}
}

// contentRegistryGap is the message Forge RegistryObject.get throws when its
// value is absent. The registration VM populates worldgen registries only, so
// block/item/entity RegistryObjects stay empty. Forge DeferredWorkQueue runs
// every task independently; a task that needs gameplay content cannot affect
// biome regions, so it is skipped and reported instead of aborting the mod.
const contentRegistryGap = "Registry Object not present: "

const gameValueGap = "branch depends on unavailable game value"

// gameValueKeyGap is the same gap reached through a hash collection keyed by
// a content value (e.g. a block-to-stripped-block map).
const gameValueKeyGap = "collection key depends on unavailable game value"

func (h *bindings) runWork(work queuedWork) {
	defer func() {
		if p := recover(); p != nil {
			if f, ok := p.(fault); ok && (strings.HasPrefix(f.message, gameValueGap) || strings.HasPrefix(f.message, gameValueKeyGap)) {
				// Same gap through the game JAR: content state (fluids, blocks)
				// is not modelled, and control flow cannot continue past it.
				h.skippedWork = append(h.skippedWork, work.modID+": "+f.message)
				return
			}
			if e, ok := p.(javaException); ok && e.class == "java/lang/NullPointerException" && strings.HasPrefix(e.message, contentRegistryGap) {
				h.skippedWork = append(h.skippedWork, work.modID+": "+e.message)
				return
			}
			// Any other interpreter gap or exception in one queued task: the
			// installed game ran it, so it is reported and the queue goes on
			// (later tasks may register regions). Budget/cancel still abort.
			if msg := tolerate(func() { panic(p) }); msg != "" {
				h.skippedWork = append(h.skippedWork, work.modID+": "+msg)
			}
		}
	}()
	h.vm.callLambda(work.callback, nil)
}

func eventPriority(name string) int {
	switch name {
	case "HIGHEST":
		return 0
	case "HIGH":
		return 1
	case "", "NORMAL":
		return 2
	case "LOW":
		return 3
	case "LOWEST":
		return 4
	default:
		fail("unbound event priority %s", name)
		return 0
	}
}

func (h *bindings) addListener(args []any) {
	if len(h.listeners) >= 1024 || len(args) == 0 {
		fail("invalid or excessive event listeners")
	}
	e := eventListener{priority: 2, modID: h.options.ModID}
	switch x := args[len(args)-1].(type) {
	case *lambda:
		e.callback = x
	case *object:
		// A Consumer implementation object (e.g. Registrate's
		// OneTimeEventReceiver): the bus calls its accept method. Its event
		// type must come from an explicit class argument.
		if x == nil {
			fail("event listener is not a bound functional interface")
		}
		target := x
		e.native = func(value any) {
			h.vm.method(reference{"java/util/function/Consumer", "accept", "(Ljava/lang/Object;)V"}, target, true, []any{value})
		}
	default:
		fail("event listener is not a bound functional interface")
	}
	for _, arg := range args[:len(args)-1] {
		switch value := arg.(type) {
		case classLiteral:
			e.event = "L" + strings.TrimSuffix(strings.TrimPrefix(string(value), "L"), ";") + ";"
		case int32: // receiveCanceled; lifecycle/config events are not canceled.
		case *object:
			if frameworkName(value.class) != "net/minecraftforge/eventbus/api/EventPriority" {
				fail("unbound listener option %s", value.class)
			}
			e.priority = value.data.(int)
		default:
			fail("unbound listener option")
		}
	}
	if e.event == "" {
		if e.callback == nil {
			fail("event listener object requires an explicit event class")
		}
		types, _ := descriptor(e.callback.instantiated)
		if len(types) != 1 {
			fail("event listener requires one concrete event argument")
		}
		e.event = types[0]
	}
	e.event = frameworkName(e.event)
	h.listeners = append(h.listeners, e)
}

func (h *bindings) fireEvent(event string, value any) {
	h.fireEventForMod(event, value, "")
}

func (h *bindings) fireEventForMod(event string, value any, modID string) {
	listeners := slices.Clone(h.listeners)
	rank := map[string]int{}
	for i, id := range h.moduleOrder {
		rank[id] = i
	}
	slices.SortStableFunc(listeners, func(a, b eventListener) int {
		if order := cmp.Compare(rank[a.modID], rank[b.modID]); order != 0 {
			return order
		}
		return cmp.Compare(a.priority, b.priority)
	})
	for _, listener := range listeners {
		if modID != "" && listener.modID != modID {
			continue
		}
		matches := listener.event == event || event == "Lnet/minecraftforge/fml/event/config/ModConfigEvent$Loading;" && listener.event == "Lnet/minecraftforge/fml/event/config/ModConfigEvent;"
		if matches {
			call := func() {
				h.withMod(listener.modID, func() {
					if listener.native != nil {
						listener.native(value)
					} else {
						h.vm.callLambda(listener.callback, []any{value})
					}
				})
			}
			if h.tolerateListeners {
				// The installed game completed this lifecycle event; an
				// interpreter gap in one listener must not drop work it
				// already enqueued or later listeners.
				if msg := tolerate(call); msg != "" {
					h.lifecycleFaults = append(h.lifecycleFaults, listener.modID+" "+event+": "+msg)
				}
				continue
			}
			call()
		}
	}
}

func (h *bindings) subscribe(target any) {
	var owner string
	static := false
	switch x := target.(type) {
	case classLiteral:
		owner = string(x)
		static = true
	case *object:
		owner = x.class
	default:
		fail("invalid event subscriber")
	}
	seen := map[string]bool{}
	for owner != "" && h.vm.fetchClass(owner) {
		c := h.vm.load(owner)
		for _, m := range c.methods {
			key := m.name + m.desc
			if seen[key] {
				continue
			}
			seen[key] = true
			a, ok := findAnnotation(m.annotations, "Lnet/minecraftforge/eventbus/api/SubscribeEvent;")
			if !ok || static != (m.access&8 != 0) {
				continue
			}
			args, ret := descriptor(m.desc)
			// NeoForge's bus (8.x) binds @SubscribeEvent methods through a
			// private lookup, so any access level is accepted there; Forge's
			// ASM handler requires public methods.
			if len(args) != 1 || ret != "V" || m.access&1 == 0 && h.options.Loader != "neoforge" {
				fail("invalid event subscriber %s.%s", owner, m.name)
			}
			l := &lambda{target: reference{owner, m.name, m.desc}, kind: 6, instantiated: m.desc}
			if !static {
				l.kind = 5
				l.captured = []any{target}
			}
			priority, _ := a.values["priority"].(enumValue)
			if len(h.listeners) >= 1024 {
				fail("event listener budget exceeded")
			}
			h.listeners = append(h.listeners, eventListener{callback: l, event: frameworkName(args[0]), priority: eventPriority(priority.name), modID: h.options.ModID})
		}
		owner = c.super
	}
}

func annotationSide(a annotation, key, side string) bool {
	values, exists := a.values[key]
	if !exists {
		return true
	}
	if side == "" {
		side = "CLIENT"
	}
	for _, value := range values.([]any) {
		if value.(enumValue).name == side {
			return true
		}
	}
	return false
}
