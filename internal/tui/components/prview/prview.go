package prview

import (
	"fmt"
	"image/color"
	"regexp"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dlvhdr/gh-dehub/v4/internal/data"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/common"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/actionview"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/carousel"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/cmpcontroller"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/fuzzyselect"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/inputbox"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/prrow"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/prssection"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/tasks"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/keys"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/markdown"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/state"
	"github.com/dlvhdr/gh-dehub/v4/internal/utils"
)

var (
	htmlCommentRegex = regexp.MustCompile("(?U)<!--(.|[[:space:]])*-->")
	lineCleanupRegex = regexp.MustCompile(`((\n)+|^)([^\r\n]*\|[^\r\n]*(\n)?)+`)
	foldBodyHeight   = 8
)

type Model struct {
	ctx                  *context.ProgramContext
	sectionId            int
	sectionType          string
	pr                   *prrow.PullRequest
	width                int
	carousel             carousel.Model
	editor               cmpcontroller.Controller
	summaryViewMore      bool
	focusedThread        int
	activityFocusTargets []int
	activityCache        activityRenderCache
	activityBodyCache    map[string]string
	reviewDiffCache      map[string][]reviewDiffLine
	viewState            prViewState
	viewStateKey         string
	rememberedViewState  bool
	viewStates           *state.Cache[prViewState]
	actionChecks         *actionview.Model
	actionChecksKey      string
	// actionChecksCache stashes the Checks-tab embedded actionview keyed
	// by "repo#number" so that switching between PR rows (or away to
	// another view and back) restores the user's selected job/step,
	// log scroll, search query, and zoom state for that PR.
	actionChecksCache *state.Cache[*actionview.Model]
	// previewFocused tracks whether the preview pane currently owns
	// keyboard focus. Update consults this to decide whether to forward
	// key messages into the embedded Checks tab's actionview. Non-key
	// messages (resize, async fetches, log streaming) are always
	// forwarded regardless of focus so background work stays current.
	previewFocused bool
}

// prViewStateCacheLimit caps session-scoped state retained for PR rows.
const prViewStateCacheLimit = 1024

// actionChecksCacheLimit caps the number of cached Checks-tab actionview
// instances retained across PR row switches.
const actionChecksCacheLimit = 1024

type prViewState struct {
	selectedTabIndex         int
	activitySnippetsExpanded bool
	activityItemsCollapsed   bool
}

type activityRenderCache struct {
	width       int
	fingerprint string
	activities  []cachedActivity
}

type WatchReason int

const (
	WatchNone WatchReason = iota
	WatchChecks
	WatchActivity
)

const (
	focusedNewComment = -1
	unfocusedActivity = -2
)

var tabs = []string{" Overview", " Activity", " Checks", " Commits", " Files Changed"}

func NewModel(ctx *context.ProgramContext) Model {
	c := carousel.New(
		carousel.WithItems(tabs),
		carousel.WithWidth(ctx.MainContentWidth),
	)

	ta := inputbox.DefaultTextArea(ctx)
	cmp := cmpcontroller.New(ctx, inputbox.ModelOpts{TextArea: &ta})

	return Model{
		pr:              nil,
		carousel:        c,
		editor:          cmp,
		focusedThread:   focusedNewComment,
		reviewDiffCache: map[string][]reviewDiffLine{},
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	cmd, handled := m.editor.Update(msg)

	if msg, ok := msg.(tea.KeyMsg); ok && msg.String() == "ctrl+d" {
		value := m.editor.Value()
		mode := m.editor.Mode()
		m.editor.Exit()
		if m.pr == nil {
			return m, nil
		}

		sid := m.sectionIdentifier()

		switch mode {
		case cmpcontroller.ModeComment:
			if len(strings.TrimSpace(value)) != 0 {
				return m, tasks.CommentOnPR(m.ctx, sid, m.pr.Data.Primary, value)
			}
			return m, nil

		case cmpcontroller.ModeThreadComment:
			thread, ok := m.FocusedReviewThread()
			if ok && len(strings.TrimSpace(value)) != 0 {
				return m, tasks.ReplyToReviewThread(m.ctx, sid, m.pr.Data.Primary, thread.Id, value)
			}
			return m, nil

		case cmpcontroller.ModeApprove:
			comment := ""
			if len(strings.TrimSpace(value)) != 0 {
				comment = value
			}
			return m, tasks.ApprovePR(m.ctx, sid, m.pr.Data.Primary, comment)

		case cmpcontroller.ModeAssign:
			added, removed := m.assigneeChanges(fuzzyselect.AllWords(value))
			if len(added) > 0 || len(removed) > 0 {
				return m, tasks.AssignPR(m.ctx, sid, m.pr.Data.Primary, added, removed)
			}
			return m, nil

		case cmpcontroller.ModeRequestReview:
			added, removed := m.reviewerChanges(fuzzyselect.AllWords(value))
			if len(added) > 0 || len(removed) > 0 {
				return m, tasks.RequestReviewPR(m.ctx, sid, m.pr.Data.Primary, added, removed)
			}
			return m, nil

		case cmpcontroller.ModeLabel:
			labels := fuzzyselect.CurrentLabels(value)
			if len(labels) > 0 || len(m.pr.Data.Primary.Labels.Nodes) > 0 {
				return m, m.label(labels)
			}
			return m, nil
		}
	}

	if handled {
		return m, cmd
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, keys.PRKeys.PrevSidebarTab):
			m.moveToPrevTab()
			cmd = m.ActivateChecks()
		case key.Matches(keyMsg, keys.PRKeys.NextSidebarTab):
			m.moveToNextTab()
			cmd = m.ActivateChecks()
		}
	}

	if m.IsChecksTab() && m.actionChecks != nil {
		// actionview-local keys (up/down/h/l/g/G/etc.) must only reach
		// the embedded actionview when the preview pane is focused.
		// Otherwise the Checks pane silently consumes navigation keys
		// meant for the PR row list. All other messages (non-local
		// keys, logs-search typing, async fetches, resize, etc.) are
		// forwarded unconditionally so the actionview's own gate
		// (UpdateEmbedded) keeps owning the final decision and the
		// logs-search input keeps receiving characters even when the
		// preview pane isn't yet recorded as focused (parent routing
		// reaches this path via IsChecksLogsSearchFocused).
		forward := true
		if keyMsg, ok := msg.(tea.KeyMsg); ok && actionview.IsLocalKey(keyMsg) && !m.previewFocused {
			forward = false
		}
		if forward {
			var checksCmd tea.Cmd
			checksModel, checksCmd := m.actionChecks.UpdateEmbedded(msg)
			m.actionChecks = &checksModel
			cmd = tea.Batch(cmd, checksCmd)
		}
	}

	return m, cmd
}

