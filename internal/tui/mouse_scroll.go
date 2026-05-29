package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/actionssection"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/scroll"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/selection"
)

const (
	// wheelRowStep is how many rows a single mouse-wheel notch moves the row
	// cursor. One row per notch matches keyboard up/down and avoids the wheel
	// feeling overly sensitive in list views.
	wheelRowStep = 1
	// wheelViewportStep is how many lines a single notch scrolls a
	// free-scrolling viewport (the sidebar preview / Activity tab and the
	// Checks logs). Three lines is the common default for content scrolling.
	wheelViewportStep = 3
	// momentumWindow is the gap below which consecutive wheel events are treated
	// as part of the same continuous (possibly OS-inertial) stream. Events
	// spaced further apart re-seed the active direction as a fresh gesture.
	momentumWindow = 250 * time.Millisecond
)

// momentumState tames macOS-style inertial ("momentum") scrolling, where the OS
// keeps emitting a decaying tail of same-direction wheel events after the
// physical gesture stops. Without intervention, that tail blocks an immediate
// reverse flick at the top/bottom of a scroll area: the residual momentum keeps
// arriving and fights the user's new direction.
//
// The policy (reversal cutoff): within a continuous stream, the active direction
// is sticky. Events matching it pass through. An opposite-direction event is
// treated as the start of a deliberate reversal only once it is confirmed by a
// second consecutive opposite event; until then it is dropped. This rejects the
// brief, dying opposite-looking tail without confirmation while still letting a
// real reverse flick (a sustained burst the other way) take over quickly. Once
// the stream settles (a gap longer than momentumWindow) the next event re-seeds
// the direction as a fresh gesture.
type momentumState struct {
	dir         int // active direction: -1 up, +1 down, 0 unset.
	lastTime    time.Time
	pendingDir  int // candidate reversal direction awaiting confirmation.
	pendingSeen int // consecutive opposite events observed so far.
}

// reversalConfirmCount is how many consecutive opposite-direction events must
// arrive within the momentum window before a reversal is accepted. Two is enough
// to distinguish a sustained reverse flick from a one-off tail blip while still
// feeling immediate.
const reversalConfirmCount = 2

// accept reports whether a wheel notch should be acted on. now is the event's
// arrival time (injected for testability).
func (s *momentumState) accept(notch int, now time.Time) bool {
	if notch == 0 {
		return false
	}
	settled := s.dir == 0 || now.Sub(s.lastTime) > momentumWindow
	s.lastTime = now

	if settled {
		// Fresh gesture (or first ever): seed the direction and accept.
		s.dir = notch
		s.pendingDir = 0
		s.pendingSeen = 0
		return true
	}

	if notch == s.dir {
		// Continuing the current stream; clear any half-formed reversal.
		s.pendingDir = 0
		s.pendingSeen = 0
		return true
	}

	// Opposite direction within the window: candidate reversal.
	if s.pendingDir == notch {
		s.pendingSeen++
	} else {
		s.pendingDir = notch
		s.pendingSeen = 1
	}
	if s.pendingSeen >= reversalConfirmCount {
		// Confirmed reversal: flip the active direction and accept.
		s.dir = notch
		s.pendingDir = 0
		s.pendingSeen = 0
		return true
	}
	// Unconfirmed opposite event: drop it (likely the dying momentum tail).
	return false
}

// wheelNotch maps a mouse-wheel button to a signed notch count: -1 for up, +1
// for down, 0 for non-vertical wheel buttons. The per-target line/row step is
// applied by the caller in Update.
func wheelNotch(button tea.MouseButton) int {
	switch button {
	case tea.MouseWheelUp:
		return -1
	case tea.MouseWheelDown:
		return 1
	default:
		return 0
	}
}

