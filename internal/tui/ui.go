package tui

import (
	"fmt"
	"os"
	"reflect"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	log "charm.land/log/v2"
	"github.com/atotto/clipboard"
	"github.com/cli/go-gh/v2/pkg/browser"
	zone "github.com/lrstanley/bubblezone/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/git"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/common"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/actionssection"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/actionview"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/footer"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/fuzzyselect"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/issuessection"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/issueview"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/notificationrow"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/notificationssection"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/notificationview"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prrow"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prssection"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prview"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/section"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/sidebar"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/tabs"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/tasks"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/keys"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/markdown"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/theme"
)

type activePane int

const (
	mainPane activePane = iota
	previewPane
)

type Model struct {
	keys               *keys.KeyMap
	sidebar            sidebar.Model
	prView             prview.Model
	issueSidebar       issueview.Model
	notificationView   notificationview.Model
	actionRunView      *actionview.Model
	actionRunViewKey   string
	actionRunViewCache map[string]*actionview.Model
	currSectionId      int
	footer             footer.Model
	prs                []section.Section
	issues             []section.Section
	notifications      []section.Section
	actions            []section.Section
	tabs               tabs.Model
	ctx                *context.ProgramContext
	taskSpinner        spinner.Model
	tasks              map[string]context.Task
	// prPreviewStates stores per-PR, per-tab sidebar scroll positions so that
	// returning to a previously viewed PR's tab restores its scroll.
	// Outer key: PR URL. Inner key: sidebar tab index. Value: viewport YOffset.
	prPreviewStates    map[string]map[int]int
	issuePreviewStates map[string]int
	copySelection      copySelectionModel
	messagePopup       *messagePopup
	mergePRPopup       *mergePRPopup
	openPRURLPopup     *openPRURLPopup
	visibleRefreshes   map[string]int
	visibleRefreshGen  int
	repoBranches       map[string]repoBranchesState
	positionOverride   string // "" means no override, "right" or "bottom"
	activePane         activePane
	// viewStates remembers per-view UI preferences (sidebar open/closed,
	// active section, focused pane) so that navigating away and back
	// restores the user's state for each view. Actions view's entry always
	// reports sidebarOpen=false because the Actions view manages its own
	// three-pane layout and does not consume the global sidebar.
	viewStates map[config.ViewType]*viewState
}

var fetchPullRequestByNumberForOpenURL = data.FetchPullRequestByNumber

type openPRURLFetchedMsg struct {
	Ref githubPRRef
	PR  data.PullRequestData
	Err error
}

type viewState struct {
	sidebarOpen   bool
	currSectionId int
	activePane    activePane
}

type repoBranchesState struct {
	data    prssection.RepoBranches
	loading bool
}

// actionViewCacheLimit caps the number of cached embedded actionview
// instances kept across view switches. Ticks scheduled by cached models
// self-terminate (returning nil when their owner is no longer routing
// messages), but unbounded cache growth would still leak memory over a
// long session.
const actionViewCacheLimit = 16

func NewModel(location config.Location) Model {
	taskSpinner := spinner.Model{Spinner: spinner.Dot}
	m := Model{
		keys:               keys.Keys,
		sidebar:            sidebar.NewModel(),
		taskSpinner:        taskSpinner,
		tasks:              map[string]context.Task{},
		prPreviewStates:    map[string]map[int]int{},
		issuePreviewStates: map[string]int{},
		actionRunViewCache: map[string]*actionview.Model{},
		visibleRefreshes:   map[string]int{},
		repoBranches:       map[string]repoBranchesState{},
		viewStates:         map[config.ViewType]*viewState{},
	}

	version := "dev"
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Sum != "" {
		version = info.Main.Version
	}

	m.ctx = &context.ProgramContext{
		RepoPath:   location.RepoPath,
		ConfigFlag: location.ConfigFlag,
		Version:    version,
		StartTask: func(task context.Task) tea.Cmd {
			log.Info("Starting task", "id", task.Id)
			task.StartTime = time.Now()
			m.tasks[task.Id] = task
			return m.taskSpinner.Tick
		},
		Theme:      *theme.DefaultTheme,
		ActivePane: "main",
	}

	m.footer = footer.NewModel(m.ctx)
	m.prView = prview.NewModel(m.ctx)
	m.issueSidebar = issueview.NewModel(m.ctx)
	m.notificationView = notificationview.NewModel(m.ctx)
	m.tabs = tabs.NewModel(m.ctx)

	return m
}

