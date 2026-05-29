package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dehub/v4/internal/config"
	"github.com/dlvhdr/gh-dehub/v4/internal/data"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/common"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/actionssection"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/footer"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/issueview"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/notificationview"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/prrow"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/prssection"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/prview"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/scroll"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/section"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/selection"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/sidebar"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/keys"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/theme"
)

func TestWheelNotch(t *testing.T) {
	tests := []struct {
		name   string
		button tea.MouseButton
		want   int
	}{
		{"up", tea.MouseWheelUp, -1},
		{"down", tea.MouseWheelDown, 1},
		{"left ignored", tea.MouseWheelLeft, 0},
		{"right ignored", tea.MouseWheelRight, 0},
		{"non-wheel ignored", tea.MouseLeft, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := wheelNotch(tc.button); got != tc.want {
				t.Fatalf("wheelNotch(%v) = %d, want %d", tc.button, got, tc.want)
			}
		})
	}
}

func newWheelTestModel(t *testing.T, activePane activePane) Model {
	t.Helper()

	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config:              &cfg,
		View:                config.PRsView,
		MainContentWidth:    100,
		MainContentHeight:   10,
		DynamicPreviewWidth: 80,
		PreviewPosition:     "right",
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	prSection := prssection.NewModel(0, ctx, config.PrsSectionConfig{}, time.Now(), time.Now())
	for i := 1; i <= 10; i++ {
		prSection.Prs = append(prSection.Prs, prrow.Data{
			Primary: testPullRequestData(i, "https://github.com/owner/repo/pull/1"),
		})
	}
	prSection.Table.SetRows(prSection.BuildRows())

	sidebarModel := sidebar.NewModel()
	sidebarModel.IsOpen = true
	sidebarModel.UpdateProgramContext(ctx)
	sidebarModel.SetContent(strings.Repeat("line\n", 200))
	sidebarModel.ScrollToOffset(20)

	prViewModel := prview.NewModel(ctx)
	prViewModel.UpdateProgramContext(ctx)

	m := Model{
		ctx:                ctx,
		keys:               keys.Keys,
		prs:                []section.Section{&prSection},
		prView:             prViewModel,
		sidebar:            sidebarModel,
		issueSidebar:       issueview.NewModel(ctx),
		notificationView:   notificationview.NewModel(ctx),
		footer:             footer.NewModel(ctx),
		activePane:         activePane,
		prPreviewStates:    map[string]map[int]int{},
		issuePreviewStates: map[string]int{},
	}
	if activePane == previewPane {
		m.ctx.ActivePane = "preview"
	} else {
		m.ctx.ActivePane = "main"
	}
	m.syncPreviewFocus()
	return m
}

// TestMouseWheelScrollsPreviewSidebar is the regression guard for the bug where
// wheel-scrolling the preview (e.g. the Activity tab) did nothing because the
// handler mutated a value-copy of the model made in View. The scroll must take
// effect on the model returned from Update.
func TestMouseWheelScrollsPreviewSidebar(t *testing.T) {
	scroll.Reset()
	t.Cleanup(scroll.Reset)

	m := newWheelTestModel(t, previewPane)
	// Register the preview region as View would, anywhere the cursor will be.
	scroll.Register(selection.ID("preview"), scroll.Bounds{X: 0, Y: 0, Width: 100, Height: 100})

	start := m.sidebar.YOffset()

	newModel, _ := m.Update(tea.MouseWheelMsg{X: 50, Y: 5, Button: tea.MouseWheelDown})
	m = newModel.(Model)
	require.Equal(t, start+wheelViewportStep, m.sidebar.YOffset(),
		"wheel down over the preview must scroll the sidebar viewport on the returned model")

	// Simulate the user pausing before reversing so momentum treats the upward
	// scroll as a fresh gesture (a single unpaused reverse event is
	// intentionally suppressed as a possible inertial tail blip).
	m.momentum = momentumState{}
	newModel, _ = m.Update(tea.MouseWheelMsg{X: 50, Y: 5, Button: tea.MouseWheelUp})
	m = newModel.(Model)
	require.Equal(t, start, m.sidebar.YOffset(),
		"wheel up must scroll the sidebar viewport back")
}

// TestMouseWheelScrollsRowsOneAtATime guards the "too sensitive" fix: a single
// wheel notch over the main pane moves the row cursor by exactly one row.
func TestMouseWheelScrollsRowsOneAtATime(t *testing.T) {
	scroll.Reset()
	t.Cleanup(scroll.Reset)

	m := newWheelTestModel(t, mainPane)
	scroll.Register(selection.ID("main"), scroll.Bounds{X: 0, Y: 0, Width: 100, Height: 100})

	start := m.getCurrSection().CurrRow()

	newModel, _ := m.Update(tea.MouseWheelMsg{X: 10, Y: 5, Button: tea.MouseWheelDown})
	m = newModel.(Model)
	require.Equal(t, start+wheelRowStep, m.getCurrSection().CurrRow(),
		"one wheel notch must move the row cursor by exactly one row")
}

