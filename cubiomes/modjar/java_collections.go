package modjar

import "strings"

type javaList struct {
	values          []any
	readonly, fixed bool
	generation      int
	copyOnWrite     bool
	emptyLegacy     bool
}
type javaListIterator struct {
	list              *javaList
	index, generation int
}
type javaPattern struct {
	source string
	flags  int32
}
type javaMatcher struct {
	pattern javaPattern
	input   string
	// match holds byte offsets of the last successful find (nil if none);
	// next is the byte offset where find() resumes.
	match []int
	next  int
}
type javaMap struct {
	entries           []javaPair
	identity, nonnull bool
	readonly          bool
	baseline          string
}

// An API can know a write's effect without knowing its old return value. This
// value is allowed only through an immediate bytecode POP, never a branch,
// local, field, argument, or return. It is deliberately not represented by nil.
type unusedNativeResult struct{ reason string }

type javaListMultimap struct {
	backing  *object
	supplier any
}

type javaSet struct {
	values   []any
	nonnull  bool
	readonly bool
	// cmp orders a comparator-sorted set; elements comparing 0 are duplicates.
	cmp any
}

func (v *machine) reserveCollection(n int) {
	if n < 0 || n > 131072-v.slots {
		fail("collection slot budget exceeded")
	}
	v.slots += n
}