func (m *Model) initScreen() tea.Msg {
	showError := func(err error) {
		styles := log.DefaultStyles()
		styles.Key = lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")).
			Bold(true)
		styles.Separator = lipgloss.NewStyle()

		logger := log.New(os.Stderr)
		logger.SetStyles(styles)
		logger.SetTimeFormat(time.RFC3339)
		logger.SetReportTimestamp(true)
		logger.SetPrefix("Reading config file")
		logger.SetReportCaller(true)

		logger.
			Fatal(
				"failed parsing config file",
				"location",
				m.ctx.ConfigFlag,
				"err",
				err,
			)
	}

	cfg, err := config.ParseConfig(
		config.Location{RepoPath: m.ctx.RepoPath, ConfigFlag: m.ctx.ConfigFlag},
	)
	if err != nil {
		showError(err)
		return initMsg{Config: cfg}
	}

	err = keys.Rebind(
		cfg.Keybindings.Universal,
		cfg.Keybindings.Issues,
		cfg.Keybindings.Prs,
		cfg.Keybindings.Notifications,
	)
	if err != nil {
		showError(err)
	}

	err = actionview.RebindActionsKeybindings(cfg.Keybindings.Actions)
	if err != nil {
		showError(err)
	}

	err = keys.RebindActionsKeys(cfg.Keybindings.Actions)
	if err != nil {
		showError(err)
	}

	return initMsg{Config: cfg}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(tea.RequestBackgroundColor, m.initScreen)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd             tea.Cmd
		tabsCmd         tea.Cmd
		sidebarCmd      tea.Cmd
		prViewCmd       tea.Cmd
		issueSidebarCmd tea.Cmd
		footerCmd       tea.Cmd
		cmds            []tea.Cmd
		currSection     = m.getCurrSection()
		currRowData     = m.getCurrRowData()
	)
	if m.shouldUpdateActionRunView(msg) {
		view, actionCmd := m.actionRunView.UpdateEmbedded(msg)
		m.actionRunView = &view
		return m, actionCmd
	}
	if m.openPRURLPopup != nil {
		return m, m.updateOpenPRURLPopup(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		log.Info("Key pressed", "key", msg.String())
		m.ctx.Error = nil
		if m.messagePopup != nil {
			if msg.String() == "esc" || msg.String() == "enter" {
				m.messagePopup = nil
			}
			return m, nil
		}
		if m.mergePRPopup != nil {
			return m, m.updateMergePRPopup(msg)
		}
		if m.ctx.View == config.ActionsView && currSection != nil {
			if as, ok := currSection.(*actionssection.Model); ok && as != nil {
				if key.Matches(msg, keys.ActionsKeys.FocusNextPane) || key.Matches(msg, keys.ActionsKeys.FocusPrevPane) {
					cmds := []tea.Cmd{
						currSection.SetIsSearching(false),
						currSection.SetIsLocalSearching(false),
					}
					if key.Matches(msg, keys.ActionsKeys.FocusNextPane) {
						as.FocusNextPane()
					} else {
						as.FocusPrevPane()
					}
					cmds = append(cmds, m.onViewedRowChanged())
					return m, tea.Batch(cmds...)
				}
			}
		}

		// When the local row-filter search is focused, vertical
		// navigation keys (Up/Down/PgUp/PgDn) exit the search and then
		// fall through to the universal navigation handlers below so
		// the same press also moves the row cursor and triggers the
		// normal preview/sidebar refresh via onViewedRowChanged. All
		// other keys (typing, esc, enter, ctrl+c, ...) still route
		// into the section's HandleLocalSearchKey as before. The
		// global search bar and prompt confirmation continue to
		// swallow every key while focused.
		if currSection != nil && currSection.IsLocalSearchFocused() &&
			!currSection.IsSearchFocused() && !currSection.IsPromptConfirmationFocused() {
			if key.Matches(msg, m.keys.Up) || key.Matches(msg, m.keys.Down) ||
				m.isPageUpKey(msg) || m.isPageDownKey(msg) {
				if exitCmd := currSection.SetIsLocalSearching(false); exitCmd != nil {
					cmds = append(cmds, exitCmd)
				}
				// Fall through; do not return.
			} else {
				cmd = m.updateSection(currSection.GetId(), currSection.GetType(), msg)
				return m, cmd
			}
		} else if currSection != nil && (currSection.IsSearchFocused() ||
			currSection.IsPromptConfirmationFocused()) {
			cmd = m.updateSection(currSection.GetId(), currSection.GetType(), msg)
			return m, cmd
		}

		// prView and issueSidebar state must only influence key routing
		// when their respective surfaces are the active view. Otherwise
		// stale focus state (e.g. the user left the PR Checks logs
		// search focused, then switched to the dashboard Actions view)
		// silently hijacks keys meant for the current view. See
		// prViewIsActive / issueSidebarIsActive for the predicate.
		if m.prViewIsActive() && m.prView.IsChecksLogsSearchFocused() {
			m.prView, cmd = m.prView.Update(msg)
			m.syncSidebar()
			return m, cmd
		}

		if m.prViewIsActive() && m.prView.IsTextInputBoxFocused() {
			if key.Matches(msg, keys.Keys.PageUp) || key.Matches(msg, keys.Keys.PageDown) ||
				key.Matches(msg, keys.Keys.PreviewTop) || key.Matches(msg, keys.Keys.PreviewBottom) {
				m.sidebar, sidebarCmd = m.sidebar.Update(msg)
				return m, sidebarCmd
			}
			m.prView, cmd = m.prView.Update(msg)
			m.syncSidebar()
			return m, cmd
		}

		if m.issueSidebarIsActive() && m.issueSidebar.IsTextInputBoxFocused() {
			if key.Matches(msg, keys.Keys.PageUp) || key.Matches(msg, keys.Keys.PageDown) ||
				key.Matches(msg, keys.Keys.PreviewTop) || key.Matches(msg, keys.Keys.PreviewBottom) {
				m.sidebar, sidebarCmd = m.sidebar.Update(msg)
				return m, sidebarCmd
			}
			m.issueSidebar, cmd, _ = m.issueSidebar.Update(msg)
			m.syncSidebar()
			return m, cmd
		}

		if m.footer.ShowConfirmQuit && (msg.String() == "y" || msg.String() == "enter") {
			return m, tea.Quit
		} else if m.footer.ShowConfirmQuit {
			m.footer.SetShowConfirmQuit(false)
			return m, nil
		}

		if m.footer.ShowAll && msg.String() == "q" {
			m.footer.ShowAll = false
			m.syncMainContentDimensions()
			return m, nil
		}
		if m.ctx.View == config.ActionsView {
			if handled, navCmd := m.handleActionsNavigation(msg, currSection); handled {
				return m, navCmd
			}
		}

		// Handle notification PR/Issue action confirmation
		if m.notificationView.HasPendingAction() {
			var action string
			m.notificationView, action = m.notificationView.Update(msg)
			m.footer.SetLeftSection("")
			if action != "" {
				return m, m.executeNotificationAction(action)
			}
			return m, nil
		}

		switch {
		// In the Actions view, ctrl+left/ctrl+right are reserved for
		// switching between the Workflows / Runs / Details panes (handled
		// further below). Skip the global FocusMain/FocusPreview handlers
		// there so the Actions-specific switch can receive the key.
		case key.Matches(msg, m.keys.FocusMain) && m.ctx.View != config.ActionsView:
			m.setActivePane(mainPane)
			return m, nil

		case key.Matches(msg, m.keys.FocusPreview) && m.ctx.View != config.ActionsView:
			if m.sidebar.IsOpen {
				m.setActivePane(previewPane)
			} else {
				m.setActivePane(mainPane)
			}
			return m, nil

		case m.isPreviewFocused() && m.isPreviewTabKey(msg):
			outgoingTab := m.prView.SelectedTabIndex()
			m.savePRPreviewStateAt(outgoingTab)
			m.prView, prViewCmd = m.prView.Update(msg)
			m.syncSidebar()
			if !m.restoreCurrentPRPreviewTab() {
				m.sidebar.ScrollToTop()
			}
			cmds = append(cmds, prViewCmd, m.reconcileVisibleRefreshes())
			return m, tea.Batch(cmds...)

		case m.isPageDownKey(msg):
			if m.isPreviewFocused() {
				m.sidebar, sidebarCmd = m.sidebar.Update(msg)
				return m, sidebarCmd
			}
			if currSection != nil {
				prevRow := currSection.CurrRow()
				nextRow := prevRow
				for range m.mainPageSize() {
					nextRow = currSection.NextRow()
					if nextRow == currSection.NumRows()-1 {
						break
					}
				}
				if prevRow != nextRow && nextRow == currSection.NumRows()-1 {
					cmds = append(cmds, currSection.FetchNextPageSectionRows()...)
				}
				cmd = m.onViewedRowChanged()
			}

		case m.isPageUpKey(msg):
			if m.isPreviewFocused() {
				m.sidebar, sidebarCmd = m.sidebar.Update(msg)
				return m, sidebarCmd
			}
			if currSection != nil {
				for range m.mainPageSize() {
					prevRow := currSection.CurrRow()
					currSection.PrevRow()
					if currSection.CurrRow() == prevRow {
						break
					}
				}
				cmd = m.onViewedRowChanged()
			}

		// On the PR Checks tab, forward any actionview-local key (step
		// nav, log scroll, pane switching, log search, etc.) to the
		// embedded actionview. actionview.IsLocalKey is the single
		// source of truth for the key set, shared with the dashboard's
		// Actions view forwarding below; feature additions only need to
		// touch actionview/keys.go. PgUp/PgDn (ctrl+up/ctrl+down) are
		// matched earlier and keep their outer-sidebar half-page scroll.
		case m.isPreviewFocused() && m.prView.IsChecksTab() && actionview.IsLocalKey(msg):
			m.prView, cmd = m.prView.Update(msg)
			m.syncSidebar()
			return m, cmd

		case m.isPreviewFocused() && m.isPreviewNavigationKey(msg):
			m.sidebar, sidebarCmd = m.sidebar.Update(msg)
			return m, sidebarCmd

		case m.isUserDefinedKeybinding(msg):
			cmd = m.executeKeybinding(msg.String())
			return m, cmd

		case key.Matches(msg, m.keys.NextView):
			cmds = append(cmds, m.switchSelectedView())

		case key.Matches(msg, m.keys.PrevView):
			cmds = append(cmds, m.switchSelectedViewBack())

		case key.Matches(msg, m.keys.PrevSection):
			prevSection := m.getSectionAt(m.getPrevSectionId())
			if prevSection != nil {
				m.setCurrSectionId(prevSection.GetId())
				cmd = m.onViewedRowChanged()
			}

		case key.Matches(msg, m.keys.NextSection):
			nextSectionId := m.getNextSectionId()
			nextSection := m.getSectionAt(nextSectionId)
			if nextSection != nil {
				m.setCurrSectionId(nextSection.GetId())
				cmd = m.onViewedRowChanged()
			}

		case key.Matches(msg, m.keys.Down):
			if currSection != nil {
				prevRow := currSection.CurrRow()
				nextRow := currSection.NextRow()
				if prevRow != nextRow && nextRow == currSection.NumRows()-1 {
					cmds = append(cmds, currSection.FetchNextPageSectionRows()...)
				}
				cmd = m.onViewedRowChanged()
			}

		case key.Matches(msg, m.keys.Up):
			if currSection != nil {
				currSection.PrevRow()
				cmd = m.onViewedRowChanged()
			}

		case key.Matches(msg, m.keys.FirstLine):
			if currSection != nil {
				currSection.FirstItem()
				cmd = m.onViewedRowChanged()
			}

		case key.Matches(msg, m.keys.LastLine):
			if currSection != nil {
				if currSection.CurrRow()+1 < currSection.NumRows() {
					cmds = append(cmds, currSection.FetchNextPageSectionRows()...)
				}
				currSection.LastItem()
				cmd = m.onViewedRowChanged()
			}

		case key.Matches(msg, m.keys.CyclePreview):
			cmds = append(cmds, m.cyclePreview())

		case key.Matches(msg, m.keys.Refresh):
			if currSection != nil {
				data.ClearEnrichmentCache()
				currSection.ResetFilters()
				currSection.ResetRows()
				m.syncSidebar()
				currSection.SetIsLoading(true)
				cmds = append(cmds, currSection.FetchNextPageSectionRows()...)
			}

		case key.Matches(msg, m.keys.Redraw):
		// TODO: this doesn't exist in bubbletea v2
		// can't find a way to just ask to send bubbletea's internal repaintMsg{},
		// so this seems like the lightest-weight alternative
		// return m, tea.Batch(tea.ExitAltScreen, tea.EnterAltScreen)

		case key.Matches(msg, m.keys.Search):
			if currSection != nil {
				cmd = currSection.SetIsSearching(true)
				return m, cmd
			}

		case key.Matches(msg, m.keys.LocalSearch) && m.prViewIsActive() && m.prView.IsChecksTab():
			// Only route `s` to the PR Checks logs search when the
			// prView is the active surface. Without the prViewIsActive
			// gate, leaving the PR Checks tab "selected" on prView
			// caused `s` to be unconditionally consumed by the hidden
			// prView while in other views (e.g. dashboard Actions),
			// preventing those views' own local-search handler from
			// firing.
			cmd = m.prView.FocusChecksLogsSearch()
			m.syncSidebar()
			return m, cmd

		case key.Matches(msg, m.keys.LocalSearch):
			if currSection != nil {
				cmd = currSection.SetIsLocalSearching(true)
				return m, cmd
			}

		case key.Matches(msg, m.keys.Help):
			m.footer.ShowAll = !m.footer.ShowAll
			m.syncMainContentDimensions()

		case key.Matches(msg, m.keys.CopyNumber):
			var cmd tea.Cmd
			if currRowData == nil || reflect.ValueOf(currRowData).IsNil() {
				cmd = m.notifyErr("Current selection isn't associated with a PR/Issue")
				return m, cmd
			}
			number := fmt.Sprint(currRowData.GetNumber())
			err := clipboard.WriteAll(number)
			if err != nil {
				cmd = m.notifyErr(fmt.Sprintf("Failed copying to clipboard %v", err))
			} else {
				cmd = m.notify(fmt.Sprintf("Copied %s to clipboard", number))
			}
			return m, cmd

		case key.Matches(msg, m.keys.CopyUrl):
			var cmd tea.Cmd
			if currRowData == nil || reflect.ValueOf(currRowData).IsNil() {
				cmd = m.notifyErr("Current selection isn't associated with a PR/Issue")
				return m, cmd
			}
			url := currRowData.GetUrl()
			err := clipboard.WriteAll(url)
			if err != nil {
				cmd = m.notifyErr(fmt.Sprintf("Failed copying to clipboard %v", err))
			} else {
				cmd = m.notify(fmt.Sprintf("Copied %s to clipboard", url))
			}
			return m, cmd

		case key.Matches(msg, m.keys.Quit):
			if !m.ctx.Config.ConfirmQuit {
				return m, tea.Quit
			}

			m.footer.SetShowConfirmQuit(true)

		case m.ctx.View == config.PRsView:
			switch {
			case key.Matches(msg, keys.PRKeys.OpenURL):
				return m, m.openPRURLPopupForInput()

			case key.Matches(msg, keys.PRKeys.Create):
				var err error
				prSection, ok := currSection.(*prssection.Model)
				if !ok || prSection == nil {
					return m, nil
				}
				repoName, ok := prSection.RepoFromFilters()
				if !ok {
					cmd, err = prSection.PrepareCreatePRForm(nil)
				} else {
					cmd, err = prSection.PrepareCreatePRForm(m.cachedRepoBranches(repoName))
				}
				if err != nil {
					m.ctx.Error = err
					return m, nil
				}
				prSection.SetPromptConfirmationAction("create_pr")
				blinkCmd := prSection.SetIsPromptConfirmationShown(true)
				return m, tea.Batch(cmd, blinkCmd)

			case m.isPreviewFocused() && (key.Matches(msg, keys.PRKeys.PrevSidebarTab) ||
				key.Matches(msg, keys.PRKeys.NextSidebarTab)):
				var scmds []tea.Cmd
				var scmd tea.Cmd
				// Save the outgoing tab's scroll under the *current* tab
				// index before the carousel moves, so returning to this
				// tab later restores the scroll the user just left.
				outgoingTab := m.prView.SelectedTabIndex()
				m.savePRPreviewStateAt(outgoingTab)
				m.prView, scmd = m.prView.Update(msg)
				scmds = append(scmds, scmd)
				m.syncSidebar()
				// After re-rendering for the new tab, restore any saved
				// scroll for that tab. Falls back to top for first-visit.
				if !m.restoreCurrentPRPreviewTab() {
					m.sidebar.ScrollToTop()
				}
				scmds = append(scmds, m.reconcileVisibleRefreshes())
				return m, tea.Batch(scmds...)

			case key.Matches(msg, m.keys.OpenGithub):
				cmds = append(cmds, m.openBrowser())

			case key.Matches(msg, keys.PRKeys.Approve):
				return m, m.openSidebarForPRInput(m.prView.SetIsApproving)

			case key.Matches(msg, keys.PRKeys.Assign):
				return m, m.openSidebarForPRInput(m.prView.SetIsAssigning)

			case key.Matches(msg, keys.PRKeys.RequestReview):
				return m, m.openSidebarForPRInput(m.prView.SetIsRequestingReview)

			case key.Matches(msg, keys.PRKeys.Label):
				return m, m.openSidebarForPRInput(m.prView.SetIsLabeling)

			case key.Matches(msg, keys.PRKeys.Comment):
				return m, m.openSidebarForInputNoScroll(m.prView.SetIsCommenting)

			case m.prView.IsActivityTab() && (key.Matches(msg, keys.PRKeys.PrevReviewThread) ||
				key.Matches(msg, keys.PRKeys.NextReviewThread)):
				moved := false
				if key.Matches(msg, keys.PRKeys.PrevReviewThread) {
					moved = m.prView.FocusPrevReviewThread()
				} else {
					moved = m.prView.FocusNextReviewThread()
				}
				if !moved {
					return m, nil
				}
				m.syncSidebar()
				return m, nil

			// PR Checks tab: , / . (step nav), ctrl+, / ctrl+. (log
			// scroll), up/down (current pane navigation), and every
			// other actionview-local key are forwarded to the embedded
			// actionview by the IsLocalKey case earlier in the outer
			// switch. Nothing to do here.

			case key.Matches(msg, keys.PRKeys.ToggleReviewThread):
				if !m.prView.IsActivityTab() {
					return m, nil
				}
				return m, m.prView.ToggleFocusedReviewThreadResolved()

			case key.Matches(msg, keys.PRKeys.CopyBranch):
				pr, ok := currRowData.(*prrow.Data)
				if !ok || pr == nil || pr.Primary == nil {
					return m, m.notifyErr("Current selection isn't associated with a PR")
				}

				branch := pr.Primary.HeadRefName
				err := clipboard.WriteAll(branch)
				if err != nil {
					return m, m.notifyErr(fmt.Sprintf("Failed copying to clipboard %v", err))
				}
				return m, m.notify(fmt.Sprintf("Copied %s to clipboard", branch))

			case key.Matches(msg, keys.PRKeys.Close):
				if currRowData != nil {
					cmd = m.promptConfirmation(currSection, "close")
				}
				return m, cmd

			case key.Matches(msg, keys.PRKeys.Ready):
				if pr, ok := currRowData.(tasks.DraftablePRData); ok && currSection != nil {
					sid := tasks.SectionIdentifier{Id: currSection.GetId(), Type: currSection.GetType()}
					cmd = tasks.TogglePRDraft(m.ctx, sid, pr)
				}
				return m, cmd

			case key.Matches(msg, keys.PRKeys.Reopen):
				if action := prOpenCloseAction(currRowData); action != "" {
					cmd = m.promptConfirmation(currSection, action)
				}
				return m, cmd

			case key.Matches(msg, keys.PRKeys.Merge):
				if currRowData != nil && currSection != nil {
					sid := tasks.SectionIdentifier{Id: currSection.GetId(), Type: currSection.GetType()}
					m.openMergePRPopup(sid, currRowData)
				}
				return m, nil

			case key.Matches(msg, keys.PRKeys.Update):
				prSection, ok := currSection.(*prssection.Model)
				if !ok || prSection == nil {
					return m, nil
				}
				pr, ok := currRowData.(*prrow.Data)
				if !ok || pr == nil || pr.Primary == nil {
					return m, m.notifyErr("Current selection isn't associated with a PR")
				}
				cmd, editErr := prSection.PrepareEditPRForm(pr, m.cachedRepoBranches(pr.Primary.Repository.NameWithOwner))
				if editErr != nil {
					m.ctx.Error = editErr
					return m, nil
				}
				prSection.SetPromptConfirmationAction("edit_pr")
				blinkCmd := prSection.SetIsPromptConfirmationShown(true)
				return m, tea.Batch(cmd, blinkCmd)

			case key.Matches(msg, keys.PRKeys.ApproveWorkflows):
				if currRowData != nil {
					cmd = m.promptConfirmation(currSection, "approveWorkflows")
				}
				return m, cmd

			case key.Matches(msg, keys.PRKeys.ViewIssues):
				cmds = append(cmds, m.switchSelectedView())

			case key.Matches(msg, keys.PRKeys.SummaryViewMore):
				if m.prView.IsActivityTab() {
					m.prView.ToggleActivitySnippetsExpanded()
					m.syncSidebar()
					return m, nil
				}
				m.prView.SetSummaryViewMore()
				m.syncSidebar()
				return m, nil
			}
		case m.ctx.View == config.IssuesView:
			switch {
			case key.Matches(msg, m.keys.OpenGithub):
				cmds = append(cmds, m.openBrowser())

			case key.Matches(msg, keys.IssueKeys.Label):
				return m, m.openSidebarForInput(m.issueSidebar.SetIsLabeling)

			case key.Matches(msg, keys.IssueKeys.Assign):
				return m, m.openSidebarForInput(m.issueSidebar.SetIsAssigning)

			case key.Matches(msg, keys.IssueKeys.Unassign):
				return m, m.openSidebarForInput(m.issueSidebar.SetIsUnassigning)

			case key.Matches(msg, keys.IssueKeys.Comment):
				return m, m.openSidebarForInput(m.issueSidebar.SetIsCommenting)

			case key.Matches(msg, keys.IssueKeys.Checkout):
				cmd, err := m.issueSidebar.Checkout()
				if err != nil {
					m.ctx.Error = err
				}
				return m, cmd

			case key.Matches(msg, keys.IssueKeys.Close):
				if currRowData != nil {
					cmd = m.promptConfirmation(currSection, "close")
				}
				return m, cmd

			case key.Matches(msg, keys.IssueKeys.Reopen):
				if action := issueOpenCloseAction(currRowData); action != "" {
					cmd = m.promptConfirmation(currSection, action)
				}
				return m, cmd

			case key.Matches(msg, keys.IssueKeys.ViewPRs):
				cmds = append(cmds, m.switchSelectedView())
			}
		case m.ctx.View == config.ActionsView:
			// Debug: trace key routing in the Actions view so we can verify
			// pane-switch keys (ctrl+left/ctrl+right) actually reach this
			// case. Captured in debug.log when the app is run with --debug.
			if as, ok := currSection.(*actionssection.Model); ok && as != nil {
				log.Debug(
					"actions view key",
					"key", msg.String(),
					"focusedPane", as.FocusedPane(),
					"matchesNext", key.Matches(msg, keys.ActionsKeys.FocusNextPane),
					"matchesPrev", key.Matches(msg, keys.ActionsKeys.FocusPrevPane),
				)
			} else {
				log.Debug("actions view key (no section)", "key", msg.String())
			}
			// Route navigation keys to the focused pane before the universal
			// Up/Down/PgUp/PgDn handlers (which assume single-table sections).
			if handled, navCmd := m.handleActionsNavigation(msg, currSection); handled {
				return m, navCmd
			}
			// When the details pane has focus, forward actionview-local
			// keys (step nav, log scroll, pane switching, log search,
			// zoom, etc.) directly to the embedded view. actionview
			// owns the canonical key set via IsLocalKey; the equivalent
			// dispatch on the PR Checks tab uses the same predicate.
			if as, ok := currSection.(*actionssection.Model); ok && as != nil &&
				as.FocusedPane() == actionssection.PaneDetails &&
				m.actionRunView != nil &&
				actionview.IsLocalKey(msg) {
				view, actionCmd := m.actionRunView.Update(msg)
				m.actionRunView = &view
				return m, actionCmd
			}
			switch {
			case key.Matches(msg, m.keys.OpenGithub):
				cmds = append(cmds, m.openBrowser())
			case key.Matches(msg, keys.ActionsKeys.FocusNextPane):
				log.Debug("actions: FocusNextPane fired")
				if as, ok := currSection.(*actionssection.Model); ok && as != nil {
					as.FocusNextPane()
					return m, m.onViewedRowChanged()
				}
			case key.Matches(msg, keys.ActionsKeys.FocusPrevPane):
				log.Debug("actions: FocusPrevPane fired")
				if as, ok := currSection.(*actionssection.Model); ok && as != nil {
					as.FocusPrevPane()
					return m, m.onViewedRowChanged()
				}
			case key.Matches(msg, keys.ActionsKeys.Rerun),
				key.Matches(msg, keys.ActionsKeys.RerunFailed),
				key.Matches(msg, keys.ActionsKeys.Cancel),
				key.Matches(msg, keys.ActionsKeys.SortOrder),
				key.Matches(msg, keys.ActionsKeys.ToggleSmartFiltering):
				if currSection != nil {
					cmd = m.updateSection(currSection.GetId(), currSection.GetType(), msg)
					return m, cmd
				}
			}
		case m.ctx.View == config.NotificationsView:
			switch {
			case key.Matches(msg, m.keys.OpenGithub):
				cmds = append(cmds, m.openBrowser())
				return m, tea.Batch(cmds...)

			// Handle Enter to (re)load notification content - check before subject handlers
			// so Enter always works, even after viewing a notification
			case key.Matches(msg, keys.NotificationKeys.View):
				cmds = append(cmds, m.loadNotificationContent())

			// Return from PR/Issue detail back to the default notification prompt
			case key.Matches(msg, keys.NotificationKeys.BackToNotification):
				return m, m.backToNotification()

			// PR keybindings when viewing a PR notification
			case m.notificationView.GetSubjectPR() != nil:
				// On the PR Checks tab, forward any actionview-local
				// key directly to the embedded actionview, bypassing the
				// PRAction mapping. This is the same dispatch the main
				// preview path uses; see actionview.IsLocalKey for the
				// canonical key set.
				if !m.prView.IsTextInputBoxFocused() &&
					m.prView.IsChecksTab() && actionview.IsLocalKey(msg) {
					m.prView, cmd = m.prView.Update(msg)
					m.syncSidebar()
					return m, cmd
				}
				// Check for PR actions first (before updating prView)
				if !m.prView.IsTextInputBoxFocused() {
					action := prview.MsgToAction(msg)
					if action != nil {
						switch action.Type {
						case prview.PRActionApprove:
							return m, m.openSidebarForPRInput(m.prView.SetIsApproving)

						case prview.PRActionAssign:
							return m, m.openSidebarForPRInput(m.prView.SetIsAssigning)

						case prview.PRActionLabel:
							return m, m.openSidebarForPRInput(m.prView.SetIsLabeling)

						case prview.PRActionComment:
							return m, m.openSidebarForInputNoScroll(m.prView.SetIsCommenting)

						case prview.PRActionDiff:
							if pr := m.notificationView.GetSubjectPR(); pr != nil {
								cmd = common.DiffPR(pr.GetNumber(), pr.GetRepoNameWithOwner(),
									pr.GetUrl(),
									m.ctx.Config.Pager.Diff,
									m.ctx.Config.RunDiffPagerInBackground(),
									m.ctx.Config.GetFullScreenDiffPagerEnv())
							}
							return m, cmd

						case prview.PRActionCheckout:
							if pr := m.notificationView.GetSubjectPR(); pr != nil {
								cmd, _ = notificationssection.CheckoutPR(
									m.ctx, pr.GetNumber(), pr.GetRepoNameWithOwner(),
								)
							}
							return m, cmd

						case prview.PRActionClose:
							cmd = m.promptConfirmationForNotificationPR("close")
							return m, cmd

						case prview.PRActionReady:
							if pr := m.notificationView.GetSubjectPR(); pr != nil {
								sid := tasks.SectionIdentifier{Id: m.currSectionId, Type: notificationssection.SectionType}
								cmd = tasks.TogglePRDraft(m.ctx, sid, pr)
							}
							return m, cmd

						case prview.PRActionReopen:
							if action := prOpenCloseAction(m.notificationView.GetSubjectPR()); action != "" {
								cmd = m.promptConfirmationForNotificationPR(action)
							}
							return m, cmd

						case prview.PRActionMerge:
							if pr := m.notificationView.GetSubjectPR(); pr != nil {
								sid := tasks.SectionIdentifier{Id: m.currSectionId, Type: notificationssection.SectionType}
								m.openMergePRPopup(sid, pr)
							}
							return m, nil

						case prview.PRActionUpdate:
							cmd = m.promptConfirmationForNotificationPR("update")
							return m, cmd

						case prview.PRActionApproveWorkflows:
							cmd = m.promptConfirmationForNotificationPR("approveWorkflows")
							return m, cmd

						case prview.PRActionSummaryViewMore:
							m.prView.SetSummaryViewMore()
							m.syncSidebar()
							return m, nil

						case prview.PRActionToggleReviewThread:
							if !m.prView.IsActivityTab() {
								return m, nil
							}
							return m, m.prView.ToggleFocusedReviewThreadResolved()

						case prview.PRActionPrevReviewThread, prview.PRActionNextReviewThread:
							// On the Checks tab, , / . are forwarded to
							// the embedded actionview by the IsLocalKey
							// short-circuit at the top of this case; this
							// arm only handles the Activity-tab review-
							// thread navigation. Other tabs are a no-op.
							if !m.prView.IsActivityTab() {
								return m, tea.Batch(cmds...)
							}
							moved := false
							if action.Type == prview.PRActionPrevReviewThread {
								moved = m.prView.FocusPrevReviewThread()
							} else {
								moved = m.prView.FocusNextReviewThread()
							}
							if !moved {
								return m, nil
							}
							m.syncSidebar()
							return m, nil

						case prview.PRActionPrevStep, prview.PRActionNextStep:
							// ctrl+, / ctrl+. are forwarded to the
							// embedded actionview by the IsLocalKey
							// short-circuit at the top of this case
							// (Checks tab). On other tabs, a no-op.
							return m, nil
						}
					}
				}

				// Handle 's' key to switch views
				if key.Matches(msg, keys.PRKeys.ViewIssues) {
					cmds = append(cmds, m.switchSelectedView())
				}

				if m.isPreviewTabKey(msg) {
					return m, nil
				}

				// No action matched - update prView for navigation (tab switching, scrolling)
				var prCmd tea.Cmd
				m.prView, prCmd = m.prView.Update(msg)
				m.syncSidebar()
				cmds = append(cmds, prCmd, m.reconcileVisibleRefreshes())

			// Issue keybindings when viewing an Issue notification
			case m.notificationView.GetSubjectIssue() != nil:
				var issueCmd tea.Cmd
				var action *issueview.IssueAction
				m.issueSidebar, issueCmd, action = m.issueSidebar.Update(msg)

				if action != nil {
					switch action.Type {
					case issueview.IssueActionLabel:
						return m, m.openSidebarForInput(m.issueSidebar.SetIsLabeling)

					case issueview.IssueActionAssign:
						return m, m.openSidebarForInput(m.issueSidebar.SetIsAssigning)

					case issueview.IssueActionUnassign:
						return m, m.openSidebarForInput(m.issueSidebar.SetIsUnassigning)

					case issueview.IssueActionComment:
						return m, m.openSidebarForInput(m.issueSidebar.SetIsCommenting)

					case issueview.IssueActionCheckout:
						cmd, err := m.issueSidebar.Checkout()
						if err != nil {
							m.ctx.Error = err
						}
						return m, cmd

					case issueview.IssueActionClose:
						cmd = m.promptConfirmationForNotificationIssue("close")
						return m, cmd

					case issueview.IssueActionReopen:
						if action := issueOpenCloseAction(m.notificationView.GetSubjectIssue()); action != "" {
							cmd = m.promptConfirmationForNotificationIssue(action)
						}
						return m, cmd
					}
				}

				// Handle 's' key to switch views
				if key.Matches(msg, keys.IssueKeys.ViewPRs) {
					cmds = append(cmds, m.switchSelectedView())
				}

				// Sync sidebar and return issueCmd for navigation
				m.syncSidebar()
				cmds = append(cmds, issueCmd)

			case key.Matches(msg, keys.NotificationKeys.MarkAsDone):
				cmds = append(
					cmds,
					m.updateSection(currSection.GetId(), currSection.GetType(), msg),
				)

			case key.Matches(msg, keys.NotificationKeys.MarkAllAsDone):
				cmd = m.promptConfirmation(currSection, "done_all")
				return m, cmd

			case key.Matches(msg, keys.NotificationKeys.Open):
				cmd = m.updateSection(currSection.GetId(), currSection.GetType(), msg)
				return m, cmd

			case key.Matches(msg, keys.NotificationKeys.SortByRepo):
				cmd = m.updateSection(currSection.GetId(), currSection.GetType(), msg)
				return m, cmd

			case key.Matches(msg, keys.NotificationKeys.SwitchToPRs),
				key.Matches(msg, keys.PRKeys.ViewIssues):
				cmds = append(cmds, m.switchSelectedView())
			}
		}

	case initMsg:
		m.ctx.Config = &msg.Config
		m.ctx.RepoUrl = msg.RepoUrl
		m.ctx.Theme = theme.ParseTheme(m.ctx.Config)
		m.ctx.Styles = context.InitStyles(m.ctx.Theme)
		m.taskSpinner.Style = lipgloss.NewStyle().
			Background(m.ctx.Theme.SelectedBackground)

		m.ctx.View = m.ctx.Config.Defaults.View
		m.tabs.SetHasSearchSection(viewHasSearchSection(m.ctx.View))
		m.currSectionId = m.getCurrentViewDefaultSection()
		// Initialize sidebar from config; Actions view's per-view entry
		// pins sidebarOpen to false (its three-pane layout doesn't use
		// the global sidebar).
		if m.ctx.View == config.ActionsView {
			m.sidebar.IsOpen = false
		} else {
			m.sidebar.IsOpen = msg.Config.Defaults.Preview.Open
		}
		m.syncPreviewFocus()
		// Seed per-view state for every known view so that the first
		// navigation away/back yields the configured defaults rather
		// than zero values.
		for _, v := range []config.ViewType{
			config.PRsView, config.IssuesView,
			config.NotificationsView, config.ActionsView,
		} {
			m.ensureViewState(v)
		}
		// Persist the live state of the starting view so the seeded
		// entry reflects the actual currSectionId for this run.
		m.captureCurrentViewState()
		m.syncMainContentDimensions()

		newSections, fetchSectionsCmds := m.fetchAllViewSections()
		m.setCurrentViewSections(newSections)
		m.tabs.SetCurrSectionId(m.currSectionId)
		cmds = append(cmds, fetchSectionsCmds, m.tabs.Init(), fetchUser,
			m.doRefreshAtInterval(), m.doUpdateFooterAtInterval(), m.reconcileVisibleRefreshes())

	case intervalRefresh:
		newSections, fetchSectionsCmds := m.fetchAllViewSections()
		m.setCurrentViewSections(newSections)
		cmds = append(cmds, fetchSectionsCmds, m.doRefreshAtInterval(), m.reconcileVisibleRefreshes())

	case userFetchedMsg:
		m.ctx.User = msg.user

	case constants.TaskFinishedMsg:
		task, ok := m.tasks[msg.TaskId]
		if ok {
			log.Info("Task finished", "id", task.Id)
			if msg.Err != nil {
				log.Error("Task finished with error", "id", task.Id, "err", msg.Err)
				task.State = context.TaskError
				task.Error = msg.Err
				m.messagePopup = newErrorMessagePopup(msg.Err)
			} else {
				task.State = context.TaskFinished
			}
			now := time.Now()
			task.FinishedTime = &now
			m.tasks[msg.TaskId] = task
			clear := tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
				return constants.ClearTaskMsg{TaskId: msg.TaskId}
			})
			cmds = append(cmds, clear)

			scmd := m.updateSection(msg.SectionId, msg.SectionType, msg.Msg)
			cmds = append(cmds, scmd)

			if prMsg, ok := msg.Msg.(tasks.UpdatePRMsg); ok && msg.SectionType == notificationssection.SectionType {
				if pr := m.notificationView.GetSubjectPR(); pr != nil && pr.GetNumber() == prMsg.PrNumber {
					if prMsg.Title != nil {
						pr.Primary.Title = *prMsg.Title
						pr.Enriched.Title = *prMsg.Title
					}
					if prMsg.Body != nil {
						pr.Primary.Body = *prMsg.Body
						pr.Enriched.Body = *prMsg.Body
					}
					if prMsg.BaseRefName != nil {
						pr.Primary.BaseRefName = *prMsg.BaseRefName
						pr.Enriched.BaseRefName = *prMsg.BaseRefName
					}
					if prMsg.IsDraft != nil {
						pr.Primary.IsDraft = *prMsg.IsDraft
						pr.Enriched.IsDraft = *prMsg.IsDraft
					}
					if prMsg.AddedReviewers != nil {
						pr.Primary.ReviewRequests.Nodes = addReviewRequestsForNotificationPR(
							pr.Primary.ReviewRequests.Nodes, prMsg.AddedReviewers.Nodes,
						)
						pr.Enriched.ReviewRequests.Nodes = addReviewRequestsForNotificationPR(
							pr.Enriched.ReviewRequests.Nodes, prMsg.AddedReviewers.Nodes,
						)
					}
					if prMsg.RemovedReviewers != nil {
						pr.Primary.ReviewRequests.Nodes = removeReviewRequestsForNotificationPR(
							pr.Primary.ReviewRequests.Nodes, prMsg.RemovedReviewers.Nodes,
						)
						pr.Enriched.ReviewRequests.Nodes = removeReviewRequestsForNotificationPR(
							pr.Enriched.ReviewRequests.Nodes, prMsg.RemovedReviewers.Nodes,
						)
					}
					if prMsg.ThreadReply != nil {
						for i := range pr.Enriched.ReviewThreads.Nodes {
							thread := &pr.Enriched.ReviewThreads.Nodes[i]
							if thread.Id == prMsg.ThreadReply.ThreadId {
								thread.Comments.Nodes = append(thread.Comments.Nodes, prMsg.ThreadReply.Comment)
								thread.Comments.TotalCount++
								break
							}
						}
					}
					if prMsg.ThreadResolved != nil {
						for i := range pr.Enriched.ReviewThreads.Nodes {
							thread := &pr.Enriched.ReviewThreads.Nodes[i]
							if thread.Id == prMsg.ThreadResolved.ThreadId {
								thread.IsResolved = prMsg.ThreadResolved.IsResolved
								break
							}
						}
					}
				}
			}

			syncCmd := m.syncSidebar()
			if m.ctx.View == config.ActionsView {
				syncCmd = tea.Batch(syncCmd, m.syncActionsSelection())
			}
			cmds = append(cmds, syncCmd)
		}

	case prview.EnrichedPrMsg:
		if msg.Err == nil {
			m.prView.SetEnrichedPR(msg.Data)
			if msg.Id >= 0 && msg.Id < len(m.prs) {
				m.prs[msg.Id].(*prssection.Model).EnrichPR(msg.Data)
			}
			syncCmd := m.syncSidebar()
			cmds = append(cmds, syncCmd)
			if m.prView.IsActivityTab() {
				m.scrollActivityToBottomIfNoSavedOffset()
			}
			cmds = append(cmds, m.prView.ActivateChecks())
			cmds = append(cmds, m.reconcileVisibleRefreshes())
		} else {
			log.Error("failed enriching pr", "err", msg.Err)
		}

	case visibleRefreshTick:
		if m.visibleRefreshes == nil || m.visibleRefreshes[msg.target.key] != msg.generation {
			log.Debug("stale visible refresh tick", "key", msg.target.key, "generation", msg.generation)
			return m, m.reconcileVisibleRefreshes()
		}
		log.Debug("visible refresh tick", "key", msg.target.key, "kind", msg.target.kind)
		return m, m.fetchVisibleRefresh(msg.target)

	case prssection.RefreshRepoBranchesMsg:
		target := m.repoBranchesRefreshTarget(msg.SectionId, msg.RepoName, 0)
		if target.key == "" {
			return m, nil
		}
		if m.repoBranches == nil {
			m.repoBranches = map[string]repoBranchesState{}
		}
		m.repoBranches[msg.RepoName] = repoBranchesState{
			data:    prssection.RepoBranches{RepoName: msg.RepoName},
			loading: true,
		}
		return m, m.fetchVisibleRefresh(target)

	case visibleRefreshFetchedMsg:
		log.Debug("visible refresh fetched", "key", msg.target.key, "kind", msg.target.kind, "err", msg.err)
		if m.visibleRefreshes != nil {
			delete(m.visibleRefreshes, msg.target.key)
		}
		if !m.isVisibleRefreshTargetCurrent(msg.target) {
			cmds = append(cmds, m.reconcileVisibleRefreshes())
			break
		}
		if msg.err != nil && msg.target.kind != visibleRefreshRepoBranches {
			log.Error("failed refreshing visible resource", "kind", msg.target.kind, "err", msg.err)
			cmds = append(cmds, m.reconcileVisibleRefreshes())
			break
		}
		switch msg.target.kind {
		case visibleRefreshPRPreview:
			if pr, ok := msg.data.(data.EnrichedPullRequestData); ok {
				cmds = append(cmds, m.applyPRPreviewRefresh(msg.target.url, pr))
			}
		case visibleRefreshPRSection:
			if sectionMsg, ok := msg.data.(prssection.SectionPullRequestsRefreshedMsg); ok {
				cmds = append(cmds, m.updateSection(msg.target.sectionId, prssection.SectionType, sectionMsg))
				cmds = append(cmds, m.syncSidebar())
				cmds = append(cmds, m.fetchCurrentPRPreviewRefresh())
			}
		case visibleRefreshRepoBranches:
			if branchMsg, ok := msg.data.(prssection.RepoBranches); ok {
				cmds = append(cmds, m.applyRepoBranchesRefresh(branchMsg, msg.target.sectionId))
			}
		}
		cmds = append(cmds, m.reconcileVisibleRefreshes())

	case notificationPRFetchedMsg:
		if msg.Err == nil {
			// Convert enriched PR to prrow.Data for display
			prData := msg.PR.ToPullRequestData()
			m.notificationView.SetSubjectPR(&prrow.Data{
				Primary:    &prData,
				Enriched:   msg.PR,
				IsEnriched: true,
			}, msg.NotificationId)
			keys.SetNotificationSubject(keys.NotificationSubjectPR)
			// Update sidebar with PR view
			width := m.sidebar.GetSidebarContentWidth()
			m.prView.SetSectionId(0)
			m.prView.SetRow(m.notificationView.GetSubjectPR())
			m.prView.SetWidth(width)
			m.prView.SetEnrichedPR(msg.PR)
			// Switch to Activity tab and scroll to bottom if there's a latest comment
			// (indicates there's new activity to show)
			if msg.LatestCommentUrl != "" {
				m.prView.GoToActivityTab()
				m.setPRSidebarContent()
				m.sidebar.ScrollToBottom()
			} else {
				// For notifications without comments (new PRs, state changes, etc.)
				// show the Overview tab without scrolling
				m.prView.GoToFirstTab()
				m.setPRSidebarContent()
			}
			m.markNotificationAsRead(msg.NotificationId)
			cmds = append(cmds, m.reconcileVisibleRefreshes())
		} else {
			log.Error("failed fetching notification PR", "err", msg.Err)
		}

	case notificationIssueFetchedMsg:
		if msg.Err == nil {
			m.notificationView.SetSubjectIssue(&msg.Issue, msg.NotificationId)
			keys.SetNotificationSubject(keys.NotificationSubjectIssue)
			// Update sidebar with Issue view
			width := m.sidebar.GetSidebarContentWidth()
			m.issueSidebar.SetSectionId(0)
			m.issueSidebar.SetRow(m.notificationView.GetSubjectIssue())
			m.issueSidebar.SetWidth(width)
			m.sidebar.ClearHeader()
			m.sidebar.SetContent(m.issueSidebar.View())
			// Scroll to bottom if there's a latest comment (indicates new activity)
			if msg.LatestCommentUrl != "" {
				m.sidebar.ScrollToBottom()
			}
			m.markNotificationAsRead(msg.NotificationId)
		} else {
			log.Error("failed fetching notification Issue", "err", msg.Err)
		}

	case notificationssection.UpdateNotificationReadStateMsg:
		m.updateNotificationSections(msg)

	case notificationssection.UpdateNotificationCommentsMsg:
		cmds = append(cmds, m.updateNotificationSections(msg))

	case spinner.TickMsg:
		if len(m.tasks) > 0 {
			taskSpinner, internalTickCmd := m.taskSpinner.Update(msg)
			m.taskSpinner = taskSpinner
			rTask := m.renderRunningTask()
			m.footer.SetRightSection(rTask)
			cmd = internalTickCmd
		}
		// Fan out the tick to the embedded actionview so its internal
		// spinners keep animating. spinner.TickMsg carries an ID; each
		// spinner.Update only consumes ticks matching its own id, so this
		// is safe to deliver to multiple sinks.
		if m.actionRunView != nil {
			view, actionCmd := m.actionRunView.UpdateEmbedded(msg)
			m.actionRunView = &view
			cmds = append(cmds, actionCmd)
		}

	case constants.ClearTaskMsg:
		m.footer.SetRightSection("")
		delete(m.tasks, msg.TaskId)

	case section.SectionMsg:
		cmd = m.updateRelevantSection(msg)

		if msg.Id == m.currSectionId {
			cmds = append(cmds, m.onViewedRowChanged())
		}

	case execProcessFinishedMsg, tea.FocusMsg:
		// Avoid refetching the entire Actions workflow list every time the
		// terminal regains focus. actionssection has no meaningful
		// PageInfo, so FetchNextPageSectionRows is a full refetch.
		if _, isFocus := msg.(tea.FocusMsg); isFocus && m.ctx.View == config.ActionsView {
			break
		}
		if currSection != nil {
			cmds = append(cmds, currSection.FetchNextPageSectionRows()...)
		}

	case tea.MouseClickMsg:
		if msg.Button != tea.MouseLeft {
			return m, nil
		}

		{
			mouse := msg.Mouse()
			pane, bounds := m.copySelectionPaneAt(mouse.X, mouse.Y)
			if pane != copySelectionPaneNone {
				x, y := clampCopySelectionPoint(mouse.X, mouse.Y, bounds)
				m.copySelection.begin(pane, x, y)
				return m, nil
			}
		}

		if zone.Get("donate").InBounds(msg) {
			log.Info("Donate clicked", "msg", msg)
			openCmd := func() tea.Msg {
				b := browser.New("", os.Stdout, os.Stdin)
				err := b.Browse("https://github.com/sponsors/dlvhdr")
				if err != nil {
					return constants.ErrMsg{Err: err}
				}
				return nil
			}
			cmds = append(cmds, openCmd)
		}

	case tea.MouseMotionMsg:
		if m.copySelection.dragging {
			mouse := msg.Mouse()
			_, bounds := m.copySelectionPaneAt(m.copySelection.startX, m.copySelection.startY)
			x, y := clampCopySelectionPoint(mouse.X, mouse.Y, bounds)
			m.copySelection.update(x, y)
			return m, nil
		}

	case tea.MouseReleaseMsg:
		if m.copySelection.dragging {
			mouse := msg.Mouse()
			_, bounds := m.copySelectionPaneAt(m.copySelection.startX, m.copySelection.startY)
			x, y := clampCopySelectionPoint(mouse.X, mouse.Y, bounds)
			m.copySelection.update(x, y)

			if !m.copySelection.moved() {
				m.copySelection.cancel()
				return m, nil
			}

			text := m.copySelectionText()
			m.copySelection.cancel()
			if strings.TrimSpace(text) == "" {
				return m, m.notifyErr("No text selected")
			}
			if err := clipboard.WriteAll(text); err != nil {
				return m, m.notifyErr(fmt.Sprintf("Failed copying to clipboard %v", err))
			}
			return m, m.notify("Copied selection to clipboard")
		}

	case tea.WindowSizeMsg:
		m.onWindowSizeChanged(msg)

	case tea.BackgroundColorMsg:
		markdown.InitializeMarkdownStyle(msg.IsDark())

	case updateFooterMsg:
		cmds = append(cmds, cmd, m.doUpdateFooterAtInterval())

	case constants.ErrMsg:
		m.messagePopup = newErrorMessagePopup(msg.Err)

	case openPRURLFetchedMsg:
		cmds = append(cmds, m.applyOpenPRURLFetchedMsg(msg))
	}

	m.syncProgramContext()

	// Forward key messages to the sidebar only when the preview pane
	// owns focus; otherwise up/down/page-up/page-down would scroll the
	// sidebar viewport even when the user is navigating the row list in
	// the main pane. Preview-focused key navigation is already handled
	// by the dedicated early-return branches in the tea.KeyMsg arm
	// above (isPreviewNavigationKey, isPageUpKey, isPageDownKey,
	// preview tab keys), so this gate doesn't regress those paths.
	// Non-key messages (window resize, async updates, mouse) always
	// flow through so the sidebar viewport stays correctly sized and
	// up-to-date.
	if _, isKey := msg.(tea.KeyMsg); !isKey || m.isPreviewFocused() {
		m.sidebar, sidebarCmd = m.sidebar.Update(msg)
	}

	// Match the same active-surface gating used in the early-return
	// branches above: prView/issueSidebar text-input forwarding must
	// only fire when their surfaces are the active view. For key
	// messages this is already handled by the early-return blocks in
	// the tea.KeyMsg arm; the guards here additionally cover any other
	// message type that this fall-through block runs for (window size,
	// etc.) and keep the dispatch consistent.
	if m.prViewIsActive() && m.prView.IsTextInputBoxFocused() {
		m.prView, prViewCmd = m.prView.Update(msg)
		m.syncSidebar()
	}

	if m.prView.ShouldUpdateChecks(msg) {
		m.prView, prViewCmd = m.prView.Update(msg)
		m.syncSidebar()
	}

	if m.issueSidebarIsActive() && m.issueSidebar.IsTextInputBoxFocused() {
		m.issueSidebar, issueSidebarCmd, _ = m.issueSidebar.Update(msg)
		m.syncSidebar()
	}

	if currSection != nil {
		if currSection.IsPromptConfirmationFocused() {
			m.footer.SetLeftSection(currSection.GetPromptConfirmation())
		}

		if !currSection.IsPromptConfirmationFocused() {
			m.footer.SetLeftSection(currSection.GetPagerContent())
		}
	}

	tm, tabsCmd := m.tabs.Update(msg)
	m.tabs = tm

	sectionCmd := m.updateCurrentSection(msg)
	if m.ctx.View == config.ActionsView && shouldSyncActionsSelection(msg) {
		sectionCmd = tea.Batch(sectionCmd, m.syncActionsSelection())
	}
	cmds = append(
		cmds,
		cmd,
		tabsCmd,
		sidebarCmd,
		footerCmd,
		sectionCmd,
		prViewCmd,
		issueSidebarCmd,
	)

	return m, tea.Batch(cmds...)
}

