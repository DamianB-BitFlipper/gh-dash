package tui

import (
	"fmt"

	"charm.land/lipgloss/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prssection"
)

func (m *Model) activeCreatePRSection() (*prssection.Model, bool) {
	currSection := m.getCurrSection()
	prSection, ok := currSection.(*prssection.Model)
	if !ok || prSection == nil {
		return nil, false
	}
	if !prSection.IsPromptConfirmationFocused() || prSection.GetPromptConfirmationAction() != "create_pr" {
		return nil, false
	}
	return prSection, true
}

func (m *Model) renderCreatePRPopup() string {
	prSection, ok := m.activeCreatePRSection()
	if !ok {
		return ""
	}

	width := min(max(50, m.ctx.ScreenWidth-10), 140)
	contentWidth := width - 4
	prSection.CreatePRForm.SetWidth(contentWidth)

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.ctx.Theme.PrimaryText).
		Render("Create PR")

	repo := ""
	if repoName, ok := prSection.RepoFromFilters(); ok {
		repo = lipgloss.NewStyle().
			Foreground(m.ctx.Theme.FaintText).
			Render(fmt.Sprintf("Repository: %s", repoName))
	}

	body := prSection.CreatePRForm.View()
	parts := []string{title}
	if repo != "" {
		parts = append(parts, repo)
	}
	parts = append(parts, "", body)

	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.ctx.Theme.PrimaryBorder).
		Background(m.ctx.Theme.SelectedBackground).
		Padding(1, 2).
		Render(lipgloss.JoinVertical(lipgloss.Left, parts...))
}
