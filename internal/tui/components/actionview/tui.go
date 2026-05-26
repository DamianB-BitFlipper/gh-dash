package actionview

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"charm.land/bubbles/v2/cursor"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/paginator"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/log/v2"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone/v2"

	data "github.com/dlvhdr/gh-dash/v4/internal/data/actions"
	api "github.com/dlvhdr/gh-dash/v4/internal/data/actionsapi"
	parser "github.com/dlvhdr/gh-dash/v4/internal/data/actionsparser"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/theme"
)

type errMsg error

var caretANSIEscapePattern = regexp.MustCompile(`\^\[\[[0-?]*[ -/]*[@-~]`)

type pane int

const (
	PaneRuns pane = iota
	PaneJobs
	PaneSteps
	PaneChecks
	PaneLogs
)

type model struct {
	width             int
	height            int
	prNumber          string
	repo              string
	runID             string // set when viewing a run directly (no PR)
	pr                api.PR
	prWithChecks      api.PRWithChecks
	workflowRuns      []data.WorkflowRun
	runsList          list.Model
	jobsList          list.Model
	stepsList         list.Model
	checksList        list.Model
	logsViewport      viewport.Model
	numHighlights     int
	focusedPane       pane
	zoomedPane        *pane
	err               error
	runsDelegate      list.ItemDelegate
	jobsDelegate      list.ItemDelegate
	stepsDelegate     list.ItemDelegate
	checksDelegate    list.ItemDelegate
	styles            styles
	logsSpinner       spinner.Model
	logsInput         textinput.Model
	inProgressSpinner spinner.Model
	flat              bool
	lastTick          time.Time
	rateLimit         api.RateLimit
	lastFetched       time.Time
	checksFetched     bool
	embedded          bool
}

type Model = model

type ModelOpts struct {
	Flat     bool
	RunID    string // non-empty when in run mode (no PR context)
	Embedded bool
	Theme    *theme.Theme
}

func NewModel(repo string, number string, opts ModelOpts) Model {
	s := makeStyles(opts.Theme)

	runsList, runsDelegate := newRunsDefaultList(s)
	runsList.Title = makePill(ListSymbol+" Runs", s.focusedPaneTitleStyle,
		s.colors.focusedColor)
	runsList.SetStatusBarItemName("run", "runs")
	runsList.SetWidth(largePaneWidth)

	jobsList, jobsDelegate := newJobsDefaultList(s)
	jobsList.Title = makePill(ListSymbol+" Jobs", s.unfocusedPaneTitleStyle,
		s.colors.unfocusedColor)
	jobsList.SetStatusBarItemName("job", "jobs")
	jobsList.SetWidth(largePaneWidth)

	stepsList, stepsDelegate := newStepsDefaultList(s)
	stepsList.Title = makePill(ListSymbol+" Steps", s.unfocusedPaneTitleStyle,
		s.colors.unfocusedColor)
	stepsList.SetStatusBarItemName("step", "steps")
	stepsList.SetWidth(largePaneWidth)

	checksList, checksDelegate := newChecksDefaultList(s)
	checksList.Title = makePill(ListSymbol+" checks", s.unfocusedPaneTitleStyle,
		s.colors.unfocusedColor)
	checksList.SetStatusBarItemName("step", "checks")
	checksList.SetWidth(largePaneWidth)

	vp := viewport.New()
	vp.LeftGutterFunc = func(info viewport.GutterContext) string {
		return lipgloss.NewStyle().Foreground(s.colors.faintColor).Render(
			fmt.Sprintf(" %*d %s ", 5, info.Index+1,
				lipgloss.NewStyle().Foreground(s.colors.fainterColor).Render("│")),
		)
	}
	vp.KeyMap.Right = rightKey
	vp.KeyMap.Left = leftKey

	vp.HighlightStyle = lipgloss.NewStyle().Foreground(s.palette.Black).Background(s.palette.Blue)
	vp.SelectedHighlightStyle = lipgloss.NewStyle().
		Foreground(s.palette.Black).
		Background(s.palette.BrightGreen)

	ls := spinner.New(spinner.WithSpinner(LogsFrames))
	ls.Style = s.faintFgStyle

	li := textinput.New()
	li.SetWidth(20)
	li.SetStyles(textinput.Styles{
		Cursor: textinput.CursorStyle{
			Color: s.colors.faintColor,
			Shape: tea.CursorBar,
			Blink: false,
		},
		Focused: textinput.StyleState{
			Text:        lipgloss.NewStyle(),
			Placeholder: s.faintFgStyle,
			Prompt:      s.faintFgStyle,
		},
		Blurred: textinput.StyleState{
			Text:        lipgloss.NewStyle(),
			Placeholder: s.faintFgStyle,
			Prompt:      s.faintFgStyle,
		},
	})
	li.SetVirtualCursor(true)
	li.Prompt = " "
	li.Placeholder = "Search..."

	ips := spinner.New(spinner.WithSpinner(InProgressFrames))
	ips.Style = lipgloss.NewStyle().Foreground(s.colors.warnColor)

	focusedPane := PaneRuns
	if opts.Flat {
		focusedPane = PaneChecks
	}

	m := model{
		jobsList:          jobsList,
		runsList:          runsList,
		stepsList:         stepsList,
		checksList:        checksList,
		prNumber:          number,
		repo:              repo,
		runID:             opts.RunID,
		runsDelegate:      runsDelegate,
		jobsDelegate:      jobsDelegate,
		stepsDelegate:     stepsDelegate,
		checksDelegate:    checksDelegate,
		logsViewport:      vp,
		styles:            s,
		logsSpinner:       ls,
		logsInput:         li,
		inProgressSpinner: ips,
		flat:              opts.Flat,
		embedded:          opts.Embedded,
		focusedPane:       focusedPane,
		lastFetched:       time.Now(),
	}
	m.setFocusedPaneStyles()
	return m
}

func (m model) Init() tea.Cmd {
	if m.runID != "" {
		return m.makeRunModeInitCmd()
	}
	return m.makeInitCmd()
}

func (m Model) UpdateEmbedded(msg tea.Msg) (Model, tea.Cmd) {
	m.embedded = true
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		// Never consume quit; the parent TUI must always be able to exit.
		if key.Matches(keyMsg, quitKey) {
			return m, nil
		}
		// Only route keys to the embedded view while logs search is focused;
		// otherwise the parent handles them.
		if !m.logsInput.Focused() {
			return m, nil
		}
	}

	return m.Update(msg)
}

func (m *Model) FocusLogsSearch() tea.Cmd {
	return m.logsInput.Focus()
}

func (m Model) IsLogsSearchFocused() bool {
	return m.logsInput.Focused()
}

func (m Model) LogsSearchValue() string {
	return m.logsInput.Value()
}

func (m *Model) clearLogsSearch() {
	m.logsInput.Blur()
	m.logsInput.Reset()
	m.numHighlights = 0
	m.logsViewport.ClearHighlights()
	ji := m.getSelectedJobItem()
	if ji != nil {
		m.logsViewport.SetContentLines(ji.renderedLogs)
	}
}

func (m Model) LogsCopySelectionContent() string {
	ji := m.getSelectedJobItem()
	if ji == nil {
		return ansi.Strip(m.logsViewport.GetContent())
	}
	if len(ji.unstyledLogs) > 0 {
		return m.visibleLogsCopySelectionContent(ji.unstyledLogs)
	}
	return ansi.Strip(m.logsViewport.GetContent())
}

func (m Model) visibleLogsCopySelectionContent(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	offset := min(max(m.logsViewport.YOffset(), 0), len(lines))
	end := min(offset+m.logsViewport.VisibleLineCount(), len(lines))
	visible := make([]string, 0, end-offset)
	for i, line := range lines[offset:end] {
		lineNumber := offset + i + 1
		visible = append(visible, fmt.Sprintf(" %5d │ %s", lineNumber, line))
	}
	return strings.Join(visible, "\n")
}