// SetPreviewFocused records whether the preview pane currently owns
// keyboard focus. The parent TUI must call this whenever the active pane
// or sidebar visibility changes; Update consults it to gate key-message
// forwarding into the embedded Checks tab actionview.
func (m *Model) SetPreviewFocused(focused bool) {
	m.previewFocused = focused
}

// IsPreviewFocused reports the focus state previously set via
// SetPreviewFocused. Exposed primarily for tests and parent-level
// diagnostics; runtime gating consults the private field directly.
func (m Model) IsPreviewFocused() bool {
	return m.previewFocused
}

func (m Model) View() string {
	if !m.hasData() {
		return ""
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.HeaderView(),
		m.BodyView(),
	)
}

func (m Model) HeaderView() string {
	if !m.hasData() {
		return ""
	}

	return m.viewHeader()
}

func (m *Model) BodyView() string {
	if !m.hasData() {
		return ""
	}

	body := strings.Builder{}
	switch m.carousel.SelectedItem() {
	case tabs[0]:
		body.WriteString(m.viewOverviewTab())
	case tabs[1]:
		if m.editor.Mode() == cmpcontroller.ModeNone {
			return m.cachedActivityBodyView()
		}
		body.WriteString(m.renderActivity())
	case tabs[2]:
		body.WriteString(m.renderChecksOverview())
		body.WriteString("\n\n")
		body.WriteString(m.renderActionChecks())
	case tabs[3]:
		body.WriteString(m.renderCommits())
	case tabs[4]:
		body.WriteString(m.renderChangedFiles())
	}

	if m.editor.Mode() != cmpcontroller.ModeNone && !m.isActivityCommentEditor() {
		body.WriteString(m.ctx.Styles.Sidebar.InputBox.Render(m.editor.View()))
	}

	return lipgloss.NewStyle().Padding(0, m.ctx.Styles.Sidebar.ContentPadding).Render(body.String())
}

func (m *Model) viewHeader() string {
	header := strings.Builder{}

	header.WriteString(m.renderFullNameAndNumber())
	header.WriteString("\n")

	header.WriteString(m.renderTitle())
	header.WriteString("\n\n")
	header.WriteString(m.renderBranches())
	header.WriteString("\n\n")
	header.WriteString(m.renderAuthor())
	header.WriteString("\n\n")
	header.WriteString(
		lipgloss.NewStyle().Width(m.width).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(m.ctx.Theme.FaintBorder).
			Render(m.carousel.View()),
	)

	header.WriteString("\n")
	return header.String()
}

func (m *Model) viewOverviewTab() string {
	body := strings.Builder{}
	reviewers := m.renderRequestedReviewers()
	if reviewers != "" {
		body.WriteString(reviewers)
		body.WriteString("\n\n")
	}

	labels := m.renderLabels()
	if labels != "" {
		body.WriteString(labels)
		body.WriteString("\n\n")
	}

	body.WriteString(m.renderSummary())
	body.WriteString("\n\n")
	body.WriteString(
		m.ctx.Styles.Common.MainTextStyle.MarginBottom(1).Underline(true).Render(" Changes"),
	)
	body.WriteString("\n")
	body.WriteString(m.renderChangesOverview())
	body.WriteString("\n\n")
	body.WriteString(
		m.ctx.Styles.Common.MainTextStyle.MarginBottom(1).Underline(true).Render(" Checks"),
	)
	body.WriteString("\n")
	body.WriteString(m.renderChecksOverview())

	return body.String()
}

func (m *Model) ViewCompletions() string {
	if !m.hasData() {
		return ""
	}
	return m.editor.ViewCompletions()
}

func (m *Model) InputBoxLineFromBottom() int {
	return m.editor.LineFromBottom()
}

func (m *Model) renderFullNameAndNumber() string {
	if !m.hasData() {
		return ""
	}

	return common.RenderPreviewHeader(
		m.ctx.Theme,
		m.width,
		fmt.Sprintf(
			"%s · #%d",
			m.pr.Data.Primary.GetRepoNameWithOwner(),
			m.pr.Data.Primary.GetNumber(),
		),
	)
}

func (m *Model) renderTitle() string {
	if !m.hasData() {
		return ""
	}

	return common.RenderPreviewTitle(
		m.ctx.Theme,
		m.ctx.Styles.Common,
		m.width,
		m.pr.Data.Primary.Title,
	)
}

