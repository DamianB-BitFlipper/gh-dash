package tui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dlvhdr/gh-dehub/v4/internal/tui/context"
)

type openPRURLPopup struct {
	input textinput.Model
}

func newOpenPRURLPopup(ctx *context.ProgramContext) *openPRURLPopup {
	input := textinput.New()
	input.Prompt = ""
	input.Placeholder = "https://github.com/owner/repo/pull/123"
	input.CharLimit = 512
	if ctx != nil {
		base := lipgloss.NewStyle().Background(ctx.Theme.SelectedBackground)
		input.SetStyles(textinput.Styles{
			Focused: textinput.StyleState{
				Text:        base.Foreground(ctx.Theme.PrimaryText),
				Placeholder: base.Foreground(ctx.Theme.FaintText),
			},
			Blurred: textinput.StyleState{
				Text:        base.Foreground(ctx.Theme.PrimaryText),
				Placeholder: base.Foreground(ctx.Theme.FaintText),
			},
			Cursor: textinput.CursorStyle{Color: ctx.Theme.FaintText, Shape: tea.CursorBar, Blink: true},
		})
	}
	input.Focus()
	return &openPRURLPopup{input: input}
}

func (p *openPRURLPopup) update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return cmd
}

func (p *openPRURLPopup) value() string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(p.input.Value())
}

func (m *Model) openPRURLPopupForInput() tea.Cmd {
	m.openPRURLPopup = newOpenPRURLPopup(m.ctx)
	return textinput.Blink
}

func (m *Model) updateOpenPRURLPopup(msg tea.Msg) tea.Cmd {
	if m.openPRURLPopup == nil {
		return nil
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "ctrl+c", "esc":
			m.openPRURLPopup = nil
			return nil
		case "enter":
			rawURL := m.openPRURLPopup.value()
			m.openPRURLPopup = nil

			ref, err := parseGitHubPRURL(rawURL)
			if err != nil {
				return m.notifyErr("Invalid PR URL")
			}

			return tea.Batch(m.openPRURLInSearchSection(ref)...)
		}
	}

	return m.openPRURLPopup.update(msg)
}

func (m Model) renderOpenPRURLPopup() string {
	if m.openPRURLPopup == nil {
		return ""
	}

	width := min(max(56, m.ctx.ScreenWidth-10), 96)
	contentWidth := width - 4
	accent := m.ctx.Theme.PrimaryBorder
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Padding(0, 1)
	inputWidth := max(1, contentWidth-inputStyle.GetHorizontalFrameSize())
	m.openPRURLPopup.input.SetWidth(inputWidth)

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.ctx.Theme.PrimaryText).
		Render("Open PR URL")

	description := lipgloss.NewStyle().
		Width(contentWidth).
		Foreground(m.ctx.Theme.FaintText).
		Render("Paste a GitHub pull request URL to open it in the PR search tab.")

	input := inputStyle.
		Width(inputWidth).
		Render(m.openPRURLPopup.input.View())

	hint := lipgloss.NewStyle().
		Foreground(m.ctx.Theme.FaintText).
		Render("enter open | esc cancel")

	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Background(m.ctx.Theme.SelectedBackground).
		Padding(1, 2).
		Render(lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			description,
			"",
			input,
			"",
			hint,
		))
}