func (m Model) View() tea.View {
	var v tea.View
	v.AltScreen = true
	v.ReportFocus = true
	v.MouseMode = tea.MouseModeAllMotion

	if m.ctx.Config == nil {
		v.Content = lipgloss.Place(
			m.ctx.ScreenWidth,
			m.ctx.ScreenHeight,
			lipgloss.Center,
			lipgloss.Center,
			"Reading config...",
		)
		return v
	}

	content := "No sections defined"
	sidebarView := ""
	currSection := m.getCurrSection()
	if currSection != nil {
		if actionsSection, ok := currSection.(*actionssection.Model); ok && m.ctx.View == config.ActionsView {
			content = m.renderActionsThreePane(actionsSection)
		} else {
			sectionView := m.renderCopySelectionContent(copySelectionPaneMain, currSection.View())
			sidebarView = m.renderCopySelectionContent(copySelectionPanePreview, m.sidebar.View())
			if m.ctx.PreviewPosition == "bottom" && m.sidebar.IsOpen {
				content = lipgloss.JoinVertical(
					lipgloss.Left,
					sectionView,
					sidebarView,
				)
			} else {
				content = lipgloss.JoinHorizontal(
					lipgloss.Top,
					sectionView,
					sidebarView,
				)
			}
		}
	}
	view := lipgloss.JoinVertical(
		lipgloss.Left,
		m.tabs.View(),
		content,
		m.footer.View(),
	)

	layers := []*lipgloss.Layer{
		lipgloss.NewLayer(zone.Scan(view)),
	}
	if logsSelectionLayer := m.renderCopySelectionPreviewLogsLayer(); logsSelectionLayer != nil {
		layers = append(layers, logsSelectionLayer)
	}

	if currSection != nil {
		searchCmp := currSection.ViewCompletions()
		if searchCmp != "" {
			y := common.HeaderHeight + common.SearchHeight + 1
			layers = append(layers, lipgloss.NewLayer(searchCmp).X(1).Y(y))
		}
	}

	prCmp := m.prView.ViewCompletions()
	previewPos := m.ctx.PreviewCursorPosition()
	if prCmp != "" {
		y := completionLayerY(previewPos.Y, sidebarView, m.prView.InputBoxLineFromBottom(), prCmp)
		layers = append(layers, lipgloss.NewLayer(prCmp).X(previewPos.X+3).Y(y))
	}

	issueCmp := m.issueSidebar.ViewCompletions()
	if issueCmp != "" {
		y := completionLayerY(previewPos.Y, sidebarView, m.issueSidebar.InputBoxLineFromButton(), issueCmp)
		layers = append(layers, lipgloss.NewLayer(issueCmp).X(previewPos.X+3).Y(y))
	}
	if popup := m.renderCreatePRPopup(); popup != "" {
		layers = append(layers, lipgloss.NewLayer(popup).
			X(max(0, (m.ctx.ScreenWidth-lipgloss.Width(popup))/2)).
			Y(max(0, (m.ctx.ScreenHeight-lipgloss.Height(popup))/2)))
	}
	if m.messagePopup != nil {
		popup := m.renderMessagePopup()
		layers = append(layers, lipgloss.NewLayer(popup).
			X(max(0, (m.ctx.ScreenWidth-lipgloss.Width(popup))/2)).
			Y(max(0, (m.ctx.ScreenHeight-lipgloss.Height(popup))/2)))
	}
	if popup := m.renderMergePRPopup(); popup != "" {
		layers = append(layers, lipgloss.NewLayer(popup).
			X(max(0, (m.ctx.ScreenWidth-lipgloss.Width(popup))/2)).
			Y(max(0, (m.ctx.ScreenHeight-lipgloss.Height(popup))/2)))
	}
	if popup := m.renderOpenPRURLPopup(); popup != "" {
		layers = append(layers, lipgloss.NewLayer(popup).
			X(max(0, (m.ctx.ScreenWidth-lipgloss.Width(popup))/2)).
			Y(max(0, (m.ctx.ScreenHeight-lipgloss.Height(popup))/2)))
	}
	comp := lipgloss.NewCompositor(layers...)
	v.SetContent(comp.Render())

	return v
}