func (h *bindings) collectionCall(r reference, receiver any, args []any) (any, bool) {
	if receiver != nil && r.name == "toArray" && r.desc == "(Ljava/util/function/IntFunction;)[Ljava/lang/Object;" {
		// Collection.toArray(IntFunction) default: toArray(generator.apply(0)).
		values := h.vm.collectionValues(receiver)
		proto := h.vm.method(reference{"java/util/function/IntFunction", "apply", "(I)Ljava/lang/Object;"}, args[0], true, []any{int32(0)})
		desc := "[Ljava/lang/Object;"
		if a, ok := proto.(*array); ok {
			desc = a.descriptor
		}
		a := h.vm.objectArray(len(values), desc)
		copy(a.values, values)
		return a, true
	}
	if receiver != nil && r.name == "toArray" && r.desc == "([Ljava/lang/Object;)[Ljava/lang/Object;" {
		// Collection.toArray(T[]): fill the argument when it fits (nulling the
		// slot after the last element), otherwise allocate the same type.
		dst, ok := args[0].(*array)
		if !ok || dst == nil {
			raiseJava("java/lang/NullPointerException", "toArray")
		}
		values := h.vm.collectionValues(receiver)
		if len(values) <= len(dst.values) {
			copy(dst.values, values)
			if len(values) < len(dst.values) {
				dst.values[len(values)] = nil
			}
			return dst, true
		}
		a := h.vm.objectArray(len(values), dst.descriptor)
		copy(a.values, values)
		return a, true
	}
	v := h.vm
	if r.owner == "com/google/common/collect/Sets" && r.name == "newHashSet" && r.desc == "()Ljava/util/HashSet;" {
		return v.alloc("java/util/HashSet", &javaSet{}), true
	}
	if r.owner == "java/util/Set" && r.name == "of" && receiver == nil {
		types, ret := descriptor(r.desc)
		if ret != "Ljava/util/Set;" {
			return nil, false
		}
		values := args
		if len(types) == 1 && types[0] == "[Ljava/lang/Object;" {
			values = args[0].(*array).values
		} else {
			for _, typ := range types {
				if typ != "Ljava/lang/Object;" {
					return nil, false
				}
			}
		}
		set := &javaSet{nonnull: true, readonly: true}
		for _, x := range values {
			if x == nil {
				raiseJava("java/lang/NullPointerException", "null set element")
			}
			v.requirePureCollectionKey(x)
			for _, old := range set.values {
				if v.collectionEquals(x, old, false) {
					raiseJava("java/lang/IllegalArgumentException", "duplicate set element")
				}
			}
			v.reserveCollection(1)
			set.values = append(set.values, x)
		}
		return v.alloc(r.owner, set), true
	}
	if r.owner == "java/util/Map" && r.name == "of" && receiver == nil {
		types, ret := descriptor(r.desc)
		if ret != "Ljava/util/Map;" || len(types)%2 != 0 {
			return nil, false
		}
		for _, typ := range types {
			if typ != "Ljava/lang/Object;" {
				return nil, false
			}
		}
		m := &javaMap{nonnull: true, readonly: true}
		for i := 0; i < len(args); i += 2 {
			if args[i] == nil || args[i+1] == nil {
				raiseJava("java/lang/NullPointerException", "null map entry")
			}
			v.requirePureCollectionKey(args[i])
			for _, e := range m.entries {
				if v.collectionEquals(args[i], e.left, false) {
					raiseJava("java/lang/IllegalArgumentException", "duplicate map key")
				}
			}
			v.reserveCollection(2)
			m.entries = append(m.entries, javaPair{args[i], args[i+1]})
		}
		return v.alloc(r.owner, m), true
	}
	if r.owner == "com/google/common/collect/Lists" && r.name == "newLinkedList" && r.desc == "()Ljava/util/LinkedList;" {
		return v.alloc("java/util/LinkedList", &javaList{}), true
	}
	if r.owner == "java/util/concurrent/ConcurrentHashMap" && r.name == "newKeySet" && r.desc == "()Ljava/util/concurrent/ConcurrentHashMap$KeySetView;" {
		return v.alloc("java/util/concurrent/ConcurrentHashMap$KeySetView", &javaSet{nonnull: true}), true
	}
	if r.owner == "com/google/common/collect/Maps" && r.name == "newIdentityHashMap" && r.desc == "()Ljava/util/IdentityHashMap;" {
		return v.alloc("java/util/IdentityHashMap", &javaMap{identity: true}), true
	}
	if (r.owner == "java/util/HashSet" || r.owner == "java/util/LinkedHashSet" || r.owner == "java/util/TreeSet") && r.name == "<init>" && r.desc == "(Ljava/util/Collection;)V" {
		set := &javaSet{}
		receiver.(*object).data = set
		for _, x := range v.collectionValues(args[0]) {
			v.method(reference{r.owner, "add", "(Ljava/lang/Object;)Z"}, receiver, true, []any{x})
		}
		return nil, true
	}
	if (r.owner == "java/util/TreeSet" || r.owner == "java/util/concurrent/ConcurrentSkipListSet") && r.name == "<init>" && (r.desc == "(Ljava/util/Comparator;)V" || r.desc == "()V") {
		cmp := any(nil)
		if len(args) == 1 {
			cmp = args[0]
		}
		if cmp == nil {
			// A null or absent comparator means natural ordering.
			cmp = v.alloc("java/util/Comparator", &javaComparator{kind: "natural"})
		}
		receiver.(*object).data = &javaSet{cmp: cmp, nonnull: r.owner == "java/util/concurrent/ConcurrentSkipListSet"}
		return nil, true
	}
	if o, ok := receiver.(*object); ok && o != nil {
		if set, ok := o.data.(*javaSet); ok && set.cmp != nil && r.name == "add" && r.desc == "(Ljava/lang/Object;)Z" {
			if args[0] == nil {
				raiseJava("java/lang/NullPointerException", "null sorted set element")
			}
			at := len(set.values)
			for i, x := range set.values {
				c := integer(v.method(reference{"java/util/Comparator", "compare", "(Ljava/lang/Object;Ljava/lang/Object;)I"}, set.cmp, true, []any{args[0], x}))
				if c == 0 {
					return int32(0), true
				}
				if c < 0 {
					at = i
					break
				}
			}
			v.reserveCollection(1)
			set.values = append(set.values[:at], append([]any{args[0]}, set.values[at:]...)...)
			return int32(1), true
		}
	}
	if (r.owner == "java/util/HashSet" || r.owner == "java/util/LinkedHashSet") && r.name == "<init>" && (r.desc == "()V" || r.desc == "(I)V" || r.desc == "(IF)V") {
		receiver.(*object).data = &javaSet{}
		return nil, true
	}
	if receiver != nil && (r.owner == "java/util/HashSet" || r.owner == "java/util/LinkedHashSet" || r.owner == "java/util/TreeSet" || r.owner == "java/util/concurrent/ConcurrentSkipListSet" || r.owner == "java/util/SortedSet" || r.owner == "java/util/NavigableSet" || r.owner == "java/util/EnumSet" || r.owner == "java/util/concurrent/ConcurrentHashMap$KeySetView" || r.owner == "java/util/Set" || r.owner == "java/util/Collection") {
		if set, ok := objectData(receiver).(*javaSet); ok {
			if r.name == "clear" && r.desc == "()V" {
				if set.readonly {
					raiseJava("java/lang/UnsupportedOperationException", "immutable set")
				}
				set.values = nil
				return nil, true
			}
			if r.name == "size" && r.desc == "()I" {
				return int32(len(set.values)), true
			}
			if r.name == "iterator" && r.desc == "()Ljava/util/Iterator;" {
				return v.alloc("java/util/Iterator", &javaListIterator{list: &javaList{values: set.values, readonly: true}, generation: 0}), true
			}
			if r.name == "stream" && r.desc == "()Ljava/util/stream/Stream;" {
				return v.alloc("java/util/stream/Stream", &javaList{values: append([]any(nil), set.values...), readonly: true}), true
			}
			if r.name == "forEach" && r.desc == "(Ljava/util/function/Consumer;)V" {
				if args[0] == nil {
					raiseJava("java/lang/NullPointerException", "null forEach consumer")
				}
				for _, item := range set.values {
					v.tick()
					v.method(reference{"java/util/function/Consumer", "accept", "(Ljava/lang/Object;)V"}, args[0], true, []any{item})
				}
				return nil, true
			}
			if r.name == "addAll" && r.desc == "(Ljava/util/Collection;)Z" {
				if set.readonly {
					raiseJava("java/lang/UnsupportedOperationException", "immutable set")
				}
				if args[0] == nil {
					raiseJava("java/lang/NullPointerException", "null collection")
				}
				changed := int32(0)
				var items []any
				switch s := objectData(args[0]).(type) {
				case *javaSet:
					items = s.values
				case *javaList:
					items = s.values
				default:
					fail("unbound collection addAll source")
				}
				for _, item := range items {
					v.tick()
					if item == nil && set.nonnull {
						raiseJava("java/lang/NullPointerException", "null concurrent set value")
					}
					v.requirePureCollectionKey(item)
					found := false
					for _, existing := range set.values {
						if v.collectionEquals(item, existing, false) {
							found = true
							break
						}
					}
					if !found {
						v.reserveCollection(1)
						set.values = append(set.values, item)
						changed = 1
					}
				}
				return changed, true
			}
			if r.name == "isEmpty" && r.desc == "()Z" {
				if len(set.values) == 0 {
					return int32(1), true
				}
				return int32(0), true
			}
			if r.name == "remove" && r.desc == "(Ljava/lang/Object;)Z" {
				if set.readonly {
					raiseJava("java/lang/UnsupportedOperationException", "immutable set")
				}
				for i, x := range set.values {
					if v.collectionEquals(args[0], x, false) {
						set.values = append(set.values[:i:i], set.values[i+1:]...)
						return int32(1), true
					}
				}
				return int32(0), true
			}
			if (r.name == "contains" || r.name == "add") && r.desc == "(Ljava/lang/Object;)Z" {
				if r.name == "add" && set.readonly {
					raiseJava("java/lang/UnsupportedOperationException", "immutable set")
				}
				if args[0] == nil && set.nonnull {
					raiseJava("java/lang/NullPointerException", "null concurrent set value")
				}
				v.requirePureCollectionKey(args[0])
				for _, x := range set.values {
					if v.collectionEquals(args[0], x, false) {
						if r.name == "add" {
							return int32(0), true
						}
						return int32(1), true
					}
				}
				if r.name == "add" {
					v.reserveCollection(1)
					set.values = append(set.values, args[0])
					return int32(1), true
				}
				return int32(0), true
			}
		}
	}
	if r.owner == "com/google/common/collect/Multimaps" && r.name == "newListMultimap" && r.desc == "(Ljava/util/Map;Lcom/google/common/base/Supplier;)Lcom/google/common/collect/ListMultimap;" {
		if args[0] == nil || args[1] == nil {
			raiseJava("java/lang/NullPointerException", "null multimap factory")
		}
		backing := args[0].(*object)
		if objectData(backing).(*javaMap).baseline != "" {
			fail("unavailable multimap backing state")
		}
		if len(backing.data.(*javaMap).entries) != 0 {
			raiseJava("java/lang/IllegalArgumentException", "nonempty multimap backing")
		}
		return v.alloc("com/google/common/collect/ListMultimap", javaListMultimap{backing, args[1]}), true
	}
	if r.owner == "com/google/common/collect/Maps" && r.name == "newHashMap" && r.desc == "()Ljava/util/HashMap;" {
		return v.alloc("java/util/HashMap", &javaMap{}), true
	}
	if r.owner == "com/google/common/collect/Maps" && r.name == "newHashMap" && r.desc == "(Ljava/util/Map;)Ljava/util/HashMap;" {
		m := &javaMap{}
		v.copyMapInto(m, args[0])
		return v.alloc("java/util/HashMap", m), true
	}
	if (r.owner == "java/util/Enumeration" || strings.HasSuffix(r.owner, "Enumeration")) && receiver != nil {
		if l, ok := objectData(receiver).(*javaList); ok {
			switch {
			case r.name == "hasMoreElements" && r.desc == "()Z":
				if len(l.values) > 0 {
					return int32(1), true
				}
				return int32(0), true
			case r.name == "nextElement" && r.desc == "()Ljava/lang/Object;":
				if len(l.values) == 0 {
					raiseJava("java/util/NoSuchElementException", "enumeration")
				}
				x := l.values[0]
				l.values = l.values[1:]
				return x, true
			}
		}
	}
	if r.owner == "com/google/common/collect/Maps" && r.name == "newTreeMap" && (r.desc == "()Ljava/util/TreeMap;" || r.desc == "(Ljava/util/Comparator;)Ljava/util/TreeMap;") {
		if r.desc != "()Ljava/util/TreeMap;" {
			// Comparator-ordered TreeMap is only safe when the key set never
			// drives registration; reject unknown ordering rather than guess.
			fail("ordered TreeMap comparator is not bound")
		}
		return v.alloc("java/util/TreeMap", &javaMap{}), true
	}
	if r.owner == "com/google/common/collect/Lists" && r.name == "newArrayList" {
		var values []any
		switch r.desc {
		case "()Ljava/util/ArrayList;":
		case "([Ljava/lang/Object;)Ljava/util/ArrayList;":
			if args[0] == nil {
				raiseJava("java/lang/NullPointerException", "null list elements")
			}
			values = args[0].(*array).values
		default:
			return nil, false
		}
		v.reserveCollection(len(values))
		return v.alloc("java/util/ArrayList", &javaList{values: append([]any(nil), values...)}), true
	}
	if (r.owner == "java/util/ArrayDeque" || r.owner == "java/util/concurrent/ConcurrentLinkedDeque") && r.name == "<init>" && r.desc == "()V" {
		receiver.(*object).data = &javaList{}
		return nil, true
	}
	if o, ok := receiver.(*object); ok && o != nil && (r.owner == "java/util/Deque" || r.owner == "java/util/ArrayDeque" || r.owner == "java/util/Queue" || r.owner == "java/util/concurrent/ConcurrentLinkedDeque") {
		if q, ok := o.data.(*javaList); ok {
			// Deque head is index 0; push/pop/peek are the stack view of that head.
			switch r.name + r.desc {
			case "push(Ljava/lang/Object;)V", "addFirst(Ljava/lang/Object;)V":
				if args[0] == nil {
					raiseJava("java/lang/NullPointerException", "null deque element")
				}
				v.reserveCollection(1)
				q.values = append([]any{args[0]}, q.values...)
				q.generation++
				return nil, true
			case "addLast(Ljava/lang/Object;)V", "add(Ljava/lang/Object;)Z", "offer(Ljava/lang/Object;)Z", "offerLast(Ljava/lang/Object;)Z":
				if args[0] == nil {
					raiseJava("java/lang/NullPointerException", "null deque element")
				}
				v.reserveCollection(1)
				q.values = append(q.values, args[0])
				q.generation++
				if r.name == "addLast" {
					return nil, true
				}
				return int32(1), true
			case "pop()Ljava/lang/Object;", "removeFirst()Ljava/lang/Object;", "remove()Ljava/lang/Object;", "poll()Ljava/lang/Object;", "pollFirst()Ljava/lang/Object;":
				if len(q.values) == 0 {
					if r.name == "poll" || r.name == "pollFirst" {
						return nil, true
					}
					raiseJava("java/util/NoSuchElementException", "empty deque")
				}
				x := q.values[0]
				q.values = q.values[1:]
				q.generation++
				return x, true
			case "peek()Ljava/lang/Object;", "peekFirst()Ljava/lang/Object;", "getFirst()Ljava/lang/Object;", "element()Ljava/lang/Object;":
				if len(q.values) == 0 {
					if r.name == "peek" || r.name == "peekFirst" {
						return nil, true
					}
					raiseJava("java/util/NoSuchElementException", "empty deque")
				}
				return q.values[0], true
			case "isEmpty()Z":
				if len(q.values) == 0 {
					return int32(1), true
				}
				return int32(0), true
			case "size()I":
				return int32(len(q.values)), true
			case "clear()V":
				q.values = nil
				q.generation++
				return nil, true
			}
		}
	}
	listOwner := r.owner == "java/util/ArrayList" || r.owner == "java/util/LinkedList" || r.owner == "java/util/Vector" || r.owner == "java/util/Stack" || r.owner == "java/util/concurrent/CopyOnWriteArrayList" || r.owner == "java/util/List" || r.owner == "java/util/Collection" || r.owner == "java/lang/Iterable"
	if r.owner == "java/util/Stack" && receiver != nil {
		if l, ok := objectData(receiver).(*javaList); ok {
			switch r.name + r.desc {
			case "push(Ljava/lang/Object;)Ljava/lang/Object;":
				v.reserveCollection(1)
				l.values = append(l.values, args[0])
				l.generation++
				return args[0], true
			case "pop()Ljava/lang/Object;", "peek()Ljava/lang/Object;":
				if len(l.values) == 0 {
					raiseJava("java/util/EmptyStackException", "")
				}
				top := l.values[len(l.values)-1]
				if r.name == "pop" {
					l.values = l.values[:len(l.values)-1]
					l.generation++
				}
				return top, true
			case "empty()Z":
				if len(l.values) == 0 {
					return int32(1), true
				}
				return int32(0), true
			}
		}
	}
	if (r.owner == "java/util/ArrayList" || r.owner == "java/util/LinkedList" || r.owner == "java/util/concurrent/CopyOnWriteArrayList") && r.name == "<init>" {
		x := &javaList{copyOnWrite: r.owner == "java/util/concurrent/CopyOnWriteArrayList"}
		switch r.desc {
		case "()V":
		case "(I)V":
			if x.copyOnWrite || r.owner == "java/util/LinkedList" {
				return nil, false
			}
			if integer(args[0]) < 0 {
				raiseJava("java/lang/IllegalArgumentException", "negative capacity")
			}
			v.reserveCollection(int(integer(args[0])))
		case "(Ljava/util/Collection;)V":
			if args[0] == nil {
				raiseJava("java/lang/NullPointerException", "Collection")
			}
			if inertOf(args[0]) != nil {
				break
			}
			switch src := objectData(args[0]).(type) {
			case *javaList:
				v.reserveCollection(len(src.values))
				x.values = append([]any(nil), src.values...)
			default:
				fail("ArrayList requires a list collection")
			}
		default:
			return nil, false
		}
		receiver.(*object).data = x
		return nil, true
	}
	if r.owner == "java/util/Arrays" && r.name == "asList" && r.desc == "([Ljava/lang/Object;)Ljava/util/List;" {
		a := args[0].(*array)
		return v.alloc("java/util/List", &javaList{values: a.values, fixed: true}), true
	}
	if (r.owner == "java/util/List" || r.owner == "java/util/Set") && r.name == "copyOf" && receiver == nil && r.desc == "(Ljava/util/Collection;)L"+r.owner+";" {
		if args[0] == nil {
			raiseJava("java/lang/NullPointerException", "copyOf")
		}
		var values []any
		for _, x := range v.collectionValues(args[0]) {
			if x == nil {
				raiseJava("java/lang/NullPointerException", "null element")
			}
			if r.owner == "java/util/Set" {
				dup := false
				for _, y := range values {
					if v.collectionEquals(x, y, false) {
						dup = true
						break
					}
				}
				if dup {
					continue
				}
			}
			values = append(values, x)
		}
		v.reserveCollection(len(values))
		if r.owner == "java/util/Set" {
			return v.alloc(r.owner, &javaSet{values: values, readonly: true, nonnull: true}), true
		}
		return v.alloc(r.owner, &javaList{values: values, readonly: true}), true
	}
	if r.owner == "java/util/List" && r.name == "of" && receiver == nil {
		types, ret := descriptor(r.desc)
		if ret != "Ljava/util/List;" {
			return nil, false
		}
		values := args
		if len(types) == 1 && types[0] == "[Ljava/lang/Object;" {
			values = args[0].(*array).values
		} else {
			for _, typ := range types {
				if typ != "Ljava/lang/Object;" {
					return nil, false
				}
			}
		}
		for _, x := range values {
			if x == nil {
				raiseJava("java/lang/NullPointerException", "null list element")
			}
		}
		v.reserveCollection(len(values))
		return v.alloc(r.owner, &javaList{values: append([]any(nil), values...), readonly: true}), true
	}
	if r.owner == "java/util/Collections" && r.name == "emptyMap" && r.desc == "()Ljava/util/Map;" {
		return v.alloc("java/util/Map", &javaMap{readonly: true}), true
	}
	if r.owner == "java/util/Collections" && r.name == "emptySet" && r.desc == "()Ljava/util/Set;" {
		return v.alloc("java/util/Set", &javaSet{readonly: true}), true
	}
	if r.owner == "java/util/Collections" && r.name == "emptyList" && r.desc == "()Ljava/util/List;" {
		return v.alloc("java/util/List", &javaList{readonly: true, emptyLegacy: true}), true
	}
	if listOwner && receiver != nil {
		x, ok := objectData(receiver).(*javaList)
		if !ok {
			return nil, false
		}
		index := func(i int32) int {
			if i < 0 || int(i) >= len(x.values) {
				raiseJava("java/lang/IndexOutOfBoundsException", "list index")
			}
			return int(i)
		}
		switch {
		case r.name == "subList" && r.desc == "(II)Ljava/util/List;":
			from, to := int(integer(args[0])), int(integer(args[1]))
			if from < 0 || to > len(x.values) || from > to {
				raiseJava("java/lang/IndexOutOfBoundsException", "subList range")
			}
			// Read-only snapshot: reads match the live view at creation time;
			// structural writes through it are not bound and fail closed.
			v.reserveCollection(to - from)
			return v.alloc("java/util/List", &javaList{values: append([]any(nil), x.values[from:to]...), readonly: true}), true
		case r.name == "size" && r.desc == "()I":
			return int32(len(x.values)), true
		case r.name == "isEmpty" && r.desc == "()Z":
			if len(x.values) == 0 {
				return int32(1), true
			}
			return int32(0), true
		case r.name == "get" && r.desc == "(I)Ljava/lang/Object;":
			return x.values[index(integer(args[0]))], true
		case r.name == "set" && r.desc == "(ILjava/lang/Object;)Ljava/lang/Object;":
			if x.readonly {
				raiseJava("java/lang/UnsupportedOperationException", "immutable list")
			}
			i := index(integer(args[0]))
			old := x.values[i]
			x.values[i] = args[1]
			return old, true
		case r.name == "remove" && r.desc == "(I)Ljava/lang/Object;":
			if x.readonly || x.fixed {
				raiseJava("java/lang/UnsupportedOperationException", "fixed list")
			}
			i := index(integer(args[0]))
			old := x.values[i]
			x.values = append(x.values[:i], x.values[i+1:]...)
			x.generation++
			return old, true
		case r.name == "add" && r.desc == "(Ljava/lang/Object;)Z":
			if x.readonly || x.fixed {
				raiseJava("java/lang/UnsupportedOperationException", "fixed list")
			}
			v.reserveCollection(1)
			x.values = append(x.values, args[0])
			x.generation++
			return int32(1), true
		case (r.owner == "java/util/ArrayList" || r.owner == "java/util/Vector") && (r.name == "trimToSize" && r.desc == "()V" || r.name == "ensureCapacity" && r.desc == "(I)V"):
			return nil, true // capacity is not observable
		case r.name == "removeIf" && r.desc == "(Ljava/util/function/Predicate;)Z":
			if args[0] == nil {
				raiseJava("java/lang/NullPointerException", "removeIf")
			}
			var keep []any
			removed := false
			for _, item := range x.values {
				v.tick()
				if integer(v.method(reference{"java/util/function/Predicate", "test", "(Ljava/lang/Object;)Z"}, args[0], true, []any{item})) != 0 {
					removed = true
				} else {
					keep = append(keep, item)
				}
			}
			if removed {
				if x.readonly || x.fixed {
					raiseJava("java/lang/UnsupportedOperationException", "fixed list")
				}
				x.values = keep
				x.generation++
				return int32(1), true
			}
			return int32(0), true
		case r.name == "add" && r.desc == "(ILjava/lang/Object;)V":
			if x.readonly || x.fixed {
				raiseJava("java/lang/UnsupportedOperationException", "fixed list")
			}
			i := int(integer(args[0]))
			if i < 0 || i > len(x.values) {
				raiseJava("java/lang/IndexOutOfBoundsException", "list index")
			}
			v.reserveCollection(1)
			x.values = append(x.values, nil)
			copy(x.values[i+1:], x.values[i:])
			x.values[i] = args[1]
			x.generation++
			return nil, true
		case r.name == "addAll" && r.desc == "(Ljava/util/Collection;)Z":
			if x.readonly || x.fixed {
				raiseJava("java/lang/UnsupportedOperationException", "fixed list")
			}
			if args[0] == nil {
				raiseJava("java/lang/NullPointerException", "null collection")
			}
			var items []any
			switch s := objectData(args[0]).(type) {
			case *javaList:
				items = s.values
			case *javaSet:
				items = s.values
			default:
				fail("unbound collection addAll source")
			}
			if len(items) == 0 {
				return int32(0), true
			}
			v.reserveCollection(len(items))
			x.values = append(x.values, items...)
			x.generation++
			return int32(1), true
		case r.name == "clear" && r.desc == "()V":
			if x.emptyLegacy {
				return nil, true
			}
			if x.readonly || x.fixed {
				raiseJava("java/lang/UnsupportedOperationException", "fixed list")
			}
			x.values = nil
			x.generation++
			return nil, true
		case r.name == "iterator" && r.desc == "()Ljava/util/Iterator;":
			if x.copyOnWrite {
				v.reserveCollection(len(x.values))
				x = &javaList{values: append([]any(nil), x.values...), readonly: true}
			}
			return v.alloc("java/util/Iterator", &javaListIterator{list: x, generation: x.generation}), true
		case r.name == "forEach" && r.desc == "(Ljava/util/function/Consumer;)V":
			if args[0] == nil {
				raiseJava("java/lang/NullPointerException", "null forEach consumer")
			}
			for _, item := range x.values {
				v.tick()
				v.method(reference{"java/util/function/Consumer", "accept", "(Ljava/lang/Object;)V"}, args[0], true, []any{item})
			}
			return nil, true
		case r.name == "toArray" && r.desc == "()[Ljava/lang/Object;":
			v.reserveCollection(len(x.values))
			v.alloc("[", nil)
			return &array{descriptor: "[Ljava/lang/Object;", values: append([]any(nil), x.values...)}, true
		}
	}
	if r.owner == "java/util/Iterator" && receiver != nil {
		if it, ok := objectData(receiver).(*javaListIterator); ok {
			switch {
			case r.name == "hasNext" && r.desc == "()Z":
				if it.index < len(it.list.values) {
					return int32(1), true
				}
				return int32(0), true
			case r.name == "next" && r.desc == "()Ljava/lang/Object;":
				if it.generation != it.list.generation {
					raiseJava("java/util/ConcurrentModificationException", "modified list")
				}
				if it.index >= len(it.list.values) {
					raiseJava("java/util/NoSuchElementException", "list exhausted")
				}
				x := it.list.values[it.index]
				it.index++
				return x, true
			}
		}
	}
	if r.owner == "com/google/common/collect/Maps" && r.name == "newConcurrentMap" && r.desc == "()Ljava/util/concurrent/ConcurrentMap;" {
		return v.alloc("java/util/concurrent/ConcurrentHashMap", &javaMap{nonnull: true}), true
	}
	mapOwner := r.owner == "java/util/HashMap" || r.owner == "java/util/WeakHashMap" || r.owner == "java/util/Hashtable" || r.owner == "java/util/LinkedHashMap" || r.owner == "java/util/TreeMap" || r.owner == "java/util/EnumMap" || r.owner == "java/util/IdentityHashMap" || r.owner == "java/util/Map" || r.owner == "java/util/concurrent/ConcurrentHashMap" || r.owner == "java/util/concurrent/ConcurrentMap" || r.owner == "it/unimi/dsi/fastutil/objects/Reference2ObjectOpenHashMap"
	if r.owner == "java/util/EnumMap" && r.name != "<init>" && strings.Contains(r.desc, "Ljava/lang/Enum;") {
		// EnumMap's bridge-free overloads (put(K extends Enum,V)) behave as
		// the erased Map methods once the key's type is checked.
		if r.name == "put" && args[0] == nil {
			raiseJava("java/lang/NullPointerException", "EnumMap key")
		}
		r.desc = strings.ReplaceAll(r.desc, "Ljava/lang/Enum;", "Ljava/lang/Object;")
	}
	if mapOwner && r.name == "<init>" && (r.desc == "()V" || r.desc == "(I)V") {
		if len(args) > 0 {
			if integer(args[0]) < 0 {
				raiseJava("java/lang/IllegalArgumentException", "negative capacity")
			}
			v.reserveCollection(int(integer(args[0])))
		}
		receiver.(*object).data = &javaMap{identity: r.owner == "java/util/IdentityHashMap" || strings.HasPrefix(r.owner, "it/unimi/"), nonnull: strings.Contains(r.owner, "concurrent/") || r.owner == "java/util/Hashtable"}
		return nil, true
	}
	if mapOwner && r.name == "<init>" && r.desc == "(Ljava/util/Map;)V" && r.owner != "java/util/EnumMap" {
		m := &javaMap{identity: r.owner == "java/util/IdentityHashMap" || strings.HasPrefix(r.owner, "it/unimi/"), nonnull: strings.Contains(r.owner, "concurrent/") || r.owner == "java/util/Hashtable"}
		v.copyMapInto(m, args[0])
		receiver.(*object).data = m
		return nil, true
	}
	if mapOwner && receiver != nil && r.name == "putAll" && r.desc == "(Ljava/util/Map;)V" {
		if m, ok := objectData(receiver).(*javaMap); ok {
			if m.readonly {
				raiseJava("java/lang/UnsupportedOperationException", "immutable map")
			}
			v.copyMapInto(m, args[0])
			return nil, true
		}
	}
	if mapOwner && receiver != nil {
		m, ok := objectData(receiver).(*javaMap)
		if !ok {
			return nil, false
		}
		if r.name == "forEach" && r.desc == "(Ljava/util/function/BiConsumer;)V" {
			if m.baseline != "" {
				fail("unavailable map baseline: %s", m.baseline)
			}
			if args[0] == nil {
				raiseJava("java/lang/NullPointerException", "null forEach action")
			}
			for _, e := range append([]javaPair(nil), m.entries...) {
				v.tick()
				v.method(reference{"java/util/function/BiConsumer", "accept", "(Ljava/lang/Object;Ljava/lang/Object;)V"}, args[0], true, []any{e.left, e.right})
			}
			return nil, true
		}
		if r.name == "clear" && r.desc == "()V" {
			if m.readonly {
				raiseJava("java/lang/UnsupportedOperationException", "immutable map")
			}
			if m.baseline != "" {
				fail("unavailable map baseline: %s", m.baseline)
			}
			m.entries = nil
			return nil, true
		}
		if r.name == "size" && r.desc == "()I" || r.name == "isEmpty" && r.desc == "()Z" {
			if m.baseline != "" {
				fail("unavailable map baseline: %s", m.baseline)
			}
			if r.name == "isEmpty" {
				if len(m.entries) == 0 {
					return int32(1), true
				}
				return int32(0), true
			}
			return int32(len(m.entries)), true
		}
		if r.name == "computeIfAbsent" && r.desc == "(Ljava/lang/Object;Ljava/util/function/Function;)Ljava/lang/Object;" {
			if m.baseline != "" {
				fail("unavailable conditional map baseline: %s", m.baseline)
			}
			if m.readonly {
				raiseJava("java/lang/UnsupportedOperationException", "immutable map")
			}
			if args[1] == nil || m.nonnull && args[0] == nil {
				raiseJava("java/lang/NullPointerException", "null mapping input")
			}
			if !m.identity {
				v.requirePureCollectionKey(args[0])
			}
			for _, e := range m.entries {
				if v.collectionEquals(args[0], e.left, m.identity) && e.right != nil {
					return e.right, true
				}
			}
			before := len(m.entries)
			x := v.method(reference{"java/util/function/Function", "apply", "(Ljava/lang/Object;)Ljava/lang/Object;"}, args[1], true, []any{args[0]})
			if m.nonnull && len(m.entries) != before {
				fail("recursive concurrent map mutation is not bound")
			}
			if !m.identity && !m.nonnull && len(m.entries) != before {
				raiseJava("java/util/ConcurrentModificationException", "mapping function changed map")
			}
			if x != nil {
				for i, e := range m.entries {
					if v.collectionEquals(args[0], e.left, m.identity) {
						m.entries[i].right = x
						return x, true
					}
				}
				v.reserveCollection(2)
				m.entries = append(m.entries, javaPair{args[0], x})
			}
			return x, true
		}
		if r.name == "getOrDefault" && r.desc == "(Ljava/lang/Object;Ljava/lang/Object;)Ljava/lang/Object;" {
			// Map.getOrDefault: the mapped value when the key is present
			// (HashMap returns a present null mapping), else the default.
			if m.baseline != "" {
				fail("unavailable map baseline: %s", m.baseline)
			}
			if !m.identity {
				v.requirePureCollectionKey(args[0])
			}
			for _, e := range m.entries {
				if v.collectionEquals(args[0], e.left, m.identity) {
					return e.right, true
				}
			}
			return args[1], true
		}
		if r.name == "containsKey" && r.desc == "(Ljava/lang/Object;)Z" {
			if m.baseline != "" {
				fail("unavailable map baseline: %s", m.baseline)
			}
			if !m.identity {
				v.requirePureCollectionKey(args[0])
			}
			for _, e := range m.entries {
				if v.collectionEquals(args[0], e.left, m.identity) {
					return int32(1), true
				}
			}
			return int32(0), true
		}
		if r.name == "hashCode" && r.desc == "()I" && !m.identity {
			if m.baseline != "" {
				fail("unavailable map baseline: %s", m.baseline)
			}
			hash := func(x any) int32 {
				if x == nil {
					return 0
				}
				return integer(v.method(reference{"java/lang/Object", "hashCode", "()I"}, x, true, nil))
			}
			var h int32
			for _, e := range m.entries {
				h += hash(e.left) ^ hash(e.right)
			}
			return h, true
		}
		if r.name == "remove" && r.desc == "(Ljava/lang/Object;)Ljava/lang/Object;" {
			if m.readonly {
				raiseJava("java/lang/UnsupportedOperationException", "immutable map")
			}
			if m.baseline != "" {
				fail("unavailable map baseline: %s", m.baseline)
			}
			if m.nonnull && args[0] == nil {
				raiseJava("java/lang/NullPointerException", "null map key")
			}
			if !m.identity {
				v.requirePureCollectionKey(args[0])
			}
			for i, e := range m.entries {
				if v.collectionEquals(args[0], e.left, m.identity) {
					m.entries = append(m.entries[:i:i], m.entries[i+1:]...)
					return e.right, true
				}
			}
			return nil, true
		}
		if r.name == "containsValue" && r.desc == "(Ljava/lang/Object;)Z" {
			if m.baseline != "" {
				fail("unavailable map baseline: %s", m.baseline)
			}
			for _, e := range m.entries {
				if v.collectionEquals(args[0], e.right, false) {
					return int32(1), true
				}
			}
			return int32(0), true
		}
		if r.name == "get" && r.desc == "(Ljava/lang/Object;)Ljava/lang/Object;" || r.name == "put" && r.desc == "(Ljava/lang/Object;Ljava/lang/Object;)Ljava/lang/Object;" || r.name == "putIfAbsent" && r.desc == "(Ljava/lang/Object;Ljava/lang/Object;)Ljava/lang/Object;" {
			if r.name != "get" && m.readonly {
				raiseJava("java/lang/UnsupportedOperationException", "immutable map")
			}
			if m.nonnull && (args[0] == nil || len(args) == 2 && args[1] == nil) {
				raiseJava("java/lang/NullPointerException", "null map key/value")
			}
			if i := inertOf(args[0]); i != nil && r.name == "put" && !m.identity && !staticGameKey(args[0]) {
				// A key built from an unavailable game value (e.g. a sound's id)
				// cannot be compared, so the map becomes a partial table: the
				// write is accepted, its previous value is itself unavailable, and every
				// later read fails closed through the baseline.
				if m.baseline == "" {
					m.baseline = inertKeyBaseline + i.origin
				}
				return v.inert("previous value of map key from "+i.origin, false), true
			}
			if r.name == "get" && strings.HasPrefix(m.baseline, inertKeyBaseline) {
				fail("unavailable map baseline: %s", m.baseline)
			}
			if !m.identity {
				v.requirePureCollectionKey(args[0])
			}
			for i, e := range m.entries {
				if !v.collectionEquals(args[0], e.left, m.identity) {
					continue
				}
				if r.name == "put" || r.name == "putIfAbsent" && e.right == nil {
					m.entries[i].right = args[1]
				}
				return e.right, true
			}
			if r.name != "get" {
				if r.name == "putIfAbsent" && m.baseline != "" {
					fail("unavailable conditional map baseline: %s", m.baseline)
				}
				v.reserveCollection(2)
				m.entries = append(m.entries, javaPair{args[0], args[1]})
			}
			if m.baseline != "" {
				return unusedNativeResult{"unavailable map baseline: " + m.baseline}, true
			}
			return nil, true
		}
		if (r.name == "values" && r.desc == "()Ljava/util/Collection;") || (r.name == "keySet" && r.desc == "()Ljava/util/Set;") || (r.name == "entrySet" && r.desc == "()Ljava/util/Set;") {
			if m.baseline != "" {
				fail("unavailable map view: %s", m.baseline)
			}
			v.reserveCollection(len(m.entries))
			switch r.name {
			case "values":
				values := make([]any, len(m.entries))
				for i, e := range m.entries {
					values[i] = e.right
				}
				return v.alloc("java/util/Collection", &javaList{values: values}), true
			case "keySet":
				values := make([]any, len(m.entries))
				for i, e := range m.entries {
					values[i] = e.left
				}
				return v.alloc("java/util/Set", &javaSet{values: values}), true
			case "entrySet":
				values := make([]any, len(m.entries))
				for i, e := range m.entries {
					values[i] = v.alloc("java/util/Map$Entry", e)
				}
				return v.alloc("java/util/Set", &javaSet{values: values}), true
			}
		}
	}
	return nil, false
}