func (m *Model) renderBranches() string {
	return lipgloss.JoinHorizontal(lipgloss.Left,
		" ",
		m.renderStatusPill(),
		" ",
		lipgloss.NewStyle().
			Foreground(m.ctx.Theme.SecondaryText).
			Render(m.pr.Data.Primary.BaseRefName+"  "+m.pr.Data.Primary.HeadRefName))
}

func (m *Model) renderStatusPill() string {
	var bgColor color.Color
	switch m.pr.Data.Primary.State {
	case "OPEN":
		if m.pr.Data.Primary.IsDraft {
			bgColor = m.ctx.Theme.FaintText.Dark
		} else {
			bgColor = m.ctx.Styles.Colors.OpenPR.Dark
		}
	case "CLOSED":
		bgColor = m.ctx.Styles.Colors.ClosedPR.Dark
	case "MERGED":
		bgColor = m.ctx.Styles.Colors.MergedPR.Dark
	}

	return m.ctx.Styles.PrView.PillStyle.
		BorderForeground(bgColor).
		Background(bgColor).
		Render(m.pr.RenderState())
}

func (m *Model) renderLabels() string {
	width := m.getIndentedContentWidth()
	labels := m.pr.Data.Primary.Labels.Nodes
	style := m.ctx.Styles.PrView.PillStyle
	if len(labels) == 0 {
		return ""
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.ctx.Styles.Common.MainTextStyle.Underline(true).Bold(true).Render(
			fmt.Sprintf("%s Labels", constants.LabelsIcon),
		),
		"",
		common.RenderLabels(labels, common.LabelOpts{
			Width:     width,
			PillStyle: style,
		}),
	)
}

type reviewerItem struct {
	text string
}

func (m *Model) renderRequestedReviewers() string {
	if !m.pr.Data.IsEnriched {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			m.ctx.Styles.Common.MainTextStyle.Underline(true).Bold(true).Render(
				fmt.Sprintf("%s Reviewers", constants.CodeReviewIcon),
			),
			"",
			lipgloss.JoinHorizontal(
				lipgloss.Top,
				m.ctx.Styles.Common.WaitingGlyph,
				" ",
				m.ctx.Styles.Common.FaintTextStyle.Render("Loading..."),
			),
		)
	}

	reviewRequests := m.pr.Data.Enriched.ReviewRequests.Nodes
	reviews := m.pr.Data.Enriched.Reviews.Nodes
	suggestedReviewers := m.pr.Data.Enriched.SuggestedReviewers
	isApproved := m.pr.Data.Primary.ReviewDecision == "APPROVED"

	if len(reviewRequests) == 0 && len(reviews) == 0 && len(suggestedReviewers) == 0 && !isApproved {
		return ""
	}

	reviewStates := make(map[string]string)
	for _, review := range reviews {
		login := review.Author.Login
		existingState := reviewStates[login]
		// Don't override APPROVED or CHANGES_REQUESTED with COMMENTED
		if review.State == "COMMENTED" &&
			(existingState == "APPROVED" || existingState == "CHANGES_REQUESTED") {
			continue
		}
		reviewStates[login] = review.State
	}

	reviewerItems := make([]reviewerItem, 0)
	faintStyle := m.ctx.Styles.Common.FaintTextStyle
	reviewerStyle := lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText)
	successStyle := lipgloss.NewStyle().Foreground(m.ctx.Theme.SuccessText)
	errorStyle := lipgloss.NewStyle().Foreground(m.ctx.Theme.ErrorText)

	shownReviewers := make(map[string]bool)

	for _, req := range reviewRequests {
		displayName := req.GetReviewerDisplayName()
		if displayName == "" {
			continue
		}
		shownReviewers[displayName] = true

		var reviewerStr string
		stateIcon := ""
		if state, hasReview := reviewStates[displayName]; hasReview && state == "COMMENTED" {
			stateIcon = m.ctx.Styles.Common.CommentGlyph
		} else {
			stateIcon = m.ctx.Styles.Common.WaitingDotGlyph
		}

		if req.IsTeam() {
			reviewerStr += reviewerStyle.Render(displayName)
		} else {
			reviewerStr += reviewerStyle.Render("@" + displayName)
		}

		if req.AsCodeOwner {
			reviewerStr = lipgloss.JoinHorizontal(lipgloss.Top,
				faintStyle.Render(constants.OwnerIcon), " ", reviewerStr)
		}
		reviewerStr = lipgloss.JoinHorizontal(lipgloss.Top, stateIcon, " ", reviewerStr)

		reviewerItems = append(reviewerItems, reviewerItem{text: reviewerStr})
	}

	for login, state := range reviewStates {
		if shownReviewers[login] {
			continue
		}
		if state != "APPROVED" && state != "CHANGES_REQUESTED" && state != "COMMENTED" {
			continue
		}
		shownReviewers[login] = true

		var stateIcon string
		switch state {
		case "APPROVED":
			stateIcon = successStyle.Render(constants.ApprovedIcon)
		case "CHANGES_REQUESTED":
			stateIcon = errorStyle.Render(constants.ChangesRequestedIcon)
		case "COMMENTED":
			stateIcon = m.ctx.Styles.Common.CommentGlyph
		}
		reviewerStr := stateIcon + " " + reviewerStyle.Render("@"+login)

		reviewerItems = append(reviewerItems, reviewerItem{text: reviewerStr})
	}

	if isApproved && !hasReviewState(reviewStates, "APPROVED") {
		reviewerStr := successStyle.Render(constants.ApprovedIcon) + " " + reviewerStyle.Render("Approved")
		reviewerItems = append(reviewerItems, reviewerItem{text: reviewerStr})
	}

	// Show suggested reviewers (= code owners) who haven't been requested or reviewed yet
	for _, suggested := range suggestedReviewers {
		login := suggested.Reviewer.Login
		if shownReviewers[login] {
			continue
		}
		if suggested.IsAuthor {
			continue
		}
		shownReviewers[login] = true

		reviewerStr := lipgloss.JoinHorizontal(
			lipgloss.Top,
			faintStyle.Render(constants.OwnerIcon), " ",
			faintStyle.Render("@"+login),
		)

		reviewerItems = append(reviewerItems, reviewerItem{text: reviewerStr})
	}

	if len(reviewerItems) == 0 {
		return ""
	}

	width := m.getIndentedContentWidth()
	var rows []string
	var currentRow strings.Builder
	currentRowWidth := 0

	for i, item := range reviewerItems {
		itemWidth := lipgloss.Width(item.text)
		separator := ", "
		separatorWidth := lipgloss.Width(separator)

		// Check if adding this item would exceed the width
		needsSeparator := i < len(reviewerItems)-1
		totalItemWidth := itemWidth
		if needsSeparator {
			totalItemWidth += separatorWidth
		}

		if currentRowWidth > 0 && currentRowWidth+totalItemWidth > width {
			// Start a new row
			rows = append(rows, currentRow.String())
			currentRow.Reset()
			currentRowWidth = 0
		}

		currentRow.WriteString(item.text)
		currentRowWidth += itemWidth

		if needsSeparator {
			currentRow.WriteString(separator)
			currentRowWidth += separatorWidth
		}
	}

	// Add the last row
	if currentRow.Len() > 0 {
		rows = append(rows, currentRow.String())
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.ctx.Styles.Common.MainTextStyle.Underline(true).Bold(true).Render(
			fmt.Sprintf("%s Reviewers", constants.CodeReviewIcon),
		),
		"",
		strings.Join(rows, "\n"),
	)
}

