package tabs

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dlvhdr/gh-dehub/v4/internal/config"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/common"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/actionssection"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/carousel"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/section"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dehub/v4/internal/utils"
)

type SectionTab struct {
	section section.Section
	spinner spinner.Model
}

type Model struct {
	sections         []section.Section
	sectionTabs      []SectionTab
	carousel         carousel.Model
	ctx              *context.ProgramContext
	hasSearchSection bool
}

func NewModel(ctx *context.ProgramContext) Model {
	c := carousel.New(
		carousel.WithHeight(1),
		carousel.WithOverflowIndicators("←", "→"),
		carousel.WithSeparators(),
	)
	m := Model{
		carousel:         c,
		hasSearchSection: true,
	}
	m.UpdateProgramContext(ctx)

	return m
}

// SetHasSearchSection controls whether the tabs render an implicit search
// section at index 0 (special-cased on the right with a search icon).
// Views like Actions that have no global search bar should set this to false
// so their first section is rendered like any other tab.
func (m *Model) SetHasSearchSection(v bool) {
	m.hasSearchSection = v
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	cmds := make([]tea.Cmd, 0)
	switch msg := msg.(type) {
	case spinner.TickMsg:
		for i, tab := range m.sectionTabs {
			if tab.section.GetIsLoading() {
				var cmd tea.Cmd
				m.sectionTabs[i].spinner, cmd = tab.spinner.Update(msg)
				cmds = append(cmds, cmd)
			}
		}
	}

	m.UpdateTabTitles()

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	tabsWidth := m.ctx.ScreenWidth
	c := m.viewSectionTabs(tabsWidth)
	content := m.ctx.Styles.Tabs.TabsRow.
		Width(m.ctx.ScreenWidth).
		Height(common.HeaderHeight).
		BorderBottom(false).
		Render(lipgloss.NewStyle().Width(tabsWidth).Render(c))

	return lipgloss.JoinVertical(lipgloss.Left, content, m.focusDivider())
}

func (m Model) viewSectionTabs(width int) string {
	if len(m.sectionTabs) == 0 {
		return ""
	}

	if !m.hasSearchSection {
		// No implicit search section; render every tab left-to-right.
		return truncateToWidth(m.renderSectionTabItems(0, len(m.sectionTabs)), width)
	}

	search := m.renderSearchSlot()
	searchWidth := min(lipgloss.Width(search), width)
	leftWidth := max(0, width-searchWidth)
	left := truncateToWidth(m.renderSectionTabItems(1, len(m.sectionTabs)), leftWidth)
	spacing := strings.Repeat(" ", max(0, leftWidth-lipgloss.Width(left)))
	return lipgloss.JoinHorizontal(lipgloss.Bottom, search, left, spacing)
}

func truncateToWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	return ansi.Truncate(s, width, constants.Ellipsis)
}

// renderSearchSlot renders the rightmost slot of the tabs row. When the
// currently-selected section has an active search (global or local), the
// search input is rendered inline so it shares the tabs row with the
// section tabs. Otherwise the historical search-icon tab is rendered.
func (m Model) renderSearchSlot() string {
	cursor := m.carousel.Cursor()
	if cursor >= 0 && cursor < len(m.sectionTabs) {
		if sv := m.sectionTabs[cursor].section.HeaderSearchView(); sv != "" {
			// Cap the inline search width so it never crowds the
			// section tabs out of the row.
			maxW := max(10, m.ctx.ScreenWidth/2)
			return lipgloss.NewStyle().
				MaxWidth(maxW).
				PaddingRight(1).
				Render(sv)
		}
	}
	return m.renderSectionTabItems(0, 1)
}

