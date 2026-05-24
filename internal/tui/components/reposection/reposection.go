package reposection

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/git"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/common"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/branch"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/search"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/section"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/table"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/tasks"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/keys"
	"github.com/dlvhdr/gh-dash/v4/internal/utils"
)

const SectionType = "repo"

type Model struct {
	section.BaseModel
	repo           *git.Repo
	Branches       []branch.Branch
	Prs            []data.PullRequestData
	isRefreshSetUp bool
	refreshId      int
}

func NewModel(
	id int,
	ctx *context.ProgramContext,
	cfg config.PrsSectionConfig,
	lastUpdated time.Time,
) Model {
	m := Model{}
	m.BaseModel = section.NewModel(
		ctx,
		section.NewSectionOptions{
			Id:          id,
			Config:      cfg.ToSectionConfig(),
			Type:        SectionType,
			Columns:     GetSectionColumns(ctx, cfg),
			Singular:    "branch",
			Plural:      "branches",
			LastUpdated: lastUpdated,
		},
	)
	m.SearchBar = search.NewModel(ctx, search.SearchOptions{Placeholder: "Search branches..."})
	m.SearchValue = ""
	m.repo = &git.Repo{Branches: []git.Branch{}}
	m.Branches = []branch.Branch{}
	m.Prs = []data.PullRequestData{}
	m.isRefreshSetUp = false

	return m
}

func (m *Model) Update(msg tea.Msg) (section.Section, tea.Cmd) {
	var cmd tea.Cmd
	cmds := make([]tea.Cmd, 0)
	var err error

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if handled, cmd := m.HandleLocalSearchKey(msg, m.BuildRows); handled {
			return m, cmd
		}

		if m.IsSearchFocused() {
			switch msg.String() {
			case "ctrl+c", "esc":
				m.SearchBar.SetValue(m.SearchValue)
				blinkCmd := m.SetIsSearching(false)
				return m, blinkCmd

			case "enter":
				m.Table.ResetCurrItem()
				m.SetIsSearching(false)
				m.SearchValue = m.SearchBar.Value()
				m.BuildRows()
				return m, nil
			}

			break
		}

		if m.IsPromptConfirmationFocused() {
			switch msg.String() {
			case "ctrl+c", "esc":
				m.PromptConfirmationBox.Reset()
				cmd = m.SetIsPromptConfirmationShown(false)
				return m, cmd

			case "enter":
				input := m.PromptConfirmationBox.Value()
				action := m.GetPromptConfirmationAction()
				branch := m.getCurrBranch().Data.Name
				sid := tasks.SectionIdentifier{Id: m.Id, Type: SectionType}
				switch action {
				case "new":
					cmd = m.newBranch(input)
				case "create_pr":
					cmd = tasks.CreatePR(m.Ctx, sid, branch, input)
				default:
					pr := findPRForRef(m.Prs, branch)
					if input == "" || input == "Y" || input == "y" {
						switch action {
						case "delete":
							cmd = m.deleteBranch()
						case "close":
							cmd = tasks.ClosePR(m.Ctx, sid, pr)
						case "reopen":
							cmd = tasks.ReopenPR(m.Ctx, sid, pr)
						case "ready":
							if pr != nil {
								cmd = tasks.TogglePRDraft(m.Ctx, sid, pr)
							}
						case "merge":
							cmd = tasks.MergePR(m.Ctx, sid, pr)
						case "update":
							cmd = tasks.UpdatePR(m.Ctx, sid, pr)
						}
					}
				}

				m.PromptConfirmationBox.Reset()
				blinkCmd := m.SetIsPromptConfirmationShown(false)

				return m, tea.Batch(cmd, blinkCmd)
			}

			break
		}

		switch {
		case key.Matches(msg, keys.BranchKeys.Checkout):
			cmd, err = m.checkout()
			if err != nil {
				m.Ctx.Error = err
			}

		case key.Matches(msg, keys.BranchKeys.Push):
			cmd, err = m.push(pushOptions{force: false})
			if err != nil {
				m.Ctx.Error = err
			}
		case key.Matches(msg, keys.BranchKeys.ForcePush):
			cmd, err = m.push(pushOptions{force: true})
			if err != nil {
				m.Ctx.Error = err
			}
		case key.Matches(msg, keys.BranchKeys.FastForward):
			cmd, err = m.fastForward()
			if err != nil {
				m.Ctx.Error = err
			}
		}

	case tasks.UpdateBranchMsg:
		if msg.IsCreated != nil && *msg.IsCreated {
			cmds = append(cmds, m.fetchPRCmd(msg.Name)...)
		}
		if msg.NewPr != nil {
			m.Prs = append(m.Prs, *msg.NewPr)
		}

	case repoMsg:
		m.repo = msg.repo
		m.SetIsLoading(false)
		m.Table.SetRows(m.BuildRows())
		if msg.resetSelection {
			m.Table.ResetCurrItem()
		}

	case SectionPullRequestsFetchedMsg:
		m.Prs = msg.Prs

	case RefreshBranchesMsg:
		if msg.id == m.refreshId {
			cmds = append(cmds, m.onRefreshBranchesMsg()...)
		}

	case RefreshPrsMsg:
		if msg.id == m.refreshId {
			cmds = append(cmds, m.onRefreshPrsMsg()...)
		}
	}

	m.updateBranchesWithPrs()

	cmds = append(cmds, cmd)

	search, searchCmd := m.SearchBar.Update(msg)
	cmds = append(cmds, searchCmd)
	m.SearchBar = search

	prompt, promptCmd := m.PromptConfirmationBox.Update(msg)
	cmds = append(cmds, promptCmd)
	m.PromptConfirmationBox = prompt

	m.Table.SetRows(m.BuildRows())

	m.Table.SetRows(m.BuildRows())
	table, tableCmd := m.Table.Update(msg)
	m.Table = table
	cmds = append(cmds, tableCmd)

	return m, tea.Batch(cmds...)
}