func (m *Model) SelectPrevCheck() (bool, tea.Cmd) {
	return m.selectCheck(-1)
}

func (m *Model) SelectNextCheck() (bool, tea.Cmd) {
	return m.selectCheck(1)
}

func (m *Model) selectCheck(delta int) (bool, tea.Cmd) {
	before := m.getSelectedCheckItem()
	if before == nil {
		return false, nil
	}

	if delta < 0 {
		m.checksList.CursorUp()
	} else {
		m.checksList.CursorDown()
	}

	after := m.getSelectedCheckItem()
	if after == nil || after.job.Id == before.job.Id {
		return false, nil
	}

	cmds := m.onCheckChanged()
	cmds = append(cmds, m.updateLists()...)
	return true, tea.Batch(cmds...)
}

func (m *Model) SelectPrevStep() (bool, tea.Cmd) {
	return m.selectStep(-1)
}

func (m *Model) SelectNextStep() (bool, tea.Cmd) {
	return m.selectStep(1)
}

func (m *Model) selectStep(delta int) (bool, tea.Cmd) {
	if len(m.stepsList.Items()) == 0 {
		return false, nil
	}

	before := m.stepsList.GlobalIndex()
	if delta < 0 {
		m.stepsList.CursorUp()
	} else {
		m.stepsList.CursorDown()
	}
	after := m.stepsList.GlobalIndex()
	if before == after {
		return false, nil
	}

	m.onStepChanged()
	return true, nil
}

func HandlesAsyncMsg(msg tea.Msg) bool {
	switch msg.(type) {
	// NOTE: spinner.TickMsg is intentionally NOT included here. spinner.TickMsg
	// is a globally-shared message type used by multiple components (e.g. the
	// parent's task spinner and the tabs' per-section loading spinners).
	// Routing it exclusively to the embedded actionview would freeze every
	// other spinner in the app. The actionview's own internal spinners keep
	// receiving ticks via the spinner model's returned Cmds.
	case startIntervalFetching,
		startRunIntervalFetching,
		runModeFetchedMsg,
		runModeIntervalTickMsg,
		prFetchedMsg,
		workflowRunsFetchedMsg,
		prChecksIntervalTickMsg,
		workflowRunStepsFetchedMsg,
		checkStepsFetchedMsg,
		jobLogsFetchedMsg,
		checkRunOutputFetchedMsg,
		reRunJobMsg,
		reRunRunMsg:
		return true
	default:
		return false
	}
}

func (m *Model) SetSize(width int, height int) {
	m.width = width
	m.height = height
	m.setHeights()
	m.setWidths()
	m.setFocusedPaneStyles()
}

