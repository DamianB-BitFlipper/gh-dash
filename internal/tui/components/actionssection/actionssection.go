package actionssection

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/actionrow"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/section"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/table"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/keys"
	"github.com/dlvhdr/gh-dash/v4/internal/utils"
)

const SectionType = "action"

type Model struct {
	section.BaseModel
	Runs []data.WorkflowRun
}

func NewModel(id int, ctx *context.ProgramContext, cfg config.ActionsSectionConfig, lastUpdated, createdAt time.Time) Model {
	m := Model{}
	m.BaseModel = section.NewModel(ctx, section.NewSectionOptions{
		Id:          id,
		Config:      cfg.ToSectionConfig(),
		Type:        SectionType,
		Columns:     GetSectionColumns(cfg, ctx),
		Singular:    m.GetItemSingularForm(),
		Plural:      m.GetItemPluralForm(),
		LastUpdated: lastUpdated,
		CreatedAt:   createdAt,
	})
	m.Runs = []data.WorkflowRun{}
	m.updateSortHeader()
	return m
}

func (m *Model) Update(msg tea.Msg) (section.Section, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if handled, cmd := m.HandleLocalSearchKey(msg, m.BuildRows); handled {
			return m, cmd
		}
		if m.IsSearchFocused() {
			switch msg.String() {
			case "ctrl+c", "esc":
				m.SearchBar.SetValue(m.SearchValue)
				return m, m.SetIsSearching(false)
			case "enter":
				m.SearchValue = m.SearchBar.Value()
				m.SyncSmartFilterWithSearchValue()
				m.SetIsSearching(false)
				m.ResetRows()
				return m, tea.Batch(m.FetchNextPageSectionRows()...)
			}
			break
		}
		if key.Matches(msg, keys.ActionsKeys.ToggleSmartFiltering) {
			if m.HasCurrentRepoNameInConfiguredFilter() || !m.HasRepoNameInConfiguredFilter() {
				m.IsFilteredByCurrentRemote = !m.IsFilteredByCurrentRemote
			}
			searchValue := m.GetSearchValue()
			if m.SearchValue != searchValue {
				m.SearchValue = searchValue
				m.SearchBar.SetValue(searchValue)
				m.SetIsSearching(false)
				m.ResetRows()
				return m, tea.Batch(m.FetchNextPageSectionRows()...)
			}
		} else if key.Matches(msg, keys.ActionsKeys.SortOrder) {
			m.ToggleSortOrder()
			m.updateSortHeader()
			m.sortRuns()
			m.Table.SetRows(m.BuildRows())
			return m, nil
		}

	case SectionActionsFetchedMsg:
		if m.LastFetchTaskId == msg.TaskId {
			m.Runs = msg.Runs
			m.sortRuns()
			m.TotalCount = msg.TotalCount
			m.SetIsLoading(false)
			m.PageInfo = &msg.PageInfo
			m.Table.SetRows(m.BuildRows())
			m.UpdateLastUpdated(time.Now())
			m.UpdateTotalItemsCount(m.TotalCount)
		}
	}

	search, searchCmd := m.SearchBar.Update(msg)
	m.SearchBar = search
	table, tableCmd := m.Table.Update(msg)
	m.Table = table
	return m, tea.Batch(cmd, searchCmd, tableCmd)
}

func GetSectionColumns(cfg config.ActionsSectionConfig, ctx *context.ProgramContext) []table.Column {
	dLayout := ctx.Config.Defaults.Layout.Actions
	sLayout := cfg.Layout
	statusLayout := config.MergeColumnConfigs(dLayout.Status, sLayout.Status)
	repoLayout := config.MergeColumnConfigs(dLayout.Repo, sLayout.Repo)
	workflowLayout := config.MergeColumnConfigs(dLayout.Workflow, sLayout.Workflow)
	branchLayout := config.MergeColumnConfigs(dLayout.Branch, sLayout.Branch)
	eventLayout := config.MergeColumnConfigs(dLayout.Event, sLayout.Event)
	actorLayout := config.MergeColumnConfigs(dLayout.Actor, sLayout.Actor)
	titleLayout := config.MergeColumnConfigs(dLayout.Title, sLayout.Title)
	updatedAtLayout := config.MergeColumnConfigs(dLayout.UpdatedAt, sLayout.UpdatedAt)
	createdAtLayout := config.MergeColumnConfigs(dLayout.CreatedAt, sLayout.CreatedAt)
	return []table.Column{
		{Title: "", Width: statusLayout.Width, Hidden: statusLayout.Hidden},
		{Title: "", Width: repoLayout.Width, Hidden: repoLayout.Hidden},
		{Title: "Workflow", Width: workflowLayout.Width, Hidden: workflowLayout.Hidden},
		{Title: "Title", Grow: utils.BoolPtr(true), Hidden: titleLayout.Hidden},
		{Title: "Branch", Width: branchLayout.Width, Hidden: branchLayout.Hidden},
		{Title: "Event", Width: eventLayout.Width, Hidden: eventLayout.Hidden},
		{Title: "Actor", Width: actorLayout.Width, Hidden: actorLayout.Hidden},
		{Title: "󱦻", Width: updatedAtLayout.Width, Hidden: updatedAtLayout.Hidden},
		{Title: "󱡢", Width: createdAtLayout.Width, Hidden: createdAtLayout.Hidden},
	}
}

