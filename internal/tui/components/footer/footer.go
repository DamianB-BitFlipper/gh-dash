package footer

import (
	"fmt"
	"path"
	"strings"

	bbHelp "charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dlvhdr/gh-dehub/v4/internal/config"
	"github.com/dlvhdr/gh-dehub/v4/internal/git"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/keys"
	"github.com/dlvhdr/gh-dehub/v4/internal/utils"
)

const viewSeparator = " │ "

type Model struct {
	ctx             *context.ProgramContext
	leftSection     *string
	rightSection    *string
	help            bbHelp.Model
	ShowAll         bool
	ShowConfirmQuit bool
}

func NewModel(ctx *context.ProgramContext) Model {
	help := bbHelp.New()
	help.ShowAll = true
	help.Styles = ctx.Styles.Help.BubbleStyles
	l := ""
	r := ""
	return Model{
		ctx:          ctx,
		help:         help,
		leftSection:  &l,
		rightSection: &r,
	}
}

func (m Model) View() string {
	var footer string

	if m.ShowConfirmQuit {
		footer = lipgloss.NewStyle().
			Render("Really quit? (Press y/enter to confirm, any other key to cancel)")
	} else {
		helpIndicator := lipgloss.NewStyle().
			Background(m.ctx.Theme.FaintText).
			Foreground(m.ctx.Theme.SelectedBackground).
			Padding(0, 1).
			Render("? help")
		viewSwitcher := m.renderViewSwitcher(m.ctx)
		leftSection := ""
		if m.leftSection != nil {
			leftSection = *m.leftSection
		}
		rightSection := ""
		if m.rightSection != nil {
			rightSection = *m.rightSection
		}
		spacing := lipgloss.NewStyle().
			Background(m.ctx.Theme.SelectedBackground).
			Render(
				strings.Repeat(
					" ",
					utils.Max(
						0,
						m.ctx.ScreenWidth-lipgloss.Width(
							viewSwitcher,
						)-lipgloss.Width(leftSection)-
							lipgloss.Width(rightSection)-
							lipgloss.Width(
								helpIndicator,
							),
					),
				),
			)

		footer = m.ctx.Styles.Common.FooterStyle.
			Render(lipgloss.JoinHorizontal(lipgloss.Top, viewSwitcher, leftSection, spacing,
				rightSection, helpIndicator))
	}

	if m.ShowAll {
		keymap := keys.CreateKeyMapForView(m.ctx.View)
		logo := m.viewLogo()
		fullHelp := m.help.View(closeHelpKeyMap{KeyMap: keymap})
		return lipgloss.JoinVertical(lipgloss.Top, footer, m.overlayLogo(fullHelp, logo))
	}

	return footer
}

func (m Model) overlayLogo(helpView, logo string) string {
	logoWidth := lipgloss.Width(logo)
	logoStart := max(0, m.ctx.ScreenWidth-logoWidth)
	helpLines := strings.Split(helpView, "\n")
	logoLines := strings.Split(logo, "\n")

	for i, logoLine := range logoLines {
		for len(helpLines) <= i {
			helpLines = append(helpLines, "")
		}

		line := ansi.Truncate(helpLines[i], logoStart, "")
		padding := strings.Repeat(" ", max(0, logoStart-lipgloss.Width(line)))
		helpLines[i] = line + padding + logoLine
	}

	return strings.Join(helpLines, "\n")
}

func (m Model) viewLogo() string {
	return lipgloss.NewStyle().
		Foreground(context.LogoColor).
		PaddingRight(1).
		Render(constants.Logo)
}

func (m *Model) SetShowConfirmQuit(val bool) {
	m.ShowConfirmQuit = val
}

func (m *Model) SetWidth(width int) {
	m.help.SetWidth(width)
}

func (m *Model) UpdateProgramContext(ctx *context.ProgramContext) {
	m.ctx = ctx
	m.help.Styles = ctx.Styles.Help.BubbleStyles
}

