package prview

import (
	"fmt"
	"sort"
	"time"

	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"

	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/markdown"
	"github.com/dlvhdr/gh-dash/v4/internal/utils"
)

type RenderedActivity struct {
	UpdatedAt      time.Time
	RenderedString string
}

func (m *Model) renderActivity() string {
	width := m.getIndentedContentWidth()
	markdownRenderer := markdown.GetMarkdownRenderer(max(1, width-4))
	bodyStyle := lipgloss.NewStyle()

	var activities []RenderedActivity
	var comments []comment

	if !m.pr.Data.IsEnriched {
		return bodyStyle.Render("Loading...")
	}

	for _, review := range m.pr.Data.Enriched.ReviewThreads.Nodes {
		path := review.Path
		line := review.Line
		for _, c := range review.Comments.Nodes {
			comments = append(comments, comment{
				Author:    c.Author.Login,
				Body:      c.Body,
				UpdatedAt: c.UpdatedAt,
				Path:      &path,
				Line:      &line,
			})
		}
	}

	for _, c := range m.pr.Data.Enriched.Comments.Nodes {
		comments = append(comments, comment{
			Author:    c.Author.Login,
			Body:      c.Body,
			UpdatedAt: c.UpdatedAt,
		})
	}

	for _, comment := range comments {
		renderedComment, err := m.renderComment(comment, markdownRenderer)
		if err != nil {
			continue
		}
		activities = append(activities, RenderedActivity{
			UpdatedAt:      comment.UpdatedAt,
			RenderedString: renderedComment,
		})
	}

	for _, review := range m.pr.Data.Primary.Reviews.Nodes {
		renderedReview, err := m.renderReview(review, markdownRenderer)
		if err != nil {
			continue
		}
		activities = append(activities, RenderedActivity{
			UpdatedAt:      review.UpdatedAt,
			RenderedString: renderedReview,
		})
	}

	// Sort oldest first.
	sort.Slice(activities, func(i, j int) bool {
		return activities[i].UpdatedAt.Before(activities[j].UpdatedAt)
	})

	body := ""
	if len(activities) == 0 {
		body = renderEmptyState()
	} else {
		var renderedActivities []string
		for _, activity := range activities {
			renderedActivities = append(renderedActivities, activity.RenderedString)
		}
		title := m.ctx.Styles.Common.MainTextStyle.MarginBottom(1).Underline(true).Render(
			fmt.Sprintf("%s  %d comments", constants.CommentsIcon, len(activities)),
		)
		body = lipgloss.JoinVertical(lipgloss.Left, renderedActivities...)
		body = lipgloss.JoinVertical(lipgloss.Left, title, body)
	}

	return bodyStyle.Render(body)
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
