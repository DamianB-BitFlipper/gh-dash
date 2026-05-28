package prview

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dehub/v4/internal/config"
	"github.com/dlvhdr/gh-dehub/v4/internal/data"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/prrow"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/theme"
)

func TestParseReviewDiffHunk(t *testing.T) {
	diffHunk := `@@ -344,4 +344,4 @@ func committedReservationKey() string {
 return f"v1-sandbox:{sandbox_id}:committed-capacity-reservation"
-def old_key(self) -> str:
+def _committed_reservation_key_pattern(self) -> str:
     return "v1-sandbox:*:committed-capacity-reservation"`

	lines, err := parseReviewDiffHunk("capacity_store.py", diffHunk)
	require.NoError(t, err)
	require.Len(t, lines, 4)
	require.Equal(t, reviewDiffLine{OldLine: 344, NewLine: 344, Prefix: ' ', Text: "return f\"v1-sandbox:{sandbox_id}:committed-capacity-reservation\""}, lines[0])
	require.Equal(t, reviewDiffLine{OldLine: 345, Prefix: '-', Text: "def old_key(self) -> str:"}, lines[1])
	require.Equal(t, reviewDiffLine{NewLine: 345, Prefix: '+', Text: "def _committed_reservation_key_pattern(self) -> str:"}, lines[2])
	require.Equal(t, reviewDiffLine{OldLine: 346, NewLine: 346, Prefix: ' ', Text: "    return \"v1-sandbox:*:committed-capacity-reservation\""}, lines[3])
}

func TestRenderReviewDiffPreview(t *testing.T) {
	m := newDiffPreviewTestModel(t)
	preview := m.renderReviewDiffPreview("thread-1", "capacity_store.py", `@@ -344,2 +344,3 @@
 return f"v1-sandbox:{sandbox_id}:committed-capacity-reservation"
+def _committed_reservation_key_pattern(self) -> str:`, 80)

	plain := ansi.Strip(preview)
	require.Contains(t, plain, "344 344")
	require.Contains(t, plain, "+ def _committed_reservation_key_pattern")
	require.NotContains(t, plain, "@@")
}

func TestRenderReviewDiffPreviewEmptyWhenNoHunk(t *testing.T) {
	m := newDiffPreviewTestModel(t)
	require.Empty(t, m.renderReviewDiffPreview("thread-1", "capacity_store.py", "", 80))
}

func TestRenderReviewDiffPreviewCollapsesLongHunk(t *testing.T) {
	m := newDiffPreviewTestModel(t)
	preview := m.renderReviewDiffPreview("thread-1", "capacity_store.py", longReviewDiffHunk(15), 80)

	plain := ansi.Strip(preview)
	require.Contains(t, plain, "line 10")
	require.NotContains(t, plain, "line 11")
	require.Contains(t, plain, "Press e to expand 4 more lines...")
}

func TestRenderReviewDiffPreviewExpandsLongHunk(t *testing.T) {
	m := newDiffPreviewTestModel(t)
	m.viewState.activitySnippetsExpanded = true
	preview := m.renderReviewDiffPreview("thread-1", "capacity_store.py", longReviewDiffHunk(15), 80)

	plain := ansi.Strip(preview)
	require.Contains(t, plain, "line 14")
	require.Contains(t, plain, "Press e to collapse")
}

func TestActivityViewStateRestoredAcrossPRRows(t *testing.T) {
	m := newDiffPreviewTestModel(t)
	pr1 := prrow.Data{Primary: testPRViewPullRequestData(1, "https://github.com/owner/repo/pull/1")}
	pr2 := prrow.Data{Primary: testPRViewPullRequestData(2, "https://github.com/owner/repo/pull/2")}

	m.SetRow(&pr1)
	m.GoToActivityTab()
	m.ToggleActivityItemsCollapsed()
	m.ToggleActivitySnippetsExpanded()

	m.SetRow(&pr2)
	require.Equal(t, 0, m.SelectedTabIndex())
	require.False(t, m.viewState.activityItemsCollapsed)
	require.False(t, m.viewState.activitySnippetsExpanded)

	m.SetRow(&pr1)
	m.RestoreSelectedTab()
	require.Equal(t, 1, m.SelectedTabIndex())
	require.True(t, m.viewState.activityItemsCollapsed)
	require.True(t, m.viewState.activitySnippetsExpanded)
}

func TestReviewDiffLinesCachesParsedHunk(t *testing.T) {
	m := newDiffPreviewTestModel(t)
	diffHunk := `@@ -344,1 +344,1 @@
 return f"v1-sandbox:{sandbox_id}:committed-capacity-reservation"`

	first := m.reviewDiffLines("thread-1", "capacity_store.py", diffHunk)
	second := m.reviewDiffLines("thread-1", "different.py", diffHunk)

	require.Len(t, first, 1)
	require.Equal(t, first, second)
	require.Len(t, m.reviewDiffCache, 1)
}

func newDiffPreviewTestModel(t *testing.T) *Model {
	t.Helper()
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../../../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)
	thm := theme.ParseTheme(&cfg)
	ctx := &context.ProgramContext{
		Config: &cfg,
		Theme:  thm,
		Styles: context.InitStyles(thm),
	}
	m := NewModel(ctx)
	m.UpdateProgramContext(ctx)
	m.SetWidth(100)
	return &m
}

func longReviewDiffHunk(additions int) string {
	var b strings.Builder
	b.WriteString("@@ -1,1 +1,")
	b.WriteString(fmt.Sprintf("%d", additions+1))
	b.WriteString(" @@\n")
	b.WriteString(" context\n")
	for i := range additions {
		b.WriteString(fmt.Sprintf("+line %02d\n", i))
	}
	return strings.TrimRight(b.String(), "\n")
}

func testPRViewPullRequestData(number int, url string) *data.PullRequestData {
	pr := &data.PullRequestData{
		Number: number,
		Url:    url,
		Repository: data.Repository{
			NameWithOwner: "owner/repo",
		},
	}
	return pr
}