func (m *Model) View() string {
	view := ""
	if m.Table.Rows == nil {
		d := m.GetDimensions()
		view = lipgloss.Place(
			d.Width,
			d.Height,
			lipgloss.Center,
			lipgloss.Center,
			"No local branches",
		)
	} else {
		view = m.Table.View()
	}

	search := m.SearchBar.View(m.Ctx)
	if m.IsLocalSearching || m.LocalSearchValue != "" {
		search = m.LocalSearchBar.View(m.Ctx)
	}

	return m.Ctx.Styles.Section.ContainerStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left, search, view),
	)
}

func GetSectionColumns(
	ctx *context.ProgramContext,
	cfg config.PrsSectionConfig,
) []table.Column {
	dLayout := ctx.Config.Defaults.Layout.Prs
	sLayout := cfg.Layout

	updatedAtLayout := config.MergeColumnConfigs(
		dLayout.UpdatedAt,
		sLayout.UpdatedAt,
	)
	repoLayout := config.MergeColumnConfigs(dLayout.Repo, sLayout.Repo)
	titleLayout := config.MergeColumnConfigs(dLayout.Title, sLayout.Title)
	authorLayout := config.MergeColumnConfigs(dLayout.Author, sLayout.Author)
	assigneesLayout := config.MergeColumnConfigs(
		dLayout.Assignees,
		sLayout.Assignees,
	)
	baseLayout := config.MergeColumnConfigs(dLayout.Base, sLayout.Base)
	reviewStatusLayout := config.MergeColumnConfigs(
		dLayout.ReviewStatus,
		sLayout.ReviewStatus,
	)
	stateLayout := config.MergeColumnConfigs(dLayout.State, sLayout.State)
	ciLayout := config.MergeColumnConfigs(dLayout.Ci, sLayout.Ci)
	linesLayout := config.MergeColumnConfigs(dLayout.Lines, sLayout.Lines)

	if !ctx.Config.Theme.Ui.Table.Compact {
		return []table.Column{
			{
				Title:  "",
				Width:  utils.IntPtr(3),
				Hidden: stateLayout.Hidden,
			},
			{
				Title:  "Title",
				Grow:   utils.BoolPtr(true),
				Hidden: titleLayout.Hidden,
			},
			{
				Title:  "Assignees",
				Width:  assigneesLayout.Width,
				Hidden: assigneesLayout.Hidden,
			},
			{
				Title:  "Base",
				Width:  baseLayout.Width,
				Hidden: baseLayout.Hidden,
			},
			{
				Title:  "󰯢",
				Width:  utils.IntPtr(4),
				Hidden: reviewStatusLayout.Hidden,
			},
			{
				Title:  "",
				Width:  &ctx.Styles.PrSection.CiCellWidth,
				Grow:   new(bool),
				Hidden: ciLayout.Hidden,
			},
			{
				Title:  "",
				Width:  linesLayout.Width,
				Hidden: linesLayout.Hidden,
			},
			{
				Title:  "",
				Width:  updatedAtLayout.Width,
				Hidden: updatedAtLayout.Hidden,
			},
		}
	}

	return []table.Column{
		{
			Title:  "",
			Width:  utils.IntPtr(3),
			Hidden: stateLayout.Hidden,
		},
		{
			Title:  "",
			Width:  repoLayout.Width,
			Hidden: repoLayout.Hidden,
		},
		{
			Title:  "Title",
			Grow:   utils.BoolPtr(true),
			Hidden: titleLayout.Hidden,
		},
		{
			Title:  "Author",
			Width:  authorLayout.Width,
			Hidden: authorLayout.Hidden,
		},
		{
			Title:  "Assignees",
			Width:  assigneesLayout.Width,
			Hidden: assigneesLayout.Hidden,
		},
		{
			Title:  "Base",
			Width:  baseLayout.Width,
			Hidden: baseLayout.Hidden,
		},
		{
			Title:  "󰯢",
			Width:  utils.IntPtr(4),
			Hidden: reviewStatusLayout.Hidden,
		},
		{
			Title:  "",
			Width:  &ctx.Styles.PrSection.CiCellWidth,
			Grow:   new(bool),
			Hidden: ciLayout.Hidden,
		},
		{
			Title:  "",
			Width:  linesLayout.Width,
			Hidden: linesLayout.Hidden,
		},
		{
			Title:  "",
			Width:  updatedAtLayout.Width,
			Hidden: updatedAtLayout.Hidden,
		},
	}
}