// registerScrollRegions registers every visible mouse-wheel scroll surface for
// the current frame, mirroring registerSelectionRegions. It records only the
// regions' screen bounds keyed by a stable id; the actual scrolling is performed
// in Update against the real model, keyed by the id At returns. The wheel engine
// hit-tests the cursor against these regions and dispatches to the innermost
// one, so scrolling follows the pointer rather than keyboard focus.
//
// Future scrollable components opt in by registering their screen bounds here
// (or via the bubblezone-resolved bounds of an existing selection region, as the
// Checks logs does below) and adding a matching case to the MouseWheelMsg
// handler in Update.
func (m *Model) registerScrollRegions() {
	contentTop := m.copySelectionContentY()

	// Main pane: the current section's row list.
	if currSection := m.getCurrSection(); currSection != nil {
		bounds := scroll.Bounds{
			X:      0,
			Y:      contentTop,
			Width:  m.ctx.MainContentWidth,
			Height: m.ctx.MainContentHeight,
		}
		scroll.Register(selection.ID("main"), bounds)
	}

	// Preview pane: the sidebar viewport.
	if m.sidebar.IsOpen {
		scroll.Register(selection.ID("preview"), m.previewPaneBounds(contentTop))

		// Inner region: the embedded Checks-tab logs viewport, when present.
		// It is registered with the bubblezone-resolved bounds of the logs
		// selection region so it wins over the surrounding preview pane.
		if logsBounds, ok := scrollBoundsFromSelection(selection.PreviewLogsID); ok {
			scroll.Register(selection.PreviewLogsID, logsBounds)
		}
	}
}

// registerActionsScrollRegions registers the three wheel-scroll columns of the
// dedicated Actions view (Workflows | Runs | Details). The Actions view uses a
// custom three-pane layout instead of the standard main/preview split, so it
// registers its own regions here rather than via registerScrollRegions. Geometry
// mirrors renderActionsThreePane: each column starts below the tabs bar and
// spans the base content height; widths come from actionsPaneWidths.
func (m *Model) registerActionsScrollRegions(section *actionssection.Model) {
	if section == nil {
		return
	}
	contentTop := m.copySelectionContentY()
	height := m.getBaseContentHeight()
	if height <= 0 {
		return
	}
	firstWidth, secondWidth, thirdWidth := actionsPaneWidths(m.ctx.ScreenWidth)

	scroll.Register(selection.ID("actions-workflows"), scroll.Bounds{
		X:      0,
		Y:      contentTop,
		Width:  firstWidth,
		Height: height,
	})
	scroll.Register(selection.ID("actions-runs"), scroll.Bounds{
		X:      firstWidth,
		Y:      contentTop,
		Width:  secondWidth,
		Height: height,
	})
	scroll.Register(selection.ID("actions-details"), scroll.Bounds{
		X:      firstWidth + secondWidth,
		Y:      contentTop,
		Width:  thirdWidth,
		Height: height,
	})
}

// previewPaneBounds returns the sidebar's screen rectangle for the current
// preview position, matching the geometry used by copy-selection.
func (m *Model) previewPaneBounds(contentTop int) scroll.Bounds {
	originX, originY := m.previewContentOrigin()
	if m.ctx.PreviewPosition == "bottom" {
		return scroll.Bounds{
			X:      originX,
			Y:      originY,
			Width:  m.ctx.DynamicPreviewWidth,
			Height: m.ctx.DynamicPreviewHeight,
		}
	}
	return scroll.Bounds{
		X:      originX,
		Y:      originY,
		Width:  m.ctx.DynamicPreviewWidth,
		Height: m.ctx.MainContentHeight,
	}
}

// scrollBoundsFromSelection resolves a selection region's current screen bounds
// (whether marker- or explicitly-registered) into scroll.Bounds.
func scrollBoundsFromSelection(id string) (scroll.Bounds, bool) {
	b, _, _, ok := selection.Region(id)
	if !ok || b.Empty() {
		return scroll.Bounds{}, false
	}
	return scroll.Bounds{X: b.X, Y: b.Y, Width: b.Width, Height: b.Height}, true
}
