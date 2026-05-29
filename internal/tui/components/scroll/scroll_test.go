package scroll

import "testing"

func TestBoundsContainsAndArea(t *testing.T) {
	b := Bounds{X: 2, Y: 3, Width: 4, Height: 2}
	if b.Area() != 8 {
		t.Fatalf("expected area 8, got %d", b.Area())
	}
	if !b.Contains(2, 3) || !b.Contains(5, 4) {
		t.Fatalf("expected corners to be contained")
	}
	if b.Contains(6, 4) || b.Contains(2, 5) {
		t.Fatalf("expected out-of-range points to be excluded")
	}
	if (Bounds{}).Contains(0, 0) {
		t.Fatalf("empty bounds should contain nothing")
	}
}

func TestRegisterIgnoresInvalid(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	Register("", Bounds{Width: 1, Height: 1})
	Register("a", Bounds{})

	if _, ok := At(0, 0); ok {
		t.Fatalf("expected no region registered from invalid inputs")
	}
}

func TestAtPicksInnermostRegion(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	// inner is fully contained within outer.
	Register("outer", Bounds{X: 0, Y: 0, Width: 100, Height: 100})
	Register("inner", Bounds{X: 10, Y: 10, Width: 5, Height: 5})

	if id, ok := At(12, 12); !ok || id != "inner" {
		t.Fatalf("expected innermost region 'inner' at (12,12), got %q (ok=%v)", id, ok)
	}

	if id, ok := At(50, 50); !ok || id != "outer" {
		t.Fatalf("expected 'outer' at (50,50), got %q (ok=%v)", id, ok)
	}

	if id, ok := At(200, 200); ok {
		t.Fatalf("expected no region outside all bounds, got %q", id)
	}
}

func TestResetClearsRegions(t *testing.T) {
	Reset()
	Register("a", Bounds{X: 0, Y: 0, Width: 10, Height: 10})
	if _, ok := At(1, 1); !ok {
		t.Fatalf("expected region before reset")
	}
	Reset()
	if _, ok := At(1, 1); ok {
		t.Fatalf("expected no region after reset")
	}
}
