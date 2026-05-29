// Package selection provides a reusable text-selection mechanism for the TUI.
// Any component (or sub-component) can opt a region of its rendered output into
// mouse copy-selection. The selection engine (see internal/tui/copy_selection.go)
// hit-tests the mouse against every registered region, picks the innermost one,
// and clamps the drag to that region's bounds so a selection started inside a
// sub-component (e.g. a single comment card) stays scoped to it.
//
// There are two ways to register a region:
//
//   - Mark / MarkStyled: wrap a rendered string with github.com/lrstanley/bubblezone
//     markers so the outer Scan resolves its screen bounds. Use this only for
//     outermost, non-clipped regions (whole panes). Markers do NOT survive being
//     embedded in content that later flows through a scrolling/wrapping viewport,
//     because the viewport splits and re-cuts lines.
//
//   - RegisterBounds: register a sub-region with explicitly computed screen
//     bounds plus its styled and plain content. Use this for sub-components that
//     live inside scrolling viewports (comment cards, table rows, diff hunks),
//     where bounds must be derived arithmetically from the viewport offset rather
//     than from markers.
package selection

import (
	"strings"
	"sync"

	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone/v2"
)

// Prefix is prepended to every selection region id. It keeps selection zones
// from colliding with other bubblezone usages and lets the engine enumerate
// selection regions by id convention.
const Prefix = "sel:"

// Well-known region ids shared across packages.
var (
	// PreviewLogsID is the region wrapping the embedded actions/checks logs
	// viewport. Its plain text is the offset-mapped visible log lines, so it is
	// registered explicitly by the actionview rather than derived from styling.
	PreviewLogsID = ID("preview-logs")
)

// Bounds is a screen-space rectangle for a registered selection region.
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

// Block describes a selectable sub-component as produced by a component, before
// its screen position is known. The engine converts ContentY (the block's first
// line offset within the scrollable content) to absolute screen bounds using the
// owning viewport's origin and scroll offset, then registers it via
// RegisterBounds.
type Block struct {
	ID       string // unique region id (use ID(...) to build).
	ContentY int    // first line offset within the content (pre-scroll).
	Height   int    // number of lines the block occupies.
	Plain    string // unstyled content for copy.
	Styled   string // styled content for the highlight overlay.
}

// Scroll describes a scrollable content area together with the selectable
// sub-component blocks inside it. The selection engine converts each block's
// content-relative position to absolute screen bounds using OriginX/OriginY
// (the screen cell of the content's first, unscrolled line), Width, the visible
// height and the scroll offset.
type Scroll struct {
	OriginX       int
	OriginY       int
	Width         int
	VisibleHeight int
	YOffset       int
	Blocks        []Block
}

// region holds everything the selection engine needs about a registered region.
type region struct {
	plain  string // unstyled content, used for copy/extraction.
	styled string // styled content, used to render the highlight overlay.
	// explicitBounds, when set, is the region's screen bounds. When unset
	// (hasBounds == false) bounds are resolved from bubblezone via the region id.
	explicitBounds Bounds
	hasBounds      bool
}

var (
	mu      sync.RWMutex
	regions = map[string]region{}
)

// zoneReady reports whether bubblezone's global manager has been initialized.
// When it hasn't (e.g. in unit tests that render views directly), Mark and the
// marker-based lookups degrade to no-ops so callers never panic.
func zoneReady() bool {
	return zone.DefaultManager != nil
}

// ID builds a stable, prefixed region id from the supplied parts.
// e.g. ID("activity", commentID) -> "sel:activity:<commentID>".
func ID(parts ...string) string {
	return Prefix + strings.Join(parts, ":")
}

// Mark registers a marker-based selection region. plainText is the unstyled
// content used for copy/extraction; styledView is the visible string embedded in
// the view. The returned string (styledView wrapped with bubblezone markers)
// must be placed into the rendered output so its screen bounds can be resolved
// via Region.
//
// Only use Mark for outermost, non-clipped regions. If id or styledView is
// empty, styledView is returned unchanged and nothing is registered.
func Mark(id, plainText, styledView string) string {
	if id == "" || styledView == "" {
		return styledView
	}
	mu.Lock()
	regions[id] = region{plain: plainText, styled: styledView}
	mu.Unlock()
	if !zoneReady() {
		return styledView
	}
	return zone.Mark(id, styledView)
}

// MarkStyled is a convenience wrapper for Mark that derives the plain text by
// stripping ANSI styling from styledView. Use it when the component has no
// cheaper source of plain text.
func MarkStyled(id, styledView string) string {
	return Mark(id, ansi.Strip(styledView), styledView)
}

// RegisterBounds registers a sub-region with explicit screen bounds. styled is
// the region's styled content (used to render the highlight overlay) and plain
// is its unstyled content (used for copy). The styled/plain content should
// correspond row-for-row to the bounds rectangle starting at its top-left.
//
// Use this for sub-components inside scrolling viewports, where bounds are
// computed from the viewport offset rather than from bubblezone markers.
func RegisterBounds(id string, b Bounds, plain, styled string) {
	if id == "" || b.Empty() {
		return
	}
	mu.Lock()
	regions[id] = region{plain: plain, styled: styled, explicitBounds: b, hasBounds: true}
	mu.Unlock()
}

// Region resolves a registered region's screen bounds, plain text and styled
// content. ok is false when the region is unknown or its bounds can't be
// resolved yet.
func Region(id string) (bounds Bounds, plain, styled string, ok bool) {
	mu.RLock()
	r, found := regions[id]
	mu.RUnlock()
	if !found {
		return Bounds{}, "", "", false
	}

	if r.hasBounds {
		return r.explicitBounds, r.plain, r.styled, true
	}

	if !zoneReady() {
		return Bounds{}, "", "", false
	}
	z := zone.Get(id)
	if z == nil || z.IsZero() {
		return Bounds{}, "", "", false
	}
	return Bounds{
		X:      z.StartX,
		Y:      z.StartY,
		Width:  max(0, z.EndX-z.StartX+1),
		Height: max(0, z.EndY-z.StartY+1),
	}, r.plain, r.styled, true
}

// InnermostAt returns the id of the registered region containing (x, y) that
// has the smallest area (i.e. the most deeply nested sub-component). It returns
// an empty string when no registered region contains the point.
func InnermostAt(x, y int) string {
	mu.RLock()
	ids := make([]string, 0, len(regions))
	for id := range regions {
		ids = append(ids, id)
	}
	mu.RUnlock()

	bestID := ""
	bestArea := 0
	for _, id := range ids {
		bounds, _, _, ok := Region(id)
		if !ok || !bounds.Contains(x, y) {
			continue
		}
		area := bounds.Area()
		if bestID == "" || area < bestArea {
			bestID = id
			bestArea = area
		}
	}
	return bestID
}

// Reset clears all registered regions. Call it at the start of each frame
// (before re-registering) so stale regions don't linger.
func Reset() {
	mu.Lock()
	clear(regions)
	mu.Unlock()
}