func (m *Model) updateBranchesWithPrs() {
	branches := make([]branch.Branch, 0)
	for _, ref := range m.repo.Branches {
		b := branch.Branch{Ctx: m.Ctx, Data: ref, Columns: m.Table.Columns}
		b.PR = findPRForRef(m.Prs, ref.Name)

		branches = append(branches, b)
	}

	slices.SortFunc(branches, func(a, b branch.Branch) int {
		if a.Data.IsCheckedOut {
			return -1
		}
		if a.Data.LastUpdatedAt != nil && b.Data.LastUpdatedAt != nil {
			return b.Data.LastUpdatedAt.Compare(*a.Data.LastUpdatedAt)
		}
		if a.Data.LastUpdatedAt != nil {
			return -1
		}
		if b.Data.LastUpdatedAt != nil {
			return 1
		}
		return strings.Compare(a.Data.Name, b.Data.Name)
	})
	m.Branches = branches
}

func (m Model) BuildRows() []table.Row {
	var rows []table.Row
	currItem := m.Table.GetCurrItem()

	for i, b := range m.getFilteredBranches() {
		rows = append(
			rows,
			b.ToTableRow(currItem == i),
		)
	}

	if rows == nil {
		rows = []table.Row{}
	}

	return rows
}

func (m *Model) getFilteredBranches() []branch.Branch {
	sorted := m.Branches
	filtered := make([]branch.Branch, 0)
	query := strings.ToLower(strings.TrimSpace(m.SearchValue))
	if local := m.LocalSearchQuery(); local != "" {
		query = local
	}
	for _, b := range sorted {
		if branchMatchesLocalSearch(b, query) {
			filtered = append(filtered, b)
		}
	}
	return filtered
}

func branchMatchesLocalSearch(b branch.Branch, query string) bool {
	if query == "" {
		return true
	}
	fields := []string{b.Data.Name}
	if b.Data.LastCommitMsg != nil {
		fields = append(fields, *b.Data.LastCommitMsg)
	}
	if b.PR != nil {
		fields = append(fields,
			b.PR.Title,
			fmt.Sprintf("%d", b.PR.Number),
			fmt.Sprintf("#%d", b.PR.Number),
			b.PR.HeadRefName,
			b.PR.BaseRefName,
			b.PR.State,
		)
	}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), query) {
			return true
		}
	}
	return false
}

func findPRForRef(prs []data.PullRequestData, branch string) *data.PullRequestData {
	for _, pr := range prs {
		if pr.HeadRefName == branch {
			return &pr
		}
	}
	return nil
}

