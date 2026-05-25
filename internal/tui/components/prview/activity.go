package prview

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"

	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/cmpcontroller"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/markdown"
	"github.com/dlvhdr/gh-dash/v4/internal/utils"
)

type cachedActivity struct {
	UpdatedAt      time.Time
	RenderedString string
	RenderedCard   string
	FocusTarget    int
	Thread         *cachedReviewThread
}

type cachedReviewThread struct {
	UnfocusedCard string
	FocusedCard   string
	Header        string
	Body          string
	Width         int
}

func (m *Model) renderActivity() string {
	width := m.getIndentedContentWidth()
	bodyStyle := lipgloss.NewStyle()

	if !m.pr.Data.IsEnriched {
		return bodyStyle.Render("Loading...")
	}

	activities := m.cachedActivities(width)
	cacheKey, canCacheBody := m.activityBodyCacheKey(m.activityCache.fingerprint)
	if canCacheBody {
		if body, ok := m.activityBodyCache[cacheKey]; ok {
			m.activityFocusTargets = m.cachedActivityFocusTargets(activities)
			return body
		}
	}

	m.activityFocusTargets = nil
	var renderedActivities []string
	if len(activities) == 0 {
		renderedActivities = append(renderedActivities, renderEmptyState())
	} else {
		for _, activity := range activities {
			if activity.FocusTarget >= 0 {
				m.activityFocusTargets = append(m.activityFocusTargets, activity.FocusTarget)
			}
			renderedActivities = append(renderedActivities, m.renderCachedActivity(activity))
		}
	}
	m.activityFocusTargets = append(m.activityFocusTargets, focusedNewComment)
	renderedActivities = append(renderedActivities, m.renderNewCommentCard())

	title := m.ctx.Styles.Common.MainTextStyle.MarginBottom(1).Underline(true).Render(
		fmt.Sprintf("%s  %d activity items", constants.CommentsIcon, len(activities)),
	)
	body := lipgloss.JoinVertical(lipgloss.Left, renderedActivities...)
	body = lipgloss.JoinVertical(lipgloss.Left, title, body)

	renderedBody := bodyStyle.Render(body)
	if canCacheBody {
		if m.activityBodyCache == nil {
			m.activityBodyCache = map[string]string{}
		}
		m.activityBodyCache[cacheKey] = renderedBody
	}
	return renderedBody
}

func (m *Model) cachedActivityBodyView() string {
	if !m.pr.Data.IsEnriched {
		return lipgloss.NewStyle().Padding(0, m.ctx.Styles.Sidebar.ContentPadding).Render(m.renderActivity())
	}

	width := m.getIndentedContentWidth()
	activities := m.cachedActivities(width)
	cacheKey, canCacheBody := m.activityBodyCacheKey(m.activityCache.fingerprint)
	if canCacheBody {
		cacheKey = "bodyview:" + cacheKey
		if body, ok := m.activityBodyCache[cacheKey]; ok {
			m.activityFocusTargets = m.cachedActivityFocusTargets(activities)
			return body
		}
	}

	body := m.renderActivity()
	body = lipgloss.NewStyle().Padding(0, m.ctx.Styles.Sidebar.ContentPadding).Render(body)
	if canCacheBody {
		if m.activityBodyCache == nil {
			m.activityBodyCache = map[string]string{}
		}
		m.activityBodyCache[cacheKey] = body
	}
	return body
}

func (m *Model) cachedActivities(width int) []cachedActivity {
	fingerprint := m.activityFingerprint(width)
	if m.activityCache.width == width && m.activityCache.fingerprint == fingerprint {
		return m.activityCache.activities
	}

	markdownRenderer := markdown.GetMarkdownRenderer(max(1, width-4))
	activities := m.buildCachedActivities(markdownRenderer)
	m.activityCache = activityRenderCache{
		width:       width,
		fingerprint: fingerprint,
		activities:  activities,
	}
	return activities
}