// TestMouseWheelOutsideRegionsIsNoop verifies a wheel event with no region under
// the cursor leaves scroll state untouched.
func TestMouseWheelOutsideRegionsIsNoop(t *testing.T) {
	scroll.Reset()
	t.Cleanup(scroll.Reset)

	m := newWheelTestModel(t, previewPane)
	scroll.Register(selection.ID("preview"), scroll.Bounds{X: 0, Y: 0, Width: 20, Height: 20})

	start := m.sidebar.YOffset()
	newModel, _ := m.Update(tea.MouseWheelMsg{X: 500, Y: 500, Button: tea.MouseWheelDown})
	m = newModel.(Model)
	require.Equal(t, start, m.sidebar.YOffset(),
		"wheel outside any region must not scroll")
}

// TestMouseWheelRebakesSelectedRow is the regression guard for the highlight
// artifact: after a wheel row-scroll the table's live rendered rows must reflect
// the new cursor (i.e. SetRows(BuildRows()) ran), not a stale highlight. Without
// the re-bake, Table.Rows lags BuildRows() because moving the cursor only
// restyles the row background and leaves per-cell highlights baked at the old
// position.
func TestMouseWheelRebakesSelectedRow(t *testing.T) {
	scroll.Reset()
	t.Cleanup(scroll.Reset)

	m := newWheelTestModel(t, mainPane)
	scroll.Register(selection.ID("main"), scroll.Bounds{X: 0, Y: 0, Width: 100, Height: 100})

	prs := m.prs[0].(*prssection.Model)
	startRow := prs.CurrRow()

	newModel, _ := m.Update(tea.MouseWheelMsg{X: 10, Y: 5, Button: tea.MouseWheelDown})
	m = newModel.(Model)

	prs = m.prs[0].(*prssection.Model)
	require.Equal(t, startRow+wheelRowStep, prs.CurrRow(), "cursor should advance one row")
	require.Equal(t, prs.BuildRows(), prs.Table.Rows,
		"after a wheel scroll the live rows must match a fresh BuildRows() so the selected-row highlight is re-baked onto the new cursor row")
}

// TestMouseWheelKeepsDragActive verifies that wheel-scrolling during an
// in-progress copy-selection drag keeps the drag alive and advances its end
// point to the cursor, so the overlay re-renders against scrolled content
// instead of going stale.
func TestMouseWheelKeepsDragActive(t *testing.T) {
	scroll.Reset()
	t.Cleanup(scroll.Reset)

	m := newWheelTestModel(t, mainPane)
	scroll.Register(selection.ID("main"), scroll.Bounds{X: 0, Y: 0, Width: 100, Height: 100})
	selection.Reset()
	t.Cleanup(selection.Reset)
	regionID := selection.ID("test-region")
	selection.RegisterBounds(regionID, selection.Bounds{X: 0, Y: 0, Width: 100, Height: 100}, "plain", "styled")
	m.copySelection.begin(regionID, 1, 1)

	newModel, _ := m.Update(tea.MouseWheelMsg{X: 7, Y: 9, Button: tea.MouseWheelDown})
	m = newModel.(Model)

	require.True(t, m.copySelection.dragging, "wheel must not cancel an in-progress drag")
	require.Equal(t, 7, m.copySelection.endX, "drag end X should follow the cursor")
	require.Equal(t, 9, m.copySelection.endY, "drag end Y should follow the cursor")
}

// TestMouseWheelRowScrollDebouncesPreview verifies the row-scroll preview
// refresh is debounced: a wheel notch bumps the settle generation and returns a
// tick command (rather than refreshing synchronously), so a fast scroll stays
// fluid.
func TestMouseWheelRowScrollDebouncesPreview(t *testing.T) {
	scroll.Reset()
	t.Cleanup(scroll.Reset)

	m := newWheelTestModel(t, mainPane)
	scroll.Register(selection.ID("main"), scroll.Bounds{X: 0, Y: 0, Width: 100, Height: 100})

	startGen := m.scrollSettleGen

	newModel, cmd := m.Update(tea.MouseWheelMsg{X: 10, Y: 5, Button: tea.MouseWheelDown})
	m = newModel.(Model)

	require.Equal(t, startGen+1, m.scrollSettleGen, "each wheel notch over rows must bump the settle generation")
	require.NotNil(t, cmd, "wheel row scroll must return the debounce tick command")

	// A second notch supersedes the first generation.
	newModel, _ = m.Update(tea.MouseWheelMsg{X: 10, Y: 5, Button: tea.MouseWheelDown})
	m = newModel.(Model)
	require.Equal(t, startGen+2, m.scrollSettleGen)
}