func hasReviewState(reviewStates map[string]string, state string) bool {
	for _, reviewState := range reviewStates {
		if reviewState == state {
			return true
		}
	}
	return false
}

func (m *Model) renderAuthor() string {
	authorAssociation := m.pr.Data.Primary.AuthorAssociation
	if authorAssociation == "" {
		authorAssociation = "unknown role"
	}
	time := lipgloss.NewStyle().Render(utils.TimeElapsed(m.pr.Data.Primary.CreatedAt))
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		" by ",
		lipgloss.NewStyle().Foreground(m.ctx.Theme.PrimaryText).Render(
			lipgloss.NewStyle().Bold(true).Render("@"+m.pr.Data.Primary.Author.Login),
		),
		lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).Render(
			lipgloss.JoinHorizontal(lipgloss.Top, " ⋅ ", time, " ago", " ⋅ "),
		),
		lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).Render(
			lipgloss.JoinHorizontal(lipgloss.Top, data.GetAuthorRoleIcon(m.pr.Data.Primary.AuthorAssociation,
				m.ctx.Theme),
				" ", lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).Render(strings.ToLower(authorAssociation))),
		),
	)
}

func (m *Model) renderSummary() string {
	width := m.getIndentedContentWidth()
	// Strip HTML comments from body and cleanup body.
	body := htmlCommentRegex.ReplaceAllString(m.pr.Data.Primary.Body, "")
	body = lineCleanupRegex.ReplaceAllString(body, "")

	desc := m.ctx.Styles.Common.MainTextStyle.Bold(true).Underline(true).Render(" Summary")
	title := lipgloss.JoinVertical(
		lipgloss.Left,
		desc,
		"",
	)
	sbody := lipgloss.NewStyle().Width(m.getIndentedContentWidth())
	body = strings.TrimSpace(body)
	if body == "" {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			sbody.Italic(true).Foreground(m.ctx.Theme.FaintText).Render("No description provided."),
		)
	}

	markdownRenderer := markdown.GetMarkdownRenderer(width)
	rendered, err := markdownRenderer.Render(body)
	if err != nil {
		return ""
	}

	bodyHeight := lipgloss.Height(rendered)
	if !m.summaryViewMore && bodyHeight > foldBodyHeight {
		rendered = lipgloss.NewStyle().MaxHeight(foldBodyHeight).Render(rendered)
		rendered = lipgloss.JoinVertical(
			lipgloss.Left,
			rendered,
			"",
			lipgloss.PlaceHorizontal(
				m.getIndentedContentWidth(), lipgloss.Center,
				lipgloss.JoinHorizontal(
					lipgloss.Top,
					lipgloss.NewStyle().Bold(true).Italic(true).Render("Press "),
					lipgloss.NewStyle().
						Background(m.ctx.Theme.SelectedBackground).
						Foreground(m.ctx.Theme.PrimaryText).
						Render("e"),
					lipgloss.NewStyle().Bold(true).Italic(true).Render(" to read more..."),
				),
			),
		)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left, title,
		lipgloss.NewStyle().
			Width(width).
			MaxWidth(width).
			Align(lipgloss.Left).
			Render(rendered),
	)
}

func (m *Model) SetSectionId(id int) {
	m.sectionId = id
	m.sectionType = prssection.SectionType
}

func (m *Model) SetSection(id int, sectionType string) {
	m.sectionId = id
	m.sectionType = sectionType
}

