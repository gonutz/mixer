package mixer

import "testing"

func TestRounding(t *testing.T) {
	p, n := float32(1.5), float32(-1.5)
	pos, neg := int(p), int(n)
	if pos != 1 || neg != -1 {
		t.Error(pos, neg)
	}
}
