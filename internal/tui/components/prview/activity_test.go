package prview

import (
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dehub/v4/internal/data"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/prrow"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/markdown"
)

func TestRenderActivityReusesCacheAcrossFocusChanges(t *testing.T) {
	markdown.InitializeMarkdownStyle(true)
	m := newDiffPreviewTestModel(t)
	m.SetRow(activityTestPR())
	m.GoToActivityTab()

	first := m.renderActivity()
	firstCache := m.activityCache
	require.NotEmpty(t, first)
	require.NotEmpty(t, firstCache.fingerprint)
	require.Len(t, firstCache.activities, 2)
	require.Len(t, m.activityBodyCache, 1)

	require.True(t, m.FocusPrevReviewThread())
	second := m.renderActivity()
	require.NotEmpty(t, second)
	require.Equal(t, firstCache.fingerprint, m.activityCache.fingerprint)
	require.Equal(t, firstCache.activities, m.activityCache.activities)
	require.Contains(t, second, "focused")
	require.Len(t, m.activityBodyCache, 2)

	third := m.renderActivity()
	require.Equal(t, second, third)
	require.Len(t, m.activityBodyCache, 2)
}

func TestSetEnrichedPRInvalidatesActivityCache(t *testing.T) {
	markdown.InitializeMarkdownStyle(true)
	m := newDiffPreviewTestModel(t)
	pr := activityTestPR()
	m.SetRow(pr)
	m.GoToActivityTab()

	_ = m.renderActivity()
	require.NotEmpty(t, m.activityCache.fingerprint)
	require.NotEmpty(t, m.activityBodyCache)

	updated := pr.Enriched
	updated.Comments.Nodes = append(updated.Comments.Nodes, data.Comment{
		Body:      "new comment",
		UpdatedAt: time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
	})
	m.SetEnrichedPR(updated)

	require.Empty(t, m.activityCache.fingerprint)
	require.Empty(t, m.activityBodyCache)
}

func TestActivityBodyViewCacheReusesPaddedBody(t *testing.T) {
	markdown.InitializeMarkdownStyle(true)
	m := newDiffPreviewTestModel(t)
	m.SetRow(activityTestPR())
	m.GoToActivityTab()

	first := m.BodyView()
	require.NotEmpty(t, first)
	require.Len(t, m.activityBodyCache, 2)

	second := m.BodyView()
	require.Equal(t, first, second)
	require.Len(t, m.activityBodyCache, 2)
}

func TestToggleActivitySnippetsExpandedInvalidatesActivityCache(t *testing.T) {
	markdown.InitializeMarkdownStyle(true)
	m := newDiffPreviewTestModel(t)
	pr := activityTestPR()
	pr.Enriched.ReviewThreads.Nodes[0].Comments.Nodes[0].DiffHunk = longReviewDiffHunk(15)
	m.SetRow(pr)
	m.GoToActivityTab()

	collapsed := ansi.Strip(m.renderActivity())
	require.Contains(t, collapsed, "Press e to expand 4 more lines...")
	require.NotContains(t, collapsed, "line 14")
	require.NotEmpty(t, m.activityCache.fingerprint)

	m.ToggleActivitySnippetsExpanded()
	require.Empty(t, m.activityCache.fingerprint)
	expanded := ansi.Strip(m.renderActivity())
	require.Contains(t, expanded, "line 14")
	require.Contains(t, expanded, "Press e to collapse")
}

func TestReviewThreadHeaderShowsUnresolvedBadge(t *testing.T) {
	markdown.InitializeMarkdownStyle(true)
	m := newDiffPreviewTestModel(t)
	m.SetRow(activityTestPR())
	m.GoToActivityTab()

	plain := ansi.Strip(m.renderActivity())
	require.Contains(t, plain, "unresolved")
}

func TestReviewThreadHeaderShowsResolvedBadge(t *testing.T) {
	markdown.InitializeMarkdownStyle(true)
	m := newDiffPreviewTestModel(t)
	pr := activityTestPR()
	pr.Enriched.ReviewThreads.Nodes[0].IsResolved = true
	m.SetRow(pr)
	m.GoToActivityTab()

	plain := ansi.Strip(m.renderActivity())
	require.Contains(t, plain, "resolved")
	require.NotContains(t, plain, "unresolved")
}

func TestToggleActivityItemsCollapsedInvalidatesActivityCache(t *testing.T) {
	markdown.InitializeMarkdownStyle(true)
	m := newDiffPreviewTestModel(t)
	m.SetRow(activityTestPRWithResolvedAndStandaloneComments())
	m.GoToActivityTab()

	_ = m.renderActivity()
	require.NotEmpty(t, m.activityCache.fingerprint)
	require.NotEmpty(t, m.activityBodyCache)

	m.ToggleActivityItemsCollapsed()
	require.Empty(t, m.activityCache.fingerprint)
	require.Empty(t, m.activityBodyCache)
}