func (m *Model) SetRow(d *prrow.Data) {
	if d == nil {
		// Stash the active checks view before clearing so a subsequent
		// row restore can bring it back.
		m.stashActiveViewState()
		m.stashActiveActionChecks()
		m.pr = nil
		m.viewState = prViewState{}
		m.viewStateKey = ""
		m.rememberedViewState = false
		m.actionChecks = nil
		m.actionChecksKey = ""
		return
	}

	newPR := d.Primary == nil || m.pr == nil || m.pr.Data == nil || m.pr.Data.Primary == nil || m.pr.Data.Primary.Url != d.Primary.Url
	if newPR {
		m.FocusNewComment()
		m.stashActiveViewState()
		m.restoreViewState(prViewStateKey(d.Primary))
		m.RestoreSelectedTab()
		m.invalidateActivityCache()
		// Cache the outgoing PR's Checks-tab actionview keyed by its
		// repo#number so returning to that PR (within the cache cap)
		// resurrects the user's selection / scroll / search state.
		m.stashActiveActionChecks()
		m.actionChecks = nil
		m.actionChecksKey = ""
	}
	m.pr = &prrow.PullRequest{Ctx: m.ctx, Data: d}
	m.clampFocusedReviewThread()
}

// stashActiveActionChecks moves the currently-active Checks-tab actionview
// into the cache so that ensureActionChecks() for the same PR later picks
// it back up instead of constructing a fresh instance.
func (m *Model) stashActiveActionChecks() {
	if m.actionChecks == nil || m.actionChecksKey == "" {
		return
	}
	if m.actionChecksCache == nil {
		m.actionChecksCache = state.NewCache[*actionview.Model](actionChecksCacheLimit)
	}
	if existing, ok := m.actionChecksCache.Get(m.actionChecksKey); ok && existing == m.actionChecks {
		return
	}
	m.actionChecksCache.Put(m.actionChecksKey, m.actionChecks)
}

func (m *Model) stashActiveViewState() {
	if m.viewStateKey == "" {
		return
	}
	if m.viewStates == nil {
		m.viewStates = state.NewCache[prViewState](prViewStateCacheLimit)
	}
	m.viewStates.Put(m.viewStateKey, m.viewState)
}

func (m *Model) restoreViewState(key string) {
	m.viewStateKey = key
	if key == "" {
		m.viewState = prViewState{}
		m.rememberedViewState = false
		return
	}
	if m.viewStates == nil {
		m.viewStates = state.NewCache[prViewState](prViewStateCacheLimit)
	}
	if s, ok := m.viewStates.Get(key); ok {
		m.viewState = s
		m.rememberedViewState = true
		return
	}
	m.viewState = prViewState{}
	m.rememberedViewState = false
	// Declare this PR's state immediately so future fields added to prViewState
	// are cached by the same stash/restore lifecycle.
	m.viewStates.Put(key, m.viewState)
}

func prViewStateKey(pr *data.PullRequestData) string {
	if pr == nil {
		return ""
	}
	repo := pr.GetRepoNameWithOwner()
	number := pr.GetNumber()
	if repo == "" || number == 0 {
		return ""
	}
	return fmt.Sprintf("%s#%d", repo, number)
}

func (m *Model) ActivateChecks() tea.Cmd {
	if !m.IsChecksTab() {
		return nil
	}
	return m.ensureActionChecks()
}

func (m *Model) FocusChecksLogsSearch() tea.Cmd {
	if !m.IsChecksTab() || m.actionChecks == nil {
		return nil
	}
	return m.actionChecks.FocusLogsSearch()
}

func (m Model) IsChecksLogsSearchFocused() bool {
	return m.actionChecks != nil && m.IsChecksTab() && m.actionChecks.IsLogsSearchFocused()
}

func (m Model) ChecksLogsSearchValue() (string, bool) {
	if !m.IsChecksTab() || m.actionChecks == nil {
		return "", false
	}
	return m.actionChecks.LogsSearchValue(), true
}

func (m Model) ChecksLogsCopySelectionContent() (string, bool) {
	if !m.IsChecksTab() || m.actionChecks == nil {
		return "", false
	}
	return m.actionChecks.LogsCopySelectionContent(), true
}

func (m *Model) ShouldUpdateChecks(msg tea.Msg) bool {
	return m.IsChecksTab() && m.actionChecks != nil && actionview.HandlesAsyncMsg(msg)
}

func (m *Model) ensureActionChecks() tea.Cmd {
	if m.pr == nil || m.pr.Data == nil || m.pr.Data.Primary == nil {
		return nil
	}

	repo := m.pr.Data.Primary.GetRepoNameWithOwner()
	number := fmt.Sprint(m.pr.Data.Primary.GetNumber())
	key := repo + "#" + number
	if m.actionChecks != nil && m.actionChecksKey == key {
		m.actionChecks.SetSize(m.getIndentedContentWidth(), m.actionChecksHeight())
		return nil
	}

	// Stash the currently-active checks (different PR) before replacing it
	// so the user can return to that PR without losing state.
	m.stashActiveActionChecks()

	// If we've seen this PR before, restore its actionview instead of
	// rebuilding it from scratch.
	if m.actionChecksCache == nil {
		m.actionChecksCache = state.NewCache[*actionview.Model](actionChecksCacheLimit)
	}
	if cached, ok := m.actionChecksCache.Get(key); ok && cached != nil {
		m.actionChecksCache.Delete(key)
		cached.SetSize(m.getIndentedContentWidth(), m.actionChecksHeight())
		m.actionChecks = cached
		m.actionChecksKey = key
		return nil
	}

	checks := actionview.NewModel(repo, number, actionview.ModelOpts{
		Flat:     true,
		Embedded: true,
		Theme:    &m.ctx.Theme,
	})
	checks.SetSize(m.getIndentedContentWidth(), m.actionChecksHeight())
	m.actionChecks = &checks
	m.actionChecksKey = key
	return checks.Init()
}

