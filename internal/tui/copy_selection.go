package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dlvhdr/gh-dehub/v4/internal/tui/common"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/prrow"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/selection"
)

// copySelectionModel tracks an in-progress mouse drag selection. Selection is
// scoped to a single registered region (see internal/tui/components/selection):
// the region the drag started in. The drag end point is always clamped to that
// region's bounds so a selection started inside a sub-component (e.g. a single
// comment card) stays scoped to it.
type copySelectionModel struct {
	dragging bool
	regionID string
	startX   int
	startY   int
	endX     int
	endY     int
}

func (s *copySelectionModel) cancel() {
	*s = copySelectionModel{}
}

func (s *copySelectionModel) begin(regionID string, x, y int) {
	s.dragging = true
	s.regionID = regionID
	s.startX = x
	s.startY = y
	s.endX = x
	s.endY = y
}

func (s *copySelectionModel) update(x, y int) {
	if !s.dragging {
		return
	}
	s.endX = x
	s.endY = y
}

func (s copySelectionModel) hasSelection() bool {
	return s.dragging && s.regionID != ""
}

func (s copySelectionModel) moved() bool {
	return s.startX != s.endX || s.startY != s.endY
}

// copySelectionRegionAt returns the innermost registered selection region
// containing the point, along with its bounds. The empty id means no region.
func copySelectionRegionAt(x, y int) (string, selection.Bounds) {
	id := selection.InnermostAt(x, y)
	if id == "" {
		return "", selection.Bounds{}
	}
	bounds, _, _, ok := selection.Region(id)
	if !ok {
		return "", selection.Bounds{}
	}
	return id, bounds
}

// copySelectionRegionBounds resolves the active region's bounds, or empty bounds
// when there is none.
func (m *Model) copySelectionRegionBounds() selection.Bounds {
	if !m.copySelection.hasSelection() {
		return selection.Bounds{}
	}
	bounds, _, _, _ := selection.Region(m.copySelection.regionID)
	return bounds
}

// copySelectionContentY is the screen Y of the first content row below the tabs
// bar (the top of both the main and preview panes).
func (m *Model) copySelectionContentY() int {
	if m.ctx.View == "repo" {
		return 1
	}
	return common.TabsHeight
}

// previewContentOrigin returns the screen top-left of the sidebar's inner
// content area (inside the border), for the current preview position.
func (m *Model) previewContentOrigin() (x, y int) {
	contentY := m.copySelectionContentY()
	if m.ctx.PreviewPosition == "bottom" {
		// Bottom mode: a top border occupies BorderWidth rows above the content.
		return 0, contentY + m.ctx.MainContentHeight + m.ctx.Styles.Sidebar.BorderWidth
	}
	// Right mode: a left border occupies BorderWidth cols left of the content.
	return m.ctx.MainContentWidth + m.ctx.Styles.Sidebar.BorderWidth, contentY
}

// registerSelectionRegions registers all per-subcomponent selection regions for
// the current frame: the main section's rows and the preview's sub-components.
// Bounds are computed arithmetically from each scroll area's origin and offset,
// because bubblezone markers do not survive a viewport's line clipping.
func (m *Model) registerSelectionRegions() {
	contentTop := m.copySelectionContentY()

	if currSection := m.getCurrSection(); currSection != nil {
		registerScroll(currSection.RowsSelectionScroll(contentTop))
	}

	if m.sidebar.IsOpen {
		registerScroll(m.previewSelectionScroll())
	}
}

// previewSelectionScroll describes the current sidebar content as a scrollable
// selection area (or an empty Scroll when it has no sub-regions).
func (m *Model) previewSelectionScroll() selection.Scroll {
	var blocks []selection.Block
	switch m.getCurrRowData().(type) {
	case *prrow.Data:
		blocks = m.prView.PreviewSelectionBlocks()
	}
	if len(blocks) == 0 {
		return selection.Scroll{}
	}

	originX, originY := m.previewContentOrigin()
	return selection.Scroll{
		// Content is horizontally padded inside the viewport.
		OriginX: originX + m.ctx.Styles.Sidebar.ContentPadding,
		// The scrollable viewport begins below the (non-scrolling) header.
		OriginY:       originY + m.sidebar.HeaderHeight(),
		Width:         max(0, m.sidebar.GetSidebarContentWidth()-2*m.ctx.Styles.Sidebar.ContentPadding),
		VisibleHeight: m.sidebar.ViewportHeight(),
		YOffset:       m.sidebar.YOffset(),
		Blocks:        blocks,
	}
}