func (v *machine) collectionEquals(a, b any, identity bool) bool {
	v.tick()
	if a == nil {
		return b == nil
	}
	if b == nil {
		return false
	}
	if staticGameKey(a) || staticGameKey(b) {
		// One game static field always holds one value: equal iff same field.
		x, ok1 := a.(*inertValue)
		y, ok2 := b.(*inertValue)
		return ok1 && ok2 && x.origin == y.origin
	}
	if o, ok := a.(*object); ok {
		if other, ok := b.(*object); ok && o == other {
			return true
		}
		if identity {
			return false
		}
		if climateRecord(o) {
			// Climate.Parameter / ParameterPoint are records: component equality.
			other, ok := b.(*object)
			return ok && other != nil && other.class == o.class && other.data == o.data
		}
		if _, tag := o.data.(registryTag); tag && !v.fetchClass(o.class) {
			// Native tag keys are interned symbols: value equality is identity.
			other, ok := b.(*object)
			return ok && other != nil && other.class == o.class && other.data == o.data
		}
		return integer(v.method(reference{o.class, "equals", "(Ljava/lang/Object;)Z"}, o, true, []any{b})) != 0
	}
	if s, ok := a.(string); ok {
		if identity {
			fail("string reference identity is not retained")
		}
		other, ok := b.(string)
		if !ok {
			if o, isObject := b.(*object); isObject && o != nil && o.class == "java/lang/String" {
				return s == javaString(o)
			}
		}
		return ok && s == other
	}
	if x, ok := a.(classLiteral); ok {
		other, ok := b.(classLiteral)
		return ok && x == other
	}
	if x, ok := a.(*array); ok {
		other, ok := b.(*array)
		return ok && x == other
	}
	// A lambda instance has Object identity equality.
	if x, ok := a.(*lambda); ok {
		other, ok := b.(*lambda)
		return ok && x == other
	}
	fail("unsupported map key equality %T", a)
	return false
}