type closeHelpKeyMap struct {
	bbHelp.KeyMap
}

func (k closeHelpKeyMap) FullHelp() [][]key.Binding {
	groups := k.KeyMap.FullHelp()
	closeHelp := key.NewBinding(
		key.WithKeys("q", "?"),
		key.WithHelp("q/?", "close help"),
	)

	for i := range groups {
		for j := range groups[i] {
			if groups[i][j].Help().Desc == "help" {
				groups[i][j] = closeHelp
				return groups
			}
		}
	}

	return append(groups, []key.Binding{closeHelp})
}

func (m *Model) renderViewButton(view config.ViewType) string {
	isActive := m.ctx.View == view

	// Define icons and labels for each view
	var icon, label string
	// Define icons - notifications has solid/outline variants
	solidBell := ""
	outlineBell := ""

	switch view {
	case config.NotificationsView:
		if m.ctx.View == config.NotificationsView {
			icon = solidBell
		} else {
			icon = outlineBell
		}
		label = " Notifications"
	case config.PRsView:
		icon = ""
		label = " Pull Requests"
	case config.IssuesView:
		icon = ""
		label = " Issues"
	case config.ActionsView:
		icon = ""
		label = " Actions"
	}

	if isActive {
		// Active: colored icon + prominent background
		iconColor := m.ctx.Theme.SuccessText
		activeStyle := lipgloss.NewStyle().
			Foreground(iconColor).
			Background(m.ctx.Styles.ViewSwitcher.ActiveView.GetBackground()).
			Bold(true)
		if label != "" {
			return activeStyle.Render(icon) + activeStyle.Render(label)
		}
		return activeStyle.Render(icon)
	}

	// Inactive: faint styling
	return m.ctx.Styles.ViewSwitcher.InactiveView.Render(icon + label)
}

func (m *Model) renderViewSwitcher(ctx *context.ProgramContext) string {
	var repo string
	if m.ctx.RepoPath != "" {
		name := path.Base(m.ctx.RepoPath)
		if m.ctx.RepoUrl != "" {
			name = git.GetRepoShortName(m.ctx.RepoUrl)
		}
		repo = ctx.Styles.Common.FooterStyle.Render(fmt.Sprintf(" %s", name))
	}

	var user string
	if ctx.User != "" {
		user = ctx.Styles.Common.FooterStyle.Render("@" + ctx.User)
	}

	parts := []string{
		ctx.Styles.ViewSwitcher.ViewsSeparator.PaddingLeft(1).
			Render(m.renderViewButton(config.PRsView)),
		ctx.Styles.ViewSwitcher.ViewsSeparator.Render(viewSeparator),
	}
	parts = append(
		parts,
		m.renderViewButton(config.ActionsView),
		ctx.Styles.ViewSwitcher.ViewsSeparator.Render(viewSeparator),
		m.renderViewButton(config.IssuesView),
		ctx.Styles.ViewSwitcher.ViewsSeparator.Render(viewSeparator),
		m.renderViewButton(config.NotificationsView),
	)
	parts = append(
		parts,
		lipgloss.NewStyle().Background(ctx.Styles.Common.FooterStyle.GetBackground()).Foreground(
			ctx.Styles.ViewSwitcher.ViewsSeparator.GetBackground(),
		).Render(" "),
		repo,
		ctx.Styles.Common.FooterStyle.Foreground(m.ctx.Theme.FaintText).Render(" • "),
		user,
		ctx.Styles.Common.FooterStyle.Foreground(m.ctx.Theme.FaintBorder).Render(" │"),
	)

	view := lipgloss.JoinHorizontal(
		lipgloss.Top,
		parts...,
	)

	return ctx.Styles.ViewSwitcher.Root.Render(view)
}

func (m *Model) SetLeftSection(leftSection string) {
	*m.leftSection = leftSection
}

func (m *Model) SetRightSection(rightSection string) {
	*m.rightSection = rightSection
}
