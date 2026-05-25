package tui

import (
	"fmt"
	"os"
	"reflect"
	"runtime/debug"
	"sort"
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
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/branch"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/branchsidebar"
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
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/reposection"
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
	keys              *keys.KeyMap
	sidebar           sidebar.Model
	prView            prview.Model
	issueSidebar      issueview.Model
	branchSidebar     branchsidebar.Model
	notificationView  notificationview.Model
	currSectionId     int
	footer            footer.Model
	repo              section.Section
	prs               []section.Section
	issues            []section.Section
	notifications     []section.Section
	tabs              tabs.Model
	ctx               *context.ProgramContext
	taskSpinner       spinner.Model
	tasks             map[string]context.Task
	prPreviewStates   map[string]prPreviewState
	copySelection     copySelectionModel
	messagePopup      *messagePopup
	mergePRPopup      *mergePRPopup
	visibleRefreshes  map[string]int
	visibleRefreshGen int
	repoBranches      map[string]repoBranchesState
	prWatchURL        string // kept for older tests; visibleRefreshes owns scheduling
	positionOverride  string // "" means no override, "right" or "bottom"
	activePane        activePane
}

type repoBranchesState struct {
	data    prssection.RepoBranches
	loading bool
}

type prPreviewState struct {
	tabIndex int
	scrollY  int
}