func (m *Model) renderActionsThreePane(section *actionssection.Model) string {
	width := max(0, m.ctx.ScreenWidth)
	totalHeight := max(0, m.getBaseContentHeight())

	height := totalHeight

	// Column width distribution:
	//   - Workflows and Runs take a left strip (~22% each).
	//   - Details takes the remainder (~56%) so the embedded actionview's
	//     sub-panes have plenty of room.
	// Minimums clamp each pane on narrow terminals.
	firstWidth, secondWidth, thirdWidth := actionsPaneWidths(width)

	// Pane layout reserves vertical space for:
	//   - a 1-line outer column header ("Workflows" / "Runs" / "Details"),
	//   - the inner Table.View() which renders its own 2-line header
	//     (common.TableHeaderHeight) for the workflow/runs panes.
	// The details pane has no inner table header, so its body uses more
	// of the available height (see headerHeight passed below).
	const outerHeaderHeight = 1
	const innerTableHeaderHeight = 2
	const headerHeight = outerHeaderHeight + innerTableHeaderHeight
	focusedPane := section.FocusedPane()
	searchView := func(w int) string {
		if focusedPane != actionssection.PaneWorkflows && focusedPane != actionssection.PaneRuns {
			return ""
		}

		ctx := *m.ctx
		ctx.MainContentWidth = w
		// Only render the search bar while the search input is actively
		// focused. Persistent local-search values still filter rows but the
		// box itself disappears so the layout stays compact.
		if section.IsLocalSearchFocused() {
			section.LocalSearchBar.UpdateProgramContext(&ctx)
			return section.LocalSearchBar.ViewWithFocusedBorder(&ctx, m.ctx.Theme.FaintBorder)
		}
		if section.IsSearchFocused() {
			section.SearchBar.UpdateProgramContext(&ctx)
			return section.SearchBar.ViewWithFocusedBorder(&ctx, m.ctx.Theme.FaintBorder)
		}
		return ""
	}

	workflowSearchView := ""
	if focusedPane == actionssection.PaneWorkflows {
		workflowSearchView = searchView(firstWidth)
	}
	workflowSearchHeight := lipgloss.Height(workflowSearchView)
	runsSearchView := ""
	if focusedPane == actionssection.PaneRuns {
		runsSearchView = searchView(secondWidth)
	}
	runsSearchHeight := lipgloss.Height(runsSearchView)

	section.SetWorkflowTableDimensions(firstWidth, max(0, height-headerHeight-workflowSearchHeight))
	section.SetRunTableDimensions(secondWidth, max(0, height-headerHeight-runsSearchHeight))

	header := func(text string, w int) string {
		return lipgloss.NewStyle().
			Width(w).
			MaxWidth(w).
			Bold(true).
			Foreground(m.ctx.Theme.PrimaryText).
			Padding(0, 1).
			Render(text)
	}
	colStyle := func(w int) lipgloss.Style {
		return lipgloss.NewStyle().
			Width(w).
			MaxWidth(w).
			Height(height).
			MaxHeight(height)
	}
	joinPane := func(parts ...string) string {
		nonEmpty := make([]string, 0, len(parts))
		for _, part := range parts {
			if part != "" {
				nonEmpty = append(nonEmpty, part)
			}
		}
		return lipgloss.JoinVertical(lipgloss.Left, nonEmpty...)
	}

	workflowTitle := "Workflows"
	if section.GetIsLoading() {
		workflowTitle = "Workflows…"
	}
	workflows := colStyle(firstWidth).Render(
		joinPane(
			workflowSearchView,
			header(workflowTitle, firstWidth),
			section.Table.View(),
		),
	)

	runsTitle := "Runs"
	if section.RunsTable.IsLoading() {
		runsTitle = "Runs…"
	}
	runs := colStyle(secondWidth).Render(
		joinPane(
			runsSearchView,
			header(runsTitle, secondWidth),
			section.RunsView(),
		),
	)

	// The details pane only has the 1-line outer header (no inner table
	// header), so its body can use more height than the workflow/runs panes.
	detailsHeaderHeight := outerHeaderHeight
	details := m.renderActionsRunDetailsPane(
		section.SelectedRun(),
		thirdWidth,
		height,
		detailsHeaderHeight,
	)
	panes := lipgloss.JoinHorizontal(lipgloss.Top, workflows, runs, details)
	return panes
}

// actionsPaneWidths splits the total width across the three Actions panes
// (workflows, runs, details). Returns widths in that order. Workflows and
// runs share a narrow left strip (~22% each); details takes the rest so the
// embedded actionview has plenty of room for its sub-panes. Each pane has a
// minimum floor so the layout doesn't collapse on narrow terminals.
func actionsPaneWidths(total int) (workflows, runs, details int) {
	if total <= 0 {
		return 0, 0, 0
	}
	const (
		minWorkflows = 20
		minRuns      = 20
		minDetails   = 30
	)

	// Each of workflows and runs takes ~22% of the width; details gets the
	// remaining ~56%.
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
		// Steal from workflows/runs proportionally to keep details usable.
		short := minDetails - details
		fromW := short / 2
		fromR := short - fromW
		if workflows-fromW < minWorkflows {
			fromW = max(0, workflows-minWorkflows)
		}
		if runs-fromR < minRuns {
			fromR = max(0, runs-minRuns)
		}
		workflows -= fromW
		runs -= fromR
		details = max(0, total-workflows-runs)
	}
	return workflows, runs, details
}

func (m *Model) renderActionsRunDetailsPane(run *data.WorkflowRun, width, height, headerHeight int) string {
	header := lipgloss.NewStyle().
		Width(width).
		MaxWidth(width).
		Bold(true).
		Foreground(m.ctx.Theme.PrimaryText).
		Padding(0, 1).
		Render("Details")

	bodyHeight := max(0, height-headerHeight)
	bodyStyle := lipgloss.NewStyle().
		Width(width).
		MaxWidth(width).
		Height(bodyHeight).
		MaxHeight(bodyHeight).
		Padding(0, 1)

	if run == nil {
		body := bodyStyle.Align(lipgloss.Center).Render(
			lipgloss.PlaceVertical(bodyHeight, lipgloss.Center, "No run selected"),
		)
		return lipgloss.JoinVertical(lipgloss.Left, header, body)
	}

	var content string
	if m.actionRunView != nil {
		contentWidth := max(0, width-2)
		m.actionRunView.SetSize(contentWidth, bodyHeight)
		content = m.actionRunView.EmbeddedView()
	} else {
		content = m.renderActionsRunPreview(run)
	}
	body := bodyStyle.Render(content)
	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}

func completionLayerY(previewTop int, previewView string, inputLineFromBottom int, completions string) int {
	return max(
		0,
		previewTop+lipgloss.Height(previewView)-common.InputBoxHeight-inputLineFromBottom-lipgloss.Height(completions),
	)
}

func (m *Model) isPreviewFocused() bool {
	return m.activePane == previewPane && m.sidebar.IsOpen
}

