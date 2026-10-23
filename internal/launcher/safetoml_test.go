package launcher

import "testing"

func TestSafeTOMLNoPanic(t *testing.T) {
	var v struct {
		Deps map[string][]struct {
			ModID string `toml:"modId"`
		} `toml:"dependencies"`
	}
	bad := []byte("[[dependencies.a]]\nmodId=\"x\"\n[[dependencies.a.b]]\nmodId=\"y\"\n")
	_ = safeTOML(bad, &v) // must not panic
}