func NewModel(location config.Location) Model {
	taskSpinner := spinner.Model{Spinner: spinner.Dot}
	m := Model{
		keys:             keys.Keys,
		sidebar:          sidebar.NewModel(),
		taskSpinner:      taskSpinner,
		tasks:            map[string]context.Task{},
		prPreviewStates:  map[string]prPreviewState{},
		visibleRefreshes: map[string]int{},
		repoBranches:     map[string]repoBranchesState{},
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
	m.branchSidebar = branchsidebar.NewModel(m.ctx)
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

	var url string
	if config.IsFeatureEnabled(config.FF_REPO_VIEW) && m.ctx.RepoPath != "" {
		res, err := git.GetOriginUrl(m.ctx.RepoPath)
		if err != nil {
			showError(err)
			return initMsg{Config: cfg}
		}
		url = res
	}

	err = keys.Rebind(
		cfg.Keybindings.Universal,
		cfg.Keybindings.Issues,
		cfg.Keybindings.Prs,
		cfg.Keybindings.Branches,
		cfg.Keybindings.Notifications,
	)
	if err != nil {
		showError(err)
	}

	return initMsg{Config: cfg, RepoUrl: url}
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

		if currSection != nil && (currSection.IsSearchFocused() || currSection.IsLocalSearchFocused() ||
			currSection.IsPromptConfirmationFocused()) {
			cmd = m.updateSection(currSection.GetId(), currSection.GetType(), msg)
			return m, cmd
		}

		if m.prView.IsTextInputBoxFocused() {
			if key.Matches(msg, keys.Keys.PageUp) || key.Matches(msg, keys.Keys.PageDown) ||
				key.Matches(msg, keys.Keys.PreviewTop) || key.Matches(msg, keys.Keys.PreviewBottom) {
				m.sidebar, sidebarCmd = m.sidebar.Update(msg)
				return m, sidebarCmd
			}
			m.prView, cmd = m.prView.Update(msg)
			m.syncSidebar()
			return m, cmd
		}

		if m.issueSidebar.IsTextInputBoxFocused() {
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
		case key.Matches(msg, m.keys.FocusMain):
			m.setActivePane(mainPane)
			return m, nil

		case key.Matches(msg, m.keys.FocusPreview):
			if m.sidebar.IsOpen {
				m.setActivePane(previewPane)
			} else {
				m.setActivePane(mainPane)
			}
			return m, nil

		case m.isPreviewFocused() && m.isPreviewTabKey(msg):
			m.prView, prViewCmd = m.prView.Update(msg)
			m.syncSidebar()
			cmds = append(cmds, prViewCmd, m.maybeSchedulePRWatch())
			return m, tea.Batch(cmds...)

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
				if prevRow != nextRow && nextRow == currSection.NumRows()-1 &&
					m.ctx.View != config.RepoView {
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

		case m.ctx.View == config.RepoView:
			switch {
			case key.Matches(msg, m.keys.OpenGithub):
				cmds = append(cmds, m.repo.(*reposection.Model).OpenGithub())

			case key.Matches(msg, keys.BranchKeys.Delete):
				if currSection != nil {
					currSection.SetPromptConfirmationAction("delete")
					cmd = currSection.SetIsPromptConfirmationShown(true)
				}
				return m, cmd

			case key.Matches(msg, keys.BranchKeys.New):
				if currSection != nil {
					currSection.SetPromptConfirmationAction("new")
					cmd = currSection.SetIsPromptConfirmationShown(true)
				}
				return m, cmd

			case key.Matches(msg, keys.BranchKeys.CreatePr):
				if currSection != nil {
					currSection.SetPromptConfirmationAction("create_pr")
					cmd = currSection.SetIsPromptConfirmationShown(true)
				}
				return m, cmd

			case key.Matches(msg, keys.BranchKeys.ViewPRs):
				cmds = append(cmds, m.switchSelectedView())
			}
		case m.ctx.View == config.PRsView:
			switch {
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
				m.prView, scmd = m.prView.Update(msg)
				scmds = append(scmds, scmd)
				m.syncSidebar()
				if m.prView.IsActivityTab() {
					m.scrollActivityToSavedOffsetOrBottom()
				}
				scmds = append(scmds, m.maybeSchedulePRWatch())
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

			case key.Matches(msg, keys.PRKeys.PrevReviewThread),
				key.Matches(msg, keys.PRKeys.NextReviewThread):
				if !m.prView.IsActivityTab() {
					return m, nil
				}
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
				if currRowData != nil {
					cmd = m.promptConfirmation(currSection, "update")
				}
				return m, cmd

			case key.Matches(msg, keys.PRKeys.ApproveWorkflows):
				if currRowData != nil {
					cmd = m.promptConfirmation(currSection, "approveWorkflows")
				}
				return m, cmd

			case key.Matches(msg, keys.PRKeys.ViewIssues):
				cmds = append(cmds, m.switchSelectedView())

			case key.Matches(msg, keys.PRKeys.SummaryViewMore):
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
							if !m.prView.IsActivityTab() {
								return m, nil
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
				cmds = append(cmds, prCmd, m.maybeSchedulePRWatch())

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
		m.currSectionId = m.getCurrentViewDefaultSection()
		m.sidebar.IsOpen = msg.Config.Defaults.Preview.Open
		m.syncMainContentDimensions()

		newSections, fetchSectionsCmds := m.fetchAllViewSections()
		m.setCurrentViewSections(newSections)
		m.tabs.SetCurrSectionId(1)
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
				m.scrollActivityToSavedOffsetOrBottom()
			}
			cmds = append(cmds, m.prView.ActivateChecks())
			cmds = append(cmds, m.maybeSchedulePRWatch())
		} else {
			log.Error("failed enriching pr", "err", msg.Err)
		}

	case visibleRefreshTick:
		if m.visibleRefreshes == nil || m.visibleRefreshes[msg.target.key] != msg.generation {
			return m, nil
		}
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

	case prWatchTick:
		return m, fetchPRWatch(msg.url)

	case visibleRefreshFetchedMsg:
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
			if prMsg, ok := msg.data.(prWatchFetchedMsg); ok {
				cmds = append(cmds, m.applyPRWatchFetched(prMsg))
			}
		case visibleRefreshPRSection:
			if sectionMsg, ok := msg.data.(prssection.SectionPullRequestsRefreshedMsg); ok {
				cmds = append(cmds, m.updateSection(msg.target.sectionId, prssection.SectionType, sectionMsg))
				cmds = append(cmds, m.syncSidebar())
			}
		case visibleRefreshRepoBranches:
			if branchMsg, ok := msg.data.(prssection.RepoBranches); ok {
				cmds = append(cmds, m.applyRepoBranchesRefresh(branchMsg, msg.target.sectionId))
			}
		}
		cmds = append(cmds, m.reconcileVisibleRefreshes())

	case prWatchFetchedMsg:
		cmds = append(cmds, m.applyPRWatchFetched(msg), m.maybeSchedulePRWatch())

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
			cmds = append(cmds, m.maybeSchedulePRWatch())
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

	case constants.ClearTaskMsg:
		m.footer.SetRightSection("")
		delete(m.tasks, msg.TaskId)

	case section.SectionMsg:
		cmd = m.updateRelevantSection(msg)

		if msg.Id == m.currSectionId {
			cmds = append(cmds, m.onViewedRowChanged())
		}

	case execProcessFinishedMsg, tea.FocusMsg:
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
	}

	m.syncProgramContext()

	var bsCmd tea.Cmd
	m.branchSidebar, bsCmd = m.branchSidebar.Update(msg)
	cmds = append(cmds, bsCmd)

	m.sidebar, sidebarCmd = m.sidebar.Update(msg)

	if m.prView.IsTextInputBoxFocused() {
		m.prView, prViewCmd = m.prView.Update(msg)
		m.syncSidebar()
	}

	if m.prView.ShouldUpdateChecks(msg) {
		m.prView, prViewCmd = m.prView.Update(msg)
		m.syncSidebar()
	}

	if m.issueSidebar.IsTextInputBoxFocused() {
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

	s := strings.Builder{}
	if m.ctx.View != config.RepoView {
		s.WriteString(m.tabs.View())
	}
	s.WriteString("\n")
	content := "No sections defined"
	sidebarView := ""
	currSection := m.getCurrSection()
	if currSection != nil {
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
	s.WriteString(content)
	s.WriteString("\n")
	s.WriteString(m.footer.View())

	layers := []*lipgloss.Layer{
		lipgloss.NewLayer(zone.Scan(s.String())),
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
	comp := lipgloss.NewCompositor(layers...)
	v.SetContent(comp.Render())

	return v
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

func (m *Model) setActivePane(pane activePane) {
	m.activePane = pane
	if m.ctx == nil {
		return
	}
	if pane == previewPane && m.sidebar.IsOpen {
		m.ctx.ActivePane = "preview"
		return
	}
	m.activePane = mainPane
	m.ctx.ActivePane = "main"
}

func (m *Model) isPreviewNavigationKey(msg tea.KeyMsg) bool {
	return key.Matches(msg, m.keys.Up) || key.Matches(msg, m.keys.Down) ||
		key.Matches(msg, m.keys.FirstLine) || key.Matches(msg, m.keys.LastLine)
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
	m.saveCurrentPRPreviewState()
	m.prView.SetSummaryViewLess()
	sidebarCmd := m.syncSidebar()
	restored := m.restoreCurrentPRPreviewState()
	enrichCmd := m.prView.EnrichCurrRow()
	if !restored {
		m.sidebar.ScrollToTop()
	}
	m.notificationView.ResetSubject()
	keys.SetNotificationSubject(keys.NotificationSubjectNone)
	return tea.Batch(sidebarCmd, enrichCmd, m.prView.ActivateChecks(), m.maybeSchedulePRWatch())
}

func (m *Model) saveCurrentPRPreviewState() {
	url := m.prView.CurrentPRURL()
	if url == "" {
		return
	}
	if m.prPreviewStates == nil {
		m.prPreviewStates = map[string]prPreviewState{}
	}
	m.prPreviewStates[url] = prPreviewState{
		tabIndex: m.prView.SelectedTabIndex(),
		scrollY:  m.sidebar.YOffset(),
	}
}

func (m *Model) restoreCurrentPRPreviewState() bool {
	pr, ok := m.getCurrRowData().(*prrow.Data)
	if !ok || pr == nil || pr.Primary == nil {
		return false
	}

	url := pr.Primary.Url
	if url == "" {
		return false
	}
	if state, ok := m.prPreviewStates[url]; ok {
		m.prView.GoToTab(state.tabIndex)
		m.syncSidebar()
		m.sidebar.ScrollToOffset(state.scrollY)
		return true
	}
	m.prView.GoToFirstTab()
	m.syncSidebar()
	return false
}

func (m *Model) scrollActivityToSavedOffsetOrBottom() {
	url := m.prView.CurrentPRURL()
	if url != "" {
		if state, ok := m.prPreviewStates[url]; ok && state.tabIndex == m.prView.SelectedTabIndex() {
			m.sidebar.ScrollToOffset(state.scrollY)
			return
		}
	}
	m.sidebar.ScrollToBottom()
}

func (m *Model) scrollFocusedActivityToBottom() {
	offset, ok := m.prView.FocusedActivityScrollOffset(m.sidebar.ViewportHeight())
	if !ok {
		return
	}
	m.sidebar.ScrollToOffset(offset)
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
	m.branchSidebar.UpdateProgramContext(m.ctx)
	m.notificationView.UpdateProgramContext(m.ctx)
}

func (m *Model) updateSection(id int, sType string, msg tea.Msg) (cmd tea.Cmd) {
	var updatedSection section.Section
	switch sType {
	case reposection.SectionType:
		m.repo, cmd = m.repo.Update(msg)

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
	if !m.sidebar.IsOpen {
		m.sidebar.IsOpen = true
		m.positionOverride = "right"
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
	m.syncMainContentDimensions()
	m.syncProgramContext()
	return m.reconcileVisibleRefreshes()
}

func (m *Model) openSidebarForPRInput(setFunc func(bool) tea.Cmd) tea.Cmd {
	m.prView.GoToFirstTab()
	return m.openSidebarForInput(setFunc)
}

func (m *Model) openSidebarForInput(setFunc func(bool) tea.Cmd) tea.Cmd {
	m.sidebar.IsOpen = true
	cmd := setFunc(true)
	m.syncMainContentDimensions()
	m.syncSidebar()
	m.sidebar.ScrollToBottom()
	return cmd
}

func (m *Model) openSidebarForInputNoScroll(setFunc func(bool) tea.Cmd) tea.Cmd {
	m.sidebar.IsOpen = true
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

func (m *Model) syncSidebar() tea.Cmd {
	if !m.sidebar.IsOpen {
		return nil
	}

	currRowData := m.getCurrRowData()
	width := m.sidebar.GetSidebarContentWidth()
	var cmd tea.Cmd

	if currRowData == nil {
		m.sidebar.ClearHeader()
		m.sidebar.SetContent("")
		return nil
	}

	switch row := currRowData.(type) {
	case branch.BranchData:
		cmd = m.branchSidebar.SetRow(&row)
		m.sidebar.ClearHeader()
		m.sidebar.SetContent(m.branchSidebar.View())
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
		m.issueSidebar.SetSectionId(m.currSectionId)
		m.issueSidebar.SetRow(row)
		m.issueSidebar.SetWidth(width)
		m.sidebar.ClearHeader()
		m.sidebar.SetContent(m.issueSidebar.View())
		// Scroll to bottom if in input mode to keep inputbox visible
		if m.issueSidebar.IsTextInputBoxFocused() {
			m.sidebar.ScrollToBottom()
		}
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
		// Show prompt to view notification (don't auto-fetch)
		// User must press Enter to view content and mark as read
		m.sidebar.ClearHeader()
		m.sidebar.SetContent(m.renderNotificationPrompt(row))
	}

	return cmd
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
	m.sidebar.SetHeader(m.prView.HeaderView())
	m.sidebar.SetContent(m.prView.BodyView())
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
	case config.RepoView:
		var cmd tea.Cmd
		s, cmd := reposection.FetchAllBranches(m.ctx)
		cmds = append(cmds, cmd)
		m.repo = &s
		return nil, tea.Batch(cmds...)
	case config.NotificationsView:
		s, notifCmd := notificationssection.FetchAllSections(m.ctx, m.notifications)
		cmds = append(cmds, notifCmd)
		m.notifications = s
		return s, tea.Batch(cmds...)
	case config.PRsView:
		s, prcmds := prssection.FetchAllSections(m.ctx, m.prs)
		cmds = append(cmds, prcmds)
		return s, tea.Batch(cmds...)
	default:
		s, issuecmds := issuessection.FetchAllSections(m.ctx)
		cmds = append(cmds, issuecmds)
		return s, tea.Batch(cmds...)
	}
}

func (m *Model) getCurrentViewSections() []section.Section {
	switch m.ctx.View {
	case config.RepoView:
		if m.repo == nil {
			return []section.Section{}
		}
		return []section.Section{m.repo}
	case config.NotificationsView:
		if len(m.notifications) == 0 {
			return []section.Section{}
		}
		return m.notifications
	case config.PRsView:
		return m.prs
	default:
		return m.issues
	}
}

func (m *Model) getCurrentViewDefaultSection() int {
	switch m.ctx.View {
	case config.RepoView:
		return 0
	case config.NotificationsView:
		return 1 // First notification section after search section
	case config.PRsView:
		return 1
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
	views := []config.ViewType{config.NotificationsView, config.PRsView, config.IssuesView}
	if config.IsFeatureEnabled(config.FF_REPO_VIEW) {
		views = append(views, config.RepoView)
	}

	// Reset notification subject when leaving notifications view
	if m.ctx.View == config.NotificationsView {
		keys.SetNotificationSubject(keys.NotificationSubjectNone)
		m.notificationView.ClearSubject()
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

	m.syncMainContentDimensions()
	m.setCurrSectionId(m.getCurrentViewDefaultSection())

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

	if m.ctx.View == config.RepoView {
		for _, keybinding := range m.ctx.Config.Keybindings.Branches {
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

type prWatchTick struct{ url string }

type prWatchFetchedMsg struct {
	url  string
	data data.EnrichedPullRequestData
	err  error
}

var fetchPullRequestForPRWatch = data.FetchPullRequest

func (m *Model) shouldWatchCurrentPR() bool {
	if !m.sidebar.IsOpen {
		return false
	}

	return m.prView.CurrentPRURL() != ""
}

func (m *Model) maybeSchedulePRWatch() tea.Cmd {
	if m.shouldWatchCurrentPR() {
		m.prWatchURL = m.prView.CurrentPRURL()
	} else {
		m.prWatchURL = ""
	}
	return m.reconcileVisibleRefreshes()
}

func fetchPRWatch(url string) tea.Cmd {
	return func() tea.Msg {
		pr, err := fetchPullRequestForPRWatch(url)
		return prWatchFetchedMsg{url: url, data: pr, err: err}
	}
}

func (m *Model) applyPRWatchFetched(msg prWatchFetchedMsg) tea.Cmd {
	if msg.url != m.prView.CurrentPRURL() || !m.shouldWatchCurrentPR() {
		return nil
	}
	if msg.err != nil {
		log.Error("failed refreshing watched PR", "err", msg.err)
		return nil
	}

	m.prView.SetEnrichedPR(msg.data)
	if pr := m.notificationView.GetSubjectPR(); pr != nil && pr.Primary != nil && pr.Primary.Url == msg.url {
		prData := msg.data.ToPullRequestData()
		m.notificationView.SetSubjectPR(&prrow.Data{
			Primary:    &prData,
			Enriched:   msg.data,
			IsEnriched: true,
		}, m.notificationView.GetSubjectId())
	}
	if m.ctx != nil && m.ctx.View == config.PRsView && m.currSectionId >= 0 && m.currSectionId < len(m.prs) {
		if prSection, ok := m.prs[m.currSectionId].(*prssection.Model); ok {
			prSection.EnrichPR(msg.data)
		}
	}

	return m.syncSidebar()
}

func (m *Model) visibleRefreshTargets() []visibleRefreshTarget {
	var targets []visibleRefreshTarget

	if m.shouldWatchCurrentPR() {
		if url := m.prView.CurrentPRURL(); url != "" {
			targets = append(targets, visibleRefreshTarget{
				key:      "pr-preview:" + url,
				kind:     visibleRefreshPRPreview,
				url:      url,
				interval: 10 * time.Second,
			})
		}
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
			return visibleRefreshFetchedMsg{target: target, data: prWatchFetchedMsg{url: target.url, data: pr, err: err}, err: err}
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