func (m Model) renderSectionTabItems(start, end int) string {
	parts := make([]string, 0, max(0, end-start)*2)
	for i := start; i < end; i++ {
		if i > start {
			parts = append(parts, m.ctx.Styles.Tabs.TabSeparator.Render("|"))
		}
		style := m.ctx.Styles.Tabs.Tab
		if m.carousel.Cursor() == i {
			style = m.ctx.Styles.Tabs.ActiveTab
		}
		parts = append(parts, style.Render(m.sectionTabTitle(i)))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (m Model) sectionTabTitle(i int) string {
	if i < 0 || i >= len(m.sectionTabs) {
		return ""
	}
	cfg := m.sectionTabs[i].section.GetConfig()
	title := cfg.Title
	isSearchIndex := m.hasSearchSection && i == 0
	if isSearchIndex {
		if title == "" {
			title = constants.SearchIcon
		}
	} else if m.sectionTabs[i].section.GetIsLoading() {
		title = fmt.Sprintf("%s %s", title, m.sectionTabs[i].spinner.View())
	} else if m.ctx.Config.Theme.Ui.SectionsShowCount {
		title = fmt.Sprintf("%s (%s)", title, utils.ShortNumber(m.sectionTabs[i].section.GetTotalCount()))
	}
	return title
}

func (m Model) focusDivider() string {
	primary := color.Color(m.ctx.Theme.PrimaryBorder)
	focus := color.Color(lipgloss.Color("#F6E58D"))
	if m.ctx.View == config.ActionsView {
		return m.actionsFocusDivider(color.Color(m.ctx.Theme.FaintBorder), focus)
	}

	line := strings.Repeat("━", max(0, m.ctx.ScreenWidth))
	if !m.ctx.SidebarOpen || m.ctx.PreviewPosition == "bottom" {
		color := primary
		if m.ctx.ActivePane == "main" {
			color = focus
		}
		return lipgloss.NewStyle().Foreground(color).Render(line)
	}

	mainWidth := max(0, min(m.ctx.MainContentWidth, m.ctx.ScreenWidth))
	previewWidth := max(0, m.ctx.ScreenWidth-mainWidth)
	mainColor := primary
	previewColor := primary
	if m.ctx.ActivePane == "preview" {
		previewColor = focus
	} else {
		mainColor = focus
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Foreground(mainColor).Render(strings.Repeat("━", mainWidth)),
		lipgloss.NewStyle().Foreground(previewColor).Render(strings.Repeat("━", previewWidth)),
	)
}

func (m Model) actionsFocusDivider(primary, focus color.Color) string {
	cursor := m.carousel.Cursor()
	if cursor < 0 || cursor >= len(m.sectionTabs) {
		return lipgloss.NewStyle().Foreground(primary).Render(strings.Repeat("━", max(0, m.ctx.ScreenWidth)))
	}

	as, ok := m.sectionTabs[cursor].section.(*actionssection.Model)
	if !ok || as == nil {
		return lipgloss.NewStyle().Foreground(primary).Render(strings.Repeat("━", max(0, m.ctx.ScreenWidth)))
	}

	workflowsWidth, runsWidth, detailsWidth := actionsDividerWidths(m.ctx.ScreenWidth)
	workflowsColor := primary
	runsColor := primary
	detailsColor := primary
	switch as.FocusedPane() {
	case actionssection.PaneWorkflows:
		workflowsColor = focus
	case actionssection.PaneRuns:
		runsColor = focus
	case actionssection.PaneDetails:
		detailsColor = focus
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Foreground(workflowsColor).Render(strings.Repeat("━", workflowsWidth)),
		lipgloss.NewStyle().Foreground(runsColor).Render(strings.Repeat("━", runsWidth)),
		lipgloss.NewStyle().Foreground(detailsColor).Render(strings.Repeat("━", detailsWidth)),
	)
}

func actionsDividerWidths(total int) (workflows, runs, details int) {
	if total <= 0 {
		return 0, 0, 0
	}
	const (
		minWorkflows = 20
		minRuns      = 20
		minDetails   = 30
	)

	workflows = total * 22 / 100
	runs = total * 22 / 100
	if workflows < minWorkflows {
		workflows = minWorkflows
	}
	if runs < minRuns {
		runs = minRuns
	}

	details = max(0, total-workflows-runs)
	if details < minDetails {
		short := minDetails - details
		fromW := min(short/2+short%2, max(0, workflows-minWorkflows))
		workflows -= fromW
		short -= fromW
		fromR := min(short, max(0, runs-minRuns))
		runs -= fromR
		details = max(0, total-workflows-runs)
	}
	return workflows, runs, details
}

func (m *Model) CurrSectionId() int {
	return m.carousel.Cursor()
}

func (m *Model) SetCurrSectionId(id int) {
	m.carousel.SetCursor(id)
}

func (m *Model) UpdateProgramContext(ctx *context.ProgramContext) {
	m.ctx = ctx
	m.carousel.SetStyles(carousel.Styles{
		Item:              ctx.Styles.Tabs.Tab,
		Selected:          ctx.Styles.Tabs.ActiveTab,
		OverflowIndicator: ctx.Styles.Tabs.OverflowIndicator,
		Separator:         ctx.Styles.Tabs.TabSeparator,
	})

	m.carousel.SetWidth(ctx.ScreenWidth)
}

func (m *Model) SetSections(sections []section.Section) {
	sectionTabs := make([]SectionTab, 0)
	for _, s := range sections {
		tab := SectionTab{section: s, spinner: spinner.New(
			spinner.WithSpinner(spinner.Dot), spinner.WithStyle(
				lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).PaddingLeft(2),
			),
		)}
		sectionTabs = append(sectionTabs, tab)
	}
	m.sectionTabs = sectionTabs
	m.UpdateTabTitles()
}

func (m *Model) UpdateTabTitles() {
	titles := make([]string, 0)
	for i, tab := range m.sectionTabs {
		cfg := tab.section.GetConfig()
		title := cfg.Title
		isSearchIndex := m.hasSearchSection && i == 0
		if isSearchIndex {
			if title == "" {
				title = constants.SearchIcon
			}
		} else if tab.section.GetIsLoading() {
			title = fmt.Sprintf("%s %s", title, m.sectionTabs[i].spinner.View())
		} else if m.ctx.Config.Theme.Ui.SectionsShowCount {
			title = fmt.Sprintf("%s (%s)", title,
				utils.ShortNumber(tab.section.GetTotalCount()))
		}

		titles = append(titles, title)
	}

	oldCursor := m.carousel.Cursor()
	m.carousel.SetItems(titles)
	m.carousel.SetCursor(oldCursor)
}

func (m *Model) SetAllLoading() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	for i := range m.sectionTabs {
		cmds = append(cmds, m.sectionTabs[i].spinner.Tick)
	}

	return cmds
}