func (m model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	cmds := make([]tea.Cmd, 0)

	if _, ok := msg.(spinner.TickMsg); !ok {
		log.Debug("got msg", "type", fmt.Sprintf("%T", msg))
	}
	switch msg := msg.(type) {
	case cursor.BlinkMsg:
		m.logsInput, cmd = m.logsInput.Update(msg)
		cmds = append(cmds, cmd)

	// `startIntervalFetching` is sent after the `refreshInterval` duration has elapsed.
	// At this point, `m.fetchPRChecksWithInterval()` checks if all checks have concluded.
	// If they did - it's a noop, otherwise we check at the interval.
	// `m.fetchPRChecksWithInterval` needs an up to date model, so this *cannot* be called
	// at the `m.makeInitCmd`. The up to date model is received by this `Update` func.
	case startIntervalFetching:
		cmds = append(cmds, m.fetchPRChecksWithInterval())

	case startRunIntervalFetching:
		cmds = append(cmds, m.fetchRunWithInterval())

	case runModeFetchedMsg, runModeIntervalTickMsg:
		var rmMsg runModeFetchedMsg
		if tickMsg, ok := msg.(runModeIntervalTickMsg); ok {
			rmMsg = tickMsg.msg.(runModeFetchedMsg)
		} else {
			rmMsg = msg.(runModeFetchedMsg)
		}

		if rmMsg.err != nil {
			log.Debug("error when fetching run", "err", rmMsg.err)
			m.err = rmMsg.err
			if m.embedded {
				// Surface the error via EmbeddedView() instead of quitting
				// the host TUI.
				return m, nil
			}
			msgCmd := tea.Printf("%s\nrepo=%s, runID=%s\nOriginal error: %v\n",
				lipgloss.NewStyle().Foreground(m.styles.colors.errorColor).Bold(true).Render(
					"❌ Workflow run not found.",
				), m.repo, m.runID, rmMsg.err)
			return m, tea.Sequence(msgCmd, tea.Quit)
		}

		m.workflowRuns = rmMsg.runs
		m.lastFetched = time.Now()
		m.stopSpinners()
		cmds = append(cmds, m.onWorkflowRunsFetched()...)

	case prFetchedMsg:
		m.pr = msg.pr

	case workflowRunsFetchedMsg, prChecksIntervalTickMsg:
		var wrMsg workflowRunsFetchedMsg
		if tickMsg, ok := msg.(prChecksIntervalTickMsg); ok {
			wrMsg = tickMsg.msg.(workflowRunsFetchedMsg)
		} else {
			wrMsg = msg.(workflowRunsFetchedMsg)
		}
		m.rateLimit = wrMsg.rateLimit
		if wrMsg.err != nil && wrMsg.rateLimit.Remaining == 0 {
			log.Warn("rate limit reached, waiting", "rateLimit", wrMsg.rateLimit)
			return m, nil
		}
		if wrMsg.err == nil {
			m.checksFetched = true
		}

		if len(wrMsg.pr.Commits.Nodes) > 0 {
			log.Debug("workflow runs fetched", "fetched",
				len(wrMsg.pr.Commits.Nodes[0].Commit.StatusCheckRollup.Contexts.Nodes))
		}

		m.prWithChecks = wrMsg.pr
		if _, ok := msg.(prChecksIntervalTickMsg); ok {
			cmds = append(cmds, m.fetchPRChecksWithInterval())
		}

		if len(wrMsg.pr.Commits.Nodes) > 0 {
			pageInfo := wrMsg.pr.Commits.Nodes[0].Commit.StatusCheckRollup.Contexts.PageInfo
			if !pageInfo.HasPreviousPage {
				m.workflowRuns = make([]data.WorkflowRun, 0)
			}

			m.mergeWorkflowRuns(wrMsg)

			if pageInfo.HasNextPage {
				log.Info("fetching next checks page", "pageInfo", pageInfo)
				cmds = append(cmds, m.makeGetNextPagePRChecksCmd(pageInfo.EndCursor))
			} else {
				m.lastFetched = time.Now()
				m.stopSpinners()
				log.Info("fetched all checks", "pageInfo", pageInfo)
				cmds = append(cmds, m.onWorkflowRunsFetched()...)
			}
		} else {
			m.stopSpinners()
		}

		if wrMsg.err != nil {
			log.Debug("error when fetching workflow runs", "err", wrMsg.err)
			m.err = wrMsg.err
			if m.embedded {
				// Surface the error via EmbeddedView() instead of quitting
				// the host TUI.
				return m, nil
			}
			msgCmd := tea.Printf("%s\nrepo=%s, number=%s\nOriginal error: %v\n",
				lipgloss.NewStyle().Foreground(m.styles.colors.errorColor).Bold(true).Render(
					"❌ Pull request not found.",
				), m.repo, m.prNumber, wrMsg.err)
			return m, tea.Sequence(msgCmd, tea.Quit)
		}

	case workflowRunStepsFetchedMsg:
		cmds = append(cmds, m.enrichRunWithJobsStepsV2(msg)...)
		cmds = append(cmds, m.updateLists()...)

	case checkStepsFetchedMsg:
		m.enrichCheckWithSteps(msg)
		cmds = append(cmds, m.updateLists()...)

	case jobLogsFetchedMsg:
		ji := m.getJobItemById(msg.jobId)
		if ji != nil {
			ji.logs = msg.logs
			ji.logsErr = msg.err
			ji.logsStderr = msg.stderr
			ji.loadingLogs = false
			currJob := m.getSelectedJobItem()
			if currJob != nil && currJob.job.Id == msg.jobId {
				cmds = append(cmds, m.renderJobLogs())
				m.goToErrorInLogs()
			}

			cmds = append(cmds, m.updateLists()...)
		}

	case checkRunOutputFetchedMsg:
		ji := m.getJobItemById(msg.jobId)
		if ji != nil {
			if ji.job.Id == msg.jobId {
				ji.renderedText = msg.renderedText
				ji.loadingLogs = false
				currJob := m.jobsList.SelectedItem()
				if currJob != nil && currJob.(*jobItem).job.Id == msg.jobId {
					cmds = append(cmds, m.renderJobLogs())
				}

				cmds = append(cmds, m.updateLists()...)
				break
			}
		}

	case reRunJobMsg:
		if msg.err != nil {
			log.Error("error rerunning job", "jobId", msg.jobId, "err", msg.err)
		}
		ji := m.getJobItemById(msg.jobId)
		if ji == nil {
			break
		}

		m.lastFetched = time.Now()
		cmds = append(cmds, m.fetchPRChecksWithInterval())

	case reRunRunMsg:
		if msg.err != nil {
			log.Error("error rerunning run", "jobId", msg.runId, "err", msg.err)
		}
		ri := m.getRunItemById(msg.runId)
		if ri == nil {
			break
		}

		m.lastFetched = time.Now()
		cmds = append(cmds, m.fetchPRChecksWithInterval())

	case tea.WindowSizeMsg:
		log.Info("window size changed", "width", msg.Width, "height", msg.Height)
		m.width = msg.Width
		m.height = msg.Height
		m.setHeights()
		m.setWidths()

		m.setFocusedPaneStyles()
	case tea.KeyPressMsg:
		// Embedded actionview must never quit the program itself; the parent
		// TUI owns quit handling. When running standalone (non-embedded),
		// honor ctrl+c so the sub-tool can still exit cleanly.
		if !m.embedded && key.Matches(msg, quitKey) {
			log.Info("quitting", "msg", msg)
			return m, tea.Quit
		}

		log.Info("key pressed", "key", msg.String())
		if m.checksList.FilterState() == list.Filtering ||
			m.runsList.FilterState() == list.Filtering ||
			m.jobsList.FilterState() == list.Filtering ||
			m.stepsList.FilterState() == list.Filtering {
			break
		}

		if m.logsInput.Focused() {
			if key.Matches(msg, cancelSearchKey) {
				m.clearLogsSearch()
			} else if key.Matches(msg, applySearchKey) {
				ji := m.getSelectedJobItem()
				if ji != nil {
					m.logsViewport.SetContentLines(ji.unstyledLogs)
					highlights := regexp.MustCompile(
						m.logsInput.Value(),
					).FindAllStringIndex(
						strings.Join(ji.unstyledLogs, "\n"), -1,
					)
					m.numHighlights = len(highlights)
					m.logsViewport.SetHighlights(highlights)
					m.logsViewport.HighlightNext()
					m.logsInput.Blur()
				}
			} else {
				m.logsInput, cmd = m.logsInput.Update(msg)
				cmds = append(cmds, cmd)
				break
			}
		}

		if key.Matches(msg, modeKey) {
			m.flat = !m.flat
			if m.flat {
				m.focusedPane = PaneChecks
			} else {
				m.focusedPane = PaneRuns
			}
			cmds = append(cmds, m.onWorkflowRunsFetched()...)
			if m.flat {
				cmds = append(cmds, m.onCheckChanged()...)
			} else {
				cmds = append(cmds, m.onRunChanged()...)
			}
		}

		if key.Matches(msg, zoomPaneKey) {
			if m.zoomedPane == nil {
				m.zoomedPane = &m.focusedPane
				m.setWidths()
			} else {
				m.zoomedPane = nil
			}
		}

		if key.Matches(msg, refreshAllKey) {
			newModel := NewModel(m.repo, m.prNumber, ModelOpts{})
			newModel.flat = m.flat
			newModel.focusedPane = m.focusedPane
			newModel.width = m.width
			newModel.height = m.height
			newModel.setHeights()
			newModel.setWidths()

			newModel.setFocusedPaneStyles()

			m.lastFetched = time.Now()
			return newModel, newModel.makeInitCmd()
		}

		if key.Matches(msg, rerunKey) {
			if m.focusedPane != PaneRuns && m.focusedPane != PaneJobs &&
				m.focusedPane != PaneChecks {
				break
			}

			ri := m.getSelectedRunItem()
			if m.focusedPane == PaneRuns && ri != nil {
				cmds = append(cmds, m.rerunRun(ri.run.Id)...)
			} else {
				ji := m.getSelectedJobItem()
				if ri == nil && ji == nil {
					break
				}
				rid := ""
				if ri != nil {
					rid = ri.run.Id
				}
				cmds = append(cmds, m.rerunJob(rid, ji.job.Id)...)
			}
		}

		if m.focusedPane == PaneLogs && key.Matches(msg, searchKey) {
			cmds = append(cmds, m.logsInput.Focus())
		}

		if key.Matches(msg, openPRKey) && m.prWithChecks.Url != "" {
			cmds = append(cmds, makeOpenUrlCmd(m.prWithChecks.Url))
		}

		if key.Matches(msg, nextPaneKey) {
			m.focusedPane = m.nextPane()
			m.zoomedPane = nil
			m.setFocusedPaneStyles()
		}

		if key.Matches(msg, prevPaneKey) {
			m.focusedPane = m.previousPane()
			m.zoomedPane = nil
			m.setFocusedPaneStyles()
		}

	case spinner.TickMsg:
		checks := m.checksList.Items()
		for _, run := range checks {
			ci := run.(*checkItem)
			if ci != nil && ci.isStatusInProgress() {
				ci.spinner, cmd = ci.spinner.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

		runs := m.runsList.Items()
		for _, run := range runs {
			ri := run.(*runItem)
			if ri != nil && ri.IsInProgress() {
				ri.spinner, cmd = ri.spinner.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

		jobs := m.jobsList.Items()
		for _, job := range jobs {
			ji := job.(*jobItem)
			if ji != nil && ji.isStatusInProgress() {
				ji.spinner, cmd = ji.spinner.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

		steps := m.stepsList.Items()
		for _, step := range steps {
			si := step.(*stepItem)
			if si != nil && si.IsInProgress() {
				si.spinner, cmd = si.spinner.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

		ji := m.getSelectedJobItem()
		if ji == nil || ji.loadingLogs {
			m.logsSpinner, cmd = m.logsSpinner.Update(msg)
			cmds = append(cmds, cmd)
		} else if ji.isStatusInProgress() {
			m.inProgressSpinner, cmd = m.inProgressSpinner.Update(msg)
			cmds = append(cmds, cmd)
		}

		ci := m.getSelectedCheckItem()
		if ci == nil || ci.loadingLogs {
			m.logsSpinner, cmd = m.logsSpinner.Update(msg)
			cmds = append(cmds, cmd)
		} else if ci.isStatusInProgress() {
			m.inProgressSpinner, cmd = m.inProgressSpinner.Update(msg)
			cmds = append(cmds, cmd)
		}

		m.checksList, cmd = m.checksList.Update(msg)
		cmds = append(cmds, cmd)
		m.runsList, cmd = m.runsList.Update(msg)
		cmds = append(cmds, cmd)
		m.jobsList, cmd = m.jobsList.Update(msg)
		cmds = append(cmds, cmd)
		m.stepsList, cmd = m.stepsList.Update(msg)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)

	case errMsg:
		m.err = msg
		if m.embedded {
			// Surface the error via EmbeddedView() instead of quitting
			// the host TUI.
			return m, nil
		}
		return m, tea.Quit
	}

	switch m.focusedPane {
	case PaneChecks:
		before := m.getSelectedCheckItem()
		m.checksList, cmd = m.checksList.Update(msg)
		cmds = append(cmds, cmd)
		after := m.getSelectedCheckItem()
		if (before == nil && after != nil) || (after == nil && before != nil) ||
			(before != nil && after != nil && before.job.Id != after.job.Id) {
			cmds = append(cmds, m.onCheckChanged()...)
			cmds = append(cmds, m.updateLists()...)
		}
	case PaneRuns:
		before := m.runsList.GlobalIndex()
		m.runsList, cmd = m.runsList.Update(msg)
		cmds = append(cmds, cmd)
		after := m.runsList.GlobalIndex()
		if before != after {
			cmds = append(cmds, m.onRunChanged()...)
			cmds = append(cmds, m.updateLists()...)
		}
	case PaneJobs:
		before := m.jobsList.GlobalIndex()
		m.jobsList, cmd = m.jobsList.Update(msg)
		cmds = append(cmds, cmd)
		after := m.jobsList.GlobalIndex()
		if before != after {
			cmds = append(cmds, m.onJobChanged()...)
		}
	case PaneSteps:
		before := m.stepsList.GlobalIndex()
		m.stepsList, cmd = m.stepsList.Update(msg)
		cmds = append(cmds, cmd)
		after := m.stepsList.GlobalIndex()
		if before != after {
			m.onStepChanged()
		}

	case PaneLogs:
		if msg, ok := msg.(tea.KeyPressMsg); ok {
			if key.Matches(msg, gotoBottomKey) {
				m.logsViewport.GotoBottom()
			}

			if key.Matches(msg, gotoTopKey) {
				m.logsViewport.GotoTop()
			}

			if key.Matches(msg, nextSearchMatchKey) {
				m.logsViewport.HighlightNext()
			}

			if key.Matches(msg, prevSearchMatchKey) {
				m.logsViewport.HighlightPrevious()
			}

			if key.Matches(msg, cancelSearchKey) {
				m.clearLogsSearch()
			}
		}
		m.logsViewport, cmd = m.logsViewport.Update(msg)

		cmds = append(cmds, cmd)
	}

	if _, ok := msg.(tea.KeyPressMsg); !ok && m.logsInput.Focused() {
		m.logsInput, cmd = m.logsInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	m.setFocusedPaneStyles()

	return m, tea.Batch(cmds...)
}

func (m Model) EmbeddedView() string {
	if m.err != nil {
		log.Error("fatal error", "err", m.err)
		return m.err.Error()
	}

	panes := ""
	if m.flat {
		panes = m.viewFlatChecks()
	} else {
		panes = m.viewHierarchicalChecks()
	}

	return lipgloss.NewStyle().Width(m.width).MaxWidth(m.width).Render(panes)
}

func (m *model) viewHierarchicalChecks() string {
	runsPane := makePointingBorder(m.paneStyle(PaneRuns).Render(m.runsList.View()))
	jobsPane := makePointingBorder(m.paneStyle(PaneJobs).Render(m.jobsList.View()))
	stepsPane := ""
	if m.shouldShowSteps() {
		stepsPane = makePointingBorder(m.paneStyle(PaneSteps).Render(m.stepsList.View()))
	}

	panes := make([]string, 0)
	if m.zoomedPane != nil {
		switch *m.zoomedPane {
		case PaneRuns:
			panes = append(panes, runsPane)
		case PaneJobs:
			panes = append(panes, jobsPane)
		case PaneSteps:
			panes = append(panes, stepsPane)
		case PaneLogs:
			panes = append(panes, m.viewLogs())
		}
	} else {
		panes = append(panes, runsPane)
		panes = append(panes, jobsPane)
		panes = append(panes, stepsPane)
		panes = append(panes, m.viewLogs())
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		panes...,
	)
}

func (m *model) viewFlatChecks() string {
	checksView := m.checksList.View()
	if !m.checksFetched && len(m.checksList.Items()) == 0 {
		checksView = lipgloss.JoinVertical(
			lipgloss.Left,
			makePill(ListSymbol+" checks", m.styles.focusedPaneTitleStyle, m.styles.colors.focusedColor),
			m.styles.faintFgStyle.Render("Loading checks..."),
		)
	}
	checksPane := makePointingBorder(m.paneStyle(PaneChecks).Render(checksView))
	stepsPane := ""
	if m.shouldShowSteps() {
		stepsPane = makePointingBorder(m.paneStyle(PaneSteps).Render(m.stepsList.View()))
	}

	panes := make([]string, 0)
	if m.zoomedPane != nil {
		switch *m.zoomedPane {
		case PaneChecks:
			panes = append(panes, checksPane)
		case PaneSteps:
			panes = append(panes, stepsPane)
		case PaneLogs:
			panes = append(panes, m.viewLogs())
		}
	} else {
		panes = append(panes, checksPane)
		panes = append(panes, stepsPane)
		panes = append(panes, m.viewLogs())
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		panes...,
	)
}

func (m *model) isRunModeInProgress() bool {
	if len(m.workflowRuns) == 0 {
		return true
	}
	for _, job := range m.workflowRuns[0].Jobs {
		if job.IsStatusInProgress() {
			return true
		}
	}
	return false
}

func (m *model) shouldShowSteps() bool {
	loadingSteps := false
	if m.flat {
		check := m.checksList.SelectedItem()
		if check != nil {
			ci := check.(*checkItem)
			loadingSteps = ci.loadingSteps
		}
	} else {
		job := m.jobsList.SelectedItem()
		if job != nil {
			ji := job.(*jobItem)
			loadingSteps = ji.loadingSteps
		}
	}

	return loadingSteps || len(m.stepsList.VisibleItems()) > 0
}

func (m *model) viewLogs() string {
	title := "Job Logs"
	w := m.logsWidth()
	if m.focusedPane == PaneLogs {
		title = makePill(title, m.styles.focusedPaneTitleStyle, m.styles.colors.focusedColor)
		s := m.styles.focusedPaneTitleBarStyle.MarginBottom(0)
		title = s.Render(title)
	} else {
		title = makePill(title, m.styles.unfocusedPaneTitleStyle, m.styles.colors.unfocusedColor)
		s := m.styles.unfocusedPaneTitleBarStyle.MarginBottom(0)
		title = s.Render(title)
	}

	if m.logsInput.Value() != "" && !m.logsInput.Focused() {
		matches := fmt.Sprintf("%d matches", m.numHighlights)
		if m.numHighlights == 0 {
			matches = "no matches"
		}
		title = lipgloss.JoinHorizontal(lipgloss.Top, title, " ",
			m.styles.faintFgStyle.Render(matches))
	}

	inputView := ""
	ji := m.getSelectedJobItem()
	if ji != nil && m.logsViewport.GetContent() != "" && ji.logsStderr == "" {
		inputView = lipgloss.NewStyle().
			Width(w).
			Border(lipgloss.RoundedBorder(), true).
			BorderForeground(
				m.styles.colors.fainterColor,
			).
			Render(m.logsInput.View())
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, inputView, m.logsContentView())
}

func (m *model) setFocusedPaneStyles() {
	// Keep checks and steps delegates always "focused" so their selected
	// item is rendered with the blue background highlight regardless of
	// which pane currently has focus. Pane focus is conveyed by the pane
	// title pill color.
	m.checksDelegate.(*checksDelegate).focused = true
	m.stepsDelegate.(*stepsDelegate).focused = true

	switch m.focusedPane {
	case PaneChecks:
		m.setListFocusedStyles(&m.checksList, &m.checksDelegate, PaneChecks)
		m.setListUnfocusedStyles(&m.stepsList, &m.stepsDelegate)
	case PaneRuns:
		m.runsDelegate.(*runsDelegate).focused = true
		m.jobsDelegate.(*jobsDelegate).focused = false
		m.setListFocusedStyles(&m.runsList, &m.runsDelegate, PaneRuns)
		m.setListUnfocusedStyles(&m.jobsList, &m.jobsDelegate)
		m.setListUnfocusedStyles(&m.stepsList, &m.stepsDelegate)
	case PaneJobs:
		m.runsDelegate.(*runsDelegate).focused = false
		m.jobsDelegate.(*jobsDelegate).focused = true
		m.setListUnfocusedStyles(&m.runsList, &m.runsDelegate)
		m.setListFocusedStyles(&m.jobsList, &m.jobsDelegate, PaneJobs)
		m.setListUnfocusedStyles(&m.stepsList, &m.stepsDelegate)
	case PaneSteps:
		m.runsDelegate.(*runsDelegate).focused = false
		m.jobsDelegate.(*jobsDelegate).focused = false
		m.setListUnfocusedStyles(&m.checksList, &m.checksDelegate)
		m.setListUnfocusedStyles(&m.runsList, &m.runsDelegate)
		m.setListUnfocusedStyles(&m.jobsList, &m.jobsDelegate)
		m.setListFocusedStyles(&m.stepsList, &m.stepsDelegate, PaneSteps)
	case PaneLogs:
		m.runsDelegate.(*runsDelegate).focused = false
		m.jobsDelegate.(*jobsDelegate).focused = false
		m.setListUnfocusedStyles(&m.checksList, &m.checksDelegate)
		m.setListUnfocusedStyles(&m.runsList, &m.runsDelegate)
		m.setListUnfocusedStyles(&m.jobsList, &m.jobsDelegate)
		m.setListUnfocusedStyles(&m.stepsList, &m.stepsDelegate)
	}

	w := m.logsWidth()
	m.logsViewport.SetWidth(w)
	m.logsInput.SetWidth(int(math.Max(float64(0), float64(
		w-lipgloss.Width(m.logsInput.Prompt)-2,
	))))
}

func (m *model) setListFocusedStyles(l *list.Model, delegate *list.ItemDelegate, p pane) {
	l.Styles.Title = m.styles.focusedPaneTitleStyle
	l.Styles.TitleBar = m.styles.unfocusedPaneTitleBarStyle
	l.Title = makePill(m.getPaneTitle(l), l.Styles.Title, m.styles.colors.focusedColor)

	w := m.getFocusedPaneWidth(l, p)
	l.SetWidth(w)
	l.Styles.StatusBar = l.Styles.StatusBar.PaddingLeft(1).Width(w)
	l.SetDelegate(*delegate)
}

func (m *model) setListUnfocusedStyles(l *list.Model, delegate *list.ItemDelegate) {
	l.Styles.Title = m.styles.unfocusedPaneTitleStyle
	l.Title = makePill(m.getPaneTitle(l), l.Styles.Title, m.styles.colors.unfocusedColor)
	l.Styles.TitleBar = m.styles.unfocusedPaneTitleBarStyle

	w := m.getUnfocusedPaneWidth()
	l.SetWidth(w)
	l.Styles.StatusBar = l.Styles.StatusBar.PaddingLeft(1).Width(w)
	l.SetDelegate(*delegate)
}

func newRunsDefaultList(styles styles) (list.Model, list.ItemDelegate) {
	d := newRunItemDelegate(styles)
	return newList(styles, d), d
}

func newJobsDefaultList(styles styles) (list.Model, list.ItemDelegate) {
	d := newJobItemDelegate(styles)
	return newList(styles, d), d
}

func newStepsDefaultList(styles styles) (list.Model, list.ItemDelegate) {
	d := newStepItemDelegate(styles)
	return newList(styles, d), d
}

func newChecksDefaultList(styles styles) (list.Model, list.ItemDelegate) {
	d := newCheckItemDelegate(styles)
	return newList(styles, d), d
}

func newList(styles styles, delegate list.ItemDelegate) list.Model {
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.KeyMap.Quit = quitKey
	l.Paginator.Type = paginator.Arabic
	l.Styles.StatusBar = l.Styles.StatusBar.Foreground(styles.colors.faintColor)
	l.Styles.StatusEmpty = l.Styles.StatusEmpty.Foreground(styles.colors.faintColor)
	l.Styles.StatusBarActiveFilter = l.Styles.StatusBarActiveFilter.Foreground(
		styles.colors.faintColor,
	)
	l.Styles.StatusBarFilterCount = l.Styles.StatusBarFilterCount.Foreground(
		styles.colors.faintColor,
	)
	l.Styles.NoItems = l.Styles.NoItems.Width(largePaneWidth).
		Foreground(styles.colors.faintColor)
	l.Styles.PaginationStyle = lipgloss.NewStyle().
		Foreground(styles.colors.faintColor).
		MarginLeft(1).
		MarginBottom(1)
	l.Styles.StatusBar = l.Styles.StatusBar.PaddingLeft(1)
	l.SetSpinner(spinner.Dot)
	l.KeyMap.NextPage = key.Binding{}
	l.KeyMap.PrevPage = key.Binding{}
	l.KeyMap.CursorUp = key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "up"))
	l.KeyMap.CursorDown = key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "down"))
	l.StartSpinner()
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	return l
}

func (m *model) updateLists() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)

	if m.flat {
		cCmds := m.updateChecksList()
		cmds = append(cmds, cCmds...)
	} else {
		rCmds := m.updateRunsList()
		cmds = append(cmds, rCmds...)

		jCmds := m.updateJobsList()
		cmds = append(cmds, jCmds...)
	}

	// the steps list is used in both modes
	sCmds := m.updateStepsList()
	cmds = append(cmds, sCmds...)

	return cmds
}

func (m *model) updateChecksList() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	if len(m.checksList.VisibleItems()) == 0 {
		return cmds
	}

	check := m.checksList.SelectedItem()
	if check == nil {
		return cmds
	}
	ci, ok := check.(*checkItem)
	if !ok {
		return cmds
	}

	if ci.loadingSteps {
		cmds = append(cmds, m.stepsList.StartSpinner())
	} else {
		m.stepsList.StopSpinner()
	}
	if len(m.checksList.VisibleItems()) > 0 || m.checksList.FilterState() == list.FilterApplied {
		m.checksList.SetShowStatusBar(true)
	} else {
		m.checksList.SetShowStatusBar(false)
	}

	return cmds
}

func (m *model) updateRunsList() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	if len(m.runsList.VisibleItems()) == 0 {
		return cmds
	}

	run := m.runsList.SelectedItem()
	if run == nil {
		return cmds
	}
	ri, ok := run.(*runItem)
	if !ok {
		return cmds
	}

	if ri.loading {
		cmds = append(cmds, m.stepsList.StartSpinner())
	} else {
		m.stepsList.StopSpinner()
	}
	if len(m.runsList.VisibleItems()) > 0 || m.runsList.FilterState() == list.FilterApplied {
		m.runsList.SetShowStatusBar(true)
	} else {
		m.runsList.SetShowStatusBar(false)
	}

	return cmds
}