// TestScrollSettleMsgOnlyLatestGenerationRefreshes verifies the settle handler
// ignores superseded ticks and acts on the current generation.
func TestScrollSettleMsgOnlyLatestGenerationRefreshes(t *testing.T) {
	scroll.Reset()
	t.Cleanup(scroll.Reset)

	m := newWheelTestModel(t, mainPane)
	m.scrollSettleGen = 5

	// A stale tick (older generation) is a no-op.
	newModel, cmd := m.Update(scrollSettleMsg{generation: 4})
	m = newModel.(Model)
	require.Nil(t, cmd, "a superseded settle tick must not trigger a refresh")

	// The current generation triggers onViewedRowChanged (non-nil cmd batch).
	newModel, cmd = m.Update(scrollSettleMsg{generation: 5})
	m = newModel.(Model)
	require.NotNil(t, cmd, "the latest settle tick must trigger the preview refresh")
}

// TestMomentumAcceptSameDirectionStream verifies a continuous same-direction
// stream always passes through.
func TestMomentumAcceptSameDirectionStream(t *testing.T) {
	var s momentumState
	now := time.Unix(0, 0)
	for i := range 10 {
		now = now.Add(5 * time.Millisecond)
		require.True(t, s.accept(1, now), "same-direction event %d should be accepted", i)
	}
}

// TestMomentumSettledGapReseedsAnyDirection verifies that after a gap longer
// than the momentum window, a wheel event in any direction is accepted as a
// fresh gesture.
func TestMomentumSettledGapReseedsAnyDirection(t *testing.T) {
	var s momentumState
	base := time.Unix(100, 0)
	require.True(t, s.accept(1, base), "first event seeds the direction")

	// After a long pause, the opposite direction is a brand-new gesture.
	require.True(t, s.accept(-1, base.Add(momentumWindow+time.Millisecond)),
		"a reversal after the stream settles must be accepted immediately")
}

// TestMomentumSuppressesTailThenHonorsConfirmedReversal models the reported bug:
// while same-direction inertial momentum is still arriving, a single opposite
// event (the dying tail or a one-off) is dropped, but a sustained reverse flick
// (confirmed by consecutive opposite events) is honored.
func TestMomentumSuppressesTailThenHonorsConfirmedReversal(t *testing.T) {
	var s momentumState
	now := time.Unix(0, 0)

	advance := func(d int) bool {
		now = now.Add(5 * time.Millisecond)
		return s.accept(d, now)
	}

	// Active downward stream.
	require.True(t, advance(1))
	require.True(t, advance(1))

	// First opposite event within the window is unconfirmed -> dropped.
	require.False(t, advance(-1), "an unconfirmed opposite event (tail blip) must be dropped")

	// A residual same-direction tail event still passes and clears the pending
	// reversal.
	require.True(t, advance(1), "residual same-direction momentum still scrolls")

	// A sustained reverse flick: two consecutive opposite events confirm.
	require.False(t, advance(-1), "first opposite event of the reversal is held")
	require.True(t, advance(-1), "second consecutive opposite event confirms the reversal")
	require.True(t, advance(-1), "the new direction now streams freely")

	// And the dying old-direction (down) tail after the reversal is suppressed.
	require.False(t, advance(1), "old-direction tail after a confirmed reversal is suppressed")
}

// TestMouseWheelReversalNotBlocked is an Update-level guard: after a downward
// wheel burst, a sustained upward flick must move the cursor back up rather than
// being swallowed entirely.
func TestMouseWheelReversalNotBlocked(t *testing.T) {
	scroll.Reset()
	t.Cleanup(scroll.Reset)

	m := newWheelTestModel(t, mainPane)
	scroll.Register(selection.ID("main"), scroll.Bounds{X: 0, Y: 0, Width: 100, Height: 100})

	down := tea.MouseWheelMsg{X: 10, Y: 5, Button: tea.MouseWheelDown}
	up := tea.MouseWheelMsg{X: 10, Y: 5, Button: tea.MouseWheelUp}

	// Scroll down a few rows.
	for range 3 {
		newModel, _ := m.Update(down)
		m = newModel.(Model)
	}
	rowAfterDown := m.prs[0].(*prssection.Model).CurrRow()
	require.Greater(t, rowAfterDown, 0, "downward scroll should have advanced the cursor")

	// A sustained upward flick must eventually move the cursor back up.
	for range 4 {
		newModel, _ := m.Update(up)
		m = newModel.(Model)
	}
	rowAfterUp := m.prs[0].(*prssection.Model).CurrRow()
	require.Less(t, rowAfterUp, rowAfterDown,
		"a sustained reverse flick must not be blocked by momentum")
}

