package modjar

import "strings"

// java.lang.invoke subset: a method handle is the same value as a lambda
// (target + reference kind + bound arguments), so handles found through a
// Lookup, bound with bindTo, or produced by LambdaMetafactory all execute
// through callLambda. Access checks are not modelled: registration code only
// reaches handles to members it could also call directly.

type methodType struct{ desc string }

func classDescriptor(c classLiteral) string {
	switch string(c) {
	case "boolean":
		return "Z"
	case "byte":
		return "B"
	case "char":
		return "C"
	case "short":
		return "S"
	case "int":
		return "I"
	case "long":
		return "J"
	case "float":
		return "F"
	case "double":
		return "D"
	case "void":
		return "V"
	}
	if strings.HasPrefix(string(c), "[") {
		return string(c)
	}
	return "L" + string(c) + ";"
}

func classArg(x any) classLiteral {
	c, ok := x.(classLiteral)
	if !ok {
		if x == nil {
			raiseJava("java/lang/NullPointerException", "null class")
		}
		fail("expected Java class literal")
	}
	return c
}

func methodTypeDesc(x any) string {
	o, ok := x.(*object)
	if ok && o != nil {
		if t, ok := o.data.(methodType); ok {
			return t.desc
		}
	}
	if x == nil {
		raiseJava("java/lang/NullPointerException", "null method type")
	}
	fail("expected MethodType")
	return ""
}

func (h *bindings) invokeCall(r reference, receiver any, args []any) (any, bool) {
	v := h.vm
	switch r.owner {
	case "java/lang/invoke/MethodHandles":
		if r.name == "lookup" && r.desc == "()Ljava/lang/invoke/MethodHandles$Lookup;" {
			caller := ""
			if n := len(v.frames); n > 0 {
				caller = v.frames[n-1]
			}
			return v.alloc("java/lang/invoke/MethodHandles$Lookup", classLiteral(caller)), true
		}
	case "java/lang/invoke/MethodHandles$Lookup":
		switch r.name + r.desc {
		case "lookupClass()Ljava/lang/Class;":
			c, _ := objectData(receiver).(classLiteral)
			if c == "" {
				fail("lookup without caller class")
			}
			return c, true
		case "findStatic(Ljava/lang/Class;Ljava/lang/String;Ljava/lang/invoke/MethodType;)Ljava/lang/invoke/MethodHandle;":
			return &lambda{target: reference{string(classArg(args[0])), javaString(args[1]), methodTypeDesc(args[2])}, kind: 6}, true
		case "findVirtual(Ljava/lang/Class;Ljava/lang/String;Ljava/lang/invoke/MethodType;)Ljava/lang/invoke/MethodHandle;":
			return &lambda{target: reference{string(classArg(args[0])), javaString(args[1]), methodTypeDesc(args[2])}, kind: 5}, true
		case "findConstructor(Ljava/lang/Class;Ljava/lang/invoke/MethodType;)Ljava/lang/invoke/MethodHandle;":
			desc := methodTypeDesc(args[1])
			if !strings.HasSuffix(desc, ")V") {
				raiseJava("java/lang/NoSuchMethodException", "constructor type must return void")
			}
			return &lambda{target: reference{string(classArg(args[0])), "<init>", desc}, kind: 8}, true
		}
	case "java/lang/invoke/MethodType":
		if r.name != "methodType" {
			break
		}
		var params []any
		switch r.desc {
		case "(Ljava/lang/Class;)Ljava/lang/invoke/MethodType;":
		case "(Ljava/lang/Class;Ljava/lang/Class;)Ljava/lang/invoke/MethodType;":
			params = []any{args[1]}
		case "(Ljava/lang/Class;[Ljava/lang/Class;)Ljava/lang/invoke/MethodType;":
			params = args[1].(*array).values
		case "(Ljava/lang/Class;Ljava/lang/Class;[Ljava/lang/Class;)Ljava/lang/invoke/MethodType;":
			params = append([]any{args[1]}, args[2].(*array).values...)
		default:
			return nil, false
		}
		var b strings.Builder
		b.WriteByte('(')
		for _, p := range params {
			b.WriteString(classDescriptor(classArg(p)))
		}
		b.WriteByte(')')
		b.WriteString(classDescriptor(classArg(args[0])))
		return v.alloc(r.owner, methodType{b.String()}), true
	case "java/lang/invoke/LambdaMetafactory":
		if r.name == "metafactory" && r.desc == "(Ljava/lang/invoke/MethodHandles$Lookup;Ljava/lang/String;Ljava/lang/invoke/MethodType;Ljava/lang/invoke/MethodType;Ljava/lang/invoke/MethodHandle;Ljava/lang/invoke/MethodType;)Ljava/lang/invoke/CallSite;" {
			impl, ok := args[4].(*lambda)
			if !ok {
				fail("expected MethodHandle")
			}
			_, site := descriptor(methodTypeDesc(args[2]))
			template := &lambda{target: impl.target, kind: impl.kind, captured: impl.captured, sam: methodTypeDesc(args[3]), instantiated: methodTypeDesc(args[5]), iface: strings.TrimSuffix(strings.TrimPrefix(site, "L"), ";")}
			return v.alloc("java/lang/invoke/CallSite", &lambda{factory: template}), true
		}
	case "java/lang/invoke/CallSite", "java/lang/invoke/ConstantCallSite":
		if r.name == "getTarget" && r.desc == "()Ljava/lang/invoke/MethodHandle;" {
			return objectData(receiver).(*lambda), true
		}
	case "java/lang/invoke/MethodHandle":
		l, ok := receiver.(*lambda)
		if !ok {
			break
		}
		switch r.name {
		case "bindTo":
			c := *l
			c.captured = append(append([]any(nil), l.captured...), args[0])
			return &c, true
		case "invokeWithArguments":
			if r.desc == "([Ljava/lang/Object;)Ljava/lang/Object;" {
				return v.callLambda(l, args[0].(*array).values), true
			}
		case "invoke", "invokeExact":
			return v.callLambda(l, args), true
		}
	}
	return nil, false
}