func (m *model) updateJobsList() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	ri := m.getSelectedRunItem()
	if ri == nil {
		return cmds
	}

	jobs := make([]list.Item, 0)
	for _, ji := range ri.jobsItems {
		jobs = append(jobs, ji)
	}
	cmds = append(cmds, m.jobsList.SetItems(jobs))
	if len(m.jobsList.VisibleItems()) > 0 || m.jobsList.FilterState() == list.FilterApplied {
		m.jobsList.SetShowStatusBar(true)
	} else {
		m.jobsList.SetShowStatusBar(false)
	}

	return cmds
}

// updateStepsList sets the step items based on the selected job
func (m *model) updateStepsList() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)

	var selectedJobItem *jobItem
	if ci := m.getSelectedCheckItem(); ci != nil && m.flat {
		selectedJobItem = &ci.jobItem
	} else {
		selectedJobItem = m.getSelectedJobItem()
	}

	existing := m.stepsList.Items()
	existingCount := len(existing)
	if selectedJobItem != nil {
		for i, si := range selectedJobItem.steps {
			if i < existingCount {
				cmds = append(cmds, m.stepsList.SetItem(i, si))
			} else {
				cmds = append(cmds, m.stepsList.InsertItem(i, si))
			}
		}

		for i := existingCount; i >= len(selectedJobItem.steps); i-- {
			m.stepsList.RemoveItem(i)
		}
	}

	if len(m.stepsList.VisibleItems()) > 0 || m.stepsList.FilterState() == list.FilterApplied {
		m.stepsList.SetShowStatusBar(true)
	} else {
		m.stepsList.SetShowStatusBar(false)
	}
	cmds = append(cmds, m.tickSteps()...)

	return cmds
}

