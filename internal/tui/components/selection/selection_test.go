package selection

import (
	"testing"
	"time"

	zone "github.com/lrstanley/bubblezone/v2"
)

func TestIDBuildsPrefixedID(t *testing.T) {
	if got := ID("activity", "42"); got != "sel:activity:42" {
		t.Fatalf("expected sel:activity:42, got %q", got)
	}
}

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

// scanForBounds marks the supplied regions inside a multi-line canvas, scans
// once, and waits for bubblezone's async worker to store the zone info.
func scanForBounds(t *testing.T, view string) {
	t.Helper()
	zone.Scan(view)
	// The zone manager stores scan results asynchronously; give the worker a
	// moment to process before querying bounds.
	time.Sleep(20 * time.Millisecond)
}

func TestRegionAndInnermostAt(t *testing.T) {
	zone.NewGlobal()
	zone.SetEnabled(true)
	t.Cleanup(func() { zone.Close() })
	Reset()

	outer := ID("outer")
	inner := ID("inner")

	// Build a canvas where `inner` is nested within `outer`.
	innerView := Mark(inner, "INNER", "INNER")
	line := "  " + innerView + "  "
	outerView := Mark(outer, "ABC\n  INNER  \nDEF", "ABC\n"+line+"\nDEF")
	scanForBounds(t, outerView)

	ob, otext, _, ok := Region(outer)
	if !ok {
		t.Fatalf("expected outer region to resolve")
	}
	if otext != "ABC\n  INNER  \nDEF" {
		t.Fatalf("unexpected outer text %q", otext)
	}

	ib, _, _, ok := Region(inner)
	if !ok {
		t.Fatalf("expected inner region to resolve")
	}

	// Inner must be strictly smaller than outer.
	if ib.Area() >= ob.Area() {
		t.Fatalf("expected inner area (%d) < outer area (%d)", ib.Area(), ob.Area())
	}

	// A point inside the inner region must resolve to inner (innermost wins).
	if got := InnermostAt(ib.X, ib.Y); got != inner {
		t.Fatalf("expected innermost at inner origin to be %q, got %q", inner, got)
	}

	// A point inside outer but outside inner must resolve to outer.
	if got := InnermostAt(ob.X, ob.Y); got != outer {
		t.Fatalf("expected innermost at outer origin to be %q, got %q", outer, got)
	}

	// A point outside all regions resolves to nothing.
	if got := InnermostAt(ob.X+ob.Width+5, ob.Y+ob.Height+5); got != "" {
		t.Fatalf("expected no region outside bounds, got %q", got)
	}
}

func TestResetClearsContent(t *testing.T) {
	zone.NewGlobal()
	zone.SetEnabled(true)
	t.Cleanup(func() { zone.Close() })

	id := ID("transient")
	Mark(id, "data", "data")
	Reset()
	if _, _, _, ok := Region(id); ok {
		t.Fatalf("expected region to be cleared after Reset")
	}
}

func TestRegisterBoundsAndInnermostAt(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	outer := ID("rb-outer")
	inner := ID("rb-inner")
	RegisterBounds(outer, Bounds{X: 0, Y: 0, Width: 20, Height: 10}, "outer-plain", "outer-styled")
	RegisterBounds(inner, Bounds{X: 2, Y: 2, Width: 4, Height: 2}, "inner-plain", "inner-styled")

	b, plain, styled, ok := Region(inner)
	if !ok || plain != "inner-plain" || styled != "inner-styled" {
		t.Fatalf("unexpected inner region: b=%+v plain=%q styled=%q ok=%v", b, plain, styled, ok)
	}

	// Point inside inner -> inner (smallest area wins).
	if got := InnermostAt(3, 3); got != inner {
		t.Fatalf("expected innermost %q, got %q", inner, got)
	}
	// Point inside outer but outside inner -> outer.
	if got := InnermostAt(15, 8); got != outer {
		t.Fatalf("expected innermost %q, got %q", outer, got)
	}
	// Outside both -> none.
	if got := InnermostAt(100, 100); got != "" {
		t.Fatalf("expected no region, got %q", got)
	}
}

// RegisterBounds works without a bubblezone global manager, since it stores
// explicit bounds rather than relying on a scan.
func TestRegisterBoundsWorksWithoutZoneManager(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	id := ID("no-zone")
	RegisterBounds(id, Bounds{X: 1, Y: 1, Width: 3, Height: 1}, "plain", "styled")
	if _, _, _, ok := Region(id); !ok {
		t.Fatalf("expected explicit-bounds region to resolve without a zone manager")
	}
}