func (m *Model) buildCachedActivities(markdownRenderer glamour.TermRenderer) []cachedActivity {
	var activities []cachedActivity

	for i, review := range m.pr.Data.Enriched.ReviewThreads.Nodes {
		if len(review.Comments.Nodes) == 0 {
			continue
		}

		thread := reviewThread{
			Id:         review.Id,
			Path:       review.Path,
			Line:       review.Line,
			IsOutdated: review.IsOutdated,
			IsResolved: review.IsResolved,
		}
		for _, c := range review.Comments.Nodes {
			if thread.DiffHunk == "" {
				thread.DiffHunk = c.DiffHunk
			}
			thread.Comments = append(thread.Comments, comment{
				Author:    c.Author.Login,
				Body:      c.Body,
				UpdatedAt: c.UpdatedAt,
			})
		}

		cachedThread, err := m.cacheReviewThread(thread, markdownRenderer)
		if err != nil {
			continue
		}
		activities = append(activities, cachedActivity{
			UpdatedAt:   thread.UpdatedAt(),
			FocusTarget: i,
			Thread:      &cachedThread,
		})
	}

	for _, c := range m.pr.Data.Enriched.Comments.Nodes {
		comment := comment{
			Author:    c.Author.Login,
			Body:      c.Body,
			UpdatedAt: c.UpdatedAt,
		}
		renderedComment, err := m.renderComment(comment, markdownRenderer)
		if err != nil {
			continue
		}
		activities = append(activities, cachedActivity{
			UpdatedAt:      comment.UpdatedAt,
			RenderedString: renderedComment,
			FocusTarget:    unfocusedActivity,
		})
	}

	for _, review := range m.pr.Data.Primary.Reviews.Nodes {
		body, err := markdownRenderer.Render(review.Body)
		if err != nil {
			continue
		}
		header := m.renderReviewHeader(review)
		activities = append(activities, cachedActivity{
			UpdatedAt:   review.UpdatedAt,
			FocusTarget: unfocusedActivity,
			RenderedCard: m.renderActivityCard(
				m.getIndentedContentWidth(),
				m.reviewBorderColor(review.State),
				header,
				body,
			),
		})
	}

	sort.Slice(activities, func(i, j int) bool {
		return activities[i].UpdatedAt.Before(activities[j].UpdatedAt)
	})
	return activities
}

func (m *Model) renderCachedActivity(activity cachedActivity) string {
	if activity.Thread != nil {
		thread := activity.Thread
		body := thread.Body
		if activity.FocusTarget == m.focusedThread {
			if m.editor.Mode() == cmpcontroller.ModeThreadComment {
				border := m.ctx.Theme.WarningText
				body = lipgloss.JoinVertical(
					lipgloss.Left,
					body,
					lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintBorder).Render("─"),
					m.renderEmbeddedActivityEditor(thread.Width),
				)
				return m.renderActivityCard(thread.Width, border, thread.Header, body)
			}
			return thread.FocusedCard
		}
		return thread.UnfocusedCard
	}

	if activity.RenderedCard != "" {
		return activity.RenderedCard
	}

	return activity.RenderedString
}

func (m *Model) activityBodyCacheKey(fingerprint string) (string, bool) {
	if m.editor.Mode() != cmpcontroller.ModeNone {
		return "", false
	}
	return fmt.Sprintf("%s|focus:%d", fingerprint, m.focusedThread), true
}

func (m *Model) cachedActivityFocusTargets(activities []cachedActivity) []int {
	targets := make([]int, 0, len(activities)+1)
	for _, activity := range activities {
		if activity.FocusTarget >= 0 {
			targets = append(targets, activity.FocusTarget)
		}
	}
	return append(targets, focusedNewComment)
}

func (m *Model) activityFingerprint(width int) string {
	if m.pr == nil || m.pr.Data == nil || !m.pr.Data.IsEnriched {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "w:%d|url:%s|", width, m.pr.Data.Primary.Url)
	for _, thread := range m.pr.Data.Enriched.ReviewThreads.Nodes {
		fmt.Fprintf(
			&b,
			"t:%s:%t:%t:%s:%d:%d|",
			thread.Id,
			thread.IsOutdated,
			thread.IsResolved,
			thread.Path,
			thread.Line,
			len(thread.Comments.Nodes),
		)
		for _, comment := range thread.Comments.Nodes {
			fmt.Fprintf(
				&b,
				"tc:%s:%s:%d:%d|",
				comment.Author.Login,
				comment.UpdatedAt.Format(time.RFC3339Nano),
				len(comment.DiffHunk),
				len(comment.Body),
			)
		}
	}
	for _, comment := range m.pr.Data.Enriched.Comments.Nodes {
		fmt.Fprintf(
			&b,
			"c:%s:%s:%d|",
			comment.Author.Login,
			comment.UpdatedAt.Format(time.RFC3339Nano),
			len(comment.Body),
		)
	}
	for _, review := range m.pr.Data.Primary.Reviews.Nodes {
		fmt.Fprintf(
			&b,
			"r:%s:%s:%s:%d|",
			review.Author.Login,
			review.State,
			review.UpdatedAt.Format(time.RFC3339Nano),
			len(review.Body),
		)
	}
	return b.String()
}

