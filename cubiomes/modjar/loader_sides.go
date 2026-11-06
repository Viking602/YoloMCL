package modjar

// resolvedSide returns the configured game distribution. Empty means CLIENT so
// unit fixtures that omit Side keep their historical meaning; worldgen sessions
// always set Side explicitly.
func (h *bindings) resolvedSide() string {
	if h.options.Side == "" {
		return "CLIENT"
	}
	return h.options.Side
}

// fabricEnvType maps the host Side onto Fabric's EnvType vocabulary.
func (h *bindings) fabricEnvType() string {
	if h.resolvedSide() == "DEDICATED_SERVER" {
		return "SERVER"
	}
	return "CLIENT"
}

func (h *bindings) distributionCall(r reference, receiver any, args []any) (any, bool) {
	if r.owner != "net/minecraftforge/fml/DistExecutor" {
		return nil, false
	}
	v := h.vm
	get := reference{"java/util/function/Supplier", "get", "()Ljava/lang/Object;"}
	side := h.resolvedSide()
	if (r.name == "runWhenOn" || r.name == "safeRunWhenOn" || r.name == "unsafeRunWhenOn") && len(args) == 2 {
		// DistExecutor.*(Dist, Supplier): run only when Dist matches this host.
		wanted, _ := objectData(args[0]).(string)
		if wanted == "" || wanted != side {
			return nil, true
		}
		if args[1] == nil {
			raiseJava("java/lang/NullPointerException", "null distribution supplier")
		}
		supplier := v.method(get, args[1], true, nil)
		if supplier == nil {
			raiseJava("java/lang/NullPointerException", "null distribution target")
		}
		if r.name == "safeRunWhenOn" {
			// SafeRunnable.run()V; plain Runnable also uses run()V.
			v.method(reference{"java/lang/Runnable", "run", "()V"}, supplier, true, nil)
			return nil, true
		}
		v.method(reference{"java/lang/Runnable", "run", "()V"}, supplier, true, nil)
		return nil, true
	}
	if r.desc != "(Ljava/util/function/Supplier;Ljava/util/function/Supplier;)Ljava/lang/Object;" {
		return nil, false
	}
	if r.name != "runForDist" && r.name != "unsafeRunForDist" && r.name != "safeRunForDist" {
		return nil, false
	}
	// This host models the installed production game (FMLEnvironment.production
	// is true). The safe variant's development referent validation is disabled
	// by FML itself there. Both suppliers must remain lazy on the inactive side.
	index := 0
	switch side {
	case "CLIENT":
	case "DEDICATED_SERVER":
		index = 1
	default:
		fail("unbound game distribution %q", side)
	}
	if args[index] == nil {
		raiseJava("java/lang/NullPointerException", "null distribution supplier")
	}
	supplier := v.method(get, args[index], true, nil)
	if supplier == nil {
		raiseJava("java/lang/NullPointerException", "null distribution target")
	}
	return v.method(get, supplier, true, nil), true
}