// prViewIsActive reports whether the prView is the active surface for the
// current view. prView state (current sidebar tab, embedded actionview
// focus, inline editor active flag, etc.) must only influence key routing
// when this returns true; otherwise stale prView state from a previously-
// visited PR can hijack keystrokes meant for the current view, e.g. when
// the dashboard Actions view is foreground but the prView's last tab was
// Checks. The notifications view is included because a PR notification
// reuses prView as its detail surface.
func (m *Model) prViewIsActive() bool {
	if m.ctx == nil {
		return false
	}
	switch m.ctx.View {
	case config.PRsView:
		return true
	case config.NotificationsView:
		return m.notificationView.GetSubjectPR() != nil
	default:
		return false
	}
}

// issueSidebarIsActive is the symmetrical predicate for the issue sidebar.
// Only IssuesView (or NotificationsView with an Issue subject) should let
// the issue sidebar's input-focus state influence key routing.
func (m *Model) issueSidebarIsActive() bool {
	if m.ctx == nil {
		return false
	}
	switch m.ctx.View {
	case config.IssuesView:
		return true
	case config.NotificationsView:
		return m.notificationView.GetSubjectIssue() != nil
	default:
		return false
	}
}

func (m *Model) setActivePane(pane activePane) {
	m.activePane = pane
	if m.ctx == nil {
		m.syncPreviewFocus()
		return
	}
	if pane == previewPane && m.sidebar.IsOpen {
		m.ctx.ActivePane = "preview"
		m.syncPreviewFocus()
		return
	}
	m.activePane = mainPane
	m.ctx.ActivePane = "main"
	m.syncPreviewFocus()
}

// syncPreviewFocus pushes the current "is the preview pane focused?"
// state into prView so it can gate key-message forwarding into the
// embedded Checks-tab actionview. Must be called whenever m.activePane
// or m.sidebar.IsOpen changes; the predicate matches isPreviewFocused.
func (m *Model) syncPreviewFocus() {
	m.prView.SetPreviewFocused(m.activePane == previewPane && m.sidebar.IsOpen)
}

func (m *Model) isPreviewNavigationKey(msg tea.KeyMsg) bool {
	return key.Matches(msg, m.keys.Up) || key.Matches(msg, m.keys.Down) ||
		m.isPageUpKey(msg) || m.isPageDownKey(msg) ||
		key.Matches(msg, m.keys.FirstLine) || key.Matches(msg, m.keys.LastLine)
}

func (m *Model) isPageDownKey(msg tea.KeyMsg) bool {
	return key.Matches(msg, m.keys.PageDown) || msg.String() == "ctrl+down"
}

func (m *Model) isPageUpKey(msg tea.KeyMsg) bool {
	return key.Matches(msg, m.keys.PageUp) || msg.String() == "ctrl+up"
}

func (m *Model) mainPageSize() int {
	if m.ctx == nil {
		return 1
	}
	return max(1, m.ctx.MainContentHeight/2)
}

func (m *Model) isPreviewTabKey(msg tea.KeyMsg) bool {
	return key.Matches(msg, keys.PRKeys.PrevSidebarTab) || key.Matches(msg, keys.PRKeys.NextSidebarTab)
}

type initMsg struct {
	Config  config.Config
	RepoUrl string
}

// Message types for notification subject fetching
type notificationPRFetchedMsg struct {
	NotificationId   string
	PR               data.EnrichedPullRequestData
	LatestCommentUrl string
	Err              error
}

type notificationIssueFetchedMsg struct {
	NotificationId   string
	Issue            data.IssueData
	LatestCommentUrl string
	Err              error
}

func (m *Model) setCurrSectionId(newSectionId int) {
	m.currSectionId = newSectionId
	m.tabs.SetCurrSectionId(newSectionId)
	// Mirror to per-view state so future view switches restore the same
	// section.
	if m.ctx != nil {
		m.ensureViewState(m.ctx.View).currSectionId = newSectionId
	}
}

// ensureViewState returns the persistent UI state for a view, creating it
// lazily on first access. For Actions view the entry's sidebarOpen is
// pinned to false since the global sidebar is not used by that view's
// three-pane layout.
func (m *Model) ensureViewState(v config.ViewType) *viewState {
	if m.viewStates == nil {
		m.viewStates = map[config.ViewType]*viewState{}
	}
	s, ok := m.viewStates[v]
	if !ok {
		s = &viewState{
			activePane: mainPane,
		}
		switch v {
		case config.ActionsView:
			s.sidebarOpen = false
			s.currSectionId = 0
		case config.NotificationsView, config.PRsView, config.IssuesView:
			s.currSectionId = 1
			if m.ctx != nil && m.ctx.Config != nil {
				s.sidebarOpen = m.ctx.Config.Defaults.Preview.Open
			}
		default:
			s.currSectionId = 1
		}
		m.viewStates[v] = s
	}
	return s
}

// captureCurrentViewState writes the live Model state for the current view
// back into viewStates so that a subsequent view switch can restore it.
func (m *Model) captureCurrentViewState() {
	if m.ctx == nil {
		return
	}
	s := m.ensureViewState(m.ctx.View)
	// Actions view's sidebarOpen is pinned to false - the global sidebar is
	// not used there. Capturing IsOpen during Actions would clobber the
	// pinned state when the user pressed `p` mid-Actions (no-op'd below).
	if m.ctx.View != config.ActionsView {
		s.sidebarOpen = m.sidebar.IsOpen
	}
	s.currSectionId = m.currSectionId
	s.activePane = m.activePane
}

// applyViewState writes the persisted state for the current view onto the
// live Model fields. The caller is responsible for invoking
// syncMainContentDimensions afterward so that downstream contexts mirror the
// new sidebar state.
func (m *Model) applyViewState() {
	if m.ctx == nil {
		return
	}
	s := m.ensureViewState(m.ctx.View)
	m.sidebar.IsOpen = s.sidebarOpen
	m.currSectionId = s.currSectionId
	m.tabs.SetCurrSectionId(s.currSectionId)
	// activePane is only meaningful when the sidebar is open; clamp to
	// mainPane otherwise so isPreviewFocused() stays consistent.
	if s.sidebarOpen {
		m.activePane = s.activePane
	} else {
		m.activePane = mainPane
	}
	m.syncPreviewFocus()
}

