package startup

import "testing"

func TestPreparedContentJoinsBeforeAttachAndReusesPreparation(t *testing.T) {
	joined, attached := 0, 0
	connection := new(int)
	p := prepareContent(func() { joined++ }, func(c *int) {
		if joined != 1 || c != connection {
			t.Fatal("attached before application preparation or lost the connection")
		}
		attached++
	})
	p.AttachContent(connection)
	p.AttachContent(connection)
	if joined != 1 || attached != 2 {
		t.Fatalf("reattaching repeated the startup wait: prepared=%d attached=%d", joined, attached)
	}
}
