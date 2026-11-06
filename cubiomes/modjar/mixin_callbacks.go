package modjar

const callbackInfoClass = "org/spongepowered/asm/mixin/injection/callback/CallbackInfo"
const callbackReturnClass = callbackInfoClass + "Returnable"

type callbackState struct {
	name                   string
	cancellable, cancelled bool
	value                  any
}

func callbackCall(r reference, receiver any, args []any) (any, bool) {
	if r.owner != callbackInfoClass && r.owner != callbackReturnClass {
		return nil, false
	}
	o, ok := receiver.(*object)
	if !ok || o == nil {
		return nil, false
	}
	s, ok := o.data.(*callbackState)
	if !ok {
		return nil, false
	}
	cancel := func() {
		if !s.cancellable {
			raiseJava("org/spongepowered/asm/mixin/injection/callback/CancellationException", "The call "+s.name+" is not cancellable.")
		}
		s.cancelled = true
	}
	switch {
	case r.name == "getId" && r.desc == "()Ljava/lang/String;":
		return s.name, true
	case r.name == "isCancellable" && r.desc == "()Z":
		if s.cancellable {
			return int32(1), true
		}
		return int32(0), true
	case r.name == "isCancelled" && r.desc == "()Z":
		if s.cancelled {
			return int32(1), true
		}
		return int32(0), true
	case r.name == "cancel" && r.desc == "()V":
		cancel()
		return nil, true
	case r.name == "getReturnValue" && r.desc == "()Ljava/lang/Object;":
		return s.value, true
	case r.name == "setReturnValue" && r.desc == "(Ljava/lang/Object;)V":
		cancel()
		s.value = args[0]
		return nil, true
	}
	return nil, false
}