// registerScroll converts a scroll area's content-relative selection Blocks into
// absolute screen bounds and registers them.
//
// Blocks that are fully scrolled out of view are skipped. Partially visible
// blocks are clipped to the visible window (both their bounds and their
// styled/plain content) so the highlight overlay and copy stay correct.
func registerScroll(s selection.Scroll) {
	originX, originY := s.OriginX, s.OriginY
	width, visibleHeight, yOffset := s.Width, s.VisibleHeight, s.YOffset
	if width <= 0 || visibleHeight <= 0 {
		return
	}
	viewTop := yOffset
	viewBottom := yOffset + visibleHeight // exclusive

	for _, block := range s.Blocks {
		if block.ID == "" || block.Height <= 0 {
			continue
		}
		blockTop := block.ContentY
		blockBottom := block.ContentY + block.Height // exclusive

		// Intersect the block's content-line range with the visible window.
		visTop := max(blockTop, viewTop)
		visBottom := min(blockBottom, viewBottom)
		if visBottom <= visTop {
			continue // fully scrolled out
		}

		// Drop the leading lines of the block that are scrolled above the window.
		skip := visTop - blockTop
		visibleRows := visBottom - visTop

		bounds := selection.Bounds{
			X:      originX,
			Y:      originY + (visTop - yOffset),
			Width:  width,
			Height: visibleRows,
		}
		plain := sliceContentLines(block.Plain, skip, visibleRows)
		styled := sliceContentLines(block.Styled, skip, visibleRows)
		selection.RegisterBounds(block.ID, bounds, plain, styled)
	}
}

// sliceContentLines returns count lines from content starting at the given line
// offset (0-based). Missing lines are dropped; it never pads.
func sliceContentLines(content string, offset, count int) string {
	if count <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if offset >= len(lines) {
		return ""
	}
	end := min(offset+count, len(lines))
	return strings.Join(lines[offset:end], "\n")
}

func clampCopySelectionPoint(x, y int, bounds selection.Bounds) (int, int) {
	if bounds.Empty() {
		return bounds.X, bounds.Y
	}
	x = min(max(x, bounds.X), bounds.X+bounds.Width-1)
	y = min(max(y, bounds.Y), bounds.Y+bounds.Height-1)
	return x, y
}

// copySelectionText extracts the plain text the user dragged over within the
// active region.
func (m *Model) copySelectionText() string {
	if !m.copySelection.hasSelection() {
		return ""
	}
	bounds, plain, _, ok := selection.Region(m.copySelection.regionID)
	if !ok {
		return ""
	}

	startX, startY := clampCopySelectionPoint(m.copySelection.startX, m.copySelection.startY, bounds)
	endX, endY := clampCopySelectionPoint(m.copySelection.endX, m.copySelection.endY, bounds)
	return extractCopySelectionText(plain, bounds, startX, startY, endX, endY)
}

// renderCopySelectionLayer renders the highlighted selection for the active
// region as an overlay layer positioned at the region's screen origin.
//
// The overlay is built from the region's *styled* content so it is visually
// identical to what's underneath, except the selected span gets the highlight
// background. Compositing it over the base view therefore preserves all
// formatting and only recolors the dragged span. The same uniform path works
// for every region (panes and computed sub-component regions) regardless of
// nesting depth. Returns nil when there is no active selection or the region
// can't be resolved.
func (m *Model) renderCopySelectionLayer() *lipgloss.Layer {
	if !m.copySelection.hasSelection() {
		return nil
	}
	bounds, _, styled, ok := selection.Region(m.copySelection.regionID)
	if !ok || styled == "" || bounds.Empty() {
		return nil
	}

	style := lipgloss.NewStyle().
		Background(m.ctx.Theme.SelectedBackground).
		Foreground(m.ctx.Theme.PrimaryText)
	highlighted := renderCopySelectionHighlight(
		styled,
		bounds,
		m.copySelection.startX,
		m.copySelection.startY,
		m.copySelection.endX,
		m.copySelection.endY,
		style,
	)
	highlighted = clipCopySelectionContent(highlighted, bounds)
	if highlighted == "" {
		return nil
	}
	return lipgloss.NewLayer(highlighted).X(bounds.X).Y(bounds.Y)
}

