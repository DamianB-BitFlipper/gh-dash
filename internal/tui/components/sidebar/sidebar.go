package sidebar

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dlvhdr/gh-dehub/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/keys"
)

type Model struct {
	IsOpen     bool
	header     string
	data       string
	viewport   viewport.Model
	ctx        *context.ProgramContext
	emptyState string
}

func NewModel() Model {
	vp := viewport.New(
		viewport.WithWidth(0),
		viewport.WithHeight(0),
	)

	return Model{
		IsOpen:     false,
		data:       "",
		viewport:   vp,
		ctx:        nil,
		emptyState: "Nothing selected...",
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Keys.Down):
			m.viewport.ScrollDown(1)

		case key.Matches(msg, keys.Keys.Up):
			m.viewport.ScrollUp(1)

		case key.Matches(msg, keys.Keys.PageDown):
			m.viewport.HalfPageDown()

		case key.Matches(msg, keys.Keys.PageUp):
			m.viewport.HalfPageUp()

		case key.Matches(msg, keys.Keys.PreviewTop), key.Matches(msg, keys.Keys.FirstLine):
			m.viewport.GotoTop()

		case key.Matches(msg, keys.Keys.PreviewBottom), key.Matches(msg, keys.Keys.LastLine):
			m.viewport.GotoBottom()
		}
	}

	return m, nil
}

func (m Model) View() string {
	if !m.IsOpen {
		return ""
	}

	if m.ctx.PreviewPosition == "bottom" {
		height := m.ctx.DynamicPreviewHeight
		width := m.ctx.DynamicPreviewWidth
		style := m.ctx.Styles.Sidebar.BottomRoot.
			Height(height).
			Width(width)
		if m.ctx.ActivePane == "preview" {
			style = focusedBorder(style)
		}

		if m.data == "" {
			return style.Align(lipgloss.Center).Render(
				lipgloss.PlaceVertical(height, lipgloss.Center, m.emptyState),
			)
		}

		return style.Render(m.renderContent())
	}

	// Right mode
	height := m.ctx.MainContentHeight
	style := m.ctx.Styles.Sidebar.Root.
		Height(height).
		Width(m.ctx.DynamicPreviewWidth)

	if m.data == "" {
		return style.Align(lipgloss.Center).Render(
			lipgloss.PlaceVertical(height, lipgloss.Center, m.emptyState),
		)
	}

	return style.Render(m.renderContent())
}

func focusedBorder(style lipgloss.Style) lipgloss.Style {
	return style.BorderForeground(lipgloss.Color("#F6E58D"))
}

func (m Model) renderContent() string {
	content := []string{}
	if m.header != "" {
		content = append(content, m.header)
	}
	content = append(
		content,
		m.viewport.View(),
		m.ctx.Styles.Sidebar.PagerStyle.
			Render(fmt.Sprintf("%d%%", int(m.viewport.ScrollPercent()*100))),
	)
	return lipgloss.JoinVertical(lipgloss.Top, content...)
}

func (m *Model) SetHeader(header string) {
	m.header = header
	m.updateViewportDimensions()
}

func (m *Model) ClearHeader() {
	m.header = ""
	m.updateViewportDimensions()
}

func (m *Model) SetContent(data string) {
	m.data = data
	m.viewport.SetContent(data)
}

func (m *Model) GetSidebarContentWidth() int {
	if m.ctx == nil || m.ctx.Config == nil {
		return 0
	}
	if m.ctx.PreviewPosition == "bottom" {
		return max(0, m.ctx.DynamicPreviewWidth)
	}
	return max(0, m.ctx.DynamicPreviewWidth-m.ctx.Styles.Sidebar.BorderWidth)
}

func (m *Model) ScrollToTop() {
	m.viewport.GotoTop()
}

func (m *Model) ScrollToBottom() {
	m.viewport.GotoBottom()
}

func (m *Model) YOffset() int {
	return m.viewport.YOffset()
}

func (m *Model) ViewportHeight() int {
	return m.viewport.Height()
}

func (m *Model) ScrollToOffset(offset int) {
	m.viewport.SetYOffset(offset)
}

func (m *Model) ScrollToPercent(percent float64) {
	totalLines := m.viewport.TotalLineCount()
	targetLine := int(float64(totalLines) * percent)
	m.viewport.SetYOffset(targetLine)
}

func (m *Model) UpdateProgramContext(ctx *context.ProgramContext) {
	if ctx == nil {
		return
	}
	m.ctx = ctx
	m.updateViewportDimensions()
}

func (m *Model) updateViewportDimensions() {
	if m.ctx == nil {
		return
	}
	headerHeight := lipgloss.Height(m.header)
	if m.ctx.PreviewPosition == "bottom" {
		m.viewport.SetHeight(max(0, m.ctx.DynamicPreviewHeight-m.ctx.Styles.Sidebar.PagerHeight-headerHeight))
	} else {
		m.viewport.SetHeight(max(0, m.ctx.MainContentHeight-m.ctx.Styles.Sidebar.PagerHeight-headerHeight))
	}
	m.viewport.SetWidth(m.GetSidebarContentWidth())
}
