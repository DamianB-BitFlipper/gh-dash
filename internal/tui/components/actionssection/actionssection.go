package actionssection

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dlvhdr/gh-dehub/v4/internal/config"
	"github.com/dlvhdr/gh-dehub/v4/internal/data"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/common"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/actionrow"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/section"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/table"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/tasks"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/keys"
	"github.com/dlvhdr/gh-dehub/v4/internal/utils"
)

const SectionType = "action"

// Pane identifies one of the three columns in the Actions three-pane layout.
// It governs which pane consumes navigation keystrokes (Up/Down/PgUp/PgDn/g/G).
type Pane int

const (
	PaneWorkflows Pane = iota
	PaneRuns
	PaneDetails
)

type Model struct {
	section.BaseModel
	RepoName         string
	Workflows        []data.Workflow
	Runs             []data.WorkflowRun
	RunsTable        table.Model
	selectedWorkflow *data.Workflow
	runsTaskId       string
	focusedPane      Pane
}

func NewModel(id int, ctx *context.ProgramContext, cfg config.ActionsSectionConfig, lastUpdated, createdAt time.Time) Model {
	m := Model{RepoName: data.ActionsRepoFromFilters(cfg.Filters)}
	m.BaseModel = section.NewModel(ctx, section.NewSectionOptions{
		Id:          id,
		Config:      cfg.ToSectionConfig(),
		Type:        SectionType,
		Columns:     GetWorkflowColumns(ctx),
		Singular:    m.GetItemSingularForm(),
		Plural:      m.GetItemPluralForm(),
		LastUpdated: lastUpdated,
		CreatedAt:   createdAt,
	})
	m.Workflows = []data.Workflow{}
	m.Runs = []data.WorkflowRun{}
	m.RunsTable = table.NewModel(
		*ctx,
		m.GetRunTableDimensions(),
		lastUpdated,
		createdAt,
		GetRunColumns(cfg, ctx),
		nil,
		"Run",
		utils.StringPtr(m.Ctx.Styles.Section.EmptyStateStyle.Render("No runs were found for this workflow")),
		"Loading...",
		false,
	)
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

		switch {
		case key.Matches(msg, keys.ActionsKeys.ToggleSmartFiltering):
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
		case key.Matches(msg, keys.ActionsKeys.SortOrder):
			m.ToggleSortOrder()
			m.updateSortHeader()
			m.sortRuns()
			m.RunsTable.SetRows(m.BuildRunRows())
			return m, nil
		case key.Matches(msg, keys.ActionsKeys.Rerun):
			if run := m.SelectedRun(); run != nil {
				return m, tasks.RerunWorkflowRun(m.Ctx, m.taskSection(), run)
			}
		case key.Matches(msg, keys.ActionsKeys.RerunFailed):
			if run := m.SelectedRun(); run != nil {
				return m, tasks.RerunFailedJobs(m.Ctx, m.taskSection(), run)
			}
		case key.Matches(msg, keys.ActionsKeys.Cancel):
			if run := m.SelectedRun(); run != nil {
				return m, tasks.CancelWorkflowRun(m.Ctx, m.taskSection(), run)
			}
		}

	case SectionWorkflowsFetchedMsg:
		if m.LastFetchTaskId == msg.TaskId {
			m.Workflows = msg.Workflows
			m.TotalCount = msg.TotalCount
			m.SetIsLoading(false)
			m.Table.SetRows(m.BuildRows())
			cmd = tea.Batch(m.SyncSelectedWorkflow()...)
			m.UpdateLastUpdated(time.Now())
			m.UpdateTotalItemsCount(m.TotalCount)
		}
	case SectionActionsFetchedMsg:
		if m.runsTaskId == msg.TaskId {
			// Remember the run the user had selected (if any) so a
			// background refresh doesn't yank their selection.
			var prevSelectedID int64
			if prev := m.SelectedRun(); prev != nil {
				prevSelectedID = prev.Id
			}
			m.Runs = msg.Runs
			m.sortRuns()
			m.RunsTable.SetIsLoading(false)
			m.RunsTable.SetRows(m.BuildRunRows())
			// Reposition the cursor onto the same run if it still exists
			// in the refreshed list; otherwise reset to the top.
			if prevSelectedID != 0 {
				filtered := m.filteredRuns()
				idx := -1
				for i := range filtered {
					if filtered[i].Id == prevSelectedID {
						idx = i
						break
					}
				}
				if idx >= 0 {
					m.RunsTable.SetCurrItem(idx)
				} else {
					m.RunsTable.ResetCurrItem()
				}
			} else {
				m.RunsTable.ResetCurrItem()
			}
			m.UpdateLastUpdated(time.Now())
		}
	case tasks.UpdateActionMsg:
		m.applyActionUpdate(msg)
	}

	search, searchCmd := m.SearchBar.Update(msg)
	m.SearchBar = search
	table, tableCmd := m.Table.Update(msg)
	m.Table = table
	runsTable, runsTableCmd := m.RunsTable.Update(msg)
	m.RunsTable = runsTable
	return m, tea.Batch(cmd, searchCmd, tableCmd, runsTableCmd)
}

