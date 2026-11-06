package modjar

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

type javaServices struct {
	service   string
	providers []string
	instances map[string]any
	resources func(string) ([][]byte, error)
	loaded    bool
}
type serviceIterator struct {
	services *javaServices
	index    int
}
type javaOptional struct{ value any }

func (h *bindings) serviceCall(r reference, receiver any, args []any) (any, bool) {
	v := h.vm
	if r.owner == "java/util/ServiceLoader" && r.name == "load" && (r.desc == "(Ljava/lang/Class;)Ljava/util/ServiceLoader;" || r.desc == "(Ljava/lang/ModuleLayer;Ljava/lang/Class;)Ljava/util/ServiceLoader;" || r.desc == "(Ljava/lang/Class;Ljava/lang/ClassLoader;)Ljava/util/ServiceLoader;") {
		// Registration has one classpath, so every class loader names it.
		arg := args[0]
		if r.desc == "(Ljava/lang/ModuleLayer;Ljava/lang/Class;)Ljava/util/ServiceLoader;" {
			arg = args[1]
		}
		service, ok := arg.(classLiteral)
		if !ok {
			fail("invalid service class")
		}
		if h.options.Resources == nil {
			fail("service configuration classpath is required")
		}
		s := &javaServices{service: string(service), instances: map[string]any{}, resources: h.options.Resources}
		return v.alloc(r.owner, s), true
	}
	if r.owner == "java/util/ServiceLoader" && r.name == "iterator" && r.desc == "()Ljava/util/Iterator;" {
		return v.alloc("java/util/Iterator", &serviceIterator{services: objectData(receiver).(*javaServices)}), true
	}
	if r.owner == "java/util/ServiceLoader" && r.name == "findFirst" && r.desc == "()Ljava/util/Optional;" {
		s := objectData(receiver).(*javaServices)
		s.discover(v)
		var x any
		if len(s.providers) > 0 {
			x = s.provider(v, 0)
		}
		return v.alloc("java/util/Optional", javaOptional{x}), true
	}
	if o, ok := receiver.(*object); ok && o != nil {
		if it, ok := o.data.(*serviceIterator); ok && r.owner == "java/util/Iterator" {
			switch {
			case r.name == "hasNext" && r.desc == "()Z":
				it.services.discover(v)
				if it.index < len(it.services.providers) {
					return int32(1), true
				}
				return int32(0), true
			case r.name == "next" && r.desc == "()Ljava/lang/Object;":
				it.services.discover(v)
				if it.index >= len(it.services.providers) {
					raiseJava("java/util/NoSuchElementException", "no service provider")
				}
				x := it.services.provider(v, it.index)
				it.index++
				return x, true
			}
		}
	}
	if r.owner == "java/util/Optional" {
		if receiver == nil {
			switch {
			case r.name == "empty" && r.desc == "()Ljava/util/Optional;":
				return v.alloc(r.owner, javaOptional{}), true
			case (r.name == "of" || r.name == "ofNullable") && r.desc == "(Ljava/lang/Object;)Ljava/util/Optional;":
				if r.name == "of" && args[0] == nil {
					raiseJava("java/lang/NullPointerException", "null optional")
				}
				return v.alloc(r.owner, javaOptional{args[0]}), true
			}
			return nil, false
		}
		x := objectData(receiver).(javaOptional).value
		switch {
		case (r.name == "isPresent" || r.name == "isEmpty") && r.desc == "()Z":
			if (x != nil) == (r.name == "isPresent") {
				return int32(1), true
			}
			return int32(0), true
		case r.name == "ifPresent" && r.desc == "(Ljava/util/function/Consumer;)V":
			if x != nil {
				v.method(reference{"java/util/function/Consumer", "accept", "(Ljava/lang/Object;)V"}, args[0], true, []any{x})
			}
			return nil, true
		case r.name == "map" && r.desc == "(Ljava/util/function/Function;)Ljava/util/Optional;":
			if x == nil {
				return receiver, true
			}
			return v.alloc(r.owner, javaOptional{v.method(reference{"java/util/function/Function", "apply", "(Ljava/lang/Object;)Ljava/lang/Object;"}, args[0], true, []any{x})}), true
		case r.name == "orElseGet" && r.desc == "(Ljava/util/function/Supplier;)Ljava/lang/Object;":
			if x == nil {
				x = v.method(reference{"java/util/function/Supplier", "get", "()Ljava/lang/Object;"}, args[0], true, nil)
			}
			return x, true
		case r.name == "orElse" && r.desc == "(Ljava/lang/Object;)Ljava/lang/Object;":
			if x == nil {
				x = args[0]
			}
			return x, true
		case (r.name == "get" || r.name == "orElseThrow") && r.desc == "()Ljava/lang/Object;":
			if x == nil {
				raiseJava("java/util/NoSuchElementException", "empty optional")
			}
			return x, true
		case r.name == "orElseThrow" && r.desc == "(Ljava/util/function/Supplier;)Ljava/lang/Object;":
			if x != nil {
				return x, true
			}
			err := v.method(reference{"java/util/function/Supplier", "get", "()Ljava/lang/Object;"}, args[0], true, nil)
			if err == nil {
				raiseJava("java/lang/NullPointerException", "null supplied exception")
			}
			o, ok := err.(*object)
			if !ok || !v.instanceOf(o, "java/lang/Throwable") {
				fail("invalid supplied exception")
			}
			panic(javaException{class: o.class, value: o})
		}
	}
	return nil, false
}

