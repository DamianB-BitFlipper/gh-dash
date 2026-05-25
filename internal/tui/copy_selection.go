package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/tui/common"
)

type copySelectionPane int

const (
	copySelectionPaneNone copySelectionPane = iota
	copySelectionPaneMain
	copySelectionPanePreview
	copySelectionPanePreviewLogs
)

type copySelectionBounds struct {
	x      int
	y      int
	width  int
	height int
}

type copySelectionModel struct {
	dragging bool
	pane     copySelectionPane
	startX   int
	startY   int
	endX     int
	endY     int
}

func (s *copySelectionModel) cancel() {
	*s = copySelectionModel{}
}

func (s *copySelectionModel) begin(pane copySelectionPane, x, y int) {
	s.dragging = true
	s.pane = pane
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
	return s.dragging && s.pane != copySelectionPaneNone
}

func (s copySelectionModel) moved() bool {
	return s.startX != s.endX || s.startY != s.endY
}

func (m *Model) copySelectionPaneAt(x, y int) (copySelectionPane, copySelectionBounds) {
	if logsBounds, ok := m.copySelectionPreviewLogsBounds(); ok && inCopySelectionBounds(x, y, logsBounds) {
		return copySelectionPanePreviewLogs, logsBounds
	}

	mainBounds, previewBounds := m.copySelectionBounds()
	if inCopySelectionBounds(x, y, previewBounds) {
		return copySelectionPanePreview, previewBounds
	}
	if inCopySelectionBounds(x, y, mainBounds) {
		return copySelectionPaneMain, mainBounds
	}
	return copySelectionPaneNone, copySelectionBounds{}
}

func (m *Model) copySelectionPreviewLogsBounds() (copySelectionBounds, bool) {
	if _, ok := m.previewLogsCopySelectionContent(); !ok {
		return copySelectionBounds{}, false
	}

	logsZone := zone.Get("preview-logs")
	if logsZone == nil || logsZone.IsZero() {
		return copySelectionBounds{}, false
	}
	return copySelectionBounds{
		x:      logsZone.StartX,
		y:      logsZone.StartY,
		width:  max(0, logsZone.EndX-logsZone.StartX+1),
		height: max(0, logsZone.EndY-logsZone.StartY+1),
	}, true
}

func (m *Model) copySelectionBounds() (copySelectionBounds, copySelectionBounds) {
	contentY := m.copySelectionContentY()
	main := copySelectionBounds{
		x:      0,
		y:      contentY,
		width:  max(0, m.ctx.MainContentWidth),
		height: max(0, m.ctx.MainContentHeight),
	}

	if !m.sidebar.IsOpen {
		return main, copySelectionBounds{}
	}

	if m.ctx.PreviewPosition == "bottom" {
		return main, copySelectionBounds{
			x:      0,
			y:      contentY + m.ctx.MainContentHeight + m.ctx.Styles.Sidebar.BorderWidth,
			width:  max(0, m.ctx.DynamicPreviewWidth),
			height: max(0, m.ctx.DynamicPreviewHeight),
		}
	}

	borderWidth := m.ctx.Styles.Sidebar.BorderWidth
	return main, copySelectionBounds{
		x:      m.ctx.MainContentWidth + borderWidth,
		y:      contentY,
		width:  max(0, m.ctx.DynamicPreviewWidth-borderWidth),
		height: max(0, m.ctx.MainContentHeight),
	}
}

func (m *Model) copySelectionPreviewDisplayBounds() copySelectionBounds {
	contentY := m.copySelectionContentY()
	if !m.sidebar.IsOpen {
		return copySelectionBounds{}
	}
	if m.ctx.PreviewPosition == "bottom" {
		return copySelectionBounds{
			x:      0,
			y:      contentY + m.ctx.MainContentHeight,
			width:  max(0, m.ctx.DynamicPreviewWidth),
			height: max(0, m.ctx.DynamicPreviewHeight+m.ctx.Styles.Sidebar.BorderWidth),
		}
	}
	return copySelectionBounds{
		x:      m.ctx.MainContentWidth,
		y:      contentY,
		width:  max(0, m.ctx.DynamicPreviewWidth),
		height: max(0, m.ctx.MainContentHeight),
	}
}

func (m *Model) copySelectionContentY() int {
	if m.ctx.View == "repo" {
		return 1
	}
	return common.TabsHeight
}

func inCopySelectionBounds(x, y int, bounds copySelectionBounds) bool {
	return bounds.width > 0 && bounds.height > 0 &&
		x >= bounds.x && x < bounds.x+bounds.width &&
		y >= bounds.y && y < bounds.y+bounds.height
}

func clampCopySelectionPoint(x, y int, bounds copySelectionBounds) (int, int) {
	if bounds.width <= 0 || bounds.height <= 0 {
		return bounds.x, bounds.y
	}
	x = min(max(x, bounds.x), bounds.x+bounds.width-1)
	y = min(max(y, bounds.y), bounds.y+bounds.height-1)
	return x, y
}

func (m *Model) copySelectionText() string {
	if !m.copySelection.hasSelection() {
		return ""
	}

	bounds, content, ok := m.copySelectionPaneContent(m.copySelection.pane)
	if !ok {
		return ""
	}

	startX, startY := clampCopySelectionPoint(m.copySelection.startX, m.copySelection.startY, bounds)
	endX, endY := clampCopySelectionPoint(m.copySelection.endX, m.copySelection.endY, bounds)
	return extractCopySelectionText(content, bounds, startX, startY, endX, endY)
}