func newActionsWheelTestModel(t *testing.T) Model {
	t.Helper()

	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config:            &cfg,
		View:              config.ActionsView,
		ScreenWidth:       200,
		ScreenHeight:      40,
		MainContentWidth:  200,
		MainContentHeight: 30,
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	as := actionssection.NewModel(0, ctx, config.ActionsSectionConfig{}, time.Now(), time.Now())
	for i := 1; i <= 10; i++ {
		as.Workflows = append(as.Workflows, data.Workflow{Id: int64(i), Name: "workflow", State: "active"})
		as.Runs = append(as.Runs, data.WorkflowRun{Id: int64(i), DisplayTitle: "run"})
	}
	as.Table.SetRows(as.BuildRows())
	as.RunsTable.SetRows(as.BuildRunRows())

	m := Model{
		ctx:                ctx,
		keys:               keys.Keys,
		actions:            []section.Section{&as},
		sidebar:            sidebar.NewModel(),
		prView:             prview.NewModel(ctx),
		issueSidebar:       issueview.NewModel(ctx),
		notificationView:   notificationview.NewModel(ctx),
		footer:             footer.NewModel(ctx),
		activePane:         mainPane,
		prPreviewStates:    map[string]map[int]int{},
		issuePreviewStates: map[string]int{},
	}
	m.footer.UpdateProgramContext(ctx)
	return m
}

// TestMouseWheelScrollsActionsWorkflows verifies wheeling over the Workflows
// column moves the workflow table cursor and debounces the heavy refresh.
func TestMouseWheelScrollsActionsWorkflows(t *testing.T) {
	scroll.Reset()
	t.Cleanup(scroll.Reset)

	m := newActionsWheelTestModel(t)
	m.registerActionsScrollRegions(m.actions[0].(*actionssection.Model))

	as := m.actions[0].(*actionssection.Model)
	startRow := as.Table.GetCurrItem()
	startGen := m.scrollSettleGen

	// The workflows column is the leftmost; X=1 is inside it.
	newModel, _ := m.Update(tea.MouseWheelMsg{X: 1, Y: common.TabsHeight + 1, Button: tea.MouseWheelDown})
	m = newModel.(Model)

	as = m.actions[0].(*actionssection.Model)
	require.Equal(t, startRow+wheelRowStep, as.Table.GetCurrItem(),
		"wheel over the workflows column should advance the workflow cursor by one")
	require.Equal(t, startGen+1, m.scrollSettleGen, "workflow wheel scroll should debounce the refresh")
}

// TestMouseWheelScrollsActionsRuns verifies wheeling over the Runs column moves
// the runs table cursor and debounces the heavy refresh.
func TestMouseWheelScrollsActionsRuns(t *testing.T) {
	scroll.Reset()
	t.Cleanup(scroll.Reset)

	m := newActionsWheelTestModel(t)
	m.registerActionsScrollRegions(m.actions[0].(*actionssection.Model))

	firstWidth, _, _ := actionsPaneWidths(m.ctx.ScreenWidth)
	as := m.actions[0].(*actionssection.Model)
	startRow := as.RunsTable.GetCurrItem()
	startGen := m.scrollSettleGen

	// X just inside the runs column (immediately right of the workflows column).
	newModel, _ := m.Update(tea.MouseWheelMsg{X: firstWidth + 1, Y: common.TabsHeight + 1, Button: tea.MouseWheelDown})
	m = newModel.(Model)

	as = m.actions[0].(*actionssection.Model)
	require.Equal(t, startRow+wheelRowStep, as.RunsTable.GetCurrItem(),
		"wheel over the runs column should advance the runs cursor by one")
	require.Equal(t, startGen+1, m.scrollSettleGen, "runs wheel scroll should debounce the refresh")
}

// TestMouseWheelActionsDetailsNoRunViewIsNoop verifies wheeling over the Details
// column is a safe no-op when no run view is loaded.
func TestMouseWheelActionsDetailsNoRunViewIsNoop(t *testing.T) {
	scroll.Reset()
	t.Cleanup(scroll.Reset)

	m := newActionsWheelTestModel(t)
	m.registerActionsScrollRegions(m.actions[0].(*actionssection.Model))

	firstWidth, secondWidth, _ := actionsPaneWidths(m.ctx.ScreenWidth)
	require.Nil(t, m.actionRunView, "test setup: no run view loaded")

	newModel, cmd := m.Update(tea.MouseWheelMsg{X: firstWidth + secondWidth + 1, Y: common.TabsHeight + 1, Button: tea.MouseWheelDown})
	m = newModel.(Model)
	require.Nil(t, cmd, "details wheel with no run view must be a no-op")
}