// Linear lookup is equivalent only for keys with pure, consistent equality and
// hashing. Arbitrary overrides can mutate the registration state, so they must
// not be silently replaced by a lookup that never calls hashCode. Unordered
// iteration remains unavailable even for these keys.
func (v *machine) requirePureCollectionKey(key any) {
	if staticGameKey(key) {
		return
	}
	requireDetermined("collection key", key)
	switch x := key.(type) {
	case nil, string, classLiteral, *array, *lambda:
		return
	case *object:
		if x == nil {
			return
		}
		if climateRecord(x) {
			return
		}
		if nativeDeferredHolder(x.class) {
			if _, ok := x.data.(deferredValue); ok {
				return
			}
		}
		switch x.class {
		case "java/lang/Object", "java/lang/String", "java/lang/Boolean", "java/lang/Byte", "java/lang/Short", "java/lang/Character", "java/lang/Integer", "java/lang/Long", "java/lang/Float", "java/lang/Double", "java/util/UUID", "net/minecraft/resources/ResourceKey", "net/minecraft/resources/ResourceLocation", "net/minecraft/tags/TagKey", "net/minecraft/world/entity/npc/VillagerType", "net/minecraftforge/fml/config/ModConfig$Type", "net/neoforged/fml/config/ModConfig$Type":
			return
		}
		owner := x.class
		custom := false
		for n := 0; n < 64 && v.fetchClass(owner); n++ {
			c := v.load(owner)
			for _, m := range c.methods {
				if m.name == "equals" && m.desc == "(Ljava/lang/Object;)Z" || m.name == "hashCode" && m.desc == "()I" {
					custom = true
				}
			}
			owner = c.super
			if owner == "java/lang/Object" || owner == "java/lang/Enum" {
				break
			}
		}
		if custom {
			// Honor overrides by executing hashCode; linear collections still
			// membership-test with equals via collectionEquals.
			_ = integer(v.method(reference{x.class, "hashCode", "()I"}, x, true, nil))
			return
		}
		if v.fetchClass(x.class) {
			return
		}
		fail("unbound collection key %s (%T)", x.class, x.data)
	default:
		fail("unbound collection key %T", key)
	}
}