func (m *Model) actionChecksHeight() int {
	if m.ctx == nil {
		return 24
	}
	if m.ctx.DynamicPreviewHeight > 0 {
		return max(12, m.ctx.DynamicPreviewHeight-8)
	}
	if m.ctx.ScreenHeight > 0 {
		return max(12, m.ctx.ScreenHeight-12)
	}
	return 24
}

type EnrichedPrMsg struct {
	Id   int
	Type string
	Data data.EnrichedPullRequestData
	Err  error
}

func (m *Model) EnrichCurrRow() tea.Cmd {
	if m == nil || m.pr == nil || m.pr.Data.IsEnriched {
		return nil
	}
	url := m.pr.Data.Primary.Url
	return func() tea.Msg {
		d, err := data.FetchPullRequest(url)
		return EnrichedPrMsg{
			Id:   m.sectionId,
			Type: prssection.SectionType,
			Data: d,
			Err:  err,
		}
	}
}

func (m *Model) SetWidth(width int) {
	if m.width != width {
		m.invalidateActivityCache()
	}
	m.width = width
	m.carousel.SetWidth(width)
	m.editor.SetWidth(
		m.getIndentedContentWidth() - m.ctx.Styles.Sidebar.InputBox.GetHorizontalFrameSize(),
	)
}

func (m *Model) IsTextInputBoxFocused() bool {
	return m.editor.Active()
}

func (m *Model) isActivityCommentEditor() bool {
	if !m.IsActivityTab() {
		return false
	}

	switch m.editor.Mode() {
	case cmpcontroller.ModeComment, cmpcontroller.ModeThreadComment:
		return true
	default:
		return false
	}
}

func (m *Model) UpdateProgramContext(ctx *context.ProgramContext) {
	m.ctx = ctx
	m.editor.UpdateProgramContext(ctx)
	m.carousel.SetStyles(
		carousel.Styles{
			Item:     lipgloss.NewStyle().Padding(0, 1).Foreground(m.ctx.Theme.FaintText),
			Selected: lipgloss.NewStyle().Padding(0, 1).Bold(true),
		},
	)

	// TODO: move this to the NewModel func
	// currently it's not possible since the styles aren't yet instantiated when NewModel is called
	m.editor.SetSelectStyles(ctx.Styles.Select)
}

func (m *Model) GetIsCommenting() bool {
	return m.editor.Mode() == cmpcontroller.ModeComment
}

func (m *Model) SetIsCommenting(isCommenting bool) tea.Cmd {
	if m.pr == nil {
		return nil
	}

	if !isCommenting {
		if m.editor.Mode() == cmpcontroller.ModeComment {
			m.editor.Exit()
		}
		return nil
	}

	if m.IsActivityTab() && m.HasFocusedReviewThread() {
		return m.SetIsThreadCommenting(true)
	}

	m.editor.SetAutocompleteSource(&fuzzyselect.UserMentionSource{WithAtSymbol: true})
	cmd := m.editor.Enter(cmpcontroller.EnterOptions{
		Mode:                             cmpcontroller.ModeComment,
		Prompt:                           constants.CommentPrompt,
		Repo:                             m.repoRef(),
		EnterFetch:                       cmpcontroller.FetchSilent,
		ConfirmDiscardOnCancel:           true,
		HideAutocompleteWhenContextEmpty: true,
	})
	return cmd
}

func (m *Model) SetIsThreadCommenting(isCommenting bool) tea.Cmd {
	if m.pr == nil {
		return nil
	}

	if !isCommenting {
		if m.editor.Mode() == cmpcontroller.ModeThreadComment {
			m.editor.Exit()
		}
		return nil
	}

	m.editor.SetAutocompleteSource(&fuzzyselect.UserMentionSource{WithAtSymbol: true})
	cmd := m.editor.Enter(cmpcontroller.EnterOptions{
		Mode:                             cmpcontroller.ModeThreadComment,
		Prompt:                           constants.ThreadCommentPrompt,
		Repo:                             m.repoRef(),
		EnterFetch:                       cmpcontroller.FetchSilent,
		ConfirmDiscardOnCancel:           true,
		HideAutocompleteWhenContextEmpty: true,
	})
	return cmd
}

func (m *Model) getIndentedContentWidth() int {
	return m.width - 2*m.ctx.Styles.Sidebar.ContentPadding
}

func (m *Model) GetIsApproving() bool {
	return m.editor.Mode() == cmpcontroller.ModeApprove
}

func (m *Model) SetIsApproving(isApproving bool) tea.Cmd {
	if m.pr == nil {
		return nil
	}

	if !isApproving {
		if m.editor.Mode() == cmpcontroller.ModeApprove {
			m.editor.Exit()
		}
		return nil
	}

	m.editor.SetAutocompleteSource(&fuzzyselect.UserMentionSource{WithAtSymbol: true})
	cmd := m.editor.Enter(cmpcontroller.EnterOptions{
		Mode:                             cmpcontroller.ModeApprove,
		Prompt:                           constants.ApprovalPrompt,
		InitialValue:                     m.ctx.Config.Defaults.PrApproveComment,
		Repo:                             m.repoRef(),
		EnterFetch:                       cmpcontroller.FetchSilent,
		ConfirmDiscardOnCancel:           true,
		HideAutocompleteWhenContextEmpty: true,
	})
	return cmd
}

func (m *Model) GetIsAssigning() bool {
	return m.editor.Mode() == cmpcontroller.ModeAssign
}