func (m Model) BuildRows() []table.Row {
	rows := make([]table.Row, 0, len(m.filteredRuns()))
	for _, run := range m.filteredRuns() {
		rows = append(rows, actionrow.Build(run))
	}
	return rows
}

func (m *Model) NumRows() int { return len(m.filteredRuns()) }

func (m *Model) GetCurrRow() data.RowData {
	idx := m.Table.GetCurrItem()
	runs := m.filteredRuns()
	if idx < 0 || idx >= len(runs) {
		return nil
	}
	run := runs[idx]
	return &run
}

func (m Model) filteredRuns() []data.WorkflowRun {
	query := m.LocalSearchQuery()
	if query == "" {
		return m.Runs
	}
	filtered := make([]data.WorkflowRun, 0, len(m.Runs))
	for _, run := range m.Runs {
		if actionRunMatchesLocalSearch(run, query) {
			filtered = append(filtered, run)
		}
	}
	return filtered
}

func actionRunMatchesLocalSearch(run data.WorkflowRun, query string) bool {
	fields := []string{
		strconv.FormatInt(run.Id, 10), run.Name, run.DisplayTitle, run.HeadBranch,
		run.Status, run.Conclusion, run.Event, run.Actor.Login, run.GetRepoNameWithOwner(),
	}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), query) {
			return true
		}
	}
	return false
}

func (m *Model) FetchNextPageSectionRows() []tea.Cmd {
	if m == nil || (m.PageInfo != nil && !m.PageInfo.HasNextPage) {
		return nil
	}
	startCursor := time.Now().String()
	if m.PageInfo != nil {
		startCursor = m.PageInfo.StartCursor
	}
	taskId := fmt.Sprintf("fetching_actions_%d_%s", m.Id, startCursor)
	m.LastFetchTaskId = taskId
	startCmd := m.Ctx.StartTask(context.Task{
		Id:           taskId,
		StartText:    fmt.Sprintf(`Fetching actions for "%s"`, m.Config.Title),
		FinishedText: fmt.Sprintf(`Actions for "%s" have been fetched`, m.Config.Title),
		State:        context.TaskStart,
	})
	fetchCmd := func() tea.Msg {
		limit := m.Config.Limit
		if limit == nil {
			limit = &m.Ctx.Config.Defaults.ActionsLimit
		}
		res, err := data.FetchActionsWorkflowRuns(m.GetFilters(), *limit, m.PageInfo)
		if err != nil {
			return constants.TaskFinishedMsg{SectionId: m.Id, SectionType: m.Type, TaskId: taskId, Err: err}
		}
		return constants.TaskFinishedMsg{
			SectionId:   m.Id,
			SectionType: m.Type,
			TaskId:      taskId,
			Msg: SectionActionsFetchedMsg{
				Runs:       res.WorkflowRuns,
				TotalCount: res.TotalCount,
				PageInfo:   res.PageInfo,
				TaskId:     taskId,
			},
		}
	}
	return []tea.Cmd{startCmd, fetchCmd}
}

func FetchAllSections(ctx *context.ProgramContext) ([]section.Section, tea.Cmd) {
	sectionConfigs := ctx.Config.ActionsSections
	fetchCmds := make([]tea.Cmd, 0, len(sectionConfigs))
	sections := make([]section.Section, 0, len(sectionConfigs))
	for i, sectionConfig := range sectionConfigs {
		sectionModel := NewModel(i+1, ctx, sectionConfig, time.Now(), time.Now())
		sections = append(sections, &sectionModel)
		fetchCmds = append(fetchCmds, sectionModel.FetchNextPageSectionRows()...)
	}
	return sections, tea.Batch(fetchCmds...)
}

type SectionActionsFetchedMsg struct {
	Runs       []data.WorkflowRun
	TotalCount int
	PageInfo   data.PageInfo
	TaskId     string
}

func (m *Model) ResetRows() { m.Runs = nil; m.BaseModel.ResetRows() }

func (m *Model) UpdateLastUpdated(t time.Time) { m.Table.UpdateLastUpdated(t) }

func (m Model) GetItemSingularForm() string { return "Run" }

func (m Model) GetItemPluralForm() string { return "Runs" }

func (m Model) GetTotalCount() int { return m.TotalCount }

func (m *Model) GetIsLoading() bool { return m.IsLoading }

func (m *Model) SetIsLoading(val bool) { m.IsLoading = val; m.Table.SetIsLoading(val) }

func (m Model) GetPagerContent() string {
	pagerContent := ""
	if m.TotalCount > 0 {
		pagerContent = fmt.Sprintf(
			"%v %v • %v %v/%v • Fetched %v",
			constants.WaitingIcon,
			m.LastUpdated().Format("01/02 15:04:05"),
			m.SingularForm,
			m.Table.GetCurrItem()+1,
			m.TotalCount,
			len(m.Table.Rows),
		)
	}
	return m.Ctx.Styles.ListViewPort.PagerStyle.Render(pagerContent)
}

func (m *Model) sortRuns() {
	sortOrder := m.GetSortOrder()
	slices.SortFunc(m.Runs, func(a, b data.WorkflowRun) int {
		if sortOrder == data.SearchSortCreated {
			return b.CreatedAt.Compare(a.CreatedAt)
		}
		return b.UpdatedAt.Compare(a.UpdatedAt)
	})
}

func (m *Model) updateSortHeader() {
	m.SetColumnTitle(3, fmt.Sprintf("Title (%s)", m.SortOrderLabel()))
}