func (m *Model) NumRows() int {
	return len(m.getFilteredBranches())
}

type SectionPullRequestsFetchedMsg struct {
	Prs        []data.PullRequestData
	TotalCount int
	PageInfo   data.PageInfo
	TaskId     string
}

func (m *Model) getCurrBranch() *branch.Branch {
	idx := m.Table.GetCurrItem()
	branches := m.getFilteredBranches()
	if idx < 0 || idx >= len(branches) {
		return nil
	}
	return &branches[idx]
}

func (m *Model) GetCurrRow() data.RowData {
	idx := m.Table.GetCurrItem()
	branches := m.getFilteredBranches()
	if idx < 0 || idx >= len(branches) {
		return nil
	}
	b := branches[idx].Data
	pr := findPRForRef(m.Prs, b.Name)
	return branch.BranchData{
		Data: b,
		PR:   pr,
	}
}

func (m *Model) FetchNextPageSectionRows() []tea.Cmd {
	if m == nil {
		return nil
	}

	var cmds []tea.Cmd
	if m.Ctx.RepoPath != "" {
		cmds = append(cmds, m.readRepoCmd()...)
		cmds = append(cmds, m.fetchRepoCmd()...)
		cmds = append(cmds, m.fetchPRsCmd())
	}

	return cmds
}

func FetchAllBranches(ctx *context.ProgramContext) (Model, tea.Cmd) {
	cmds := make([]tea.Cmd, 0)

	t := config.RepoView
	cfg := config.PrsSectionConfig{
		Title: "Local Branches",
		Type:  &t,
	}
	m := NewModel(
		0,
		ctx,
		cfg,
		time.Now(),
	)
	m.refreshId = nextID()

	if ctx.RepoPath != "" {
		cmds = append(cmds, m.readRepoCmd()...)
		cmds = append(cmds, m.fetchRepoCmd()...)
		cmds = append(cmds, m.fetchPRsCmd())
	}

	if !m.isRefreshSetUp {
		m.isRefreshSetUp = true
		cmds = append(cmds, m.tickRefreshBranchesCmd())
		cmds = append(cmds, m.tickFetchPrsCmd())
	}

	return m, tea.Batch(cmds...)
}

func (m Model) GetDimensions() constants.Dimensions {
	if m.Ctx == nil {
		return constants.Dimensions{}
	}
	return constants.Dimensions{
		Width:  m.Ctx.MainContentWidth - m.Ctx.Styles.Section.ContainerStyle.GetHorizontalPadding(),
		Height: m.Ctx.MainContentHeight - common.SearchHeight,
	}
}

func (m *Model) UpdateProgramContext(ctx *context.ProgramContext) {
	m.Ctx = ctx
	newDimensions := m.GetDimensions()
	tableDimensions := constants.Dimensions{
		Height: newDimensions.Height,
		Width:  newDimensions.Width,
	}
	m.Table.SetDimensions(tableDimensions)
	m.Table.UpdateProgramContext(ctx)
	m.Table.SyncViewPortContent()
	m.SearchBar.UpdateProgramContext(ctx)
}

func (m *Model) ResetRows() {
	m.Prs = nil
}

func (m *Model) GetItemSingularForm() string {
	return "Branch"
}

func (m *Model) GetItemPluralForm() string {
	return "Branches"
}

func (m *Model) GetTotalCount() int {
	return len(m.Branches)
}

func (m *Model) SetIsLoading(val bool) {
	m.IsLoading = val
	m.Table.SetIsLoading(val)
}

func (m *Model) GetPagerContent() string {
	s := lipgloss.NewStyle().Background(m.Ctx.Styles.ListViewPort.PagerStyle.GetBackground())
	mod := s.Foreground(lipgloss.Color("#e0af68")).Render(
		fmt.Sprintf(" %d", len(m.repo.Status.Modified)))
	plus := s.Foreground(m.Ctx.Theme.SuccessText).Render(
		fmt.Sprintf(" %d", len(m.repo.Status.Added)))
	minus := s.Foreground(m.Ctx.Theme.ErrorText).Render(
		fmt.Sprintf(" %d", len(m.repo.Status.Removed)))
	spacer := s.Render(" ")
	return m.Ctx.Styles.ListViewPort.PagerStyle.Render(
		lipgloss.JoinHorizontal(lipgloss.Top, plus, spacer, minus, spacer, mod))
}