func GetWorkflowColumns(ctx *context.ProgramContext) []table.Column {
	return []table.Column{
		{Title: "State", Width: utils.IntPtr(8)},
		{Title: "Workflow", Grow: utils.BoolPtr(true)},
		{Title: "󱦻", Width: utils.IntPtr(5)},
	}
}

func GetRunColumns(cfg config.ActionsSectionConfig, ctx *context.ProgramContext) []table.Column {
	_ = cfg
	_ = ctx
	return []table.Column{
		{Title: "", Width: utils.IntPtr(3)},
		{Title: "Title", Grow: utils.BoolPtr(true)},
		{Title: "󱦻", Width: utils.IntPtr(5)},
	}
}

func (m Model) BuildRows() []table.Row {
	rows := make([]table.Row, 0, len(m.filteredWorkflows()))
	for _, workflow := range m.filteredWorkflows() {
		rows = append(rows, table.Row{
			workflow.State,
			workflow.Name,
			utils.TimeElapsed(workflow.GetUpdatedAt()),
		})
	}
	return rows
}

func (m Model) BuildRunRows() []table.Row {
	rows := make([]table.Row, 0, len(m.filteredRuns()))
	faint := lipgloss.NewStyle().Foreground(m.Ctx.Theme.FaintText)
	// In compact mode rows are 1 line tall, so suppress the branch
	// subtitle to avoid clipping and to honor the user's preference.
	showBranch := !m.Ctx.Config.Theme.Ui.Table.Compact
	for _, run := range m.filteredRuns() {
		titleCell := run.GetTitle()
		if showBranch {
			if branch := strings.TrimSpace(run.HeadBranch); branch != "" {
				// Use a plain newline rather than lipgloss.JoinVertical
				// here: JoinVertical pre-pads the shorter line with
				// unstyled spaces, which causes the inner faint-style
				// reset to truncate the selection background before the
				// padding. With a plain newline the table renderer pads
				// each line with cell-styled spaces, so the selection
				// background extends across the full row.
				titleCell = titleCell + "\n" + faint.Render(branch)
			}
		}
		rows = append(rows, table.Row{
			actionrow.StatusIcon(run),
			titleCell,
			utils.TimeElapsed(run.UpdatedAt),
		})
	}
	return rows
}

func (m *Model) NumRows() int {
	return len(m.filteredWorkflows())
}

func (m *Model) GetCurrRow() data.RowData {
	idx := m.Table.GetCurrItem()
	workflows := m.filteredWorkflows()
	if idx < 0 || idx >= len(workflows) {
		return nil
	}
	workflow := workflows[idx]
	return &workflow
}

func (m Model) taskSection() tasks.SectionIdentifier {
	return tasks.SectionIdentifier{Id: m.Id, Type: m.Type}
}

