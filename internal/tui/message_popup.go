package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"

	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
)

type messagePopup struct {
	title string
	body  string
	kind  messagePopupKind
}

type messagePopupKind int

const (
	messagePopupKindError messagePopupKind = iota
)

func newErrorMessagePopup(err error) *messagePopup {
	body := "Unknown error"
	if err != nil && strings.TrimSpace(err.Error()) != "" {
		body = strings.TrimSpace(err.Error())
	}
	return &messagePopup{title: "Error", body: body, kind: messagePopupKindError}
}

func (m Model) renderMessagePopup() string {
	if m.messagePopup == nil {
		return ""
	}

	width := min(max(40, m.ctx.ScreenWidth-8), 120)
	maxBodyHeight := max(3, m.ctx.ScreenHeight-10)

	body := lipgloss.NewStyle().
		Width(width - 4).
		Render(m.messagePopup.body)
	body = clampPopupBody(body, maxBodyHeight)

	accent := m.messagePopupAccentColor()
	glyph := m.messagePopupGlyph()
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(accent).
		Render(glyph + " " + m.messagePopup.title)
	hint := lipgloss.NewStyle().
		Foreground(m.ctx.Theme.FaintText).
		Render("esc/enter to dismiss")

	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Background(m.ctx.Theme.SelectedBackground).
		Padding(1, 2).
		Render(lipgloss.JoinVertical(lipgloss.Left, title, "", body, "", hint))
}

func (m Model) messagePopupAccentColor() compat.AdaptiveColor {
	switch m.messagePopup.kind {
	case messagePopupKindError:
		return m.ctx.Theme.ErrorText
	default:
		return m.ctx.Theme.PrimaryText
	}
}

func (m Model) messagePopupGlyph() string {
	switch m.messagePopup.kind {
	case messagePopupKindError:
		return m.ctx.Styles.Common.FailureGlyph
	default:
		return ""
	}
}

func clampPopupBody(body string, maxHeight int) string {
	lines := strings.Split(body, "\n")
	if len(lines) <= maxHeight {
		return body
	}

	visible := max(1, maxHeight-1)
	clamped := append([]string{}, lines[:visible]...)
	clamped = append(clamped, constants.Ellipsis+" truncated")
	return strings.Join(clamped, "\n")
}