func (m *Model) invalidateActivityCache() {
	m.activityCache = activityRenderCache{}
	m.activityBodyCache = nil
}

func renderEmptyState() string {
	return lipgloss.NewStyle().Italic(true).Render("No comments...")
}

type comment struct {
	Author    string
	UpdatedAt time.Time
	Body      string
	Path      *string
	Line      *int
}

type reviewThread struct {
	Id         string
	Path       string
	Line       int
	IsOutdated bool
	IsResolved bool
	DiffHunk   string
	Comments   []comment
}

func (thread reviewThread) UpdatedAt() time.Time {
	updatedAt := time.Time{}
	for _, comment := range thread.Comments {
		if comment.UpdatedAt.After(updatedAt) {
			updatedAt = comment.UpdatedAt
		}
	}

	return updatedAt
}

func (m *Model) renderComment(
	comment comment,
	markdownRenderer glamour.TermRenderer,
) (string, error) {
	width := m.getIndentedContentWidth()
	authorAndTime := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.ctx.Styles.Common.MainTextStyle.Render(comment.Author),
		" ",
		lipgloss.NewStyle().
			Foreground(m.ctx.Theme.FaintText).
			Render(utils.TimeElapsed(comment.UpdatedAt)),
	)

	var metadata []string
	metadata = append(metadata, authorAndTime)
	if comment.Path != nil && comment.Line != nil {
		metadata = append(metadata, lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).Render(
			fmt.Sprintf(
				"%s#l%d",
				*comment.Path,
				*comment.Line,
			),
		))
	}

	body := lineCleanupRegex.ReplaceAllString(comment.Body, "")
	body, err := markdownRenderer.Render(body)

	return m.renderActivityCard(
		width,
		m.ctx.Theme.SecondaryBorder,
		lipgloss.JoinHorizontal(
			lipgloss.Top,
			m.ctx.Styles.Common.CommentGlyph,
			" ",
			lipgloss.JoinVertical(lipgloss.Left, metadata...),
		),
		body,
	), err
}

func (m *Model) cacheReviewThread(
	thread reviewThread,
	markdownRenderer glamour.TermRenderer,
) (cachedReviewThread, error) {
	width := m.getIndentedContentWidth()
	if len(thread.Comments) == 0 {
		return cachedReviewThread{}, nil
	}

	header := m.renderReviewThreadHeader(thread, false)
	focusedHeader := m.renderReviewThreadHeader(thread, true)

	var renderedComments []string
	if preview := m.renderReviewDiffPreview(thread.Id, thread.Path, thread.DiffHunk, max(1, width-4)); preview != "" {
		renderedComments = append(renderedComments, preview)
	}
	for i, comment := range thread.Comments {
		if i == 0 && len(renderedComments) > 0 {
			renderedComments = append(renderedComments, lipgloss.NewStyle().
				Foreground(m.ctx.Theme.FaintBorder).
				Render("─"))
		}
		renderedComment, err := m.renderThreadComment(comment, markdownRenderer)
		if err != nil {
			return cachedReviewThread{}, err
		}

		if i > 0 {
			renderedComments = append(renderedComments, lipgloss.NewStyle().
				Foreground(m.ctx.Theme.FaintBorder).
				Render("─"))
		}
		renderedComments = append(renderedComments, renderedComment)
	}

	body := lipgloss.JoinVertical(lipgloss.Left, renderedComments...)
	return cachedReviewThread{
		UnfocusedCard: m.renderActivityCard(width, m.ctx.Theme.SecondaryBorder, header, body),
		FocusedCard:   m.renderActivityCard(width, m.ctx.Theme.WarningText, focusedHeader, body),
		Header:        focusedHeader,
		Body:          body,
		Width:         width,
	}, nil
}