func (m *Model) SetIsAssigning(isAssigning bool) tea.Cmd {
	if m.pr == nil {
		return nil
	}

	if !isAssigning {
		if m.editor.Mode() == cmpcontroller.ModeAssign {
			m.editor.Exit()
		}
		return nil
	}

	m.editor.SetAutocompleteSource(&fuzzyselect.UserMentionSource{WithAtSymbol: false})
	cmd := m.editor.Enter(cmpcontroller.EnterOptions{
		Mode:                             cmpcontroller.ModeAssign,
		Prompt:                           constants.EditAssigneesPrompt,
		InitialValue:                     strings.Join(m.prAssignees(), "\n"),
		Repo:                             m.repoRef(),
		EnterFetch:                       cmpcontroller.FetchSilent,
		HideAutocompleteWhenContextEmpty: false,
	})
	m.editor.ShowCompletions()
	return cmd
}

func (m *Model) GetIsRequestingReview() bool {
	return m.editor.Mode() == cmpcontroller.ModeRequestReview
}

func (m *Model) SetIsRequestingReview(isRequestingReview bool) tea.Cmd {
	if m.pr == nil {
		return nil
	}

	if !isRequestingReview {
		if m.editor.Mode() == cmpcontroller.ModeRequestReview {
			m.editor.Exit()
		}
		return nil
	}

	m.editor.SetAutocompleteSource(&fuzzyselect.UserMentionSource{WithAtSymbol: false})
	cmd := m.editor.Enter(cmpcontroller.EnterOptions{
		Mode:                             cmpcontroller.ModeRequestReview,
		Prompt:                           constants.RequestReviewPrompt,
		InitialValue:                     strings.Join(m.prReviewers(), "\n"),
		Repo:                             m.repoRef(),
		EnterFetch:                       cmpcontroller.FetchSilent,
		HideAutocompleteWhenContextEmpty: false,
	})
	m.editor.ShowCompletions()
	return cmd
}

func (m *Model) prAssignees() []string {
	var assignees []string
	for _, n := range m.pr.Data.Primary.Assignees.Nodes {
		assignees = append(assignees, n.Login)
	}
	return assignees
}

func (m *Model) prReviewers() []string {
	var reviewers []string
	for _, n := range m.pr.Data.Primary.ReviewRequests.Nodes {
		if reviewer := n.GetReviewerDisplayName(); reviewer != "" {
			reviewers = append(reviewers, reviewer)
		}
	}
	return reviewers
}

func (m *Model) assigneeChanges(next []string) ([]string, []string) {
	return stringListChanges(m.prAssignees(), next)
}

func (m *Model) reviewerChanges(next []string) ([]string, []string) {
	return stringListChanges(m.prReviewers(), next)
}

func stringListChanges(currentItems []string, next []string) ([]string, []string) {
	current := map[string]bool{}
	for _, login := range currentItems {
		current[login] = true
	}

	nextSet := map[string]bool{}
	added := []string{}
	for _, login := range next {
		if login == "" || nextSet[login] {
			continue
		}
		nextSet[login] = true
		if !current[login] {
			added = append(added, login)
		}
	}

	removed := []string{}
	for _, login := range currentItems {
		if !nextSet[login] {
			removed = append(removed, login)
		}
	}

	return added, removed
}

func (m *Model) GoToFirstTab() {
	m.GoToTab(0)
}

func (m *Model) GoToTab(index int) {
	m.carousel.SetCursor(index)
	m.viewState.selectedTabIndex = m.carousel.Cursor()
}

func (m *Model) GoToActivityTab() {
	m.GoToTab(1) // Activity is the second tab (index 1)
}

func (m *Model) RestoreSelectedTab() {
	m.carousel.SetCursor(m.viewState.selectedTabIndex)
}

func (m Model) HasRememberedViewState() bool {
	return m.rememberedViewState
}

func (m *Model) moveToPrevTab() {
	m.carousel.MoveLeft()
	m.viewState.selectedTabIndex = m.carousel.Cursor()
}

func (m *Model) moveToNextTab() {
	m.carousel.MoveRight()
	m.viewState.selectedTabIndex = m.carousel.Cursor()
}

func (m Model) SelectedTabIndex() int {
	return m.carousel.Cursor()
}

func (m Model) SelectedTab() string {
	return m.carousel.SelectedItem()
}

func (m Model) IsActivityTab() bool {
	return m.carousel.SelectedItem() == tabs[1]
}

func (m Model) HasFocusedReviewThread() bool {
	_, ok := m.FocusedReviewThread()
	return ok
}

func (m Model) FocusedReviewThread() (*data.ReviewThreadWithComments, bool) {
	if m.pr == nil || m.pr.Data == nil || !m.pr.Data.IsEnriched {
		return nil, false
	}
	threads := m.pr.Data.Enriched.ReviewThreads.Nodes
	if m.focusedThread < 0 || m.focusedThread >= len(threads) {
		return nil, false
	}
	if len(threads[m.focusedThread].Comments.Nodes) == 0 {
		return nil, false
	}

	return &threads[m.focusedThread], true
}

func (m *Model) FocusNextReviewThread() bool {
	return m.focusReviewThread(1)
}

func (m *Model) FocusPrevReviewThread() bool {
	return m.focusReviewThread(-1)
}

func (m *Model) focusReviewThread(delta int) bool {
	if m.pr == nil || m.pr.Data == nil || !m.pr.Data.IsEnriched {
		return false
	}
	focusTargets := m.activityFocusTargets
	if len(focusTargets) == 0 {
		focusTargets = m.activityFocusTargetsFromData()
	}
	if len(focusTargets) == 0 {
		return false
	}

	current := -1
	for i, target := range focusTargets {
		if target == m.focusedThread {
			current = i
			break
		}
	}
	if current == -1 {
		m.focusedThread = focusTargets[0]
		return true
	}

	next := current + delta
	if next < 0 || next >= len(focusTargets) {
		return false
	}
	m.focusedThread = focusTargets[next]
	return true
}