func (m *model) getSelectedCheckItem() *checkItem {
	check := m.checksList.SelectedItem()
	if check == nil {
		return nil
	}
	ci, ok := check.(*checkItem)
	if !ok {
		return nil
	}

	return ci
}

func (m *model) getSelectedRunItem() *runItem {
	run := m.runsList.SelectedItem()
	if run == nil {
		return nil
	}
	ri, ok := run.(*runItem)
	if !ok {
		return nil
	}

	return ri
}

func (m *model) getSelectedJobItem() *jobItem {
	if m.flat {
		check := m.checksList.SelectedItem()
		if check == nil {
			return nil
		}
		ci, ok := check.(*checkItem)
		if !ok {
			return nil
		}
		return &ci.jobItem
	} else {
		job := m.jobsList.SelectedItem()
		if job == nil {
			return nil
		}
		ji, ok := job.(*jobItem)
		if !ok {
			return nil
		}
		return ji
	}
}

func (m *model) logsWidth() int {
	if m.width == 0 {
		return 0
	}

	if m.zoomedPane != nil && *m.zoomedPane == PaneLogs {
		return m.width - 1
	}

	var borders int
	if m.flat {
		borders = 1
	} else {
		borders = 2
	}

	steps := 0
	if m.shouldShowSteps() {
		steps = m.stepsList.Width()
		borders = borders + 1
	}

	w := m.width - steps - borders
	if m.flat {
		w -= m.checksList.Width()
	} else {
		w -= m.runsList.Width() + m.jobsList.Width()
	}
	return w
}

