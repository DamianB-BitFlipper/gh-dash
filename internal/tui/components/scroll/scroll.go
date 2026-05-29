// Package scroll provides a reusable mouse-wheel scrolling mechanism for the
// TUI. It mirrors the design of internal/tui/components/selection: any component
// (or sub-component) can opt a region of its rendered output into mouse-wheel
// scrolling by registering its screen bounds together with a stable id.
//
// The registry holds pure data (bounds keyed by id), never behavior. The wheel
// engine (see internal/tui/ui.go) hit-tests the mouse position against every
// registered region via At, picks the innermost one (smallest area, so nested
// scroll areas win over their containing pane), and then performs the scroll on
// the real model keyed by the returned id. Keeping the registry data-only avoids
// capturing a stale value-copy of the model in a closure (Update and View use
// value receivers), which would silently drop mutations to value-type fields.
//
// Lifecycle: call Reset at the start of each render pass, then Register every
// visible scroll region for that frame. This keeps stale regions from lingering
// across frames, exactly like the selection registry.
package scroll

import "sync"

// Bounds is a screen-space rectangle for a registered scroll region.
type Bounds struct {
	X      int
	Y      int
	Width  int
	Height int
}

// Empty reports whether the bounds have no area.
func (b Bounds) Empty() bool {
	return b.Width <= 0 || b.Height <= 0
}

// Area returns the rectangle's area, used to pick the innermost region when
// regions overlap (smallest area wins).
func (b Bounds) Area() int {
	if b.Empty() {
		return 0
	}
	return b.Width * b.Height
}

// Contains reports whether (x, y) falls inside the bounds.
func (b Bounds) Contains(x, y int) bool {
	return !b.Empty() &&
		x >= b.X && x < b.X+b.Width &&
		y >= b.Y && y < b.Y+b.Height
}

var (
	mu      sync.RWMutex
	regions = map[string]Bounds{}
)

// Register records a scroll region for the current frame. Calls with empty
// bounds or an empty id are ignored so callers can register unconditionally
// without guarding every edge case.
func Register(id string, b Bounds) {
	if id == "" || b.Empty() {
		return
	}
	mu.Lock()
	regions[id] = b
	mu.Unlock()
}

// At returns the id of the innermost registered region containing (x, y), i.e.
// the one with the smallest area. ok is false when no region contains the point.
func At(x, y int) (id string, ok bool) {
	mu.RLock()
	defer mu.RUnlock()

	best := ""
	bestArea := 0
	for rid, b := range regions {
		if !b.Contains(x, y) {
			continue
		}
		area := b.Area()
		if best == "" || area < bestArea {
			best = rid
			bestArea = area
		}
	}
	return best, best != ""
}

// Reset clears all registered regions. Call it at the start of each frame
// (before re-registering) so stale regions don't linger.
func Reset() {
	mu.Lock()
	clear(regions)
	mu.Unlock()
}