func TestActivityItemsCollapsedKeepsUnresolvedThreadsExpanded(t *testing.T) {
	markdown.InitializeMarkdownStyle(true)
	m := newDiffPreviewTestModel(t)
	m.SetRow(activityTestPRWithResolvedAndStandaloneComments())
	m.GoToActivityTab()
	m.ToggleActivityItemsCollapsed()

	plain := ansi.Strip(m.renderActivity())
	require.Contains(t, plain, "open thread body")
	require.Contains(t, plain, "unresolved")
}

func TestActivityItemsCollapsedCompactsResolvedThreadsAndStandaloneComments(t *testing.T) {
	markdown.InitializeMarkdownStyle(true)
	m := newDiffPreviewTestModel(t)
	m.SetRow(activityTestPRWithResolvedAndStandaloneComments())
	m.GoToActivityTab()

	expanded := ansi.Strip(m.renderActivity())
	require.Contains(t, expanded, "closed thread body")
	require.Contains(t, expanded, "standalone comment body")

	m.ToggleActivityItemsCollapsed()
	collapsed := ansi.Strip(m.renderActivity())
	require.Contains(t, collapsed, "resolved")
	require.Contains(t, collapsed, "standalone")
	require.NotContains(t, collapsed, "closed thread body")
	require.NotContains(t, collapsed, "standalone comment body")
}

func TestActivityItemsCollapsedCompactsReviewSummaries(t *testing.T) {
	markdown.InitializeMarkdownStyle(true)
	m := newDiffPreviewTestModel(t)
	m.SetRow(activityTestPR())
	m.GoToActivityTab()

	expanded := ansi.Strip(m.renderActivity())
	require.Contains(t, expanded, "reviewer reviewed")
	require.Contains(t, expanded, "review body")

	m.ToggleActivityItemsCollapsed()
	collapsed := ansi.Strip(m.renderActivity())
	require.Contains(t, collapsed, "reviewer reviewed")
	require.NotContains(t, collapsed, "review body")
}

func activityTestPR() *prrow.Data {
	updatedAt := time.Date(2024, 1, 2, 10, 0, 0, 0, time.UTC)
	primary := data.PullRequestData{
		Number: 1,
		Url:    "https://github.com/owner/repo/pull/1",
		Reviews: data.Reviews{Nodes: []data.Review{{
			Body:      "review body",
			State:     "COMMENTED",
			UpdatedAt: updatedAt.Add(time.Hour),
		}}},
	}
	primary.Repository.NameWithOwner = "owner/repo"
	primary.Reviews.Nodes[0].Author.Login = "reviewer"

	enriched := data.EnrichedPullRequestData{
		Number: 1,
		Url:    primary.Url,
		ReviewThreads: data.ReviewThreadsWithComments{Nodes: []data.ReviewThreadWithComments{{
			Id:   "thread-1",
			Path: "file.go",
			Line: 12,
			Comments: data.ReviewComments{Nodes: []data.ReviewComment{{
				Body:      "thread comment",
				DiffHunk:  "@@ -12,1 +12,1 @@\n-old\n+new",
				UpdatedAt: updatedAt,
			}}},
		}}},
	}
	enriched.ReviewThreads.Nodes[0].Comments.Nodes[0].Author.Login = "commenter"

	return &prrow.Data{
		Primary:    &primary,
		Enriched:   enriched,
		IsEnriched: true,
	}
}

func activityTestPRWithResolvedAndStandaloneComments() *prrow.Data {
	pr := activityTestPR()
	updatedAt := time.Date(2024, 1, 2, 10, 0, 0, 0, time.UTC)
	pr.Enriched.ReviewThreads.Nodes[0].Comments.Nodes[0].Body = "open thread body"
	pr.Enriched.ReviewThreads.Nodes = append(pr.Enriched.ReviewThreads.Nodes, data.ReviewThreadWithComments{
		Id:         "thread-2",
		Path:       "file.go",
		Line:       24,
		IsResolved: true,
		Comments: data.ReviewComments{Nodes: []data.ReviewComment{{
			Body:      "closed thread body",
			DiffHunk:  "@@ -24,1 +24,1 @@\n-old\n+new",
			UpdatedAt: updatedAt.Add(time.Minute),
		}}},
	})
	pr.Enriched.ReviewThreads.Nodes[1].Comments.Nodes[0].Author.Login = "resolver"
	pr.Enriched.Comments.Nodes = append(pr.Enriched.Comments.Nodes, data.Comment{
		Body:      "standalone comment body",
		UpdatedAt: updatedAt.Add(2 * time.Minute),
	})
	pr.Enriched.Comments.Nodes[0].Author.Login = "standalone"
	return pr
}