func (m *model) loadingLogsView() string {
	return m.fullScreenMessageView(
		lipgloss.JoinVertical(lipgloss.Left, m.logsSpinner.View()),
	)
}

func (m *model) fullScreenMessageView(message string) string {
	return lipgloss.Place(
		m.logsWidth(),
		m.getLogsViewportHeight()-1,
		lipgloss.Center,
		0.75,
		message,
	)
}

func (m *model) noLogsView(message string) string {
	emptySetArt := strings.Builder{}
	for _, char := range emptySetIllustration {
		if char == '╱' {
			emptySetArt.WriteString(lipgloss.NewStyle().Foreground(
				m.styles.colors.errorColor,
			).Render("╱"))
		} else {
			emptySetArt.WriteString(m.styles.watermarkIllustrationStyle.Render(string(char)))
		}
	}

	return m.fullScreenMessageView(
		lipgloss.JoinVertical(
			lipgloss.Center,
			emptySetArt.String(),
			m.styles.noLogsStyle.Render(message),
		),
	)
}

func (m *model) enrichRunWithJobsStepsV2(msg workflowRunStepsFetchedMsg) []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	jobsMap := make(map[string]api.CheckRunWithSteps)
	checks := msg.data.Resource.WorkflowRun.CheckSuite.CheckRuns.Nodes
	for _, check := range checks {
		jobsMap[fmt.Sprintf("%d", check.DatabaseId)] = check
	}

	ri := m.getRunItemById(msg.runId)
	if ri == nil {
		log.Error("run not found when trying to enrich with steps", "msg.runId", msg.runId)
		return cmds
	}

	selectedJob := m.getSelectedJobItem()
	ri.loading = false
	for jIdx, ji := range ri.jobsItems {
		ri.jobsItems[jIdx].loadingSteps = false
		jobWithSteps, ok := jobsMap[ji.job.Id]
		if !ok {
			continue
		}

		steps := make([]*stepItem, 0)
		for _, step := range jobWithSteps.Steps.Nodes {
			si := NewStepItem(step, jobWithSteps.Url, m.styles)
			if selectedJob != nil && selectedJob.job.Id == ji.job.Id {
				cmds = append(cmds, si.Tick())
			}

			steps = append(steps, &si)
		}

		ri.jobsItems[jIdx].steps = steps
	}

	return cmds
}

func (m *model) enrichCheckWithSteps(msg checkStepsFetchedMsg) {
	ci := m.getCheckItemById(msg.checkId)
	if ci == nil {
		log.Error("check not found when trying to enrich with steps", "msg", msg)
		return
	}

	ci.loadingSteps = false

	steps := make([]*stepItem, 0)
	for _, step := range msg.steps {
		si := NewStepItem(step, ci.job.Link, m.styles)
		steps = append(steps, &si)
	}

	ci.steps = steps
}

func (m *model) onCheckChanged() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	m.resetStepsState()
	cmds = append(cmds, m.updateStepsList()...)
	cmds = append(cmds, m.tickSteps()...)
	cmds = append(cmds, m.logsSpinner.Tick, m.inProgressSpinner.Tick)

	currCheck := m.getSelectedCheckItem()
	if currCheck == nil {
		log.Error("check changed but current check is nil")
		return nil
	}

	if currCheck.hasInProgressSteps() || currCheck.loadingSteps {
		cmds = append(cmds, m.makeFetchCheckStepsCmd(currCheck.job.Id))
	}

	if !currCheck.initiatedLogsFetch && !currCheck.isStatusInProgress() {
		cmds = append(cmds, m.makeFetchJobLogsCmd())
	}
	cmds = append(cmds, m.onJobChanged()...)

	return cmds
}

func (m *model) onRunChanged() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	m.jobsList.ResetSelected()
	m.jobsList.ResetFilter()
	newRun := m.runsList.SelectedItem()

	ri, ok := newRun.(*runItem)
	if !ok {
		log.Error("run changed but there is no run", "newRun", newRun)
		return cmds
	}

	if ri.loading {
		cmds = append(cmds, m.makeFetchWorkflowRunStepsCmd(ri.run.Id))
	}

	cmds = append(cmds, m.updateLists()...)
	cmds = append(cmds, m.onJobChanged()...)

	jobs := m.jobsList.Items()
	for _, job := range jobs {
		ji := job.(*jobItem)
		cmds = append(cmds, ji.Tick())
	}

	m.logsViewport.GotoTop()

	return cmds
}

func (m *model) onJobChanged() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	m.resetStepsState()
	cmds = append(cmds, m.updateStepsList()...)
	cmds = append(cmds, m.tickSteps()...)
	cmds = append(cmds, m.logsSpinner.Tick, m.inProgressSpinner.Tick)

	currJob := m.getSelectedJobItem()
	if currJob != nil && !currJob.initiatedLogsFetch && !currJob.isStatusInProgress() {
		log.Debug("onJobChanged - fetching logs", "currJob", currJob)
		cmds = append(cmds, m.makeFetchJobLogsCmd())
	} else if currJob == nil {
		log.Error("job changed but current job is nil")
	}

	cmds = append(cmds, m.renderJobLogs())
	m.goToErrorInLogs()

	return cmds
}

func (m *model) onStepChanged() {
	ji := m.getSelectedJobItem()
	step := m.stepsList.SelectedItem()
	cursor := m.stepsList.Cursor()

	if ji == nil || step == nil {
		return
	}

	if cursor == len(m.stepsList.Items())-1 {
		m.logsViewport.GotoBottom()
		return
	}

	for i, log := range ji.logs {
		if log.Time.After(step.(*stepItem).step.StartedAt) {
			m.logsViewport.SetYOffset(i - 1)
			return
		}
	}
}