func clipCopySelectionContent(content string, bounds selection.Bounds) string {
	if bounds.Empty() || content == "" {
		return ""
	}

	lines := strings.Split(content, "\n")
	if len(lines) > bounds.Height {
		lines = lines[:bounds.Height]
	}
	for i, line := range lines {
		lineWidth := lipgloss.Width(line)
		if lineWidth > bounds.Width {
			lines[i] = ansi.Cut(line, 0, bounds.Width)
		}
	}
	return strings.Join(lines, "\n")
}

func renderCopySelectionHighlight(
	content string,
	bounds selection.Bounds,
	startX int,
	startY int,
	endX int,
	endY int,
	style lipgloss.Style,
) string {
	if bounds.Empty() {
		return content
	}
	startX, startY = clampCopySelectionPoint(startX, startY, bounds)
	endX, endY = clampCopySelectionPoint(endX, endY, bounds)

	startCol := startX - bounds.X
	endCol := endX - bounds.X
	startRow := startY - bounds.Y
	endRow := endY - bounds.Y
	if startRow > endRow || (startRow == endRow && startCol > endCol) {
		startRow, endRow = endRow, startRow
		startCol, endCol = endCol, startCol
	}

	lines := strings.Split(content, "\n")
	if startRow >= len(lines) {
		return content
	}
	endRow = min(endRow, len(lines)-1)

	for row := startRow; row <= endRow; row++ {
		line := lines[row]
		from := 0
		to := bounds.Width
		if row == startRow {
			from = startCol
		}
		if row == endRow {
			to = endCol + 1
		}
		if to <= from {
			continue
		}

		lineWidth := lipgloss.Width(line)
		prefix := ansi.Cut(line, 0, from)
		selected := ansi.Strip(ansi.Cut(line, from, to))
		suffix := ansi.Cut(line, to, lineWidth)
		selectedWidth := to - from
		if width := lipgloss.Width(selected); width < selectedWidth {
			selected += strings.Repeat(" ", selectedWidth-width)
		}
		lines[row] = prefix + style.Render(selected) + suffix
	}

	return strings.Join(lines, "\n")
}

func extractCopySelectionText(content string, bounds selection.Bounds, startX, startY, endX, endY int) string {
	if bounds.Empty() {
		return ""
	}
	startX, startY = clampCopySelectionPoint(startX, startY, bounds)
	endX, endY = clampCopySelectionPoint(endX, endY, bounds)

	startCol := startX - bounds.X
	endCol := endX - bounds.X
	startRow := startY - bounds.Y
	endRow := endY - bounds.Y
	if startRow > endRow || (startRow == endRow && startCol > endCol) {
		startRow, endRow = endRow, startRow
		startCol, endCol = endCol, startCol
	}

	lines := strings.Split(ansi.Strip(content), "\n")
	if startRow >= len(lines) {
		return ""
	}
	endRow = min(endRow, len(lines)-1)

	selected := make([]string, 0, endRow-startRow+1)
	for row := startRow; row <= endRow; row++ {
		line := lines[row]
		from := 0
		to := bounds.Width
		if row == startRow {
			from = startCol
		}
		if row == endRow {
			to = endCol + 1
		}
		selected = append(selected, strings.TrimRight(copySelectionSliceCells(line, from, to), " "))
	}

	return strings.TrimRight(strings.Join(selected, "\n"), "\n")
}

func copySelectionSliceCells(line string, from, to int) string {
	if to <= from {
		return ""
	}
	var b strings.Builder
	cell := 0
	for _, r := range line {
		w := lipgloss.Width(string(r))
		if w == 0 {
			w = 1
		}
		if cell+w > from && cell < to {
			b.WriteRune(r)
		}
		cell += w
		if cell >= to {
			break
		}
	}
	return b.String()
}