func (m *Model) copySelectionPaneContent(pane copySelectionPane) (copySelectionBounds, string, bool) {
	mainBounds, previewBounds := m.copySelectionBounds()
	switch pane {
	case copySelectionPaneMain:
		if currSection := m.getCurrSection(); currSection != nil {
			return mainBounds, currSection.View(), true
		}
	case copySelectionPanePreview:
		return previewBounds, m.copySelectionPreviewContent(), true
	case copySelectionPanePreviewLogs:
		bounds, ok := m.copySelectionPreviewLogsBounds()
		if !ok {
			return copySelectionBounds{}, "", false
		}
		content, ok := m.previewLogsCopySelectionContent()
		return bounds, ansi.Strip(content), ok
	}
	return copySelectionBounds{}, "", false
}

func (m *Model) copySelectionPreviewContent() string {
	return stripCopySelectionPreviewBorder(
		m.sidebar.View(),
		m.ctx.PreviewPosition,
		m.ctx.Styles.Sidebar.BorderWidth,
	)
}

func stripCopySelectionPreviewBorder(content, position string, borderWidth int) string {
	if borderWidth <= 0 || content == "" {
		return ansi.Strip(content)
	}
	lines := strings.Split(ansi.Strip(content), "\n")
	if position == "bottom" {
		if len(lines) <= borderWidth {
			return ""
		}
		return strings.Join(lines[borderWidth:], "\n")
	}

	for i, line := range lines {
		lines[i] = copySelectionSliceCells(line, borderWidth, lipgloss.Width(line))
	}
	return strings.Join(lines, "\n")
}

func (m *Model) renderCopySelectionContent(pane copySelectionPane, content string) string {
	if !m.copySelection.hasSelection() || m.copySelection.pane != pane {
		return content
	}
	if pane == copySelectionPanePreviewLogs {
		return content
	}

	mainBounds, _ := m.copySelectionBounds()
	bounds := mainBounds
	if pane == copySelectionPanePreview {
		bounds = m.copySelectionPreviewDisplayBounds()
	}

	style := lipgloss.NewStyle().
		Background(m.ctx.Theme.SelectedBackground).
		Foreground(m.ctx.Theme.PrimaryText)
	return renderCopySelectionHighlight(
		content,
		bounds,
		m.copySelection.startX,
		m.copySelection.startY,
		m.copySelection.endX,
		m.copySelection.endY,
		style,
	)
}

func (m *Model) renderCopySelectionPreviewLogsLayer() *lipgloss.Layer {
	if !m.copySelection.hasSelection() || m.copySelection.pane != copySelectionPanePreviewLogs {
		return nil
	}

	bounds, ok := m.copySelectionPreviewLogsBounds()
	if !ok {
		return nil
	}
	content, ok := m.previewLogsCopySelectionContent()
	if !ok || content == "" {
		return nil
	}

	style := lipgloss.NewStyle().
		Background(m.ctx.Theme.SelectedBackground).
		Foreground(m.ctx.Theme.PrimaryText)
	highlighted := renderCopySelectionHighlight(
		content,
		bounds,
		m.copySelection.startX,
		m.copySelection.startY,
		m.copySelection.endX,
		m.copySelection.endY,
		style,
	)
	highlighted = clipCopySelectionContent(highlighted, bounds)
	return lipgloss.NewLayer(highlighted).X(bounds.x).Y(bounds.y)
}

func (m *Model) previewLogsCopySelectionContent() (string, bool) {
	if content, ok := m.prView.ChecksLogsCopySelectionContent(); ok {
		return content, true
	}
	return m.actionsLogsCopySelectionContent()
}

func clipCopySelectionContent(content string, bounds copySelectionBounds) string {
	if bounds.width <= 0 || bounds.height <= 0 || content == "" {
		return ""
	}

	lines := strings.Split(content, "\n")
	if len(lines) > bounds.height {
		lines = lines[:bounds.height]
	}
	for i, line := range lines {
		lineWidth := lipgloss.Width(line)
		if lineWidth > bounds.width {
			lines[i] = ansi.Cut(line, 0, bounds.width)
		}
	}
	return strings.Join(lines, "\n")
}

func renderCopySelectionHighlight(
	content string,
	bounds copySelectionBounds,
	startX int,
	startY int,
	endX int,
	endY int,
	style lipgloss.Style,
) string {
	if bounds.width <= 0 || bounds.height <= 0 {
		return content
	}
	startX, startY = clampCopySelectionPoint(startX, startY, bounds)
	endX, endY = clampCopySelectionPoint(endX, endY, bounds)

	startCol := startX - bounds.x
	endCol := endX - bounds.x
	startRow := startY - bounds.y
	endRow := endY - bounds.y
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
		to := bounds.width
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

func extractCopySelectionText(content string, bounds copySelectionBounds, startX, startY, endX, endY int) string {
	if bounds.width <= 0 || bounds.height <= 0 {
		return ""
	}
	startX, startY = clampCopySelectionPoint(startX, startY, bounds)
	endX, endY = clampCopySelectionPoint(endX, endY, bounds)

	startCol := startX - bounds.x
	endCol := endX - bounds.x
	startRow := startY - bounds.y
	endRow := endY - bounds.y
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
		to := bounds.width
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