func (m *model) renderJobLogs() tea.Cmd {
	ji := m.getSelectedJobItem()
	if ji == nil || ji.loadingLogs {
		m.logsViewport.SetContent("")
	}

	if ji == nil {
		return nil
	}

	if ji.isStatusInProgress() {
		return m.inProgressSpinner.Tick
	}

	if ji.logsErr != nil {
		m.logsViewport.SetContent(ji.logsStderr)
		m.setHeights()

		return nil
	}

	if len(ji.renderedLogs) != 0 {
		m.logsViewport.SetContentLines(ji.renderedLogs)
		m.setHeights()

		return nil
	}

	if ji.job.Title != "" || ji.job.Kind == data.JobKindCheckRun ||
		ji.job.Kind == data.JobKindExternal {
		m.logsViewport.SetContent(ji.renderedText)
		m.logsViewport.SetWidth(5)
		m.setHeights()

		return nil
	}

	ji.renderedLogs, ji.unstyledLogs = m.renderLogs(ji)
	m.logsViewport.SetContentLines(ji.renderedLogs)
	m.setHeights()

	return nil
}

func (m *model) logsContentView() string {
	if m.prWithChecks.Number != 0 && len(m.prWithChecks.Commits.Nodes) > 0 &&
		m.prWithChecks.Commits.Nodes[0].Commit.StatusCheckRollup.Contexts.CheckRunCount == 0 {
		return m.fullScreenMessageView(
			lipgloss.JoinVertical(
				lipgloss.Center,
				lipgloss.NewStyle().Foreground(m.styles.palette.BrightWhite).Render(checkmarkSignArt),
				"",
				m.styles.faintFgStyle.Bold(true).Render(
					fmt.Sprintf("No checks reported on the '%s' branch", m.prWithChecks.HeadRefName),
				),
			),
		)
	}

	ji := m.getSelectedJobItem()
	if ji == nil {
		return m.fullScreenMessageView(
			m.styles.faintFgStyle.Bold(true).Render("Nothing selected..."),
		)
	}

	if ji.job.Conclusion == api.ConclusionSkipped {
		return m.noLogsView("This job was skipped")
	}

	if ji.job.Kind == data.JobKindStatusContext {
		if ji.job.Link != "" {
			return m.noLogsView("This status check does not expose logs here. Press o to open it on GitHub.")
		}
		return m.noLogsView("This status check does not expose logs here.")
	}

	if ji.isStatusInProgress() {
		text := ""
		if ji.job.State == api.StatusWaiting && ji.job.PendingEnv != "" {
			text = lipgloss.NewStyle().Foreground(
				m.styles.colors.warnColor,
			).Render("Waiting for review: " + ji.job.PendingEnv +
				" needs approval to start deploying changes.")
		} else {
			text = "This job is still in progress"
		}

		return m.fullScreenMessageView(
			m.renderFullScreenLogsSpinner(text, "view the job on github.com"),
		)
	}

	if ji.loadingLogs || (ji.loadingSteps && len(ji.steps) == 0) {
		return m.loadingLogsView()
	}

	if ji.job.Bucket == data.CheckBucketCancel {
		return m.fullScreenMessageView(lipgloss.JoinVertical(lipgloss.Center,
			m.styles.faintFgStyle.Render(stopSignArt),
			m.styles.faintFgStyle.Bold(true).Render("This job was cancelled")))
	}

	if ji.logsErr != nil && strings.Contains(ji.logsStderr, "HTTP 410:") {
		return m.fullScreenMessageView(
			"The logs for this run have expired and are no longer available.",
		)
	}

	if ji.logsErr != nil && strings.Contains(ji.logsStderr, "is still in progress;") {
		return m.fullScreenMessageView(m.renderFullScreenLogsSpinner(
			"This run is still in progress", "view the run on github.com",
		))
	}

	return zone.Mark("preview-logs", m.logsViewport.View())
}

func (m *model) getRunItemById(runId string) *runItem {
	for _, run := range m.runsList.Items() {
		ri := run.(*runItem)
		if ri.run.Id == runId {
			return ri
		}
	}
	return nil
}

func (m *model) getRunItemByName(runName string) *runItem {
	for _, run := range m.runsList.Items() {
		ri := run.(*runItem)
		if ri.run.Name == runName {
			return ri
		}
	}
	return nil
}

func (m *model) getCheckItemById(checkId string) *checkItem {
	for _, check := range m.checksList.Items() {
		ci := check.(*checkItem)
		if ci.job.Id == checkId {
			return ci
		}
	}
	return nil
}

func (m *model) getJobItemById(jobId string) *jobItem {
	if m.flat {
		for _, check := range m.checksList.Items() {
			ci := check.(*checkItem)
			if ci.job.Id == jobId {
				return &ci.jobItem
			}
		}
	} else {
		for _, run := range m.runsList.Items() {
			ri := run.(*runItem)
			for i := range ri.jobsItems {
				if ri.jobsItems[i].job.Id == jobId {
					return ri.jobsItems[i]
				}
			}
		}
	}
	return nil
}

func (m *model) renderLogs(ji *jobItem) ([]string, []string) {
	w := m.logsViewport.Width()
	expand := ExpandSymbol + " "
	lines := make([]string, 0)
	unstyledLines := make([]string, 0)
	for i, log := range ji.logs {
		unstyled := sanitizeLogText(log.Log)
		rendered := unstyled
		switch log.Kind {
		case data.LogKindError:
			ji.errorLine = i
			rendered = strings.Replace(rendered, parser.ErrorMarker, "", 1)
			unstyled = rendered
			rendered = m.styles.errorBgStyle.Width(w).Render(lipgloss.JoinHorizontal(lipgloss.Top,
				m.styles.errorTitleStyle.Render("Error: "), m.styles.errorStyle.Render(rendered)))
		case data.LogKindCommand:
			rendered = strings.Replace(rendered, parser.CommandMarker, "", 1)
			unstyled = rendered
			rendered = m.styles.commandStyle.Render(rendered)
		case data.LogKindGroupStart:
			rendered = strings.Replace(rendered, parser.GroupStartMarker, expand, 1)
			unstyled = rendered
			rendered = m.styles.groupStartMarkerStyle.Render(rendered)
		case data.LogKindJobCleanup:
			rendered = m.styles.stepStartMarkerStyle.Render(rendered)
		case data.LogKindStepStart:
			rendered = strings.Replace(rendered, parser.GroupStartMarker, expand, 1)
			unstyled = rendered
			rendered = m.styles.stepStartMarkerStyle.Render(rendered)
		case data.LogKindStepNone:
			sep := ""
			unstyledSep := ""
			if log.Depth > 0 {
				dm := strings.Repeat(
					fmt.Sprintf("%s  ", Separator), log.Depth,
				)
				unstyledSep = dm
				sep = m.styles.separatorStyle.Render(dm)
			}
			unstyled = unstyledSep + unstyled
			rendered = sep + rendered
		}
		lines = append(lines, rendered)
		unstyledLines = append(unstyledLines, unstyled)
	}
	return lines, unstyledLines
}

func sanitizeLogText(s string) string {
	return caretANSIEscapePattern.ReplaceAllString(ansi.Strip(s), "")
}

func (m *model) getFocusedPaneWidth(l *list.Model, p pane) int {
	if m.zoomedPane != nil && p == *m.zoomedPane {
		return m.width - 1
	}
	return m.paneWidth()
}

func (m *model) getPaneTitle(l *list.Model) string {
	_, itemsName := l.StatusBarItemName()
	return strings.ToUpper(string(itemsName[0])) + itemsName[1:]
}

func (m *model) getUnfocusedPaneWidth() int {
	return m.paneWidth()
}