func (m *Model) applyActionUpdate(msg tasks.UpdateActionMsg) {
	for i := range m.Runs {
		if int(m.Runs[i].Id) != msg.RunID || m.Runs[i].Repository.FullName != msg.Repo {
			continue
		}
		if msg.Status != nil {
			m.Runs[i].Status = *msg.Status
		}
		if msg.Conclusion != nil {
			m.Runs[i].Conclusion = *msg.Conclusion
		}
	}
	m.sortRuns()
	m.RunsTable.SetRows(m.BuildRunRows())
}

func (m Model) filteredWorkflows() []data.Workflow {
	query := m.LocalSearchQuery()
	if query == "" {
		return m.Workflows
	}
	filtered := make([]data.Workflow, 0, len(m.Workflows))
	for _, workflow := range m.Workflows {
		fields := []string{strconv.FormatInt(workflow.Id, 10), workflow.Name, workflow.Path, workflow.State, workflow.RepoName}
		for _, field := range fields {
			if strings.Contains(strings.ToLower(field), query) {
				filtered = append(filtered, workflow)
				break
			}
		}
	}
	return filtered
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

func (m *Model) SyncSelectedWorkflow() []tea.Cmd {
	idx := m.Table.GetCurrItem()
	workflows := m.filteredWorkflows()
	if idx < 0 || idx >= len(workflows) {
		m.selectedWorkflow = nil
		m.Runs = nil
		m.RunsTable.SetRows(nil)
		return nil
	}
	// Copy by value so the stored pointer is independent of subsequent
	// mutations to the (possibly re-sliced) Workflows slice.
	workflow := workflows[idx]
	if m.selectedWorkflow != nil && m.selectedWorkflow.Id == workflow.Id {
		return nil
	}
	wfCopy := workflow
	m.selectedWorkflow = &wfCopy
	m.Runs = nil
	m.PageInfo = nil
	m.RunsTable.ResetCurrItem()
	m.RunsTable.SetRows(nil)
	m.RunsTable.SetIsLoading(true)
	return m.fetchWorkflowRuns()
}

func (m *Model) FetchNextPageSectionRows() []tea.Cmd {
	if m == nil || (m.PageInfo != nil && !m.PageInfo.HasNextPage) {
		return nil
	}
	if m.RepoName == "" {
		return nil
	}
	return m.fetchWorkflows()
}

func (m *Model) fetchWorkflows() []tea.Cmd {
	taskId := fmt.Sprintf("fetching_action_workflows_%d_%d", m.Id, time.Now().UnixNano())
	m.LastFetchTaskId = taskId
	m.SetIsLoading(true)
	startCmd := m.Ctx.StartTask(context.Task{
		Id:           taskId,
		StartText:    fmt.Sprintf(`Fetching workflows for "%s"`, m.Config.Title),
		FinishedText: fmt.Sprintf(`Workflows for "%s" have been fetched`, m.Config.Title),
		State:        context.TaskStart,
	})
	fetchCmd := func() tea.Msg {
		res, err := data.FetchActionsWorkflows(m.Config.Filters)
		if err != nil {
			return constants.TaskFinishedMsg{SectionId: m.Id, SectionType: m.Type, TaskId: taskId, Err: err}
		}
		return constants.TaskFinishedMsg{
			SectionId:   m.Id,
			SectionType: m.Type,
			TaskId:      taskId,
			Msg: SectionWorkflowsFetchedMsg{
				Workflows:  res.Workflows,
				TotalCount: res.TotalCount,
				TaskId:     taskId,
			},
		}
	}
	return []tea.Cmd{startCmd, fetchCmd}
}

func (m *Model) fetchWorkflowRuns() []tea.Cmd {
	if m.selectedWorkflow == nil {
		return nil
	}
	// Capture identifiers up front so the fetch closure doesn't race with
	// later workflow switches mutating m.selectedWorkflow.
	workflowID := m.selectedWorkflow.Id
	workflowName := m.selectedWorkflow.Name
	repoName := m.RepoName
	startCursor := time.Now().String()
	if m.PageInfo != nil {
		startCursor = m.PageInfo.StartCursor
	}
	taskId := fmt.Sprintf("fetching_action_runs_%d_%d_%s", m.Id, workflowID, startCursor)
	m.runsTaskId = taskId
	startCmd := m.Ctx.StartTask(context.Task{
		Id:           taskId,
		StartText:    fmt.Sprintf(`Fetching runs for "%s"`, workflowName),
		FinishedText: fmt.Sprintf(`Runs for "%s" have been fetched`, workflowName),
		State:        context.TaskStart,
	})
	fetchCmd := func() tea.Msg {
		limit := m.Config.Limit
		if limit == nil {
			limit = &m.Ctx.Config.Defaults.ActionsLimit
		}
		res, err := data.FetchActionsWorkflowRunsForWorkflow(repoName, workflowID, *limit)
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

func FetchAllSections(
	ctx *context.ProgramContext,
	existing []section.Section,
) ([]section.Section, tea.Cmd) {
	sectionConfigs := actionSectionConfigs(ctx.Config.ActionsSections, ctx.Config.RepoPaths)
	fetchCmds := make([]tea.Cmd, 0, len(sectionConfigs))
	sections := make([]section.Section, 0, len(sectionConfigs))
	for i, sectionConfig := range sectionConfigs {
		sectionModel := NewModel(i, ctx, sectionConfig, time.Now(), time.Now())
		// Preserve in-memory state from the previous section instance so that
		// interval refreshes don't reset the user's selected workflow/run,
		// local search, sort order, or focused pane.
		if i < len(existing) && existing[i] != nil {
			if old, ok := existing[i].(*Model); ok {
				sectionModel.Workflows = old.Workflows
				sectionModel.Runs = old.Runs
				sectionModel.selectedWorkflow = old.selectedWorkflow
				sectionModel.TotalCount = old.TotalCount
				sectionModel.LastFetchTaskId = old.LastFetchTaskId
				sectionModel.runsTaskId = old.runsTaskId
				sectionModel.focusedPane = old.focusedPane
				sectionModel.SearchValue = old.SearchValue
				sectionModel.LocalSearchValue = old.LocalSearchValue
				sectionModel.SortOrder = old.SortOrder
				sectionModel.IsFilteredByCurrentRemote = old.IsFilteredByCurrentRemote
				sectionModel.SearchBar.SetValue(old.SearchValue)
				// Rebuild rows from the preserved data then restore the
				// user's cursor position in both tables.
				sectionModel.Table.SetRows(sectionModel.BuildRows())
				sectionModel.Table.SetCurrItem(old.Table.GetCurrItem())
				sectionModel.RunsTable.SetRows(sectionModel.BuildRunRows())
				sectionModel.RunsTable.SetCurrItem(old.RunsTable.GetCurrItem())
			}
		}
		sections = append(sections, &sectionModel)
		fetchCmds = append(fetchCmds, sectionModel.FetchNextPageSectionRows()...)
	}
	return sections, tea.Batch(fetchCmds...)
}

func actionSectionConfigs(configs []config.ActionsSectionConfig, repoPaths map[string]string) []config.ActionsSectionConfig {
	configured := make([]config.ActionsSectionConfig, 0, len(configs))
	for _, cfg := range configs {
		if data.ActionsRepoFromFilters(cfg.Filters) != "" {
			configured = append(configured, cfg)
		}
	}
	if len(configured) > 0 {
		return configured
	}

	repos := common.ExpandRepoPaths(repoPaths)
	configured = make([]config.ActionsSectionConfig, 0, len(repos))
	for _, repo := range repos {
		configured = append(configured, config.ActionsSectionConfig{
			Title:   repo.Name,
			Filters: "repo:" + repo.Name,
		})
	}
	return configured
}

type SectionWorkflowsFetchedMsg struct {
	Workflows  []data.Workflow
	TotalCount int
	TaskId     string
}

type SectionActionsFetchedMsg struct {
	Runs       []data.WorkflowRun
	TotalCount int
	PageInfo   data.PageInfo
	TaskId     string
}

func (m *Model) ResetRows() {
	m.Workflows = nil
	m.Runs = nil
	m.selectedWorkflow = nil
	m.RunsTable.SetRows(nil)
	m.BaseModel.ResetRows()
}

func (m *Model) UpdateLastUpdated(t time.Time) { m.Table.UpdateLastUpdated(t) }

func (m Model) GetItemSingularForm() string {
	return "Workflow"
}

func (m Model) GetItemPluralForm() string {
	return "Workflows"
}

func (m Model) GetTotalCount() int { return m.TotalCount }

func (m *Model) GetIsLoading() bool {
	// Reflect both the workflows fetch and the runs fetch so the tab
	// spinner keeps spinning while either is in flight.
	return m.IsLoading || m.RunsTable.IsLoading()
}

func (m *Model) SetIsLoading(val bool) { m.IsLoading = val; m.Table.SetIsLoading(val) }

func (m Model) GetPagerContent() string {
	pagerContent := ""
	if m.TotalCount > 0 {
		pagerContent = fmt.Sprintf(
			"%v %v • Workflow %v/%v • Runs %v/%v",
			constants.WaitingIcon,
			m.LastUpdated().Format("01/02 15:04:05"),
			m.Table.GetCurrItem()+1,
			m.TotalCount,
			m.RunsTable.GetCurrItem()+1,
			max(1, len(m.RunsTable.Rows)),
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
	if len(m.RunsTable.Columns) > 1 {
		m.RunsTable.Columns[1].Title = fmt.Sprintf("Title (%s)", m.SortOrderLabel())
	}
}

func (m *Model) NextRow() int {
	return m.BaseModel.NextRow()
}

func (m *Model) PrevRow() int {
	return m.BaseModel.PrevRow()
}

func (m *Model) FirstItem() int {
	return m.BaseModel.FirstItem()
}

func (m *Model) LastItem() int {
	return m.BaseModel.LastItem()
}

func (m *Model) PageDown() int {
	return m.BaseModel.PageDown()
}

func (m *Model) PageUp() int {
	return m.BaseModel.PageUp()
}

func (m *Model) NextRun() int {
	return m.RunsTable.NextItem()
}

func (m *Model) PrevRun() int {
	return m.RunsTable.PrevItem()
}

// FocusedPane returns which of the three Actions panes currently consumes
// navigation keys.
func (m *Model) FocusedPane() Pane {
	return m.focusedPane
}

// FocusNextPane cycles forward through the panes (Workflows -> Runs ->
// Details -> Workflows).
func (m *Model) FocusNextPane() {
	m.focusedPane = (m.focusedPane + 1) % 3
}

// FocusPrevPane cycles backward through the panes.
func (m *Model) FocusPrevPane() {
	m.focusedPane = (m.focusedPane + 2) % 3
}

// SetFocusedPane sets the focused pane explicitly.
func (m *Model) SetFocusedPane(p Pane) {
	m.focusedPane = p
}

func (m *Model) SelectedRun() *data.WorkflowRun {
	runs := m.filteredRuns()
	idx := m.RunsTable.GetCurrItem()
	if idx < 0 || idx >= len(runs) {
		return nil
	}
	run := runs[idx]
	return &run
}

func (m *Model) GetRunTableDimensions() constants.Dimensions {
	d := m.GetDimensions()
	return constants.Dimensions{Width: max(0, d.Width), Height: max(0, d.Height-2)}
}

func (m *Model) SetWorkflowTableDimensions(width, height int) {
	m.Table.SetDimensions(constants.Dimensions{Width: max(0, width), Height: max(0, height)})
	m.Table.SyncViewPortContent()
}

func (m *Model) SetRunTableDimensions(width, height int) {
	m.RunsTable.SetDimensions(constants.Dimensions{Width: max(0, width), Height: max(0, height)})
	m.RunsTable.SyncViewPortContent()
}

func (m *Model) UpdateProgramContext(ctx *context.ProgramContext) {
	m.BaseModel.UpdateProgramContext(ctx)
	m.RunsTable.UpdateProgramContext(ctx)
	m.RunsTable.SyncViewPortContent()
}

func (m Model) RunsView() string {
	return m.RunsTable.View()
}
