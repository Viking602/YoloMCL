package modjar

func arrayAccess(x any, index int) *array {
	if x == nil {
		raiseJava("java/lang/NullPointerException", "")
	}
	a, ok := x.(*array)
	if !ok || a == nil {
		fail("invalid array reference")
	}
	if index < 0 || index >= len(a.values) {
		raiseJava("java/lang/ArrayIndexOutOfBoundsException", "")
	}
	return a
}