func (m *Model) renderReviewThreadHeader(thread reviewThread, isFocused bool) string {
	var badges []string
	if isFocused {
		badges = append(badges, "focused")
	}
	badges = append(badges, fmt.Sprintf("%d comments", len(thread.Comments)))
	if thread.IsResolved {
		badges = append(badges, m.ctx.Styles.Common.SuccessGlyph+" resolved")
	} else {
		badges = append(badges, m.ctx.Styles.Common.UnresolvedGlyph+" unresolved")
	}
	if thread.IsOutdated {
		badges = append(badges, "outdated")
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.JoinHorizontal(
			lipgloss.Top,
			m.ctx.Styles.Common.CommentGlyph,
			" ",
			m.ctx.Styles.Common.MainTextStyle.Render("Review thread"),
			" ",
			lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).Render(joinMetadata(badges)),
		),
		lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).Render(
			fmt.Sprintf("%s#l%d", thread.Path, thread.Line),
		),
	)
}

func (m *Model) renderNewCommentCard() string {
	width := m.getIndentedContentWidth()
	border := m.ctx.Theme.SecondaryBorder
	badges := []string{"PR comment"}
	if m.IsNewCommentFocused() {
		border = m.ctx.Theme.WarningText
		badges = append([]string{"focused"}, badges...)
	}

	header := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.ctx.Styles.Common.CommentGlyph,
		" ",
		m.ctx.Styles.Common.MainTextStyle.Render("New Comment"),
		" ",
		lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).Render(joinMetadata(badges)),
	)

	body := ""
	if m.IsNewCommentFocused() && m.editor.Mode() == cmpcontroller.ModeComment {
		body = m.renderEmbeddedActivityEditor(width)
	}

	return m.renderActivityCard(
		width,
		border,
		header,
		body,
	)
}

func (m *Model) renderEmbeddedActivityEditor(cardWidth int) string {
	innerWidth := max(1, cardWidth-4)
	inputStyle := m.ctx.Styles.Sidebar.InputBox.Width(innerWidth)
	m.editor.SetWidth(max(1, innerWidth-inputStyle.GetHorizontalFrameSize()))
	return inputStyle.Render(m.editor.View())
}

func (m *Model) renderThreadComment(
	comment comment,
	markdownRenderer glamour.TermRenderer,
) (string, error) {
	header := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.ctx.Styles.Common.MainTextStyle.Render(comment.Author),
		" ",
		lipgloss.NewStyle().
			Foreground(m.ctx.Theme.FaintText).
			Render(utils.TimeElapsed(comment.UpdatedAt)),
	)

	body := lineCleanupRegex.ReplaceAllString(comment.Body, "")
	body, err := markdownRenderer.Render(body)
	return lipgloss.JoinVertical(lipgloss.Left, header, body), err
}

func joinMetadata(items []string) string {
	metadata := ""
	for i, item := range items {
		if i > 0 {
			metadata += " · "
		}
		metadata += item
	}

	return metadata
}

func (m *Model) renderActivityCard(
	width int,
	border compat.AdaptiveColor,
	header string,
	body string,
) string {
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		MarginBottom(1).
		Render(lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			"",
			body,
		))
}

func (m *Model) renderReview(
	review data.Review,
	markdownRenderer glamour.TermRenderer,
) (string, error) {
	width := m.getIndentedContentWidth()
	header := m.renderReviewHeader(review)
	body, err := markdownRenderer.Render(review.Body)
	return m.renderActivityCard(
		width,
		m.reviewBorderColor(review.State),
		header,
		body,
	), err
}

func (m *Model) reviewBorderColor(state string) compat.AdaptiveColor {
	switch state {
	case "APPROVED":
		return m.ctx.Theme.SuccessText
	case "CHANGES_REQUESTED":
		return m.ctx.Theme.ErrorText
	case "PENDING":
		return m.ctx.Theme.WarningText
	}

	return m.ctx.Theme.FaintBorder
}

func (m *Model) renderReviewHeader(review data.Review) string {
	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		m.renderReviewDecision(review.State),
		" ",
		m.ctx.Styles.Common.MainTextStyle.Render(review.Author.Login),
		" ",
		lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).Render(
			"reviewed "+utils.TimeElapsed(review.UpdatedAt),
		),
	)
}

func (m *Model) renderReviewDecision(decision string) string {
	switch decision {
	case "PENDING":
		return m.ctx.Styles.Common.WaitingGlyph
	case "COMMENTED":
		return lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).Render("󰈈")
	case "APPROVED":
		return m.ctx.Styles.Common.SuccessGlyph
	case "CHANGES_REQUESTED":
		return m.ctx.Styles.Common.FailureGlyph
	}

	return ""
}