func (m *Model) updateNotificationSections(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	for i := range m.notifications {
		if m.notifications[i] != nil {
			var cmd tea.Cmd
			m.notifications[i], cmd = m.notifications[i].Update(msg)
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

func (m *Model) markNotificationAsRead(notificationId string) {
	readStateMsg := notificationssection.UpdateNotificationReadStateMsg{
		Id:     notificationId,
		Unread: false,
	}
	m.updateNotificationSections(readStateMsg)
}

func (m *Model) onViewedRowChanged() tea.Cmd {
	if m.ctx.View == config.ActionsView {
		return m.syncActionsSelection()
	}
	// Save scroll state for whatever row is currently shown in the
	// sidebar before we re-render it for the newly selected row.
	m.saveCurrentPRPreviewState()
	m.saveCurrentIssuePreviewState()
	m.prView.SetSummaryViewLess()
	sidebarCmd := m.syncSidebar()
	restored := m.restoreCurrentPRPreviewState() || m.restoreCurrentIssuePreviewState()
	enrichCmd := m.prView.EnrichCurrRow()
	if !restored {
		m.sidebar.ScrollToTop()
	}
	m.notificationView.ResetSubject()
	keys.SetNotificationSubject(keys.NotificationSubjectNone)
	return tea.Batch(sidebarCmd, enrichCmd, m.prView.ActivateChecks(), m.reconcileVisibleRefreshes())
}

// handleActionsNavigation routes navigation keys (Up/Down/PgUp/PgDn/g/G) to
// the currently-focused pane of the Actions three-pane layout. Returns
// (handled, cmd) where handled=true short-circuits the universal navigation
// handlers in the main Update loop.
func (m *Model) handleActionsNavigation(msg tea.KeyMsg, currSection section.Section) (bool, tea.Cmd) {
	as, ok := currSection.(*actionssection.Model)
	if !ok || as == nil {
		return false, nil
	}

	isNav := key.Matches(msg, m.keys.Up) ||
		key.Matches(msg, m.keys.Down) ||
		key.Matches(msg, m.keys.FirstLine) ||
		key.Matches(msg, m.keys.LastLine) ||
		m.isPageUpKey(msg) ||
		m.isPageDownKey(msg)
	if !isNav {
		return false, nil
	}

	switch as.FocusedPane() {
	case actionssection.PaneWorkflows:
		// Fall through to the universal handlers which already navigate the
		// workflow table via currSection.NextRow/PrevRow/etc.
		return false, nil
	case actionssection.PaneRuns:
		switch {
		case key.Matches(msg, m.keys.Up):
			as.PrevRun()
		case key.Matches(msg, m.keys.Down):
			as.NextRun()
		case key.Matches(msg, m.keys.FirstLine):
			as.RunsTable.FirstItem()
		case key.Matches(msg, m.keys.LastLine):
			as.RunsTable.LastItem()
		case m.isPageUpKey(msg):
			as.RunsTable.PageUp()
		case m.isPageDownKey(msg):
			as.RunsTable.PageDown()
		}
		return true, m.onViewedRowChanged()
	case actionssection.PaneDetails:
		if m.actionRunView == nil {
			return true, nil
		}
		// Forward the navigation key to the embedded actionview even when
		// its logs search isn't focused. We bypass UpdateEmbedded's
		// "logs-search-only" guard by calling Update directly.
		view, actionCmd := m.actionRunView.Update(msg)
		m.actionRunView = &view
		return true, actionCmd
	}
	return false, nil
}

func (m *Model) syncActionsSelection() tea.Cmd {
	if m.ctx == nil || m.ctx.View != config.ActionsView {
		return nil
	}
	section, ok := m.getCurrSection().(*actionssection.Model)
	if !ok || section == nil {
		return nil
	}
	cmds := section.SyncSelectedWorkflow()
	run := section.SelectedRun()
	if run == nil {
		m.actionRunView = nil
		m.actionRunViewKey = ""
		return tea.Batch(cmds...)
	}
	// Use the same width math as renderActionsRunDetailsPane so the first
	// fetched data is sized correctly before the next render call resizes.
	thirdWidth := max(0, m.ctx.ScreenWidth-2*((m.ctx.ScreenWidth+2)/3))
	width := max(0, thirdWidth-2) // account for body padding
	cmds = append(cmds, m.ensureActionRunView(run, width))
	return tea.Batch(cmds...)
}

// saveCurrentPRPreviewState records the sidebar viewport offset for the
// PR currently shown in the preview pane, keyed by URL and the currently
// selected sidebar tab. This is what lets the user navigate between sidebar
// tabs (or away to another row/view) and return to the same scroll position.
func (m *Model) saveCurrentPRPreviewState() {
	m.savePRPreviewStateAt(m.prView.SelectedTabIndex())
}

// savePRPreviewStateAt records the sidebar viewport offset for the PR
// currently shown, under the supplied tab index. Use this when saving the
// scroll for an *outgoing* tab (i.e. before the carousel moves to a new tab).
func (m *Model) savePRPreviewStateAt(tabIndex int) {
	url := m.prView.CurrentPRURL()
	if url == "" {
		return
	}
	if m.prPreviewStates == nil {
		m.prPreviewStates = map[string]map[int]int{}
	}
	tabs := m.prPreviewStates[url]
	if tabs == nil {
		tabs = map[int]int{}
		m.prPreviewStates[url] = tabs
	}
	tabs[tabIndex] = m.sidebar.YOffset()
}

// restoreCurrentPRPreviewState restores the preview state for the currently
// selected row. For PR rows it looks up the saved scroll for the saved tab,
// honoring the per-tab map. Returns true if any restoration happened.
func (m *Model) restoreCurrentPRPreviewState() bool {
	pr, ok := m.getCurrRowData().(*prrow.Data)
	if !ok || pr == nil || pr.Primary == nil {
		return false
	}

	url := pr.Primary.Url
	if url == "" {
		return false
	}
	if tabs, ok := m.prPreviewStates[url]; ok && len(tabs) > 0 {
		// Pick the most recently-recorded tab as the "last viewed" tab
		// for this PR. We don't track recency explicitly; preferring the
		// already-selected tab if it has saved state, then falling back
		// to any saved tab, is a faithful approximation.
		selected := m.prView.SelectedTabIndex()
		var tabIdx int
		var off int
		if savedOff, hasSelected := tabs[selected]; hasSelected {
			tabIdx, off = selected, savedOff
		} else {
			// Deterministic-ish: pick the lowest tab index that has saved
			// state. The set is small (5 tabs max), so this is fine.
			best := -1
			for k := range tabs {
				if best == -1 || k < best {
					best = k
				}
			}
			tabIdx, off = best, tabs[best]
		}
		m.prView.GoToTab(tabIdx)
		m.syncSidebar()
		m.sidebar.ScrollToOffset(off)
		return true
	}
	m.prView.GoToFirstTab()
	m.syncSidebar()
	return false
}

// restoreCurrentPRPreviewTab restores the saved scroll for the *currently*
// selected sidebar tab (i.e. after the carousel has moved). Unlike
// restoreCurrentPRPreviewState it does not switch tabs - it only applies a
// scroll offset if one was recorded for this PR's current tab.
func (m *Model) restoreCurrentPRPreviewTab() bool {
	url := m.prView.CurrentPRURL()
	if url == "" {
		return false
	}
	tabs, ok := m.prPreviewStates[url]
	if !ok {
		return false
	}
	off, ok := tabs[m.prView.SelectedTabIndex()]
	if !ok {
		return false
	}
	m.sidebar.ScrollToOffset(off)
	return true
}

// scrollActivityToBottomIfNoSavedOffset is invoked once after PR
// enrichment completes. If the user has previously interacted with the
// Activity tab for this PR, their saved offset wins; otherwise the
// default is "show the most recent activity" - i.e. scroll to bottom.
// This is *not* called on plain tab navigation; that path uses
// restoreCurrentPRPreviewTab so revisiting Activity preserves the user's
// scroll position even if they hadn't yet scrolled it themselves.
func (m *Model) scrollActivityToBottomIfNoSavedOffset() {
	if m.restoreCurrentPRPreviewTab() {
		return
	}
	m.sidebar.ScrollToBottom()
}

// saveCurrentIssuePreviewState records the sidebar viewport offset for the
// Issue currently shown in the preview pane, keyed by URL.
func (m *Model) saveCurrentIssuePreviewState() {
	issue, ok := m.getCurrRowData().(*data.IssueData)
	if !ok || issue == nil {
		return
	}
	url := issue.Url
	if url == "" {
		return
	}
	if m.issuePreviewStates == nil {
		m.issuePreviewStates = map[string]int{}
	}
	m.issuePreviewStates[url] = m.sidebar.YOffset()
}

// restoreCurrentIssuePreviewState applies any saved scroll offset for the
// currently selected Issue row. Returns true if restoration happened.
func (m *Model) restoreCurrentIssuePreviewState() bool {
	issue, ok := m.getCurrRowData().(*data.IssueData)
	if !ok || issue == nil {
		return false
	}
	url := issue.Url
	if url == "" {
		return false
	}
	off, ok := m.issuePreviewStates[url]
	if !ok {
		return false
	}
	m.sidebar.ScrollToOffset(off)
	return true
}

func (m *Model) onWindowSizeChanged(msg tea.WindowSizeMsg) {
	log.Info("window size changed", "width", msg.Width, "height", msg.Height)
	m.footer.SetWidth(msg.Width)
	m.ctx.ScreenWidth = msg.Width
	m.ctx.ScreenHeight = msg.Height
	if m.ctx.Config != nil {
		if m.ctx.Config.Defaults.Preview.Position == "auto" ||
			m.ctx.Config.Defaults.Preview.Position == "" {
			m.positionOverride = ""
		}
		m.syncMainContentDimensions()
		m.syncSidebar()
	}
}

func (m *Model) syncProgramContext() {
	for _, section := range m.getCurrentViewSections() {
		section.UpdateProgramContext(m.ctx)
	}
	m.tabs.UpdateProgramContext(m.ctx)
	m.footer.UpdateProgramContext(m.ctx)
	m.sidebar.UpdateProgramContext(m.ctx)
	m.prView.UpdateProgramContext(m.ctx)
	m.issueSidebar.UpdateProgramContext(m.ctx)
	m.notificationView.UpdateProgramContext(m.ctx)
	// Keep prView's preview-focus flag in sync with the current pane
	// state. This covers any code path that mutates m.activePane or
	// m.sidebar.IsOpen without going through setActivePane / the
	// sidebar helpers (e.g. tests that build Model literals directly).
	m.syncPreviewFocus()
}

func (m *Model) updateSection(id int, sType string, msg tea.Msg) (cmd tea.Cmd) {
	var updatedSection section.Section
	switch sType {
	case notificationssection.SectionType:
		if id < len(m.notifications) && m.notifications[id] != nil {
			m.notifications[id], cmd = m.notifications[id].Update(msg)
		}

	case prssection.SectionType:
		updatedSection, cmd = m.prs[id].Update(msg)
		m.prs[id] = updatedSection
	case issuessection.SectionType:
		updatedSection, cmd = m.issues[id].Update(msg)
		m.issues[id] = updatedSection
	case actionssection.SectionType:
		updatedSection, cmd = m.actions[id].Update(msg)
		m.actions[id] = updatedSection
	}

	currSection := m.getCurrSection()
	if currSection != nil && id == currSection.GetId() {
		switch msg.(type) {
		case prssection.SectionPullRequestsFetchedMsg, prssection.SectionPullRequestsRefreshedMsg:
			cmd = m.onViewedRowChanged()
		}
	}

	return cmd
}

func (m *Model) updateRelevantSection(msg section.SectionMsg) (cmd tea.Cmd) {
	return m.updateSection(msg.Id, msg.Type, msg)
}

func (m *Model) updateCurrentSection(msg tea.Msg) (cmd tea.Cmd) {
	section := m.getCurrSection()
	if section == nil {
		return nil
	}
	return m.updateSection(section.GetId(), section.GetType(), msg)
}

const minTableWidthForRightPreview = 80

func (m *Model) resolvePreviewPosition() string {
	pos := m.ctx.Config.Defaults.Preview.Position
	if pos == "" {
		pos = "auto"
	}

	if m.positionOverride != "" {
		return m.positionOverride
	}

	if pos == "right" || pos == "bottom" {
		return pos
	}

	// auto: check if right mode would leave enough room for the main content
	w := m.ctx.Config.Defaults.Preview.Width
	if w > 0 && w < 1 {
		w *= float64(m.ctx.ScreenWidth)
	}
	previewWidth := min(int(w), m.ctx.ScreenWidth)
	tableWidth := m.ctx.ScreenWidth - previewWidth
	if tableWidth < minTableWidthForRightPreview {
		return "bottom"
	}
	return "right"
}

func (m *Model) getBaseContentHeight() int {
	if m.footer.ShowAll {
		// Measure actual footer height — the ExpandedHelpHeight constant
		// doesn't account for custom keybindings or view-specific bindings.
		footerHeight := lipgloss.Height(m.footer.View())
		return m.ctx.ScreenHeight - common.TabsHeight - footerHeight
	}
	return m.ctx.ScreenHeight - common.TabsHeight - common.FooterHeight
}

func (m *Model) syncMainContentDimensions() {
	m.ctx.PreviewPosition = m.resolvePreviewPosition()

	if !m.sidebar.IsOpen {
		m.setActivePane(mainPane)
		m.ctx.MainContentWidth = m.ctx.ScreenWidth
		m.ctx.MainContentHeight = m.getBaseContentHeight()
		m.ctx.DynamicPreviewWidth = 0
		m.ctx.DynamicPreviewHeight = 0
		m.ctx.SidebarOpen = false
		return
	}

	m.ctx.SidebarOpen = true

	if m.ctx.PreviewPosition == "bottom" {
		m.ctx.MainContentWidth = m.ctx.ScreenWidth

		// Subtract border height: lipgloss Height() sets content height,
		// and BorderTop adds an extra row outside of that.
		availableHeight := m.getBaseContentHeight() - m.ctx.Styles.Sidebar.BorderWidth
		h := m.ctx.Config.Defaults.Preview.Height
		if h > 0 && h < 1 {
			h *= float64(availableHeight)
		}
		m.ctx.DynamicPreviewHeight = min(int(h), availableHeight)
		m.ctx.MainContentHeight = availableHeight - m.ctx.DynamicPreviewHeight
		m.ctx.DynamicPreviewWidth = m.ctx.ScreenWidth
	} else {
		m.ctx.MainContentHeight = m.getBaseContentHeight()

		w := m.ctx.Config.Defaults.Preview.Width
		if w > 0 && w < 1 {
			w *= float64(m.ctx.ScreenWidth)
		}
		m.ctx.DynamicPreviewWidth = min(int(w), m.ctx.ScreenWidth)
		m.ctx.MainContentWidth = m.ctx.ScreenWidth - m.ctx.DynamicPreviewWidth
		m.ctx.DynamicPreviewHeight = 0
	}
}

func (m *Model) cyclePreview() tea.Cmd {
	// The Actions view does not use the global sidebar - its three-pane
	// layout renders independently of m.sidebar. Toggling here would
	// silently mutate the per-view preference of the other views (since
	// the cycle is global) without producing any visible effect in
	// Actions, so we treat `p` as a no-op while Actions is focused.
	if m.ctx != nil && m.ctx.View == config.ActionsView {
		return nil
	}

	if !m.sidebar.IsOpen {
		m.sidebar.IsOpen = true
		m.positionOverride = "right"
		m.mirrorSidebarOpenToCurrentView()
		m.syncPreviewFocus()
		m.syncMainContentDimensions()
		m.syncProgramContext()
		return tea.Batch(m.syncSidebar(), m.reconcileVisibleRefreshes())
	}

	if m.ctx.PreviewPosition == "right" {
		m.positionOverride = "bottom"
		m.syncMainContentDimensions()
		m.syncProgramContext()
		return tea.Batch(m.syncSidebar(), m.reconcileVisibleRefreshes())
	}

	m.sidebar.IsOpen = false
	m.positionOverride = ""
	m.mirrorSidebarOpenToCurrentView()
	m.syncPreviewFocus()
	m.syncMainContentDimensions()
	m.syncProgramContext()
	return m.reconcileVisibleRefreshes()
}

// mirrorSidebarOpenToCurrentView writes m.sidebar.IsOpen into the persisted
// state for the current view so that future view switches restore the same
// preference. Actions view's entry stays pinned to false; sidebar mutations
// during Actions cyclePreview are already short-circuited above.
func (m *Model) mirrorSidebarOpenToCurrentView() {
	if m.ctx == nil || m.ctx.View == config.ActionsView {
		return
	}
	m.ensureViewState(m.ctx.View).sidebarOpen = m.sidebar.IsOpen
}

func (m *Model) openSidebarForPRInput(setFunc func(bool) tea.Cmd) tea.Cmd {
	m.prView.GoToFirstTab()
	return m.openSidebarForInput(setFunc)
}

func (m *Model) openSidebarForInput(setFunc func(bool) tea.Cmd) tea.Cmd {
	m.sidebar.IsOpen = true
	m.mirrorSidebarOpenToCurrentView()
	m.syncPreviewFocus()
	cmd := setFunc(true)
	m.syncMainContentDimensions()
	m.syncSidebar()
	m.sidebar.ScrollToBottom()
	return cmd
}

func (m *Model) openSidebarForInputNoScroll(setFunc func(bool) tea.Cmd) tea.Cmd {
	m.sidebar.IsOpen = true
	m.mirrorSidebarOpenToCurrentView()
	m.syncPreviewFocus()
	cmd := setFunc(true)
	m.syncMainContentDimensions()
	m.syncSidebar()
	return cmd
}

func (m *Model) backToNotification() tea.Cmd {
	if m.notificationView.GetSubjectPR() == nil && m.notificationView.GetSubjectIssue() == nil {
		return nil
	}

	m.notificationView.ClearSubject()
	keys.SetNotificationSubject(keys.NotificationSubjectNone)
	keys.SetPRPreviewContext(keys.PRPreviewContextNone)
	m.sidebar.ScrollToTop()
	return m.syncSidebar()
}

func (m *Model) promptConfirmation(currSection section.Section, action string) tea.Cmd {
	if currSection != nil {
		currSection.SetPromptConfirmationAction(action)
		return currSection.SetIsPromptConfirmationShown(true)
	}
	return nil
}

func (m *Model) openPRURLInSearchSection(ref githubPRRef) []tea.Cmd {
	if len(m.prs) == 0 || m.prs[0] == nil {
		return []tea.Cmd{m.notifyErr("PR search section is unavailable")}
	}

	searchSection, ok := m.prs[0].(*prssection.Model)
	if !ok || searchSection == nil {
		return []tea.Cmd{m.notifyErr("PR search section is unavailable")}
	}

	query := ref.searchQuery()
	searchSection.SearchValue = query
	searchSection.SearchBar.SetValue(query)
	searchSection.SyncSmartFilterWithSearchValue()
	searchSection.SetIsSearching(false)
	searchSection.SetIsLocalSearching(false)
	searchSection.ResetRows()
	searchSection.SetIsLoading(true)
	m.setCurrSectionId(0)
	m.syncSidebar()

	return []tea.Cmd{func() tea.Msg {
		pr, err := fetchPullRequestByNumberForOpenURL(ref.Owner, ref.Repo, ref.Number)
		if err != nil {
			return openPRURLFetchedMsg{Ref: ref, Err: err}
		}
		return openPRURLFetchedMsg{Ref: ref, PR: pr}
	}}
}

func (m *Model) applyOpenPRURLFetchedMsg(msg openPRURLFetchedMsg) tea.Cmd {
	if len(m.prs) == 0 || m.prs[0] == nil {
		return m.notifyErr("PR search section is unavailable")
	}

	searchSection, ok := m.prs[0].(*prssection.Model)
	if !ok || searchSection == nil {
		return m.notifyErr("PR search section is unavailable")
	}

	searchSection.SetIsLoading(false)
	if msg.Err != nil {
		return m.notifyErr(fmt.Sprintf("Failed opening PR URL: %v", msg.Err))
	}

	searchSection.Prs = []prrow.Data{{Primary: &msg.PR}}
	searchSection.TotalCount = 1
	searchSection.PageInfo = &data.PageInfo{HasNextPage: false}
	searchSection.Table.SetRows(searchSection.BuildRows())
	searchSection.Table.UpdateLastUpdated(time.Now())
	searchSection.UpdateTotalItemsCount(1)
	return m.onViewedRowChanged()
}

func prOpenCloseAction(row any) string {
	switch pr := row.(type) {
	case *prrow.Data:
		if pr == nil || pr.Primary == nil {
			return ""
		}
		return openCloseAction(pr.Primary.State)
	case prrow.Data:
		if pr.Primary == nil {
			return ""
		}
		return openCloseAction(pr.Primary.State)
	case *data.PullRequestData:
		if pr == nil {
			return ""
		}
		return openCloseAction(pr.State)
	case data.PullRequestData:
		return openCloseAction(pr.State)
	}
	return ""
}

func issueOpenCloseAction(row any) string {
	switch issue := row.(type) {
	case *data.IssueData:
		if issue == nil {
			return ""
		}
		return openCloseAction(issue.State)
	case data.IssueData:
		return openCloseAction(issue.State)
	}
	return ""
}

func openCloseAction(state string) string {
	switch state {
	case "OPEN":
		return "close"
	case "CLOSED":
		return "reopen"
	}
	return ""
}

func (m *Model) renderActionsRunPreview(run *data.WorkflowRun) string {
	if run == nil {
		return ""
	}
	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.ctx.Styles.Common.MainTextStyle.Render(run.GetTitle()),
		"",
		fmt.Sprintf("Repository: %s", run.GetRepoNameWithOwner()),
		fmt.Sprintf("Workflow:   %s", run.Name),
		fmt.Sprintf("Branch:     %s", run.HeadBranch),
		fmt.Sprintf("Event:      %s", run.Event),
		fmt.Sprintf("Actor:      %s", run.Actor.Login),
		fmt.Sprintf("Status:     %s", actionsRunStatus(run)),
		fmt.Sprintf("Updated:    %s", run.UpdatedAt.Format(time.RFC1123)),
		"",
		m.ctx.Styles.Common.FaintTextStyle.Render("Press o to open this workflow run on GitHub."),
	)
}

func actionsRunStatus(run *data.WorkflowRun) string {
	if run.Conclusion != "" {
		return run.Conclusion
	}
	return run.Status
}

func (m *Model) ensureActionRunView(run *data.WorkflowRun, width int) tea.Cmd {
	if run == nil || run.GetRepoNameWithOwner() == "" || run.Id == 0 {
		// Stash whatever was active before clearing so we can resurrect it
		// if the user navigates back to the same run.
		m.cacheActionRunView()
		m.actionRunView = nil
		m.actionRunViewKey = ""
		return nil
	}
	key := fmt.Sprintf("%s:%d", run.GetRepoNameWithOwner(), run.Id)
	if m.actionRunView != nil && m.actionRunViewKey == key {
		m.actionRunView.SetSize(width, m.ctx.MainContentHeight)
		return nil
	}
	// Switching to a different run within the same session: keep the
	// outgoing view around so the user can come back to it without losing
	// scroll/selection state.
	m.cacheActionRunView()
	if cached, ok := m.actionRunViewCache[key]; ok && cached != nil {
		delete(m.actionRunViewCache, key)
		cached.SetSize(width, m.ctx.MainContentHeight)
		m.actionRunView = cached
		m.actionRunViewKey = key
		return nil
	}
	view := actionview.NewModel(run.GetRepoNameWithOwner(), "", actionview.ModelOpts{
		RunID:    strconv.FormatInt(run.Id, 10),
		Embedded: true,
		Theme:    &m.ctx.Theme,
	})
	view.SetSize(width, m.ctx.MainContentHeight)
	m.actionRunView = &view
	m.actionRunViewKey = key
	return m.actionRunView.Init()
}

func (m *Model) shouldUpdateActionRunView(msg tea.Msg) bool {
	if m.ctx == nil || m.ctx.View != config.ActionsView || m.actionRunView == nil {
		return false
	}
	// Defensive: key messages must always reach the parent's main Update so
	// that quit, navigation, and section keybindings are never swallowed by
	// the embedded actionview. HandlesAsyncMsg already excludes key messages,
	// but guard explicitly in case it ever changes.
	if _, ok := msg.(tea.KeyMsg); ok {
		return false
	}
	return actionview.HandlesAsyncMsg(msg)
}

func (m Model) actionsLogsCopySelectionContent() (string, bool) {
	if m.ctx == nil || m.ctx.View != config.ActionsView || m.actionRunView == nil {
		return "", false
	}
	return m.actionRunView.LogsCopySelectionContent(), true
}

func (m *Model) syncSidebar() tea.Cmd {
	if !m.sidebar.IsOpen {
		return nil
	}

	currRowData := m.getCurrRowData()
	width := m.sidebar.GetSidebarContentWidth()
	var cmd tea.Cmd

	if currRowData == nil {
		keys.SetPRPreviewContext(keys.PRPreviewContextNone)
		m.sidebar.ClearHeader()
		m.sidebar.SetContent("")
		return nil
	}

	switch row := currRowData.(type) {
	case *prrow.Data:
		sectionType := prssection.SectionType
		if currSection := m.getCurrSection(); currSection != nil {
			sectionType = currSection.GetType()
		}
		m.prView.SetSection(m.currSectionId, sectionType)
		m.prView.SetRow(row)
		m.prView.SetWidth(width)
		cmd = m.prView.ActivateChecks()
		m.setPRSidebarContent()
	case *data.IssueData:
		keys.SetPRPreviewContext(keys.PRPreviewContextNone)
		m.issueSidebar.SetSectionId(m.currSectionId)
		m.issueSidebar.SetRow(row)
		m.issueSidebar.SetWidth(width)
		m.sidebar.ClearHeader()
		m.sidebar.SetContent(m.issueSidebar.View())
		// Scroll to bottom if in input mode to keep inputbox visible
		if m.issueSidebar.IsTextInputBoxFocused() {
			m.sidebar.ScrollToBottom()
		}
	case *data.WorkflowRun:
		keys.SetPRPreviewContext(keys.PRPreviewContextNone)
		cmd = m.ensureActionRunView(row, width)
		m.sidebar.ClearHeader()
		if m.actionRunView == nil {
			m.sidebar.SetContent(m.renderActionsRunPreview(row))
		} else {
			m.actionRunView.SetSize(width, m.ctx.MainContentHeight)
			m.sidebar.SetContent(m.actionRunView.EmbeddedView())
		}
	case *data.Workflow:
		keys.SetPRPreviewContext(keys.PRPreviewContextNone)
		// Stash any active run-detail view so navigating back to the same
		// run later restores the user's logs scroll / selection.
		m.cacheActionRunView()
		m.actionRunView = nil
		m.actionRunViewKey = ""
		m.sidebar.ClearHeader()
		m.sidebar.SetContent(m.renderActionsWorkflowPreview(row))
	case *notificationrow.Data:
		notifId := row.GetId()

		// Check if we already have cached data for this notification (user already viewed it)
		if m.notificationView.GetSubjectId() == notifId {
			// Use cached data
			if m.notificationView.GetSubjectPR() != nil {
				m.prView.SetSection(m.currSectionId, notificationssection.SectionType)
				m.prView.SetRow(m.notificationView.GetSubjectPR())
				m.prView.SetWidth(width)
				cmd = m.prView.ActivateChecks()
				m.setPRSidebarContent()
			} else if m.notificationView.GetSubjectIssue() != nil {
				m.issueSidebar.SetSectionId(0)
				m.issueSidebar.SetRow(m.notificationView.GetSubjectIssue())
				m.issueSidebar.SetWidth(width)
				m.sidebar.ClearHeader()
				m.sidebar.SetContent(m.issueSidebar.View())
				// Scroll to bottom if in input mode to keep inputbox visible
				if m.issueSidebar.IsTextInputBoxFocused() {
					m.sidebar.ScrollToBottom()
				}
			}
			return nil
		}

		// Clear cached subject when navigating to a different notification
		// so key dispatch doesn't route keys to the wrong subject's handler.
		m.notificationView.ClearSubject()
		keys.SetNotificationSubject(keys.NotificationSubjectNone)
		keys.SetPRPreviewContext(keys.PRPreviewContextNone)
		// Show prompt to view notification (don't auto-fetch)
		// User must press Enter to view content and mark as read
		m.sidebar.ClearHeader()
		m.sidebar.SetContent(m.renderNotificationPrompt(row))
	}

	return cmd
}

func (m *Model) renderActionsWorkflowPreview(workflow *data.Workflow) string {
	if workflow == nil {
		return ""
	}
	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.ctx.Styles.Common.MainTextStyle.Render(workflow.Name),
		"",
		fmt.Sprintf("Repository: %s", workflow.GetRepoNameWithOwner()),
		fmt.Sprintf("Path:       %s", workflow.Path),
		fmt.Sprintf("State:      %s", workflow.State),
		"",
		m.ctx.Styles.Common.FaintTextStyle.Render("Press enter to view runs for this workflow."),
	)
}

func addReviewRequestsForNotificationPR(
	reviewRequests, addedReviewRequests []data.ReviewRequestNode,
) []data.ReviewRequestNode {
	newReviewRequests := reviewRequests
	for _, reviewer := range addedReviewRequests {
		if !reviewRequestsContainReviewer(newReviewRequests, reviewer) {
			newReviewRequests = append(newReviewRequests, reviewer)
		}
	}

	return newReviewRequests
}

func removeReviewRequestsForNotificationPR(
	reviewRequests, removedReviewRequests []data.ReviewRequestNode,
) []data.ReviewRequestNode {
	newReviewRequests := []data.ReviewRequestNode{}
	for _, reviewer := range reviewRequests {
		if !reviewRequestsContainReviewer(removedReviewRequests, reviewer) {
			newReviewRequests = append(newReviewRequests, reviewer)
		}
	}

	return newReviewRequests
}

func reviewRequestsContainReviewer(
	reviewRequests []data.ReviewRequestNode,
	reviewer data.ReviewRequestNode,
) bool {
	for _, reviewRequest := range reviewRequests {
		if reviewRequest.GetReviewerDisplayName() == reviewer.GetReviewerDisplayName() {
			return true
		}
	}
	return false
}

func (m *Model) setPRSidebarContent() {
	m.syncPRPreviewHelpContext()
	m.sidebar.SetHeader(m.prView.HeaderView())
	m.sidebar.SetContent(m.prView.BodyView())
}

func (m *Model) syncPRPreviewHelpContext() {
	switch {
	case m.prView.IsActivityTab():
		keys.SetPRPreviewContext(keys.PRPreviewContextActivity)
	case m.prView.IsChecksTab():
		keys.SetPRPreviewContext(keys.PRPreviewContextChecks)
	default:
		keys.SetPRPreviewContext(keys.PRPreviewContextNone)
	}
}

func (m *Model) renderNotificationPrompt(row *notificationrow.Data) string {
	var content strings.Builder

	subjectType := row.GetSubjectType()
	leftMargin := "      " // Left margin for content

	// Styles
	normalText := lipgloss.NewStyle().Foreground(m.ctx.Theme.PrimaryText)
	faintText := lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText)
	// Highlighted key style for main prompt (with background)
	highlightKeyStyle := lipgloss.NewStyle().
		Foreground(m.ctx.Theme.PrimaryText).
		Background(m.ctx.Theme.FaintBorder).
		Padding(0, 1)
	// Simple key style for table (no background)
	keyStyle := lipgloss.NewStyle().
		Foreground(m.ctx.Theme.PrimaryText)
	actionStyle := lipgloss.NewStyle().Foreground(m.ctx.Theme.SuccessText)
	headerStyle := lipgloss.NewStyle().
		Foreground(m.ctx.Theme.PrimaryText).
		Bold(true)

	// Determine subject type display name and primary action
	typeName := "PR"
	enterAction := "view"
	if subjectType == "Issue" {
		typeName = "Issue"
	} else if subjectType != "PullRequest" {
		typeName = subjectType
		enterAction = "open in browser"
	}

	// Main prompt: "Press Enter to view the PR" or "Press Enter to open in browser"
	content.WriteString("\n")
	content.WriteString(leftMargin)
	content.WriteString(normalText.Render("Press "))
	content.WriteString(highlightKeyStyle.Render("Enter"))
	if enterAction == "view" {
		content.WriteString(normalText.Render(fmt.Sprintf(" to %s the %s", enterAction, typeName)))
	} else {
		content.WriteString(normalText.Render(fmt.Sprintf(" to %s", enterAction)))
	}
	content.WriteString("\n")

	// Note about marking as read
	content.WriteString(leftMargin)
	content.WriteString(faintText.Render("(Note: this will mark it as read)"))
	content.WriteString("\n")

	content.WriteString("\n")

	// Other Actions header
	content.WriteString(leftMargin)
	content.WriteString(headerStyle.Render("Other Actions"))
	content.WriteString("\n\n")

	// Key-action pairs (simple list without borders)
	actions := []struct {
		key    string
		action string
	}{
		{"D", "mark as done"},
		{"m", "mark as read"},
		{"u", "unsubscribe"},
		{"b", "toggle bookmark"},
		{"t", "toggle filtering"},
		{"S", "sort by repo"},
		{"o", "open in browser"},
	}

	keyWidth := 7 // Width for key column
	for _, a := range actions {
		content.WriteString(leftMargin)
		// Right-align the key in its column
		padding := strings.Repeat(" ", keyWidth-len(a.key))
		content.WriteString(padding)
		content.WriteString(keyStyle.Render(a.key))
		content.WriteString("  ")
		content.WriteString(actionStyle.Render(a.action))
		content.WriteString("\n")
	}

	// Add Enter and Esc at the end
	content.WriteString(leftMargin)
	padding := strings.Repeat(" ", keyWidth-len("Enter"))
	content.WriteString(padding)
	content.WriteString(keyStyle.Render("Enter"))
	content.WriteString("  ")
	content.WriteString(actionStyle.Render(enterAction))
	content.WriteString("\n")
	content.WriteString(leftMargin)
	escPadding := strings.Repeat(" ", keyWidth-len("Esc"))
	content.WriteString(escPadding)
	content.WriteString(keyStyle.Render("Esc"))
	content.WriteString("  ")
	content.WriteString(actionStyle.Render("go back"))

	return content.String()
}