// paneWidth returns the width to use for each side pane (runs/jobs/steps/checks).
// In standalone mode, it falls back to the legacy fixed largePaneWidth.
// When embedded, panes share the available width with the logs pane.
func (m *model) paneWidth() int {
	if !m.embedded || m.width <= 0 {
		return largePaneWidth
	}
	visiblePanes := m.visibleSidePaneCount()
	// Reserve at least ~30 columns for the logs pane.
	const minLogsWidth = 30
	available := max(0, m.width-minLogsWidth)
	if visiblePanes <= 0 {
		return largePaneWidth
	}
	w := available / visiblePanes
	if w < 8 {
		w = 8
	}
	if w > largePaneWidth {
		return largePaneWidth
	}
	return w
}

// visibleSidePaneCount returns the number of side panes that are currently
// rendered alongside the logs pane (varies by flat vs hierarchical mode and
// whether the steps pane is shown).
func (m *model) visibleSidePaneCount() int {
	if m.flat {
		return 1 // checksList only
	}
	if m.shouldShowSteps() {
		return 3 // runs, jobs, steps
	}
	return 2 // runs, jobs
}

func (m *model) goToErrorInLogs() {
	currJob := m.getSelectedJobItem()
	if currJob == nil {
		return
	}

	if currJob.errorLine > 0 {
		for i, step := range m.stepsList.VisibleItems() {
			if api.IsFailureConclusion(step.(*stepItem).step.Conclusion) {
				m.stepsList.Select(i)
				break
			}
		}
		m.logsViewport.SetYOffset(currJob.errorLine)
	} else {
		m.logsViewport.GotoTop()
	}
}

func (m *model) getLogsViewportHeight() int {
	h := m.getMainContentHeight()

	// TODO: take borders from logsInput view
	vph := h - 1
	if m.logsViewport.GetContent() != "" {
		vph -= lipgloss.Height(m.logsInput.View()) + 2 // borders
	}
	m.logsViewport.SetHeight(vph)

	return vph
}

func (m *model) getMainContentHeight() int {
	return m.height
}

func (m *model) setHeights() {
	h := m.getMainContentHeight()

	m.checksList.SetHeight(h)
	m.runsList.SetHeight(h)
	m.jobsList.SetHeight(h)
	m.stepsList.SetHeight(h)

	lh := m.getLogsViewportHeight()
	m.logsViewport.SetHeight(lh)
}

func (m *model) setWidths() {
	// Apply responsive pane widths first; logsWidth() consults the side
	// panes' widths to compute the remaining space.
	pw := m.paneWidth()
	m.runsList.SetWidth(pw)
	m.jobsList.SetWidth(pw)
	m.stepsList.SetWidth(pw)
	m.checksList.SetWidth(pw)

	w := m.logsWidth()
	m.logsViewport.SetWidth(w)
	m.logsInput.SetWidth(w - 10)
}

func (m *model) renderFullScreenLogsSpinner(message string, cta string) string {
	return lipgloss.JoinVertical(
		lipgloss.Center,
		lipgloss.JoinHorizontal(lipgloss.Center,
			m.inProgressSpinner.View(),
			"  ",
			lipgloss.NewStyle().Foreground(m.styles.colors.warnColor).Render(message)),
		"",
		m.styles.faintFgStyle.Render("(logs will be available when it is complete)"),
		"",
		lipgloss.JoinHorizontal(lipgloss.Top, m.styles.faintFgStyle.Render("Press "),
			m.styles.keyStyle.Render("o"),
			m.styles.faintFgStyle.Render(" to "),
			m.styles.faintFgStyle.Render(cta)),
	)
}

func (m *model) onWorkflowRunsFetched() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)

	if m.flat {
		before := m.getSelectedCheckItem()

		cmds = append(cmds, m.buildFlatChecksLists()...)

		if before == nil && len(m.checksList.Items()) > 0 {
			cmds = append(cmds, m.onCheckChanged()...)
		} else if len(m.checksList.Items()) > 0 {
			currCheck := m.getSelectedCheckItem()
			if currCheck != nil && currCheck.hasInProgressSteps() {
				cmds = append(cmds, m.makeFetchCheckStepsCmd(currCheck.job.Id))
			}
		}

		// reselect previously selected item as now its index may have changed
		if before != nil {
			for i, ci := range m.checksList.VisibleItems() {
				ci := ci.(*checkItem)
				if ci.job.Id == before.job.Id {
					m.checksList.Select(i)
					break
				}
			}
		}

		if before != nil && !before.initiatedLogsFetch {
			cmds = append(cmds, m.logsSpinner.Tick, m.makeFetchJobLogsCmd())
		}
	} else {
		selectedRun := m.runsList.SelectedItem()
		var before *runItem
		if selectedRun != nil {
			before = selectedRun.(*runItem)
		}

		cmds = append(cmds, m.buildHierachicalChecksLists()...)

		if len(m.runsList.Items()) > 0 {
			ri := m.runsList.SelectedItem().(*runItem)
			cmds = append(cmds, m.makeFetchWorkflowRunStepsCmd(ri.run.Id))
			if before == nil || before.run.Id != ri.run.Id {
				cmds = append(cmds, m.onRunChanged()...)
			}
		}

		currJob := m.getSelectedJobItem()
		if currJob != nil && !currJob.initiatedLogsFetch {
			cmds = append(cmds, m.logsSpinner.Tick, m.makeFetchJobLogsCmd())
		}
	}

	cmds = append(cmds, m.updateLists()...)

	return cmds
}

func (m *model) buildFlatChecksLists() []tea.Cmd {
	existingChecks := map[string]*checkItem{}
	for _, ci := range m.checksList.Items() {
		ci := ci.(*checkItem)
		existingChecks[ci.job.Id] = ci
	}

	cmds := make([]tea.Cmd, 0)
	sorted := make([]data.WorkflowJob, 0)
	for _, run := range m.workflowRuns {
		sorted = append(sorted, run.Jobs...)
	}
	data.SortJobs(sorted)

	items := make([]list.Item, 0)
	for _, job := range sorted {
		ci := NewCheckItem(job, m.styles)

		// restore previous item if exists and override with new data
		existing, ok := existingChecks[job.Id]
		if ok {
			newJobData := ci.job
			ci.jobItem = existing.jobItem
			ci.job = newJobData
		}
		items = append(items, &ci)
		cmds = append(cmds, ci.Tick())
	}
	m.checksList.SetItems(items)
	return cmds
}

func (m *model) buildHierachicalChecksLists() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	for i, run := range m.workflowRuns {
		ri := m.getRunItemByName(run.Name)
		if ri == nil {
			nr := NewRunItem(run, m.styles)
			ri = &nr
			cmds = append(cmds, ri.Tick())
			cmds = append(cmds, m.runsList.InsertItem(i, ri))
		}
		ri.run = &run

		jobs := make([]*jobItem, 0)
		for _, job := range run.Jobs {
			ji := m.getJobItemById(job.Id)
			if ji == nil {
				nji := NewJobItem(job, m.styles)
				cmds = append(cmds, nji.Tick(), m.inProgressSpinner.Tick)
				ji = &nji
			}
			ji.job = &job
			jobs = append(jobs, ji)
		}

		ri.jobsItems = jobs
	}
	return cmds
}

func (m *model) paneStyle(pane pane) lipgloss.Style {
	// the border of the pane is the actually rendered by the previous pane
	prev := m.previousPane()
	if prev != m.focusedPane && prev == pane {
		return m.styles.focusedPaneStyle
	}

	return m.styles.paneStyle
}

func (m *model) stopSpinners() {
	m.checksList.StopSpinner()
	m.runsList.StopSpinner()
	m.jobsList.StopSpinner()
}

func (m *model) resetStepsState() {
	m.logsViewport.ClearHighlights()
	m.numHighlights = 0
	m.logsInput.Reset()
	m.stepsList.ResetSelected()
	m.stepsList.ResetFilter()
}

func (m *model) tickSteps() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	steps := m.stepsList.Items()
	for _, step := range steps {
		si := step.(*stepItem)
		cmds = append(cmds, si.Tick())
	}
	return cmds
}