func (m Model) activityFocusTargetsFromData() []int {
	if m.pr == nil || m.pr.Data == nil || !m.pr.Data.IsEnriched {
		return nil
	}
	targets := []int{}
	for i, thread := range m.pr.Data.Enriched.ReviewThreads.Nodes {
		if len(thread.Comments.Nodes) > 0 {
			targets = append(targets, i)
		}
	}
	return append(targets, focusedNewComment)
}

func (m Model) IsNewCommentFocused() bool {
	return m.focusedThread == focusedNewComment
}

func (m *Model) FocusNewComment() {
	m.focusedThread = focusedNewComment
}

func (m *Model) SetInitialActivityFocus() {
	if m.focusedThread == 0 {
		m.FocusNewComment()
	}
}

func (m *Model) ToggleFocusedReviewThreadResolved() tea.Cmd {
	thread, ok := m.FocusedReviewThread()
	if !ok || m.pr == nil {
		return nil
	}

	sid := m.sectionIdentifier()
	return tasks.ToggleReviewThreadResolved(
		m.ctx,
		sid,
		m.pr.Data.Primary,
		thread.Id,
		thread.IsResolved,
	)
}

func (m Model) sectionIdentifier() tasks.SectionIdentifier {
	sectionType := m.sectionType
	if sectionType == "" {
		sectionType = prssection.SectionType
	}
	return tasks.SectionIdentifier{Id: m.sectionId, Type: sectionType}
}

func (m *Model) clampFocusedReviewThread() {
	if m.pr == nil || m.pr.Data == nil || !m.pr.Data.IsEnriched {
		m.FocusNewComment()
		return
	}
	threads := m.pr.Data.Enriched.ReviewThreads.Nodes
	if m.IsNewCommentFocused() {
		return
	}
	if m.focusedThread >= 0 && m.focusedThread < len(threads) && len(threads[m.focusedThread].Comments.Nodes) > 0 {
		return
	}
	m.FocusNewComment()
}

func (m Model) IsOverviewTab() bool {
	return m.carousel.SelectedItem() == tabs[0]
}

func (m Model) IsChecksTab() bool {
	return m.carousel.SelectedItem() == tabs[2]
}

func (m Model) WatchReason() WatchReason {
	switch {
	case m.IsOverviewTab(), m.IsChecksTab():
		return WatchChecks
	case m.IsActivityTab():
		return WatchActivity
	default:
		return WatchNone
	}
}

func (m Model) CurrentPRURL() string {
	if m.pr == nil || m.pr.Data == nil || m.pr.Data.Primary == nil {
		return ""
	}
	return m.pr.Data.Primary.Url
}

func (m *Model) SetSummaryViewMore() {
	m.summaryViewMore = true
}

func (m *Model) SetSummaryViewLess() {
	m.summaryViewMore = false
}

func (m *Model) ToggleActivitySnippetsExpanded() {
	m.viewState.activitySnippetsExpanded = !m.viewState.activitySnippetsExpanded
	m.invalidateActivityCache()
}

func (m *Model) ToggleActivityItemsCollapsed() {
	m.viewState.activityItemsCollapsed = !m.viewState.activityItemsCollapsed
	m.invalidateActivityCache()
}

func (m *Model) SetEnrichedPR(data data.EnrichedPullRequestData) {
	if m.pr == nil || m.pr.Data == nil || m.pr.Data.Primary == nil {
		return
	}
	if m.pr.Data.Primary.Url == data.Url {
		primary := data.ToPullRequestData()
		primary.Commits = m.pr.Data.Primary.Commits
		m.pr.Data.Primary = &primary
		m.pr.Data.Enriched = data
		m.pr.Data.IsEnriched = true
		m.invalidateActivityCache()
		m.reviewDiffCache = map[string][]reviewDiffLine{}
		m.clampFocusedReviewThread()
	}
}

func (m *Model) GetIsLabeling() bool {
	return m.editor.Mode() == cmpcontroller.ModeLabel
}

// SetIsLabeling enters or exits labeling mode
func (m *Model) SetIsLabeling(isLabeling bool) tea.Cmd {
	if m.pr == nil {
		return nil
	}

	if !isLabeling {
		if m.editor.Mode() == cmpcontroller.ModeLabel {
			m.editor.Exit()
		}
		return nil
	}

	labels := make([]string, 0, len(m.pr.Data.Primary.Labels.Nodes)+1)
	for _, label := range m.pr.Data.Primary.Labels.Nodes {
		labels = append(labels, label.Name)
	}
	labels = append(labels, "")

	m.editor.SetAutocompleteSource(&fuzzyselect.LabelSource{})
	cmd := m.editor.Enter(cmpcontroller.EnterOptions{
		Mode:                             cmpcontroller.ModeLabel,
		Prompt:                           constants.LabelPrompt,
		InitialValue:                     strings.Join(labels, ", "),
		Repo:                             m.repoRef(),
		EnterFetch:                       cmpcontroller.FetchSilent,
		HideAutocompleteWhenContextEmpty: false,
		ConfirmDiscardOnCancel:           false,
	})
	m.editor.ShowCompletions()
	return cmd
}

func (m *Model) repoRef() cmpcontroller.RepoRef {
	owner, repo := m.pr.Data.Primary.GetRepoNameAndOwner()
	return cmpcontroller.RepoRef{
		NameWithOwner: m.pr.Data.Primary.GetRepoNameWithOwner(),
		Owner:         owner,
		Name:          repo,
	}
}

func (m *Model) hasData() bool {
	return m.pr != nil && m.pr.Data != nil
}