// loadNotificationContent fetches and displays notification content, marking it as read
func (m *Model) loadNotificationContent() tea.Cmd {
	currRowData := m.getCurrRowData()
	row, ok := currRowData.(*notificationrow.Data)
	if !ok || row == nil {
		return nil
	}

	notifId := row.GetId()
	subjectType := row.GetSubjectType()
	subjectUrl := row.GetUrl()
	latestCommentUrl := row.GetLatestCommentUrl()

	// Show loading indicator
	width := m.sidebar.GetSidebarContentWidth()
	m.notificationView.SetRow(row)
	m.notificationView.SetWidth(width)
	m.sidebar.ClearHeader()
	m.sidebar.SetContent(m.notificationView.View())

	switch subjectType {
	case "PullRequest":
		return tea.Batch(
			func() tea.Msg {
				_ = data.MarkNotificationRead(notifId)
				return notificationssection.UpdateNotificationReadStateMsg{
					Id:     notifId,
					Unread: false,
				}
			},
			func() tea.Msg {
				pr, err := data.FetchPullRequest(subjectUrl)
				return notificationPRFetchedMsg{
					NotificationId:   notifId,
					PR:               pr,
					LatestCommentUrl: latestCommentUrl,
					Err:              err,
				}
			},
		)
	case "Issue":
		return tea.Batch(
			func() tea.Msg {
				_ = data.MarkNotificationRead(notifId)
				return notificationssection.UpdateNotificationReadStateMsg{
					Id:     notifId,
					Unread: false,
				}
			},
			func() tea.Msg {
				issue, err := data.FetchIssue(subjectUrl)
				return notificationIssueFetchedMsg{
					NotificationId:   notifId,
					Issue:            issue,
					LatestCommentUrl: latestCommentUrl,
					Err:              err,
				}
			},
		)
	default:
		// For discussions, releases, etc. - mark as read and open in browser
		// since we can't show rich content for these types
		return tea.Batch(
			func() tea.Msg {
				_ = data.MarkNotificationRead(notifId)
				return notificationssection.UpdateNotificationReadStateMsg{
					Id:     notifId,
					Unread: false,
				}
			},
			m.openBrowser(),
		)
	}
}

func (m *Model) fetchAllViewSections() ([]section.Section, tea.Cmd) {
	cmds := make([]tea.Cmd, 0)
	cmds = append(cmds, m.tabs.SetAllLoading()...)

	switch m.ctx.View {
	case config.NotificationsView:
		s, notifCmd := notificationssection.FetchAllSections(m.ctx, m.notifications)
		cmds = append(cmds, notifCmd)
		m.notifications = s
		return s, tea.Batch(cmds...)
	case config.ActionsView:
		s, actionsCmd := actionssection.FetchAllSections(m.ctx, m.actions)
		cmds = append(cmds, actionsCmd)
		m.actions = s
		return s, tea.Batch(cmds...)
	case config.PRsView:
		s, prcmds := prssection.FetchAllSections(m.ctx, m.prs)
		cmds = append(cmds, prcmds)
		return s, tea.Batch(cmds...)
	default:
		s, issuecmds := issuessection.FetchAllSections(m.ctx, m.issues)
		cmds = append(cmds, issuecmds)
		return s, tea.Batch(cmds...)
	}
}

func (m *Model) getCurrentViewSections() []section.Section {
	switch m.ctx.View {
	case config.NotificationsView:
		if len(m.notifications) == 0 {
			return []section.Section{}
		}
		return m.notifications
	case config.PRsView:
		return m.prs
	case config.ActionsView:
		return m.actions
	default:
		return m.issues
	}
}

// viewHasSearchSection reports whether the given view renders an implicit
// search section at index 0. Views without one (currently only Actions) are
// 0-indexed and require tabs to render every section like a normal tab.
func viewHasSearchSection(v config.ViewType) bool {
	return v != config.ActionsView
}

// shouldSyncActionsSelection reports whether the parent should re-sync the
// embedded actionview after handling the given message. Doing it on every
// message (including spinner ticks) is wasteful and re-creates the action
// run view's spinners constantly.
func shouldSyncActionsSelection(msg tea.Msg) bool {
	switch msg.(type) {
	case tea.KeyMsg,
		tea.WindowSizeMsg,
		actionssection.SectionWorkflowsFetchedMsg,
		actionssection.SectionActionsFetchedMsg:
		return true
	}
	return false
}

func (m *Model) getCurrentViewDefaultSection() int {
	switch m.ctx.View {
	case config.NotificationsView:
		return 1 // First notification section after search section
	case config.PRsView:
		return 1
	case config.ActionsView:
		return 0 // Actions sections are 0-indexed; no search section
	default:
		return 1
	}
}

func (m *Model) setCurrentViewSections(newSections []section.Section) {
	if newSections == nil {
		return
	}

	// Handle notifications view with search section like PRs/Issues
	if m.ctx.View == config.NotificationsView {
		missingSearchSection := len(newSections) == 0 ||
			(len(newSections) > 0 && newSections[0].GetId() != 0)
		s := make([]section.Section, 0)
		if missingSearchSection {
			// Check if we have an existing search section to preserve
			if len(m.notifications) > 0 && m.notifications[0] != nil &&
				m.notifications[0].GetId() == 0 {
				// Preserve existing search section with its filter state
				s = append(s, m.notifications[0])
			} else {
				// Create new search section only if none exists
				search := notificationssection.NewModel(
					0,
					m.ctx,
					config.NotificationsSectionConfig{
						Title:   "",
						Filters: "archived:false",
					},
					time.Now(),
				)
				s = append(s, &search)
			}
		}
		m.notifications = append(s, newSections...)
		m.tabs.SetSections(m.notifications)
		return
	}

	missingSearchSection := len(newSections) == 0 ||
		(len(newSections) > 0 && newSections[0].GetId() != 0)
	s := make([]section.Section, 0)
	if m.ctx.View == config.PRsView {
		if missingSearchSection {
			search := prssection.NewModel(
				0,
				m.ctx,
				config.PrsSectionConfig{
					Title:   "",
					Filters: "archived:false",
				},
				time.Now(),
				time.Now(),
			)
			s = append(s, &search)
		}
		m.prs = append(s, newSections...)
		newSections = m.prs
	} else if m.ctx.View == config.ActionsView {
		// Actions view has no global search section; sections are 0-indexed
		// and each corresponds to a configured repository.
		m.actions = newSections
		newSections = m.actions
	} else {
		if missingSearchSection {
			search := issuessection.NewModel(
				0,
				m.ctx,
				config.IssuesSectionConfig{
					Title:   "",
					Filters: "",
				},
				time.Now(),
				time.Now(),
			)
			s = append(s, &search)
		}
		m.issues = append(s, newSections...)
		newSections = m.issues
	}

	m.tabs.SetSections(newSections)
}

func (m *Model) switchSelectedView() tea.Cmd {
	return m.switchSelectedViewInDirection(1)
}

func (m *Model) switchSelectedViewBack() tea.Cmd {
	return m.switchSelectedViewInDirection(-1)
}

func (m *Model) switchSelectedViewInDirection(direction int) tea.Cmd {
	views := []config.ViewType{config.PRsView}
	views = append(views, config.ActionsView, config.IssuesView, config.NotificationsView)

	// Persist the outgoing view's UI state before we mutate Model fields so
	// that returning to this view later restores the user's preferences.
	m.captureCurrentViewState()

	// Reset notification subject when leaving notifications view
	if m.ctx.View == config.NotificationsView {
		keys.SetNotificationSubject(keys.NotificationSubjectNone)
		keys.SetPRPreviewContext(keys.PRPreviewContextNone)
		m.notificationView.ClearSubject()
	}

	// Stash the embedded actionview when leaving the Actions view so the
	// user's selected job/step/log-scroll survives a round-trip through
	// another view. Cached entries' interval ticks self-terminate (the
	// parent stops routing messages to them) so memory growth is bounded
	// by actionViewCacheLimit.
	if m.ctx.View == config.ActionsView {
		m.cacheActionRunView()
		m.actionRunView = nil
		m.actionRunViewKey = ""
	}

	currIndex := 0
	for i, view := range views {
		if view == m.ctx.View {
			currIndex = i
			break
		}
	}
	nextIndex := (currIndex + direction + len(views)) % len(views)
	m.ctx.View = views[nextIndex]
	m.tabs.SetHasSearchSection(viewHasSearchSection(m.ctx.View))

	// Restore the entering view's persisted UI state. Actions view's
	// sidebarOpen entry is pinned to false, so this also handles the
	// "Actions doesn't use the global sidebar" case without mutating any
	// other view's preferences.
	m.applyViewState()

	m.syncMainContentDimensions()

	var cmds []tea.Cmd
	currSections := m.getCurrentViewSections()
	if len(currSections) == 0 {
		newSections, fetchSectionsCmds := m.fetchAllViewSections()
		currSections = newSections
		cmds = append(cmds, m.tabs.SetAllLoading()...)
		cmds = append(cmds, fetchSectionsCmds)
	}
	m.setCurrentViewSections(currSections)
	cmds = append(cmds, m.onViewedRowChanged())

	return tea.Batch(cmds...)
}

// cacheActionRunView stashes the current embedded actionview keyed by its
// run identifier so that returning to the same workflow run restores the
// user's selected job/step, log scroll, search query, and zoom state. The
// cache is capped at actionViewCacheLimit; oldest unused entries (other
// than the one being inserted) are evicted on overflow.
func (m *Model) cacheActionRunView() {
	if m.actionRunView == nil || m.actionRunViewKey == "" {
		return
	}
	if m.actionRunViewCache == nil {
		m.actionRunViewCache = map[string]*actionview.Model{}
	}
	if existing, ok := m.actionRunViewCache[m.actionRunViewKey]; ok && existing == m.actionRunView {
		// Already cached at this key (same pointer); nothing to do.
		return
	}
	m.actionRunViewCache[m.actionRunViewKey] = m.actionRunView
	if len(m.actionRunViewCache) > actionViewCacheLimit {
		// Evict an arbitrary entry other than the one we just inserted.
		for k := range m.actionRunViewCache {
			if k == m.actionRunViewKey {
				continue
			}
			delete(m.actionRunViewCache, k)
			break
		}
	}
}