func (s *javaServices) discover(v *machine) {
	if s.loaded {
		return
	}
	resources, err := s.resources("META-INF/services/" + strings.ReplaceAll(s.service, "/", "."))
	if err != nil {
		fail("service configuration: %v", err)
	}
	seen := map[string]bool{}
	var providers []string
	for _, data := range resources {
		if !utf8.Valid(data) {
			fail("invalid UTF-8 service configuration")
		}
		for _, line := range strings.Split(string(data), "\n") {
			v.tick()
			name, _, _ := strings.Cut(line, "#")
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			for i, ch := range name {
				if !(unicode.IsLetter(ch) || ch == '_' || ch == '$' || i > 0 && (unicode.IsDigit(ch) || ch == '.')) {
					fail("invalid service provider name")
				}
			}
			name = strings.ReplaceAll(name, ".", "/")
			if !seen[name] {
				seen[name] = true
				providers = append(providers, name)
			}
			if len(providers) > 256 {
				fail("service provider budget")
			}
		}
	}
	s.providers = providers
	s.loaded = true
}

func (s *javaServices) provider(v *machine, index int) any {
	name := s.providers[index]
	if x, ok := s.instances[name]; ok {
		return x
	}
	o := v.alloc(name, nil)
	if !v.instanceOf(o, s.service) {
		raiseJava("java/util/ServiceConfigurationError", "provider does not implement service")
	}
	c := v.load(name)
	public := false
	for _, m := range c.methods {
		if m.name == "<init>" && m.desc == "()V" && m.access&1 != 0 {
			public = true
		}
	}
	if c.access&1 == 0 || c.access&0x400 != 0 || !public {
		raiseJava("java/util/ServiceConfigurationError", "provider requires a public no-arg constructor")
	}
	defer func() {
		if p := recover(); p != nil {
			if e, ok := p.(javaException); ok {
				msg := "provider " + name + " initialization failed: " + e.Error()
				if len(v.frames) > 0 {
					msg += " at " + strings.Join(v.frames, " -> ")
				}
				raiseJava("java/util/ServiceConfigurationError", msg)
			}
			panic(p)
		}
	}()
	v.initialize(name)
	v.method(reference{name, "<init>", "()V"}, o, false, nil)
	s.instances[name] = o
	return o
}
