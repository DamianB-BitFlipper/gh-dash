package prview

import (
	"fmt"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dlvhdr/gh-dehub/v4/internal/data"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/selection"
	"github.com/dlvhdr/gh-dehub/v4/internal/utils"
)

func (m *Model) renderChangesOverview() string {
	w := m.getIndentedContentWidth() - 2
	changes := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(m.ctx.Theme.FaintBorder).
		Width(w).
		Padding(1)

	commits := lipgloss.NewStyle().
		Width(w).
		Padding(1)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), true).
		BorderForeground(m.ctx.Theme.FaintBorder).
		Width(m.getIndentedContentWidth())

	time := lipgloss.NewStyle().Render(utils.TimeElapsed(m.pr.Data.Primary.UpdatedAt))
	return box.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			changes.Render(
				lipgloss.JoinHorizontal(lipgloss.Top,
					lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).Render(" "),
					fmt.Sprintf("%d files changed", m.pr.Data.Primary.Files.TotalCount),
					" ",
					m.pr.RenderLines(false)),
			),
			commits.Render(
				lipgloss.JoinHorizontal(
					lipgloss.Top,
					lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).Render(" "),
					fmt.Sprintf("%d commits", m.pr.Data.Primary.Commits.TotalCount),
					" ",
					lipgloss.NewStyle().
						Foreground(m.ctx.Theme.FaintText).
						Render(fmt.Sprintf("%s ago", time)),
				),
			),
		),
	)
}

func (m *Model) renderChangedFiles() string {
	files := make([]string, 0)
	for _, file := range m.pr.Data.Primary.Files.Nodes {
		files = append(files, m.renderFile(file))
	}

	return lipgloss.JoinVertical(lipgloss.Left, files...)
}

// filesSelectionBlocks reproduces the Files-changed layout to compute each
// file row's content offset and height for selection scoping. Must match the
// assembly order in renderChangedFiles.
func (m *Model) filesSelectionBlocks() []selection.Block {
	if m.pr == nil || m.pr.Data == nil || m.pr.Data.Primary == nil {
		return nil
	}
	var blocks []selection.Block
	contentY := 0
	for _, file := range m.pr.Data.Primary.Files.Nodes {
		rendered := m.renderFile(file)
		h := lipgloss.Height(rendered)
		blocks = append(blocks, selection.Block{
			ID:       selection.ID("pr-file", file.Path),
			ContentY: contentY,
			Height:   h,
			Plain:    ansi.Strip(rendered),
			Styled:   rendered,
		})
		contentY += h
	}
	return blocks
}

func (m *Model) renderFile(file data.ChangedFile) string {
	icon := m.renderChangeTypeIcon(file.ChangeType)
	additions := lipgloss.NewStyle().
		Foreground(m.ctx.Theme.SuccessText).
		Width(5).
		Render(fmt.Sprintf("+%d", file.Additions))
	deletions := lipgloss.NewStyle().
		Foreground(m.ctx.Theme.ErrorText).
		Width(5).
		Render(fmt.Sprintf("-%d", file.Deletions))
	prefix := lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.JoinHorizontal(lipgloss.Top, additions, deletions),
		" ",
		icon,
		" ",
	)

	path := file.Path
	remaining := m.getIndentedContentWidth() - lipgloss.Width(prefix)
	if len(path) > remaining {
		path = lipgloss.JoinVertical(lipgloss.Left, path[0:remaining], " "+path[remaining:])
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		prefix,
		path,
	)
}

func (m *Model) renderChangeTypeIcon(changeType string) string {
	switch changeType {
	case "ADDED":
		return lipgloss.NewStyle().Foreground(m.ctx.Theme.SuccessText).Render("")
	case "DELETED":
		return lipgloss.NewStyle().Foreground(m.ctx.Theme.ErrorText).Render("")
	case "RENAMED":
		return lipgloss.NewStyle().Foreground(m.ctx.Theme.WarningText).Render("")
	case "COPIED":
		return lipgloss.NewStyle().Foreground(m.ctx.Theme.WarningText).Render("")
	case "MODIFIED":
		return lipgloss.NewStyle().Foreground(m.ctx.Theme.WarningText).Render("")
	case "CHANGED":
		return lipgloss.NewStyle().Foreground(m.ctx.Theme.WarningText).Render("")
	default:
		return ""
	}
}