func (m *Model) isUserDefinedKeybinding(msg tea.KeyMsg) bool {
	if m.ctx == nil || m.ctx.Config == nil {
		return false
	}
	for _, keybinding := range m.ctx.Config.Keybindings.Universal {
		if keybinding.Builtin == "" && keybinding.Key == msg.String() {
			return true
		}
	}

	if m.ctx.View == config.IssuesView {
		for _, keybinding := range m.ctx.Config.Keybindings.Issues {
			if keybinding.Builtin == "" && keybinding.Key == msg.String() {
				return true
			}
		}
	}

	if m.ctx.View == config.PRsView {
		for _, keybinding := range m.ctx.Config.Keybindings.Prs {
			if keybinding.Builtin == "" && keybinding.Key == msg.String() {
				return true
			}
		}
	}

	if m.ctx.View == config.NotificationsView {
		for _, keybinding := range m.ctx.Config.Keybindings.Notifications {
			if keybinding.Builtin == "" && keybinding.Key == msg.String() {
				return true
			}
		}

		currRowData := m.getCurrRowData()
		if nData, ok := currRowData.(*notificationrow.Data); ok {
			switch nData.Notification.Subject.Type {
			case "PullRequest":
				for _, keybinding := range m.ctx.Config.Keybindings.Prs {
					if keybinding.Builtin == "" && keybinding.Key == msg.String() {
						return true
					}
				}
			case "Issue":
				for _, keybinding := range m.ctx.Config.Keybindings.Issues {
					if keybinding.Builtin == "" && keybinding.Key == msg.String() {
						return true
					}
				}
			}
		}
	}

	if m.ctx.View == config.ActionsView {
		for _, keybinding := range m.ctx.Config.Keybindings.Actions {
			if keybinding.Builtin == "" && keybinding.Key == msg.String() {
				return true
			}
		}
	}

	return false
}

func (m *Model) renderRunningTask() string {
	tasks := make([]context.Task, 0, len(m.tasks))
	for _, value := range m.tasks {
		tasks = append(tasks, value)
	}
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].FinishedTime != nil && tasks[j].FinishedTime == nil {
			return false
		}
		if tasks[j].FinishedTime != nil && tasks[i].FinishedTime == nil {
			return true
		}
		if tasks[j].FinishedTime != nil && tasks[i].FinishedTime != nil {
			return tasks[i].FinishedTime.After(*tasks[j].FinishedTime)
		}

		return tasks[i].StartTime.After(tasks[j].StartTime)
	})
	task := tasks[0]

	var currTaskStatus string
	switch task.State {
	case context.TaskStart:
		currTaskStatus = lipgloss.JoinHorizontal(
			lipgloss.Top,
			m.taskSpinner.View(),
			lipgloss.NewStyle().
				Background(m.ctx.Theme.SelectedBackground).Render(task.StartText),
		)
	case context.TaskError:
		currTaskStatus = lipgloss.NewStyle().
			Foreground(m.ctx.Theme.ErrorText).
			Background(m.ctx.Theme.SelectedBackground).
			Render(fmt.Sprintf("%s %s", constants.FailureIcon, task.Error.Error()))
	case context.TaskFinished:
		currTaskStatus = lipgloss.NewStyle().
			Foreground(m.ctx.Theme.SuccessText).
			Background(m.ctx.Theme.SelectedBackground).
			Render(fmt.Sprintf("%s %s", constants.SuccessIcon, task.FinishedText))
	}

	var numProcessing int
	for _, task := range m.tasks {
		if task.State == context.TaskStart {
			numProcessing += 1
		}
	}

	stats := ""
	if numProcessing > 1 {
		stats = lipgloss.NewStyle().
			Foreground(m.ctx.Theme.FaintText).
			Background(m.ctx.Theme.SelectedBackground).
			Render(fmt.Sprintf("[ %d] ", numProcessing))
	}

	return lipgloss.NewStyle().
		Padding(0, 1).
		Height(1).
		Background(m.ctx.Theme.SelectedBackground).
		Render(strings.TrimSpace(lipgloss.JoinHorizontal(lipgloss.Top, stats, currTaskStatus)))
}

type userFetchedMsg struct {
	user string
}

func fetchUser() tea.Msg {
	user, err := data.CurrentLoginName()
	if err != nil {
		return constants.ErrMsg{
			Err: err,
		}
	}

	return userFetchedMsg{
		user: user,
	}
}

type intervalRefresh time.Time

type visibleRefreshKind string

const (
	visibleRefreshPRPreview    visibleRefreshKind = "pr-preview"
	visibleRefreshPRSection    visibleRefreshKind = "pr-section"
	visibleRefreshRepoBranches visibleRefreshKind = "repo-branches"
)

type visibleRefreshTarget struct {
	key       string
	kind      visibleRefreshKind
	url       string
	repoName  string
	sectionId int
	interval  time.Duration
}

type visibleRefreshTick struct {
	target     visibleRefreshTarget
	generation int
}

type visibleRefreshFetchedMsg struct {
	target visibleRefreshTarget
	data   tea.Msg
	err    error
}

var fetchPullRequestForPRWatch = data.FetchPullRequest

func (m *Model) shouldWatchCurrentPR() bool {
	if !m.sidebar.IsOpen {
		return false
	}

	return m.prView.CurrentPRURL() != ""
}

func (m *Model) applyPRPreviewRefresh(url string, pr data.EnrichedPullRequestData) tea.Cmd {
	if url != m.prView.CurrentPRURL() || !m.shouldWatchCurrentPR() {
		return nil
	}

	m.prView.SetEnrichedPR(pr)
	if subjectPR := m.notificationView.GetSubjectPR(); subjectPR != nil && subjectPR.Primary != nil && subjectPR.Primary.Url == url {
		prData := pr.ToPullRequestData()
		m.notificationView.SetSubjectPR(&prrow.Data{
			Primary:    &prData,
			Enriched:   pr,
			IsEnriched: true,
		}, m.notificationView.GetSubjectId())
	}
	if m.ctx != nil && m.ctx.View == config.PRsView && m.currSectionId >= 0 && m.currSectionId < len(m.prs) {
		if prSection, ok := m.prs[m.currSectionId].(*prssection.Model); ok {
			prSection.EnrichPR(pr)
		}
	}

	return m.syncSidebar()
}

func (m *Model) visibleRefreshTargets() []visibleRefreshTarget {
	var targets []visibleRefreshTarget

	if target := m.currentPRPreviewRefreshTarget(); target.key != "" {
		targets = append(targets, target)
	}

	if m.ctx != nil && m.ctx.Config != nil && m.ctx.View == config.PRsView &&
		m.ctx.Config.Defaults.RefetchIntervalMinutes > 0 {
		currSection, ok := m.getCurrSection().(*prssection.Model)
		if ok && currSection != nil && !currSection.IsSearchFocused() && !currSection.IsPromptConfirmationFocused() {
			key := fmt.Sprintf("pr-section:%d:%s", currSection.GetId(), currSection.GetFilters())
			targets = append(targets, visibleRefreshTarget{
				key:       key,
				kind:      visibleRefreshPRSection,
				sectionId: currSection.GetId(),
				interval:  time.Minute * time.Duration(m.ctx.Config.Defaults.RefetchIntervalMinutes),
			})
		}
	}

	if m.ctx != nil && m.ctx.Config != nil && m.ctx.View == config.PRsView {
		currSection, ok := m.getCurrSection().(*prssection.Model)
		if ok && currSection != nil {
			if repoName, ok := currSection.RepoFromFilters(); ok {
				interval := time.Duration(0)
				if m.ctx.Config.Defaults.RefetchIntervalMinutes > 0 {
					interval = time.Minute * time.Duration(m.ctx.Config.Defaults.RefetchIntervalMinutes)
				}
				if target := m.repoBranchesRefreshTarget(currSection.GetId(), repoName, interval); target.key != "" {
					targets = append(targets, target)
				}
			}
		}
	}

	return targets
}

func (m *Model) prPreviewRefreshInterval() time.Duration {
	if m.ctx == nil || m.ctx.Config == nil {
		return 10 * time.Second
	}
	seconds := m.ctx.Config.Defaults.PreviewRefreshIntervalSeconds
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func (m *Model) currentPRPreviewRefreshTarget() visibleRefreshTarget {
	if !m.shouldWatchCurrentPR() {
		return visibleRefreshTarget{}
	}
	interval := m.prPreviewRefreshInterval()
	if interval <= 0 {
		return visibleRefreshTarget{}
	}
	url := m.prView.CurrentPRURL()
	if url == "" {
		return visibleRefreshTarget{}
	}
	return visibleRefreshTarget{
		key:      "pr-preview:" + url,
		kind:     visibleRefreshPRPreview,
		url:      url,
		interval: interval,
	}
}

func (m *Model) fetchCurrentPRPreviewRefresh() tea.Cmd {
	target := m.currentPRPreviewRefreshTarget()
	if target.key == "" {
		return nil
	}
	return m.fetchVisibleRefresh(target)
}

func (m *Model) isVisibleRefreshTargetCurrent(target visibleRefreshTarget) bool {
	for _, current := range m.visibleRefreshTargets() {
		if current.key == target.key && current.kind == target.kind {
			return true
		}
	}
	return false
}

func (m *Model) repoBranchesRefreshTarget(sectionId int, repoName string, interval time.Duration) visibleRefreshTarget {
	if m.ctx == nil || m.ctx.Config == nil || repoName == "" {
		return visibleRefreshTarget{}
	}
	if _, ok := common.GetRepoLocalPath(repoName, m.ctx.Config.RepoPaths); !ok {
		return visibleRefreshTarget{}
	}
	return visibleRefreshTarget{
		key:       "repo-branches:" + repoName,
		kind:      visibleRefreshRepoBranches,
		repoName:  repoName,
		sectionId: sectionId,
		interval:  interval,
	}
}

func (m *Model) cachedRepoBranches(repoName string) *prssection.RepoBranches {
	if m.repoBranches == nil {
		return nil
	}
	state, ok := m.repoBranches[repoName]
	if !ok || state.loading {
		return nil
	}
	data := state.data
	return &data
}

func (m *Model) applyRepoBranchesRefresh(branches prssection.RepoBranches, sectionId int) tea.Cmd {
	if m.repoBranches == nil {
		m.repoBranches = map[string]repoBranchesState{}
	}
	m.repoBranches[branches.RepoName] = repoBranchesState{data: branches}

	if sectionId >= 0 && sectionId < len(m.prs) {
		if prSection, ok := m.prs[sectionId].(*prssection.Model); ok {
			if prSection.ApplyCreatePRBranches(branches) {
				return nil
			}
		}
	}

	if currSection, ok := m.getCurrSection().(*prssection.Model); ok {
		if currSection.ApplyCreatePRBranches(branches) {
			return nil
		}
	}

	return nil
}

func (m *Model) reconcileVisibleRefreshes() tea.Cmd {
	targets := m.visibleRefreshTargets()
	if m.visibleRefreshes == nil {
		m.visibleRefreshes = map[string]int{}
	}

	desired := map[string]visibleRefreshTarget{}
	for _, target := range targets {
		desired[target.key] = target
	}
	for key := range m.visibleRefreshes {
		if _, ok := desired[key]; !ok {
			delete(m.visibleRefreshes, key)
		}
	}

	cmds := make([]tea.Cmd, 0, len(targets))
	for _, target := range targets {
		if target.interval <= 0 {
			continue
		}
		if _, exists := m.visibleRefreshes[target.key]; exists {
			continue
		}
		m.visibleRefreshGen++
		generation := m.visibleRefreshGen
		m.visibleRefreshes[target.key] = generation
		log.Debug("scheduling visible refresh", "key", target.key, "kind", target.kind, "interval", target.interval)
		cmds = append(cmds, tea.Tick(target.interval, func(t time.Time) tea.Msg {
			return visibleRefreshTick{target: target, generation: generation}
		}))
	}

	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) fetchVisibleRefresh(target visibleRefreshTarget) tea.Cmd {
	switch target.kind {
	case visibleRefreshPRPreview:
		return func() tea.Msg {
			pr, err := fetchPullRequestForPRWatch(target.url)
			return visibleRefreshFetchedMsg{target: target, data: pr, err: err}
		}
	case visibleRefreshPRSection:
		if target.sectionId < 0 || target.sectionId >= len(m.prs) {
			return nil
		}
		prSection, ok := m.prs[target.sectionId].(*prssection.Model)
		if !ok {
			return nil
		}
		cmd := prSection.RefreshSectionRows()
		if cmd == nil {
			return nil
		}
		return func() tea.Msg {
			msg := cmd()
			if errMsg, ok := msg.(constants.ErrMsg); ok {
				return visibleRefreshFetchedMsg{target: target, err: errMsg.Err}
			}
			return visibleRefreshFetchedMsg{target: target, data: msg}
		}
	case visibleRefreshRepoBranches:
		repoPath, ok := common.GetRepoLocalPath(target.repoName, m.ctx.Config.RepoPaths)
		if !ok {
			return nil
		}
		repoPath = common.ExpandRepoPath(repoPath)
		return func() tea.Msg {
			repo, err := git.GetRepo(repoPath)
			branches := prssection.RepoBranches{RepoName: target.repoName, Err: err}
			if err == nil {
				branches = repoBranchesFromGitRepo(target.repoName, repo)
			}
			return visibleRefreshFetchedMsg{target: target, data: branches, err: err}
		}
	default:
		return nil
	}
}

func repoBranchesFromGitRepo(repoName string, repo *git.Repo) prssection.RepoBranches {
	branches := make([]fuzzyselect.Suggestion, 0, len(repo.Branches))
	base := ""
	for _, branch := range repo.Branches {
		detail := ""
		if branch.IsCheckedOut {
			detail = "current"
		}
		branches = append(branches, fuzzyselect.Suggestion{Value: branch.Name, Detail: detail})
		if base == "" && (branch.Name == "main" || branch.Name == "master") {
			base = branch.Name
		}
	}
	return prssection.RepoBranches{
		RepoName: repoName,
		Branches: branches,
		Head:     repo.HeadBranchName,
		Base:     base,
	}
}

func (m *Model) doRefreshAtInterval() tea.Cmd {
	if m.ctx.Config.Defaults.RefetchIntervalMinutes == 0 {
		return nil
	}

	return tea.Tick(
		time.Minute*time.Duration(m.ctx.Config.Defaults.RefetchIntervalMinutes),
		func(t time.Time) tea.Msg {
			return intervalRefresh(t)
		},
	)
}

type updateFooterMsg struct{}

func (m *Model) doUpdateFooterAtInterval() tea.Cmd {
	return tea.Tick(
		time.Second*10,
		func(t time.Time) tea.Msg {
			return updateFooterMsg{}
		},
	)
}

// promptConfirmationForNotificationPR shows a confirmation prompt for PR actions
// when viewing a PR from a notification. This is separate from section-based
// confirmation because the notification section doesn't know about PR actions.
func (m *Model) promptConfirmationForNotificationPR(action string) tea.Cmd {
	prompt := m.notificationView.SetPendingPRAction(action)
	if prompt == "" {
		return nil
	}
	m.footer.SetLeftSection(m.ctx.Styles.ListViewPort.PagerStyle.Render(prompt))
	return nil
}

// promptConfirmationForNotificationIssue shows a confirmation prompt for Issue actions
// when viewing an Issue from a notification.
func (m *Model) promptConfirmationForNotificationIssue(action string) tea.Cmd {
	prompt := m.notificationView.SetPendingIssueAction(action)
	if prompt == "" {
		return nil
	}
	m.footer.SetLeftSection(m.ctx.Styles.ListViewPort.PagerStyle.Render(prompt))
	return nil
}

// executeNotificationAction executes a PR/Issue action after user confirmation
func (m *Model) executeNotificationAction(action string) tea.Cmd {
	if action == "" {
		return nil
	}

	sid := tasks.SectionIdentifier{Id: m.currSectionId, Type: notificationssection.SectionType}
	pr := m.notificationView.GetSubjectPR()
	issue := m.notificationView.GetSubjectIssue()

	switch action {
	case "pr_close":
		if pr != nil {
			return tasks.ClosePR(m.ctx, sid, pr)
		}
	case "pr_reopen":
		if pr != nil {
			return tasks.ReopenPR(m.ctx, sid, pr)
		}
	case "pr_ready":
		if pr != nil {
			return tasks.TogglePRDraft(m.ctx, sid, pr)
		}
	case "pr_merge":
		if pr != nil {
			return tasks.MergePR(m.ctx, sid, pr)
		}
	case "pr_update":
		if pr != nil {
			return tasks.UpdatePR(m.ctx, sid, pr)
		}
	case "pr_approveWorkflows":
		if pr != nil {
			return tasks.ApproveWorkflows(m.ctx, sid, pr)
		}
	case "issue_close":
		if issue != nil {
			return tasks.CloseIssue(m.ctx, sid, issue)
		}
	case "issue_reopen":
		if issue != nil {
			return tasks.ReopenIssue(m.ctx, sid, issue)
		}
	}

	return nil
}
