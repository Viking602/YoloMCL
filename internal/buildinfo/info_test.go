package buildinfo

import "testing"

func TestReleaseTagValidation(t *testing.T) {
	for _, tag := range []string{"v1.0.0", "v0.12.10", "v2.0.0-rc.1"} {
		if !ValidTag(tag) {
			t.Errorf("rejected release tag %q", tag)
		}
	}
	for _, tag := range []string{"v1.0..0", "1.0.0", "v01.0.0", "v1.0.0-01", "v1.0.0;echo", "v1.0.0\n", "main"} {
		if ValidTag(tag) {
			t.Errorf("accepted invalid tag %q", tag)
		}
	}
}