// collectionValues snapshots any java.util.Collection: native list/set state
// directly, otherwise through the collection's own iterator() bytecode.
func (v *machine) collectionValues(c any) []any {
	if c == nil {
		raiseJava("java/lang/NullPointerException", "collection")
	}
	requireDetermined("collection", c)
	switch x := objectData(c).(type) {
	case *javaList:
		return append([]any(nil), x.values...)
	case *javaSet:
		return append([]any(nil), x.values...)
	}
	it := v.method(reference{"java/util/Collection", "iterator", "()Ljava/util/Iterator;"}, c, true, nil)
	var out []any
	for n := 0; ; n++ {
		if n > 131072 {
			fail("collection slot budget exceeded")
		}
		has := v.method(reference{"java/util/Iterator", "hasNext", "()Z"}, it, true, nil)
		requireDetermined("iterator", has)
		if integer(has) == 0 {
			return out
		}
		out = append(out, v.method(reference{"java/util/Iterator", "next", "()Ljava/lang/Object;"}, it, true, nil))
	}
}

// copyMapInto applies put semantics for every entry of a natively backed map.
// Maps without native entries fail closed rather than being guessed.
func (v *machine) copyMapInto(dst *javaMap, src any) {
	if src == nil {
		raiseJava("java/lang/NullPointerException", "map copy source")
	}
	if inertOf(src) != nil {
		// Game maps absent from the registration classpath contribute no entries.
		// Treating them as empty lets mods still construct/putAll overlays without
		// inventing vanilla contents.
		return
	}
	o, ok := src.(*object)
	var from *javaMap
	if ok {
		from, ok = o.data.(*javaMap)
	}
	var entries []javaPair
	if ok && from != nil {
		if from.baseline != "" {
			// Same as an inert game map: contents are not on the registration
			// classpath, so the copy contributes no entries.
			return
		}
		entries = append([]javaPair(nil), from.entries...)
	} else {
		// Fall back to Map.entrySet iteration for bytecode-backed maps.
		set := v.method(reference{"java/util/Map", "entrySet", "()Ljava/util/Set;"}, src, true, nil)
		for _, e := range v.collectionValues(set) {
			left := v.method(reference{"java/util/Map$Entry", "getKey", "()Ljava/lang/Object;"}, e, true, nil)
			right := v.method(reference{"java/util/Map$Entry", "getValue", "()Ljava/lang/Object;"}, e, true, nil)
			entries = append(entries, javaPair{left, right})
		}
	}
	v.reserveCollection(2 * len(entries))
next:
	for _, e := range entries {
		if dst.nonnull && (e.left == nil || e.right == nil) {
			raiseJava("java/lang/NullPointerException", "null concurrent map entry")
		}
		if !dst.identity {
			v.requirePureCollectionKey(e.left)
		}
		for i, d := range dst.entries {
			if v.collectionEquals(e.left, d.left, dst.identity) {
				dst.entries[i].right = e.right
				continue next
			}
		}
		dst.entries = append(dst.entries, e)
	}
}

// staticGameKey reports an unavailable game static field value. Its identity
// is the field itself, so it can key a collection without its contents.
func staticGameKey(x any) bool {
	i, ok := x.(*inertValue)
	return ok && i.nonNull && strings.HasPrefix(i.origin, "static ")
}

// climateRecord reports a natively bound Climate record whose data is its
// complete component state ([2]int64 span or [7][2]int64 point).
func climateRecord(o *object) bool {
	switch o.class {
	case "net/minecraft/world/level/biome/Climate$Parameter":
		_, ok := o.data.([2]int64)
		return ok
	case "net/minecraft/world/level/biome/Climate$ParameterPoint":
		_, ok := o.data.([7][2]int64)
		return ok
	}
	return false
}

// inertKeyBaseline marks a map that received a write keyed by an unavailable
// game value; see the put binding.
const inertKeyBaseline = "inert key "
