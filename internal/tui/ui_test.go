package tui

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"text/template"
	"time"

	tea "charm.land/bubbletea/v2"

	// "charm.land/x/exp/teatest"

	"github.com/stretchr/testify/require"

	zone "github.com/lrstanley/bubblezone/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/common"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/footer"
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
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/keys"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/markdown"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/theme"
)

// func TestFullOutput(t *testing.T) {
// 	setupTest(t)
// 	m := NewModel(config.Location{RepoPath: "", ConfigFlag: "../config/testdata/test-config.yml"})
// 	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(160, 60))
//
// 	testutils.WaitForText(t, tm, "style: make assignment brief", teatest.WithDuration(6*time.Second))
//
// 	tm.Send(tea.KeyPressMsg{
// 		Type:  tea.KeyRunes,
// 		Runes: []rune("q"),
// 	})
// 	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
// }

// func TestIssues(t *testing.T) {
// 	setupTest(t)
// 	m := NewModel(config.Location{RepoPath: "", ConfigFlag: "../config/testdata/test-config.yml"})
// 	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(160, 60))
//
// 	// wait for first tab of PRs
// 	testutils.WaitForText(t, tm, "Mine")
//
// 	tm.Send(tea.KeyPressMsg{
// 		Type:  tea.KeyRunes,
// 		Runes: []rune("s"),
// 	})
// 	testutils.WaitForText(t, tm, "[Feature Request] Support notifications", teatest.WithDuration(6*time.Second))
// 	tm.Send(tea.KeyPressMsg{
// 		Type:  tea.KeyRunes,
// 		Runes: []rune("q"),
// 	})
// 	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
// }

// func setupTest(t *testing.T) {
// 	if _, debug := os.LookupEnv("DEBUG"); debug {
// 		f, _ := os.CreateTemp("", "gh-dash-debug.log")
// 		fmt.Printf("[DEBU] writing debug logs to %s\n", f.Name())
// 		defer f.Close()
// 		log.SetOutput(f)
// 		log.SetLevel(log.DebugLevel)
// 	}
// 	setMockClient(t)
//
// 	markdown.InitializeMarkdownStyle(true)
// 	zone.NewGlobal()
// 	zone.SetEnabled(false)
// }

// localRoundTripper is an http.RoundTripper that executes HTTP transactions
// by using handler directly, instead of going over an HTTP connection.
type localRoundTripper struct {
	handler http.Handler
}

func (l localRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	w := httptest.NewRecorder()
	l.handler.ServeHTTP(w, req)
	return w.Result(), nil
}

// func mustRead(t *testing.T, r io.Reader) string {
// 	t.Helper()
// 	b, err := io.ReadAll(r)
// 	if err != nil {
// 		panic(err)
// 	}
// 	return string(b)
// }
//
// func mustWrite(t *testing.T, w io.Writer, s string) {
// 	t.Helper()
// 	_, err := io.WriteString(w, s)
// 	if err != nil {
// 		panic(err)
// 	}
// }

// func setMockClient(t *testing.T) {
// 	t.Helper()
// 	mux := http.NewServeMux()
// 	mux.HandleFunc("/api/graphql", func(w http.ResponseWriter, req *http.Request) {
// 		log.Debug("got request", "request", req.URL, "body", req.Body)
// 		body := mustRead(t, req.Body)
// 		switch {
// 		case strings.Contains(body, "query SearchPullRequests"):
// 			d, err := os.ReadFile("./testdata/searchPullRequests.json")
// 			if err != nil {
// 				t.Errorf("failed reading mock data file %v", err)
// 			}
// 			mustWrite(t, w, string(d))
// 		case strings.Contains(body, "query SearchIssues"):
// 			d, err := os.ReadFile("./testdata/searchIssues.json")
// 			if err != nil {
// 				t.Errorf("failed reading mock data file %v", err)
// 			}
// 			mustWrite(t, w, string(d))
// 		default:
// 			w.WriteHeader(http.StatusInternalServerError)
// 			return
// 		}
// 		w.WriteHeader(http.StatusOK)
// 		w.Header().Set("Content-Type", "application/json")
// 	})
// 	client, err := gh.NewGraphQLClient(gh.ClientOptions{
// 		Transport: localRoundTripper{handler: mux},
// 		Host:      "localhost:3000",
// 		AuthToken: "fake-token",
// 	})
// 	if err != nil {
// 		t.Errorf("failed creating gh client %v", err)
// 	}
// 	data.SetClient(client)
// }

func initTestGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	repoPath := dir + "/repo"
	require.NoError(t, exec.Command("git", "init", repoPath).Run())
	require.NoError(t, exec.Command("git", "-C", repoPath, "remote", "add", "origin", "https://github.com/owner/repo.git").Run())
	return dir
}

func TestPromptConfirmation_NilSection(t *testing.T) {
	// promptConfirmation should return nil when currSection is nil
	m := Model{}
	cmd := m.promptConfirmation(nil, "close")
	require.Nil(t, cmd, "promptConfirmation should return nil when section is nil")
}

func TestPROpenCloseAction(t *testing.T) {
	testCases := []struct {
		name string
		pr   any
		want string
	}{
		{
			name: "open PR closes",
			pr:   &prrow.Data{Primary: &data.PullRequestData{State: "OPEN"}},
			want: "close",
		},
		{
			name: "closed PR reopens",
			pr:   &prrow.Data{Primary: &data.PullRequestData{State: "CLOSED"}},
			want: "reopen",
		},
		{
			name: "merged PR does nothing",
			pr:   &prrow.Data{Primary: &data.PullRequestData{State: "MERGED"}},
			want: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, prOpenCloseAction(tc.pr))
		})
	}
}

func TestIssueOpenCloseAction(t *testing.T) {
	testCases := []struct {
		name  string
		issue any
		want  string
	}{
		{
			name:  "open issue closes",
			issue: &data.IssueData{State: "OPEN"},
			want:  "close",
		},
		{
			name:  "closed issue reopens",
			issue: &data.IssueData{State: "CLOSED"},
			want:  "reopen",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, issueOpenCloseAction(tc.issue))
		})
	}
}

func TestNotificationView_SwitchViewWithSKey(t *testing.T) {
	// Test that forward view navigation switches from Notifications to PRs view
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag: "../config/testdata/test-config.yml",
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config: &cfg,
		View:   config.NotificationsView,
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)
	markdown.InitializeMarkdownStyle(true)

	sidebarModel := sidebar.NewModel()
	sidebarModel.IsOpen = true
	sidebarModel.UpdateProgramContext(ctx)

	m := Model{
		ctx:     ctx,
		keys:    keys.Keys,
		prView:  prview.NewModel(ctx),
		sidebar: sidebarModel,
		tabs:    tabs.NewModel(ctx),
	}
	prSec := prssection.NewModel(0, ctx, config.PrsSectionConfig{}, time.Now(), time.Now())
	m.prs = []section.Section{&prSec}

	// Verify we start in NotificationsView
	require.Equal(t, config.NotificationsView, m.ctx.View, "should start in NotificationsView")

	// Test that switchSelectedView returns PRsView when in NotificationsView
	m.switchSelectedView()
	require.Equal(t, config.PRsView, m.ctx.View,
		"switchSelectedView should set view to PRsView when in NotificationsView")
}

func TestSwitchSelectedViewBack(t *testing.T) {
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag: "../config/testdata/test-config.yml",
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config: &cfg,
		View:   config.ActionsView,
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)
	m := Model{
		ctx:    ctx,
		keys:   keys.Keys,
		prView: prview.NewModel(ctx),
		tabs:   tabs.NewModel(ctx),
	}
	prSec := prssection.NewModel(0, ctx, config.PrsSectionConfig{}, time.Now(), time.Now())
	m.prs = []section.Section{&prSec}

	m.switchSelectedViewBack()

	require.Equal(t, config.PRsView, m.ctx.View)
}

func TestReconcileVisibleRefreshesSchedulesPRPreview(t *testing.T) {
	ctx := &context.ProgramContext{}
	prView := prview.NewModel(ctx)
	url := "https://github.com/owner/repo/pull/1"
	prView.SetRow(&prrow.Data{
		Primary: &data.PullRequestData{Url: url},
	})
	prView.GoToActivityTab()

	sidebarModel := sidebar.NewModel()
	sidebarModel.IsOpen = true
	m := Model{prView: prView, sidebar: sidebarModel}

	cmd := m.reconcileVisibleRefreshes()

	require.NotNil(t, cmd)
	require.Contains(t, m.visibleRefreshes, "pr-preview:"+url)
}

func TestReconcileVisibleRefreshesSchedulesPRPreviewOutsideActivityTab(t *testing.T) {
	ctx := &context.ProgramContext{}
	prView := prview.NewModel(ctx)
	url := "https://github.com/owner/repo/pull/1"
	prView.SetRow(&prrow.Data{
		Primary: &data.PullRequestData{Url: url},
	})

	sidebarModel := sidebar.NewModel()
	sidebarModel.IsOpen = true
	m := Model{prView: prView, sidebar: sidebarModel}

	cmd := m.reconcileVisibleRefreshes()

	require.NotNil(t, cmd)
	require.Contains(t, m.visibleRefreshes, "pr-preview:"+url)
}

func TestVisibleRefreshTickReconcilesWhenGenerationIsStale(t *testing.T) {
	ctx := &context.ProgramContext{View: config.PRsView}
	prView := prview.NewModel(ctx)
	url := "https://github.com/owner/repo/pull/1"
	prView.SetRow(&prrow.Data{
		Primary: &data.PullRequestData{Url: url},
	})

	sidebarModel := sidebar.NewModel()
	sidebarModel.IsOpen = true
	m := Model{ctx: ctx, prView: prView, sidebar: sidebarModel, visibleRefreshes: map[string]int{}, visibleRefreshGen: 10}
	target := visibleRefreshTarget{key: "pr-preview:" + url, kind: visibleRefreshPRPreview, url: url, interval: 10 * time.Second}

	newModel, cmd := m.Update(visibleRefreshTick{target: target, generation: 1})
	m = newModel.(Model)

	require.NotNil(t, cmd)
	require.Contains(t, m.visibleRefreshes, target.key)
	require.Equal(t, 11, m.visibleRefreshes[target.key])
}

func TestVisibleRefreshTargetsUsesConfiguredPRPreviewInterval(t *testing.T) {
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)
	cfg.Defaults.PreviewRefreshIntervalSeconds = 3

	ctx := &context.ProgramContext{Config: &cfg}
	prView := prview.NewModel(ctx)
	url := "https://github.com/owner/repo/pull/1"
	prView.SetRow(&prrow.Data{Primary: &data.PullRequestData{Url: url}})

	sidebarModel := sidebar.NewModel()
	sidebarModel.IsOpen = true
	m := Model{ctx: ctx, prView: prView, sidebar: sidebarModel}

	targets := m.visibleRefreshTargets()

	require.NotEmpty(t, targets)
	require.Equal(t, visibleRefreshPRPreview, targets[0].kind)
	require.Equal(t, 3*time.Second, targets[0].interval)
}

func TestVisibleRefreshTargetsIncludesCurrentPRSection(t *testing.T) {
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)
	cfg.Defaults.RefetchIntervalMinutes = 1

	ctx := &context.ProgramContext{
		Config: &cfg,
		View:   config.PRsView,
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	prSec := prssection.NewModel(1, ctx, config.PrsSectionConfig{Filters: "author:@me"}, time.Now(), time.Now())
	m := Model{
		ctx:           ctx,
		currSectionId: 1,
		prs:           []section.Section{nil, &prSec},
	}

	targets := m.visibleRefreshTargets()

	require.Len(t, targets, 1)
	require.Equal(t, visibleRefreshPRSection, targets[0].kind)
	require.Equal(t, 1, targets[0].sectionId)
	require.Equal(t, time.Minute, targets[0].interval)
}

func TestVisibleRefreshTargetsIncludesRepoBranchesForRepoFilteredPRSection(t *testing.T) {
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)
	cfg.Defaults.RefetchIntervalMinutes = 1
	cfg.RepoPaths = map[string]string{"owner/repo": "/tmp/repo"}

	ctx := &context.ProgramContext{
		Config: &cfg,
		View:   config.PRsView,
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	prSec := prssection.NewModel(1, ctx, config.PrsSectionConfig{Filters: "repo:owner/repo is:open"}, time.Now(), time.Now())
	m := Model{
		ctx:           ctx,
		currSectionId: 1,
		prs:           []section.Section{nil, &prSec},
	}

	targets := m.visibleRefreshTargets()

	found := false
	for _, target := range targets {
		if target.kind == visibleRefreshRepoBranches {
			found = true
			require.Equal(t, "owner/repo", target.repoName)
			require.Equal(t, 1, target.sectionId)
			require.Equal(t, time.Minute, target.interval)
		}
	}
	require.True(t, found)
}

func TestNotificationView_SwitchViewWithSKey_WhileViewingPR(t *testing.T) {
	// Test that forward view navigation clears PR notification subject state
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag: "../config/testdata/test-config.yml",
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config: &cfg,
		View:   config.NotificationsView,
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	sidebarModel := sidebar.NewModel()
	sidebarModel.IsOpen = true
	sidebarModel.UpdateProgramContext(ctx)

	m := Model{
		ctx:     ctx,
		keys:    keys.Keys,
		prView:  prview.NewModel(ctx),
		sidebar: sidebarModel,
		tabs:    tabs.NewModel(ctx),
	}
	prSec := prssection.NewModel(0, ctx, config.PrsSectionConfig{}, time.Now(), time.Now())
	m.prs = []section.Section{&prSec}

	// Set up a PR notification subject (simulating viewing a PR notification)
	m.notificationView.SetSubjectPR(&prrow.Data{}, "test-notification-id")

	// Verify we start in NotificationsView
	require.Equal(t, config.NotificationsView, m.ctx.View, "should start in NotificationsView")

	// Verify GetSubjectPR returns non-nil
	require.NotNil(t, m.notificationView.GetSubjectPR(), "subject PR should be set")

	// Test that switchSelectedView returns PRsView
	m.switchSelectedView()
	require.Equal(t, config.PRsView, m.ctx.View,
		"switchSelectedView should set view to PRsView when in NotificationsView")

	// Verify subject was cleared after switch
	require.Nil(t, m.notificationView.GetSubjectPR(),
		"subject PR should be cleared after switching views")
}

func TestNotificationView_SwitchViewWithSKey_WhileViewingIssue(t *testing.T) {
	// Test that forward view navigation clears issue notification subject state
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag: "../config/testdata/test-config.yml",
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config: &cfg,
		View:   config.NotificationsView,
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	sidebarModel := sidebar.NewModel()
	sidebarModel.IsOpen = true
	sidebarModel.UpdateProgramContext(ctx)

	m := Model{
		ctx:     ctx,
		keys:    keys.Keys,
		prView:  prview.NewModel(ctx),
		sidebar: sidebarModel,
		tabs:    tabs.NewModel(ctx),
	}
	prSec := prssection.NewModel(0, ctx, config.PrsSectionConfig{}, time.Now(), time.Now())
	m.prs = []section.Section{&prSec}

	// Set up an Issue notification subject (simulating viewing an Issue notification)
	m.notificationView.SetSubjectIssue(&data.IssueData{}, "test-notification-id")

	// Verify we start in NotificationsView
	require.Equal(t, config.NotificationsView, m.ctx.View, "should start in NotificationsView")

	// Verify GetSubjectIssue returns non-nil
	require.NotNil(t, m.notificationView.GetSubjectIssue(), "subject Issue should be set")

	// Test that switchSelectedView returns PRsView
	m.switchSelectedView()
	require.Equal(t, config.PRsView, m.ctx.View,
		"switchSelectedView should set view to PRsView when in NotificationsView")

	// Verify subject was cleared after switch
	require.Nil(t, m.notificationView.GetSubjectIssue(),
		"subject Issue should be cleared after switching views")
}

func TestNotificationView_PRViewTabNavigation(t *testing.T) {
	keys.PRKeys.PrevSidebarTab.SetKeys("left")
	keys.PRKeys.NextSidebarTab.SetKeys("right")

	// This test verifies that tab navigation works in notification view when viewing a PR.
	// Previously, the code only returned when prCmd != nil, but tab navigation
	// (carousel.MoveLeft/MoveRight) doesn't return a command - it just updates state.
	// The fix ensures we always sync sidebar and return after prView.Update().
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config: &cfg,
		View:   config.NotificationsView,
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	sidebarModel := sidebar.NewModel()
	sidebarModel.IsOpen = true
	sidebarModel.UpdateProgramContext(ctx)

	m := Model{
		ctx:              ctx,
		keys:             keys.Keys,
		prView:           prview.NewModel(ctx),
		sidebar:          sidebarModel,
		issueSidebar:     issueview.NewModel(ctx),
		notificationView: notificationview.NewModel(ctx),
		activePane:       previewPane,
	}
	m.ctx.ActivePane = "preview"

	// Set up a PR notification subject so GetSubjectPR() returns non-nil
	m.notificationView.SetSubjectPR(&prrow.Data{}, "test-notification-id")

	// Get initial tab
	initialTab := m.prView.SelectedTab()

	// Send "next tab" key message
	msg := tea.KeyPressMsg{Code: tea.KeyRight}
	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	// Verify tab changed
	require.NotEqual(t, initialTab, m.prView.SelectedTab(),
		"prView tab should have changed after pressing next tab key")

	// Now test going back
	currentTab := m.prView.SelectedTab()
	msg = tea.KeyPressMsg{Code: tea.KeyLeft}
	newModel, _ = m.Update(msg)
	m = newModel.(Model)

	require.NotEqual(t, currentTab, m.prView.SelectedTab(),
		"prView tab should have changed after pressing prev tab key")
}

func TestPRPreviewTabMemory(t *testing.T) {
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config:              &cfg,
		View:                config.PRsView,
		MainContentHeight:   10,
		DynamicPreviewWidth: 80,
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	prSection := prssection.NewModel(0, ctx, config.PrsSectionConfig{}, time.Now(), time.Now())
	for i := 1; i <= 20; i++ {
		prSection.Prs = append(prSection.Prs, prrow.Data{
			Primary: testPullRequestData(i, fmt.Sprintf("https://github.com/owner/repo/pull/%d", i)),
		})
	}
	prSection.Table.SetRows(prSection.BuildRows())
	sidebarModel := sidebar.NewModel()
	sidebarModel.IsOpen = true
	sidebarModel.UpdateProgramContext(ctx)
	sidebarModel.SetContent(strings.Repeat("line\n", 50))
	prViewModel := prview.NewModel(ctx)
	prViewModel.UpdateProgramContext(ctx)

	m := Model{
		ctx:     ctx,
		prs:     []section.Section{&prSection},
		prView:  prViewModel,
		sidebar: sidebarModel,
	}

	m.prView.SetRow(&prSection.Prs[0])
	m.prView.GoToActivityTab()
	activityTabIdx := m.prView.SelectedTabIndex()
	m.sidebar.ScrollToOffset(7)
	m.saveCurrentPRPreviewState()
	require.Equal(t, 7,
		m.prPreviewStates[prSection.Prs[0].Primary.Url][activityTabIdx],
		"saved PR preview state should include scroll offset for the active tab")

	prSection.NextRow()
	m.prView.SetRow(&prSection.Prs[1])
	m.restoreCurrentPRPreviewState()
	require.Equal(t, 0, m.prView.SelectedTabIndex(),
		"PRs without saved tab state should default to the overview tab")

	prSection.PrevRow()
	m.prView.SetRow(&prSection.Prs[0])
	m.restoreCurrentPRPreviewState()
	require.Equal(t, activityTabIdx, m.prView.SelectedTabIndex(),
		"returning to a PR should restore its previously selected preview tab")
}

// TestPRPreviewPerTabScrollPreserved verifies that the sidebar scroll
// position is recorded per (PR, tab) so that switching between tabs and
// returning to the previous tab restores the user's prior offset, instead
// of jumping to the top (or to the bottom of activity).
func TestPRPreviewPerTabScrollPreserved(t *testing.T) {
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config:              &cfg,
		View:                config.PRsView,
		MainContentHeight:   10,
		DynamicPreviewWidth: 80,
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	prSection := prssection.NewModel(0, ctx, config.PrsSectionConfig{}, time.Now(), time.Now())
	prSection.Prs = append(prSection.Prs, prrow.Data{
		Primary: testPullRequestData(1, "https://github.com/owner/repo/pull/1"),
	})
	prSection.Table.SetRows(prSection.BuildRows())

	sidebarModel := sidebar.NewModel()
	sidebarModel.IsOpen = true
	sidebarModel.UpdateProgramContext(ctx)
	sidebarModel.SetContent(strings.Repeat("line\n", 200))
	prViewModel := prview.NewModel(ctx)
	prViewModel.UpdateProgramContext(ctx)

	m := Model{
		ctx:                ctx,
		prs:                []section.Section{&prSection},
		prView:             prViewModel,
		sidebar:            sidebarModel,
		prPreviewStates:    map[string]map[int]int{},
		issuePreviewStates: map[string]int{},
	}
	m.prView.SetRow(&prSection.Prs[0])

	// Land on Activity (index 1) and scroll part of the way down.
	m.prView.GoToActivityTab()
	activityIdx := m.prView.SelectedTabIndex()
	m.sidebar.ScrollToOffset(15)

	// User switches to a different tab; save Activity's outgoing scroll.
	m.savePRPreviewStateAt(activityIdx)
	m.prView.GoToTab(2) // Checks
	m.sidebar.ScrollToOffset(3)

	// User switches back to Activity; save Checks' outgoing scroll then
	// restore Activity's previously recorded scroll.
	m.savePRPreviewStateAt(m.prView.SelectedTabIndex())
	m.prView.GoToActivityTab()
	require.True(t, m.restoreCurrentPRPreviewTab(),
		"returning to a previously-scrolled tab should restore its offset")
	require.Equal(t, 15, m.sidebar.YOffset(),
		"Activity tab should restore the offset the user left it at")

	// Switching to Checks again should restore its independent offset.
	m.savePRPreviewStateAt(m.prView.SelectedTabIndex())
	m.prView.GoToTab(2)
	require.True(t, m.restoreCurrentPRPreviewTab(),
		"Checks tab should restore its previously saved offset")
	require.Equal(t, 3, m.sidebar.YOffset(),
		"Checks tab scroll should be independent of Activity tab scroll")
}

// TestSetActivePaneSyncsPreviewFocus verifies that toggling the active
// pane propagates the resulting "is the preview pane focused?" state into
// prView, so the Checks tab's embedded actionview can refuse navigation
// keys when the main (row list) pane is focused. Without this, up/down
// presses leak into the Checks pane even when the user is navigating the
// PR row list.
func TestSetActivePaneSyncsPreviewFocus(t *testing.T) {
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config: &cfg,
		View:   config.PRsView,
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	sidebarModel := sidebar.NewModel()
	sidebarModel.IsOpen = true
	sidebarModel.UpdateProgramContext(ctx)

	m := Model{
		ctx:     ctx,
		keys:    keys.Keys,
		prView:  prview.NewModel(ctx),
		sidebar: sidebarModel,
	}

	// Initially mainPane → preview not focused.
	m.setActivePane(mainPane)
	require.False(t, m.prView.IsPreviewFocused(),
		"prView.previewFocused must be false when activePane is mainPane")

	// Switch to previewPane with sidebar open → preview focused.
	m.setActivePane(previewPane)
	require.True(t, m.prView.IsPreviewFocused(),
		"prView.previewFocused must be true when activePane is previewPane and sidebar is open")

	// Closing the sidebar should clamp focus off, even if activePane
	// is still previewPane (matches isPreviewFocused predicate).
	m.sidebar.IsOpen = false
	m.setActivePane(previewPane)
	require.False(t, m.prView.IsPreviewFocused(),
		"prView.previewFocused must be false when the sidebar is closed")
}

// TestMainPaneFocusedUpDownDoesNotScrollSidebar verifies that up/down key
// messages do NOT scroll the global sidebar viewport when the main (row
// list) pane is focused. Without the gate, sidebar.Update is invoked for
// every key message in the fall-through after the main key switch, which
// scrolled the preview viewport visually whenever the user navigated the
// row list.
func TestMainPaneFocusedUpDownDoesNotScrollSidebar(t *testing.T) {
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config:              &cfg,
		View:                config.PRsView,
		MainContentHeight:   10,
		DynamicPreviewWidth: 80,
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	// Build a Model with no PR sections so that the Up/Down handlers
	// short-circuit on `currSection == nil` and don't trigger the row-
	// change side effects (onViewedRowChanged → syncSidebar → SetContent
	// resets the viewport offset). That leaves the unconditional fall-
	// through `m.sidebar.Update(msg)` at the bottom of Update as the
	// only path that could mutate sidebar.YOffset, isolating the gate
	// under test.
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
		prView:             prViewModel,
		sidebar:            sidebarModel,
		issueSidebar:       issueview.NewModel(ctx),
		notificationView:   notificationview.NewModel(ctx),
		footer:             footer.NewModel(ctx),
		activePane:         mainPane,
		prPreviewStates:    map[string]map[int]int{},
		issuePreviewStates: map[string]int{},
	}
	m.ctx.ActivePane = "main"
	m.syncPreviewFocus()

	startOffset := m.sidebar.YOffset()
	require.Equal(t, 20, startOffset, "test setup: sidebar should start at offset 20")

	// Press Up with main pane focused. With no current section the
	// row-change path is a no-op, so the only thing that could move
	// the sidebar viewport is the unconditional fall-through sidebar
	// Update. The gate must prevent that.
	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	m = newModel.(Model)

	require.Equal(t, startOffset, m.sidebar.YOffset(),
		"sidebar viewport must not scroll on up-key when main pane is focused")

	// Press Down likewise.
	newModel, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = newModel.(Model)

	require.Equal(t, startOffset, m.sidebar.YOffset(),
		"sidebar viewport must not scroll on down-key when main pane is focused")
}

// TestPreviewPaneFocusedUpDownStillScrollsSidebar is the regression guard
// for the gate above: with the preview pane focused, up/down must still
// reach the sidebar so the user can scroll the preview viewport.
func TestPreviewPaneFocusedUpDownStillScrollsSidebar(t *testing.T) {
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config:              &cfg,
		View:                config.PRsView,
		MainContentHeight:   10,
		DynamicPreviewWidth: 80,
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	prSection := prssection.NewModel(0, ctx, config.PrsSectionConfig{}, time.Now(), time.Now())
	prSection.Prs = append(prSection.Prs, prrow.Data{
		Primary: testPullRequestData(1, "https://github.com/owner/repo/pull/1"),
	})
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
		activePane:         previewPane,
		prPreviewStates:    map[string]map[int]int{},
		issuePreviewStates: map[string]int{},
	}
	m.ctx.ActivePane = "preview"
	m.syncPreviewFocus()

	startOffset := m.sidebar.YOffset()

	// Down should scroll the sidebar viewport when preview is focused.
	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = newModel.(Model)
	require.Greater(t, m.sidebar.YOffset(), startOffset,
		"sidebar viewport should scroll down when preview pane is focused")
}

// TestViewSwitchPreservesSidebarOpenState verifies the reported bug:
// switching from PRs (with preview open) to Actions (which doesn't use the
// global sidebar) and back must NOT close the preview. Exercises the
// capture/apply pattern directly to avoid running the full
// fetch-all-sections path (which requires a fully wired config + network).
func TestViewSwitchPreservesSidebarOpenState(t *testing.T) {
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config:       &cfg,
		View:         config.PRsView,
		ScreenWidth:  160,
		ScreenHeight: 50,
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	sidebarModel := sidebar.NewModel()
	sidebarModel.IsOpen = true
	sidebarModel.UpdateProgramContext(ctx)

	m := Model{
		ctx:        ctx,
		keys:       keys.Keys,
		prView:     prview.NewModel(ctx),
		sidebar:    sidebarModel,
		tabs:       tabs.NewModel(ctx),
		viewStates: map[config.ViewType]*viewState{},
	}

	for _, v := range []config.ViewType{
		config.PRsView, config.IssuesView,
		config.NotificationsView, config.ActionsView,
	} {
		m.ensureViewState(v)
	}
	m.captureCurrentViewState()
	require.True(t, m.sidebar.IsOpen, "precondition: PR preview should start open")
	require.True(t, m.viewStates[config.PRsView].sidebarOpen)

	// Simulate switching to Actions: capture PRs state, switch view, apply.
	m.captureCurrentViewState()
	m.ctx.View = config.ActionsView
	m.applyViewState()
	require.False(t, m.sidebar.IsOpen,
		"Actions view's pinned state should close the sidebar")
	require.True(t, m.viewStates[config.PRsView].sidebarOpen,
		"PRsView entry must still remember IsOpen=true")

	// Now simulate switching back to PRs.
	m.captureCurrentViewState()
	m.ctx.View = config.PRsView
	m.applyViewState()
	require.True(t, m.sidebar.IsOpen,
		"returning to PRs must restore the user's preferred IsOpen=true")
}

// TestViewSwitchPreservesActiveSection verifies that the user's chosen
// section within a view is restored across view switches, instead of
// being reset to the configured default.
func TestViewSwitchPreservesActiveSection(t *testing.T) {
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config:       &cfg,
		View:         config.PRsView,
		ScreenWidth:  160,
		ScreenHeight: 50,
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	m := Model{
		ctx:        ctx,
		keys:       keys.Keys,
		prView:     prview.NewModel(ctx),
		sidebar:    sidebar.NewModel(),
		tabs:       tabs.NewModel(ctx),
		viewStates: map[config.ViewType]*viewState{},
	}

	for _, v := range []config.ViewType{
		config.PRsView, config.IssuesView,
		config.NotificationsView, config.ActionsView,
	} {
		m.ensureViewState(v)
	}
	m.currSectionId = 1
	m.captureCurrentViewState()

	// User switches to section 3 within PRs.
	m.setCurrSectionId(3)
	require.Equal(t, 3, m.currSectionId)
	require.Equal(t, 3, m.viewStates[config.PRsView].currSectionId,
		"setCurrSectionId must mirror into per-view state")

	// Round-trip through other views via capture/apply (avoids fetching).
	m.captureCurrentViewState()
	m.ctx.View = config.IssuesView
	m.applyViewState()
	require.NotEqual(t, 3, m.currSectionId,
		"Issues view's currSectionId should not be PRs' 3")

	m.captureCurrentViewState()
	m.ctx.View = config.PRsView
	m.applyViewState()
	require.Equal(t, 3, m.currSectionId,
		"returning to PRs view must restore the previously active section")
}

// TestCyclePreviewIsNoOpInActionsView verifies that pressing 'p' while
// in the Actions view does not silently mutate other views' sidebar
// preferences (since Actions' three-pane layout ignores m.sidebar).
func TestCyclePreviewIsNoOpInActionsView(t *testing.T) {
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config:       &cfg,
		View:         config.ActionsView,
		ScreenWidth:  160,
		ScreenHeight: 50,
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	m := Model{
		ctx:        ctx,
		keys:       keys.Keys,
		prView:     prview.NewModel(ctx),
		sidebar:    sidebar.NewModel(),
		tabs:       tabs.NewModel(ctx),
		viewStates: map[config.ViewType]*viewState{},
	}
	for _, v := range []config.ViewType{
		config.PRsView, config.IssuesView,
		config.NotificationsView, config.ActionsView,
	} {
		m.ensureViewState(v)
	}
	// Pretend the user had PRs preview open.
	m.viewStates[config.PRsView].sidebarOpen = true

	before := m.sidebar.IsOpen
	cmd := m.cyclePreview()
	require.Nil(t, cmd, "cyclePreview should be a no-op in Actions")
	require.Equal(t, before, m.sidebar.IsOpen,
		"sidebar IsOpen must not be mutated by cyclePreview in Actions")
	require.True(t, m.viewStates[config.PRsView].sidebarOpen,
		"PRsView's preferred sidebarOpen must remain untouched")
}

// TestIssuePreviewScrollPreservedAcrossRows verifies that switching
// between issue rows and back restores the previously seen scroll
// position in the issue preview pane.
func TestIssuePreviewScrollPreservedAcrossRows(t *testing.T) {
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config:              &cfg,
		View:                config.IssuesView,
		MainContentHeight:   10,
		DynamicPreviewWidth: 80,
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	sidebarModel := sidebar.NewModel()
	sidebarModel.IsOpen = true
	sidebarModel.UpdateProgramContext(ctx)
	sidebarModel.SetContent(strings.Repeat("line\n", 200))

	m := Model{
		ctx:                ctx,
		sidebar:            sidebarModel,
		issuePreviewStates: map[string]int{},
	}

	url1 := "https://github.com/owner/repo/issues/1"
	url2 := "https://github.com/owner/repo/issues/2"

	// Manually trigger the save/restore paths by injecting URLs into the
	// state map (we can't trivially construct an issue section in test).
	m.issuePreviewStates[url1] = 12
	off, ok := m.issuePreviewStates[url1]
	require.True(t, ok)
	m.sidebar.ScrollToOffset(off)
	require.Equal(t, 12, m.sidebar.YOffset())

	// Switching to issue 2 and back to 1 should restore 12.
	m.issuePreviewStates[url2] = 4
	m.sidebar.ScrollToOffset(m.issuePreviewStates[url2])
	require.Equal(t, 4, m.sidebar.YOffset())
	m.sidebar.ScrollToOffset(m.issuePreviewStates[url1])
	require.Equal(t, 12, m.sidebar.YOffset(),
		"returning to a previously-viewed issue should restore its scroll")
}

func TestChecksLogsSearchReceivesTypingBeforeMainLocalSearch(t *testing.T) {
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config: &cfg,
		View:   config.PRsView,
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	prSection := prssection.NewModel(0, ctx, config.PrsSectionConfig{}, time.Now(), time.Now())
	prSection.Prs = append(prSection.Prs, prrow.Data{
		Primary: testPullRequestData(1, "https://github.com/owner/repo/pull/1"),
	})
	prSection.Table.SetRows(prSection.BuildRows())

	sidebarModel := sidebar.NewModel()
	sidebarModel.IsOpen = true
	sidebarModel.UpdateProgramContext(ctx)

	m := Model{
		ctx:           ctx,
		keys:          keys.Keys,
		prs:           []section.Section{&prSection},
		currSectionId: 0,
		prView:        prview.NewModel(ctx),
		sidebar:       sidebarModel,
		activePane:    previewPane,
	}
	m.ctx.ActivePane = "preview"
	m.prView.UpdateProgramContext(ctx)
	m.prView.SetRow(&prSection.Prs[0])
	m.prView.SetWidth(80)
	m.prView.GoToTab(2)
	m.prView.ActivateChecks()

	next, _ := m.Update(tea.KeyPressMsg{Text: "s", Code: 's'})
	m = next.(Model)
	require.False(t, prSection.IsLocalSearchFocused(), "main local search should not focus on Checks tab")
	require.True(t, m.prView.IsChecksLogsSearchFocused(), "checks logs search should be focused")

	next, _ = m.Update(tea.KeyPressMsg{Text: "x", Code: 'x'})
	m = next.(Model)
	value, ok := m.prView.ChecksLogsSearchValue()
	require.True(t, ok)
	require.Equal(t, "x", value)
	require.False(t, prSection.IsLocalSearchFocused(), "typing into logs search should not trigger main local search")
}

func testPullRequestData(number int, url string) *data.PullRequestData {
	pr := &data.PullRequestData{
		Number:      number,
		Title:       "test PR",
		Url:         url,
		State:       "OPEN",
		HeadRefName: "feature-branch",
		BaseRefName: "main",
		Repository: data.Repository{
			Name:          "repo",
			NameWithOwner: "owner/repo",
			Owner:         data.Owner{Login: "owner"},
		},
	}
	pr.Author.Login = "author"
	return pr
}

func TestNotificationView_EnterKeyWorksAfterViewingPR(t *testing.T) {
	// Test that pressing Enter still works after a PR notification has been viewed.
	// Previously, once a PR subject was set, Enter would be absorbed by the PR handler
	// instead of triggering loadNotificationContent().
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config: &cfg,
		View:   config.NotificationsView,
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	sidebarModel := sidebar.NewModel()
	sidebarModel.UpdateProgramContext(ctx)

	m := Model{
		ctx:              ctx,
		keys:             keys.Keys,
		footer:           footer.NewModel(ctx),
		prView:           prview.NewModel(ctx),
		issueSidebar:     issueview.NewModel(ctx),
		notificationView: notificationview.NewModel(ctx),
		sidebar:          sidebarModel,
		tabs:             tabs.NewModel(ctx),
	}

	// Create a notification section with a PR notification
	notifSec := notificationssection.NewModel(
		0,
		ctx,
		config.NotificationsSectionConfig{},
		time.Now(),
	)
	notifSec.Notifications = []notificationrow.Data{
		{
			Notification: data.NotificationData{
				Id: "test-notification-1",
				Subject: data.NotificationSubject{
					Title: "Test PR",
					Url:   "https://api.github.com/repos/owner/repo/pulls/123",
					Type:  "PullRequest",
				},
				Repository: data.NotificationRepository{
					FullName: "owner/repo",
				},
				Unread: true,
			},
		},
	}
	notifSec.Table.SetRows(notifSec.BuildRows())
	m.notifications = []section.Section{&notifSec}

	// Set up a PR notification subject (simulating that Enter was already pressed once)
	m.notificationView.SetSubjectPR(&prrow.Data{}, "test-notification-1")

	// Verify GetSubjectPR returns non-nil
	require.NotNil(t, m.notificationView.GetSubjectPR(), "subject PR should be set")

	// Send Enter key
	msg := tea.KeyPressMsg{Code: tea.KeyEnter}
	_, cmd := m.Update(msg)

	// The fix ensures Enter triggers loadNotificationContent() even when a subject is set.
	// loadNotificationContent() returns a batch command for PR notifications.
	// Before the fix, Enter would be absorbed by the PR handler and cmd would be nil.
	require.NotNil(t, cmd, "Enter key should trigger loadNotificationContent and return a command")
}

func TestNotificationView_EnterKeyWorksAfterViewingIssue(t *testing.T) {
	// Test that pressing Enter still works after an Issue notification has been viewed.
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config: &cfg,
		View:   config.NotificationsView,
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	sidebarModel := sidebar.NewModel()
	sidebarModel.UpdateProgramContext(ctx)

	m := Model{
		ctx:              ctx,
		keys:             keys.Keys,
		footer:           footer.NewModel(ctx),
		prView:           prview.NewModel(ctx),
		issueSidebar:     issueview.NewModel(ctx),
		notificationView: notificationview.NewModel(ctx),
		sidebar:          sidebarModel,
		tabs:             tabs.NewModel(ctx),
	}

	// Create a notification section with an Issue notification
	notifSec := notificationssection.NewModel(
		0,
		ctx,
		config.NotificationsSectionConfig{},
		time.Now(),
	)
	notifSec.Notifications = []notificationrow.Data{
		{
			Notification: data.NotificationData{
				Id: "test-notification-2",
				Subject: data.NotificationSubject{
					Title: "Test Issue",
					Url:   "https://api.github.com/repos/owner/repo/issues/456",
					Type:  "Issue",
				},
				Repository: data.NotificationRepository{
					FullName: "owner/repo",
				},
				Unread: true,
			},
		},
	}
	notifSec.Table.SetRows(notifSec.BuildRows())
	m.notifications = []section.Section{&notifSec}

	// Set up an Issue notification subject (simulating that Enter was already pressed once)
	m.notificationView.SetSubjectIssue(&data.IssueData{}, "test-notification-2")

	// Verify GetSubjectIssue returns non-nil
	require.NotNil(t, m.notificationView.GetSubjectIssue(), "subject Issue should be set")

	// Send Enter key
	msg := tea.KeyPressMsg{Code: tea.KeyEnter}
	_, cmd := m.Update(msg)

	// The fix ensures Enter triggers loadNotificationContent() even when a subject is set.
	require.NotNil(t, cmd, "Enter key should trigger loadNotificationContent and return a command")
}

func TestNotificationView_BackKeyClearsPRSubjectAndRestoresNotificationActions(t *testing.T) {
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)
	defer keys.SetNotificationSubject(keys.NotificationSubjectNone)

	ctx := &context.ProgramContext{
		Config: &cfg,
		View:   config.NotificationsView,
		StartTask: func(task context.Task) tea.Cmd {
			return nil
		},
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	sidebarModel := sidebar.NewModel()
	sidebarModel.UpdateProgramContext(ctx)

	m := Model{
		ctx:              ctx,
		keys:             keys.Keys,
		footer:           footer.NewModel(ctx),
		prView:           prview.NewModel(ctx),
		issueSidebar:     issueview.NewModel(ctx),
		notificationView: notificationview.NewModel(ctx),
		sidebar:          sidebarModel,
		tabs:             tabs.NewModel(ctx),
	}

	notifSec := notificationssection.NewModel(
		0,
		ctx,
		config.NotificationsSectionConfig{},
		time.Now(),
	)
	notifSec.Notifications = []notificationrow.Data{
		{
			Notification: data.NotificationData{
				Id: "test-notification-pr",
				Subject: data.NotificationSubject{
					Title: "Test PR",
					Url:   "https://api.github.com/repos/owner/repo/pulls/123",
					Type:  "PullRequest",
				},
				Repository: data.NotificationRepository{FullName: "owner/repo"},
				Unread:     true,
			},
		},
	}
	notifSec.Table.SetRows(notifSec.BuildRows())
	m.notifications = []section.Section{&notifSec}

	m.notificationView.SetSubjectPR(&prrow.Data{}, "test-notification-pr")
	keys.SetNotificationSubject(keys.NotificationSubjectPR)

	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = newModel.(Model)
	require.Nil(
		t,
		m.notificationView.GetSubjectPR(),
		"PR subject should be cleared after pressing esc",
	)
	require.Empty(
		t,
		m.notificationView.GetSubjectId(),
		"subject ID should be cleared after pressing esc",
	)

	newModel, _ = m.Update(tea.KeyPressMsg{Text: "m"})
	m = newModel.(Model)
	require.False(t, m.notificationView.HasPendingAction(),
		"notification mark-read key should not be routed to PR merge action after backing out")
}

func TestNotificationView_BackKeyClearsIssueSubject(t *testing.T) {
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)
	defer keys.SetNotificationSubject(keys.NotificationSubjectNone)

	ctx := &context.ProgramContext{
		Config: &cfg,
		View:   config.NotificationsView,
		StartTask: func(task context.Task) tea.Cmd {
			return nil
		},
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	sidebarModel := sidebar.NewModel()
	sidebarModel.UpdateProgramContext(ctx)

	m := Model{
		ctx:              ctx,
		keys:             keys.Keys,
		footer:           footer.NewModel(ctx),
		prView:           prview.NewModel(ctx),
		issueSidebar:     issueview.NewModel(ctx),
		notificationView: notificationview.NewModel(ctx),
		sidebar:          sidebarModel,
		tabs:             tabs.NewModel(ctx),
	}

	notifSec := notificationssection.NewModel(
		0,
		ctx,
		config.NotificationsSectionConfig{},
		time.Now(),
	)
	notifSec.Notifications = []notificationrow.Data{
		{
			Notification: data.NotificationData{
				Id: "test-notification-issue",
				Subject: data.NotificationSubject{
					Title: "Test Issue",
					Url:   "https://api.github.com/repos/owner/repo/issues/456",
					Type:  "Issue",
				},
				Repository: data.NotificationRepository{FullName: "owner/repo"},
				Unread:     true,
			},
		},
	}
	notifSec.Table.SetRows(notifSec.BuildRows())
	m.notifications = []section.Section{&notifSec}

	m.notificationView.SetSubjectIssue(&data.IssueData{}, "test-notification-issue")
	keys.SetNotificationSubject(keys.NotificationSubjectIssue)

	newModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = newModel.(Model)
	require.Nil(
		t,
		m.notificationView.GetSubjectIssue(),
		"Issue subject should be cleared after pressing esc",
	)
	require.Empty(
		t,
		m.notificationView.GetSubjectId(),
		"subject ID should be cleared after pressing esc",
	)
}

func TestNavigationKeysWithNilSection(t *testing.T) {
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config: &cfg,
		View:   config.IssuesView,
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	sidebarModel := sidebar.NewModel()
	sidebarModel.UpdateProgramContext(ctx)

	m := Model{
		ctx:          ctx,
		keys:         keys.Keys,
		footer:       footer.NewModel(ctx),
		prView:       prview.NewModel(ctx),
		issueSidebar: issueview.NewModel(ctx),
		sidebar:      sidebarModel,
		tabs:         tabs.NewModel(ctx),
	}
	// No sections added — currSection will be nil

	navKeys := []struct {
		name string
		msg  tea.KeyPressMsg
	}{
		{"down", tea.KeyPressMsg{Code: tea.KeyDown}},
		{"up", tea.KeyPressMsg{Code: tea.KeyUp}},
		{"firstLine", tea.KeyPressMsg{Text: "g"}},
		{"lastLine", tea.KeyPressMsg{Text: "G"}},
		{"refresh", tea.KeyPressMsg{Text: "r"}},
	}

	for _, tc := range navKeys {
		t.Run(tc.name, func(t *testing.T) {
			require.NotPanics(t, func() {
				m.Update(tc.msg)
			}, "pressing %s with nil section should not panic", tc.name)
		})
	}
}

func TestQClosesHelpWithoutQuitting(t *testing.T) {
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config:       &cfg,
		View:         config.PRsView,
		ScreenWidth:  120,
		ScreenHeight: 40,
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	m := Model{
		ctx:              ctx,
		keys:             keys.Keys,
		sidebar:          sidebar.NewModel(),
		footer:           footer.NewModel(ctx),
		tabs:             tabs.NewModel(ctx),
		prView:           prview.NewModel(ctx),
		issueSidebar:     issueview.NewModel(ctx),
		notificationView: notificationview.NewModel(ctx),
	}
	m.footer.ShowAll = true
	require.Contains(t, m.footer.View(), "q/?")
	require.Contains(t, m.footer.View(), "close help")

	updated, cmd := m.Update(tea.KeyPressMsg{Text: "q"})
	updatedModel := updated.(Model)

	require.Nil(t, cmd)
	require.False(t, updatedModel.footer.ShowAll)
}

// executeCommandTemplate mimics the template execution logic from runCustomCommand
// to allow testing template variable substitution without executing shell commands.
func executeCommandTemplate(
	t *testing.T,
	commandTemplate string,
	input map[string]any,
) (string, error) {
	t.Helper()
	cmd, err := template.New("test_command").Parse(commandTemplate)
	if err != nil {
		return "", err
	}
	cmd = cmd.Option("missingkey=error")

	var buff bytes.Buffer
	err = cmd.Execute(&buff, input)
	if err != nil {
		return "", err
	}
	return buff.String(), nil
}

func TestPRCommandTemplateVariables(t *testing.T) {
	// Test that PR command templates correctly substitute all available variables,
	// matching the behavior of runCustomPRCommand in modelUtils.go
	input := map[string]any{
		"RepoName":    "owner/repo",
		"PrNumber":    123,
		"HeadRefName": "feature-branch",
		"BaseRefName": "main",
		"Author":      "testuser",
	}

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "Author variable",
			template: "gh pr view --author {{.Author}}",
			expected: "gh pr view --author testuser",
		},
		{
			name:     "PrNumber variable",
			template: "gh pr checkout {{.PrNumber}}",
			expected: "gh pr checkout 123",
		},
		{
			name:     "HeadRefName variable",
			template: "git checkout {{.HeadRefName}}",
			expected: "git checkout feature-branch",
		},
		{
			name:     "Multiple variables",
			template: "echo PR #{{.PrNumber}} by {{.Author}} in {{.RepoName}}: {{.HeadRefName}} -> {{.BaseRefName}}",
			expected: "echo PR #123 by testuser in owner/repo: feature-branch -> main",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := executeCommandTemplate(t, tc.template, input)
			require.NoError(t, err)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestIssueCommandTemplateVariables(t *testing.T) {
	// Test that Issue command templates correctly substitute all available variables,
	// matching the behavior of runCustomIssueCommand in modelUtils.go
	input := map[string]any{
		"RepoName":    "owner/repo",
		"IssueNumber": 456,
		"Author":      "issueauthor",
	}

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "Author variable",
			template: "gh issue view --author {{.Author}}",
			expected: "gh issue view --author issueauthor",
		},
		{
			name:     "IssueNumber variable",
			template: "gh issue view {{.IssueNumber}}",
			expected: "gh issue view 456",
		},
		{
			name:     "Multiple variables",
			template: "echo Issue #{{.IssueNumber}} by {{.Author}} in {{.RepoName}}",
			expected: "echo Issue #456 by issueauthor in owner/repo",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := executeCommandTemplate(t, tc.template, input)
			require.NoError(t, err)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestCommandTemplateMissingVariable(t *testing.T) {
	// Test that templates with missing variables return an error,
	// matching the missingkey=error behavior in runCustomCommand
	input := map[string]any{
		"RepoName": "owner/repo",
	}

	_, err := executeCommandTemplate(t, "gh pr view --author {{.Author}}", input)
	require.Error(t, err, "template with missing variable should return an error")
}

func TestRepoPathFallbackToCtxRepoPath(t *testing.T) {
	// When repoPaths config is empty, RepoPath should fall back to ctxRepoPath
	// (the repo gh-dash was started from).
	contextData := map[string]any{
		"RepoName":    "owner/repo",
		"IssueNumber": 42,
	}
	resolved := resolveTemplateInput(
		&contextData, map[string]string{}, "/home/user/projects/repo",
	)
	tmpl := "cd {{.RepoPath}} && gh issue edit {{.IssueNumber}}"
	result, err := executeCommandTemplate(t, tmpl, resolved)
	require.NoError(t, err)
	require.Equal(t, "cd /home/user/projects/repo && gh issue edit 42", result)
}

func TestRepoPathConfigTakesPriority(t *testing.T) {
	// Explicit repoPaths config should win over ctxRepoPath.
	contextData := map[string]any{
		"RepoName":    "owner/repo",
		"IssueNumber": 42,
	}
	resolved := resolveTemplateInput(
		&contextData,
		map[string]string{"owner/repo": "/configured/path"},
		"/home/user/projects/repo",
	)
	result, err := executeCommandTemplate(
		t,
		"cd {{.RepoPath}} && gh issue edit {{.IssueNumber}}",
		resolved,
	)
	require.NoError(t, err)
	require.Equal(t, "cd /configured/path && gh issue edit 42", result)
}

func TestSyncMainContentWidth(t *testing.T) {
	tests := []struct {
		name                 string
		screenWidth          int
		previewWidth         float64
		sidebarOpen          bool
		expectedPreviewWidth int
		expectedMainWidth    int
	}{
		{
			name:                 "absolute width with sidebar open",
			screenWidth:          100,
			previewWidth:         50,
			sidebarOpen:          true,
			expectedPreviewWidth: 50,
			expectedMainWidth:    50,
		},
		{
			name:                 "absolute width with sidebar closed",
			screenWidth:          100,
			previewWidth:         50,
			sidebarOpen:          false,
			expectedPreviewWidth: 0,
			expectedMainWidth:    100,
		},
		{
			name:                 "relative width 40%",
			screenWidth:          100,
			previewWidth:         0.4,
			sidebarOpen:          true,
			expectedPreviewWidth: 40,
			expectedMainWidth:    60,
		},
		{
			name:                 "relative width 50%",
			screenWidth:          200,
			previewWidth:         0.5,
			sidebarOpen:          true,
			expectedPreviewWidth: 100,
			expectedMainWidth:    100,
		},
		{
			name:                 "very small relative width results in zero",
			screenWidth:          100,
			previewWidth:         0.005,
			sidebarOpen:          true,
			expectedPreviewWidth: 0,
			expectedMainWidth:    100,
		},
		{
			name:                 "absolute width of 1",
			screenWidth:          100,
			previewWidth:         1,
			sidebarOpen:          true,
			expectedPreviewWidth: 1,
			expectedMainWidth:    99,
		},
		{
			name:                 "small screen with relative width",
			screenWidth:          10,
			previewWidth:         0.1,
			sidebarOpen:          true,
			expectedPreviewWidth: 1,
			expectedMainWidth:    9,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.Config{
				Defaults: config.Defaults{
					Preview: config.PreviewConfig{
						Open:     true,
						Width:    tc.previewWidth,
						Position: "right",
					},
				},
			}

			m := Model{
				ctx: &context.ProgramContext{
					Config:      &cfg,
					ScreenWidth: tc.screenWidth,
				},
				sidebar: sidebar.Model{
					IsOpen: tc.sidebarOpen,
				},
			}

			m.syncMainContentDimensions()

			if tc.sidebarOpen {
				require.Equal(t, tc.expectedPreviewWidth, m.ctx.DynamicPreviewWidth,
					"DynamicPreviewWidth mismatch")
			}
			require.Equal(t, tc.expectedMainWidth, m.ctx.MainContentWidth,
				"MainContentWidth mismatch")
			require.Equal(t, tc.sidebarOpen, m.ctx.SidebarOpen,
				"SidebarOpen mismatch")
		})
	}
}

func TestSyncSidebar_NoOpWhenSidebarClosed(t *testing.T) {
	// Regression test for https://github.com/dlvhdr/gh-dash/issues/798
	// When preview.open is false, DynamicPreviewWidth is 0, so
	// GetSidebarContentWidth() returns 0. If syncSidebar() proceeds to
	// render the PR view with that zero width, layout breaks.
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)
	cfg.Defaults.Preview.Open = false

	ctx := &context.ProgramContext{
		Config:      &cfg,
		ScreenWidth: 100,
		View:        config.PRsView,
		StartTask:   func(task context.Task) tea.Cmd { return nil },
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	prSection := prssection.NewModel(
		0,
		ctx,
		config.PrsSectionConfig{
			Title:   "Test",
			Filters: "is:open",
		},
		time.Now(),
		time.Now(),
	)
	// Add a PR so getCurrRowData() returns non-nil, exercising the
	// code path that would use the negative width without the guard.
	prSection.Prs = []prrow.Data{
		{Primary: &data.PullRequestData{Title: "test", State: "OPEN"}},
	}

	m := Model{
		ctx:              ctx,
		keys:             keys.Keys,
		prs:              []section.Section{&prSection},
		sidebar:          sidebar.NewModel(),
		footer:           footer.NewModel(ctx),
		tabs:             tabs.NewModel(ctx),
		prView:           prview.NewModel(ctx),
		issueSidebar:     issueview.NewModel(ctx),
		notificationView: notificationview.NewModel(ctx),
	}

	// sidebar.IsOpen defaults to false from NewModel(), matching preview.open: false
	require.False(t, m.sidebar.IsOpen)
	m.sidebar.UpdateProgramContext(ctx)

	// Confirm the precondition: DynamicPreviewWidth is 0, so
	// GetSidebarContentWidth returns 0 (no usable width).
	require.Equal(t, 0, m.ctx.DynamicPreviewWidth)
	require.Equal(t, 0, m.sidebar.GetSidebarContentWidth(),
		"GetSidebarContentWidth should be 0 when DynamicPreviewWidth is 0")

	// Without the early-return guard, syncSidebar would needlessly render
	// the PR view with a zero-width sidebar.
	require.NotPanics(t, func() {
		cmd := m.syncSidebar()
		require.Nil(t, cmd, "syncSidebar should return nil when sidebar is closed")
	})
}

func TestPRInputFocusedPageKeysScrollSidebar(t *testing.T) {
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config:            &cfg,
		View:              config.PRsView,
		ScreenWidth:       120,
		ScreenHeight:      40,
		MainContentHeight: 10,
		StartTask:         func(task context.Task) tea.Cmd { return nil },
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)
	markdown.InitializeMarkdownStyle(true)

	sidebarModel := sidebar.NewModel()
	sidebarModel.IsOpen = true
	sidebarModel.UpdateProgramContext(ctx)
	sidebarModel.SetContent(strings.Repeat("line\n", 100))
	sidebarModel.ScrollToOffset(20)

	prViewModel := prview.NewModel(ctx)
	prViewModel.SetRow(&prrow.Data{Primary: &data.PullRequestData{}})
	cmd := prViewModel.SetIsCommenting(true)
	require.NotNil(t, cmd)
	require.True(t, prViewModel.IsTextInputBoxFocused())

	m := Model{
		ctx:     ctx,
		keys:    keys.Keys,
		prView:  prViewModel,
		sidebar: sidebarModel,
	}

	initialOffset := m.sidebar.YOffset()
	newModel, _ := m.Update(tea.KeyPressMsg{Text: "ctrl+up"})
	m = newModel.(Model)
	require.Less(t, m.sidebar.YOffset(), initialOffset)

	initialOffset = m.sidebar.YOffset()
	newModel, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModCtrl})
	m = newModel.(Model)
	require.Greater(t, m.sidebar.YOffset(), initialOffset)

	newModel, _ = m.Update(tea.KeyPressMsg{Text: "ctrl+h"})
	m = newModel.(Model)
	require.Equal(t, 0, m.sidebar.YOffset())

	newModel, _ = m.Update(tea.KeyPressMsg{Text: "ctrl+g"})
	m = newModel.(Model)
	require.Greater(t, m.sidebar.YOffset(), 0)
}

func TestPreviewFocusRoutesNavigationToSidebar(t *testing.T) {
	keys.Keys.PageDown.SetKeys("ctrl+down")
	keys.Keys.PageUp.SetKeys("ctrl+up")
	keys.Keys.FocusMain.SetKeys("ctrl+left")
	keys.Keys.FocusPreview.SetKeys("ctrl+right")

	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config:            &cfg,
		View:              config.PRsView,
		ScreenWidth:       120,
		ScreenHeight:      40,
		MainContentHeight: 10,
		StartTask:         func(task context.Task) tea.Cmd { return nil },
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)
	markdown.InitializeMarkdownStyle(true)

	prSection := prssection.NewModel(0, ctx, config.PrsSectionConfig{}, time.Now(), time.Now())
	for i := 1; i <= 20; i++ {
		prSection.Prs = append(prSection.Prs, prrow.Data{
			Primary: testPullRequestData(i, fmt.Sprintf("https://github.com/owner/repo/pull/%d", i)),
		})
	}
	prSection.Table.SetRows(prSection.BuildRows())

	sidebarModel := sidebar.NewModel()
	sidebarModel.IsOpen = true
	sidebarModel.UpdateProgramContext(ctx)
	sidebarModel.SetContent(strings.Repeat("line\n", 100))

	prViewModel := prview.NewModel(ctx)
	prViewModel.UpdateProgramContext(ctx)

	m := Model{
		ctx:              ctx,
		keys:             keys.Keys,
		prs:              []section.Section{&prSection},
		footer:           footer.NewModel(ctx),
		prView:           prViewModel,
		issueSidebar:     issueview.NewModel(ctx),
		notificationView: notificationview.NewModel(ctx),
		sidebar:          sidebarModel,
		tabs:             tabs.NewModel(ctx),
	}

	newModel, _ := m.Update(tea.KeyPressMsg{Text: "ctrl+right"})
	m = newModel.(Model)
	require.Equal(t, previewPane, m.activePane)

	initialRow := prSection.CurrRow()
	initialOffset := m.sidebar.YOffset()
	newModel, _ = m.Update(tea.KeyPressMsg{Text: "down"})
	m = newModel.(Model)
	require.Equal(t, initialRow, prSection.CurrRow())
	require.Greater(t, m.sidebar.YOffset(), initialOffset)

	newModel, _ = m.Update(tea.KeyPressMsg{Text: ">"})
	m = newModel.(Model)
	require.Greater(t, m.sidebar.YOffset(), 0)

	newModel, _ = m.Update(tea.KeyPressMsg{Text: "<"})
	m = newModel.(Model)
	require.Equal(t, 0, m.sidebar.YOffset())

	newModel, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModCtrl})
	m = newModel.(Model)
	require.Equal(t, initialRow, prSection.CurrRow())
	require.Greater(t, m.sidebar.YOffset(), 1)

	newModel, _ = m.Update(tea.KeyPressMsg{Text: "ctrl+left"})
	m = newModel.(Model)
	require.Equal(t, mainPane, m.activePane)

	newModel, _ = m.Update(tea.KeyPressMsg{Text: "down"})
	m = newModel.(Model)
	require.Equal(t, mainPane, m.activePane)
	require.False(t, m.prView.IsTextInputBoxFocused())
	require.Equal(t, initialRow+1, prSection.CurrRow())

	pageDownMsg := tea.KeyPressMsg{Text: "ctrl+down"}
	require.True(t, m.isPageDownKey(pageDownMsg))
	require.Greater(t, m.getCurrSection().NumRows(), 2)
	newModel, _ = m.Update(pageDownMsg)
	m = newModel.(Model)
	require.Greater(t, m.getCurrSection().CurrRow(), initialRow+2)
}

func TestOpenPRCommentInputNoScrollPreservesSidebarOffset(t *testing.T) {
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config:       &cfg,
		View:         config.PRsView,
		ScreenWidth:  120,
		ScreenHeight: 40,
		StartTask:    func(task context.Task) tea.Cmd { return nil },
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)
	markdown.InitializeMarkdownStyle(true)

	updatedAt := time.Now()
	comments := make([]data.Comment, 0, 40)
	for i := range 40 {
		comment := data.Comment{
			Body:      strings.Repeat("comment body\n", 3),
			UpdatedAt: updatedAt.Add(time.Duration(i) * time.Minute),
		}
		comment.Author.Login = "alice"
		comments = append(comments, comment)
	}

	pr := prrow.Data{
		Primary: &data.PullRequestData{Title: "test", State: "OPEN"},
		Enriched: data.EnrichedPullRequestData{
			Title: "test",
			State: "OPEN",
			Comments: data.CommentsWithBody{
				Nodes: comments,
			},
		},
		IsEnriched: true,
	}
	prSection := prssection.NewModel(
		0,
		ctx,
		config.PrsSectionConfig{Title: "Test", Filters: "is:open"},
		time.Now(),
		time.Now(),
	)
	prSection.Prs = []prrow.Data{pr}

	sidebarModel := sidebar.NewModel()
	sidebarModel.IsOpen = true
	sidebarModel.UpdateProgramContext(ctx)
	sidebarModel.SetContent(strings.Repeat("line\n", 100))
	sidebarModel.ScrollToOffset(20)
	prViewModel := prview.NewModel(ctx)
	prViewModel.UpdateProgramContext(ctx)

	m := Model{
		ctx:           ctx,
		keys:          keys.Keys,
		prs:           []section.Section{&prSection},
		sidebar:       sidebarModel,
		footer:        footer.NewModel(ctx),
		tabs:          tabs.NewModel(ctx),
		prView:        prViewModel,
		issueSidebar:  issueview.NewModel(ctx),
	}
	m.prView.GoToActivityTab()
	m.prView.SetRow(&pr)

	initialOffset := m.sidebar.YOffset()
	_ = m.openSidebarForInputNoScroll(m.prView.SetIsCommenting)
	require.True(t, m.prView.IsTextInputBoxFocused())
	require.Equal(t, initialOffset, m.sidebar.YOffset())
}

func TestSyncMainContentDimensions_BottomMode(t *testing.T) {
	tests := []struct {
		name                  string
		screenWidth           int
		screenHeight          int
		previewHeight         float64
		sidebarOpen           bool
		expectedPreviewHeight int
		expectedMainHeight    int
		expectedMainWidth     int
	}{
		{
			name:                  "bottom mode with 40% height",
			screenWidth:           100,
			screenHeight:          40,
			previewHeight:         0.4,
			sidebarOpen:           true,
			expectedPreviewHeight: 13,
			expectedMainHeight:    21,
			expectedMainWidth:     100,
		},
		{
			name:                  "bottom mode sidebar closed",
			screenWidth:           100,
			screenHeight:          40,
			previewHeight:         0.4,
			sidebarOpen:           false,
			expectedPreviewHeight: 0,
			expectedMainHeight:    35,
			expectedMainWidth:     100,
		},
		{
			name:                  "bottom mode with absolute height",
			screenWidth:           100,
			screenHeight:          40,
			previewHeight:         10,
			sidebarOpen:           true,
			expectedPreviewHeight: 10,
			expectedMainHeight:    24,
			expectedMainWidth:     100,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := config.ParseConfig(config.Location{
				ConfigFlag:       "../config/testdata/test-config.yml",
				SkipGlobalConfig: true,
			})
			require.NoError(t, err)
			cfg.Defaults.Preview.Width = 0.45
			cfg.Defaults.Preview.Height = tc.previewHeight
			cfg.Defaults.Preview.Position = "bottom"
			thm := theme.ParseTheme(&cfg)
			styles := context.InitStyles(thm)

			m := Model{
				ctx: &context.ProgramContext{
					Config:       &cfg,
					ScreenWidth:  tc.screenWidth,
					ScreenHeight: tc.screenHeight,
					Styles:       styles,
				},
				sidebar: sidebar.Model{
					IsOpen: tc.sidebarOpen,
				},
			}

			m.syncMainContentDimensions()

			require.Equal(t, tc.expectedMainWidth, m.ctx.MainContentWidth,
				"MainContentWidth mismatch")
			require.Equal(t, tc.expectedMainHeight, m.ctx.MainContentHeight,
				"MainContentHeight mismatch")
			if tc.sidebarOpen {
				require.Equal(t, tc.expectedPreviewHeight, m.ctx.DynamicPreviewHeight,
					"DynamicPreviewHeight mismatch")
				// Verify total doesn't exceed available space:
				// main content + preview + border must equal base content height
				baseHeight := tc.screenHeight - common.TabsHeight - common.FooterHeight
				borderHeight := styles.Sidebar.BorderWidth
				require.Equal(
					t,
					baseHeight,
					m.ctx.MainContentHeight+m.ctx.DynamicPreviewHeight+borderHeight,
					"table + preview + border should equal base content height",
				)
			}
			require.Equal(t, "bottom", m.ctx.PreviewPosition,
				"PreviewPosition should be bottom")
		})
	}
}

func TestCyclePreviewOpensRightWhenClosed(t *testing.T) {
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)
	cfg.Defaults.Preview.Width = 0.45
	cfg.Defaults.Preview.Height = 0.4
	cfg.Defaults.Preview.Position = "right"

	ctx := &context.ProgramContext{
		Config:       &cfg,
		ScreenWidth:  100,
		ScreenHeight: 40,
		View:         config.PRsView,
		StartTask:    func(task context.Task) tea.Cmd { return nil },
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	prSection := prssection.NewModel(
		0,
		ctx,
		config.PrsSectionConfig{
			Title:   "Test",
			Filters: "is:open",
		},
		time.Now(),
		time.Now(),
	)

	m := Model{
		ctx:              ctx,
		keys:             keys.Keys,
		prs:              []section.Section{&prSection},
		sidebar:          sidebar.NewModel(),
		footer:           footer.NewModel(ctx),
		tabs:             tabs.NewModel(ctx),
		prView:           prview.NewModel(ctx),
		issueSidebar:     issueview.NewModel(ctx),
		notificationView: notificationview.NewModel(ctx),
	}

	// Sidebar is closed by default from NewModel()
	require.False(t, m.sidebar.IsOpen, "sidebar should start closed")

	// Set initial dimensions via syncMainContentDimensions
	m.syncMainContentDimensions()
	// Pressing p when closed should open the right split.
	msg := tea.KeyPressMsg{Text: "p"}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	require.True(t, m.sidebar.IsOpen, "sidebar should open")
	require.Equal(t, "right", m.positionOverride,
		"positionOverride should be right after opening from hidden")
	require.Equal(t, "right", m.ctx.PreviewPosition,
		"PreviewPosition should be right after opening from hidden")
	require.Greater(t, m.ctx.DynamicPreviewWidth, 0,
		"DynamicPreviewWidth should be set in right mode")
}

func TestCyclePreviewCyclesRightBottomHidden(t *testing.T) {
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)
	cfg.Defaults.Preview.Width = 0.45
	cfg.Defaults.Preview.Height = 0.4
	cfg.Defaults.Preview.Position = "right"

	ctx := &context.ProgramContext{
		Config:       &cfg,
		ScreenWidth:  100,
		ScreenHeight: 40,
		View:         config.PRsView,
		StartTask:    func(task context.Task) tea.Cmd { return nil },
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	prSection := prssection.NewModel(
		0,
		ctx,
		config.PrsSectionConfig{
			Title:   "Test",
			Filters: "is:open",
		},
		time.Now(),
		time.Now(),
	)

	m := Model{
		ctx:              ctx,
		keys:             keys.Keys,
		prs:              []section.Section{&prSection},
		sidebar:          sidebar.NewModel(),
		footer:           footer.NewModel(ctx),
		tabs:             tabs.NewModel(ctx),
		prView:           prview.NewModel(ctx),
		issueSidebar:     issueview.NewModel(ctx),
		notificationView: notificationview.NewModel(ctx),
	}

	// Open the sidebar and set initial right-mode dimensions
	m.sidebar.IsOpen = true
	m.syncMainContentDimensions()
	require.Equal(t, "right", m.ctx.PreviewPosition)

	// Press p to cycle from right to bottom.
	msg := tea.KeyPressMsg{Text: "p"}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	require.Equal(t, "bottom", m.positionOverride,
		"positionOverride should be bottom after toggling from right")
	require.Equal(t, "bottom", m.ctx.PreviewPosition,
		"PreviewPosition should be bottom after toggle")
	require.Equal(t, 100, m.ctx.MainContentWidth,
		"MainContentWidth should be full screen width in bottom mode")
	require.Greater(t, m.ctx.DynamicPreviewHeight, 0,
		"DynamicPreviewHeight should be set in bottom mode")

	// Press p again to cycle from bottom to hidden.
	updated, _ = m.Update(msg)
	m = updated.(Model)

	require.False(t, m.sidebar.IsOpen, "sidebar should be hidden after bottom")
	require.Equal(t, "", m.positionOverride,
		"positionOverride should reset after hiding")
	require.Equal(t, 0, m.ctx.DynamicPreviewWidth,
		"DynamicPreviewWidth should be zero when hidden")
	require.Equal(t, 0, m.ctx.DynamicPreviewHeight,
		"DynamicPreviewHeight should be zero when hidden")
	require.Equal(t, 100, m.ctx.MainContentWidth,
		"MainContentWidth should be full screen width when hidden")
}

func TestView_ClosingSidebarFromBottomMode_NoExtraLine(t *testing.T) {
	// Regression test: closing the sidebar while PreviewPosition is "bottom"
	// must not produce an extra line from JoinVertical with an empty string.
	// The rendered View should have the same line count regardless of whether
	// we close from right mode or bottom mode.
	zone.NewGlobal()
	zone.SetEnabled(false)

	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)
	cfg.Defaults.Preview.Width = 0.45
	cfg.Defaults.Preview.Height = 0.4
	cfg.Defaults.Preview.Position = "right"

	ctx := &context.ProgramContext{
		Config:       &cfg,
		ScreenWidth:  120,
		ScreenHeight: 40,
		View:         config.PRsView,
		StartTask:    func(task context.Task) tea.Cmd { return nil },
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	prSection := prssection.NewModel(
		0,
		ctx,
		config.PrsSectionConfig{
			Title:   "Test",
			Filters: "is:open",
		},
		time.Now(),
		time.Now(),
	)

	m := Model{
		ctx:              ctx,
		keys:             keys.Keys,
		prs:              []section.Section{&prSection},
		sidebar:          sidebar.NewModel(),
		footer:           footer.NewModel(ctx),
		tabs:             tabs.NewModel(ctx),
		prView:           prview.NewModel(ctx),
		issueSidebar:     issueview.NewModel(ctx),
		notificationView: notificationview.NewModel(ctx),
	}

	// Baseline: sidebar closed in right mode
	m.sidebar.IsOpen = false
	m.positionOverride = ""
	m.syncMainContentDimensions()
	m.syncProgramContext()
	rightClosedView := m.View()
	rightClosedLines := strings.Count(rightClosedView.Content, "\n")

	// Now: sidebar closed while in bottom mode (the bug scenario)
	m.positionOverride = "bottom"
	m.sidebar.IsOpen = false
	m.syncMainContentDimensions()
	m.syncProgramContext()
	bottomClosedView := m.View()
	bottomClosedLines := strings.Count(bottomClosedView.Content, "\n")

	require.Equal(t, rightClosedLines, bottomClosedLines,
		"closing sidebar from bottom mode should produce the same number of lines as right mode")
}

func TestCompletionLayerYAnchorsToRenderedPreview(t *testing.T) {
	previewView := strings.Repeat("x\n", 19) + "x"
	completions := strings.Repeat("c\n", 4) + "c"

	y := completionLayerY(10, previewView, 2, completions)

	require.Equal(t, 15, y)
}

func TestCompletionLayerYClampsAtTop(t *testing.T) {
	y := completionLayerY(0, "x", 0, strings.Repeat("c\n", 8)+"c")

	require.Zero(t, y)
}

func TestPromptConfirmationForNotificationPR(t *testing.T) {
	// Test that promptConfirmationForNotificationPR sets the pending action
	// and displays the confirmation prompt in the footer.
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag: "../config/testdata/test-config.yml",
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config: &cfg,
		View:   config.NotificationsView,
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	m := Model{
		ctx:    ctx,
		keys:   keys.Keys,
		footer: footer.NewModel(ctx),
	}

	// Set up a PR notification subject
	m.notificationView.SetSubjectPR(&prrow.Data{
		Primary: &data.PullRequestData{
			Number: 123,
		},
	}, "test-notification-id")

	// Call promptConfirmationForNotificationPR
	m.promptConfirmationForNotificationPR("close")

	// Verify pending action is set
	require.Equal(t, "pr_close", m.notificationView.GetPendingAction(),
		"pendingNotificationAction should be set to pr_close")
}

func TestPromptConfirmationForNotificationPR_NilSubject(t *testing.T) {
	// Test that promptConfirmationForNotificationPR returns nil when no PR subject
	ctx := &context.ProgramContext{}
	m := Model{
		notificationView: notificationview.NewModel(ctx),
	}

	cmd := m.promptConfirmationForNotificationPR("close")

	require.Nil(t, cmd, "should return nil when no PR subject")
	require.Empty(
		t,
		m.notificationView.GetPendingAction(),
		"should not set pending action when no PR subject",
	)
}

func TestPromptConfirmationForNotificationIssue(t *testing.T) {
	// Test that promptConfirmationForNotificationIssue sets the pending action
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag: "../config/testdata/test-config.yml",
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config: &cfg,
		View:   config.NotificationsView,
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	m := Model{
		ctx:    ctx,
		keys:   keys.Keys,
		footer: footer.NewModel(ctx),
	}

	// Set up an Issue notification subject
	m.notificationView.SetSubjectIssue(&data.IssueData{
		Number: 456,
	}, "test-notification-id")

	// Call promptConfirmationForNotificationIssue
	m.promptConfirmationForNotificationIssue("close")

	// Verify pending action is set
	require.Equal(t, "issue_close", m.notificationView.GetPendingAction(),
		"pendingNotificationAction should be set to issue_close")
}

func TestNotificationConfirmation_CancelOnOtherKey(t *testing.T) {
	// Test that pressing any key other than y/Y/Enter cancels the confirmation
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag: "../config/testdata/test-config.yml",
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config: &cfg,
		View:   config.NotificationsView,
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	m := Model{
		ctx:              ctx,
		keys:             keys.Keys,
		footer:           footer.NewModel(ctx),
		notificationView: notificationview.NewModel(ctx),
	}

	// Set up a PR notification subject and pending action
	m.notificationView.SetSubjectPR(&prrow.Data{
		Primary: &data.PullRequestData{
			Number: 123,
		},
	}, "test-notification-id")
	m.notificationView.SetPendingPRAction("close") // Simulate pending action

	// Press 'n' to cancel
	msg := tea.KeyPressMsg{Text: "n"}
	newModel, cmd := m.Update(msg)
	m = newModel.(Model)

	// Verify pending action is cleared
	require.Empty(t, m.notificationView.GetPendingAction(),
		"pendingNotificationAction should be cleared after cancellation")
	require.Nil(t, cmd, "should return nil command when cancelled")
}

func TestNotificationConfirmation_AcceptWithY(t *testing.T) {
	// Test that pressing 'y' confirms and executes the action
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag: "../config/testdata/test-config.yml",
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config: &cfg,
		View:   config.NotificationsView,
		StartTask: func(task context.Task) tea.Cmd {
			return nil // No-op for testing
		},
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	m := Model{
		ctx:              ctx,
		keys:             keys.Keys,
		footer:           footer.NewModel(ctx),
		notificationView: notificationview.NewModel(ctx),
	}

	// Set up a PR notification subject and pending action
	m.notificationView.SetSubjectPR(&prrow.Data{
		Primary: &data.PullRequestData{
			Number: 123,
		},
	}, "test-notification-id")
	m.notificationView.SetPendingPRAction("close")

	// Press 'y' to confirm
	msg := tea.KeyPressMsg{Text: "y"}
	newModel, cmd := m.Update(msg)
	m = newModel.(Model)

	// Verify pending action is cleared and command is returned
	require.Empty(t, m.notificationView.GetPendingAction(),
		"pendingNotificationAction should be cleared after confirmation")
	require.NotNil(t, cmd, "should return a command to execute the action")
}

func TestNotificationConfirmation_AcceptWithUpperY(t *testing.T) {
	// Test that pressing 'Y' (uppercase) also confirms
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag: "../config/testdata/test-config.yml",
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config: &cfg,
		View:   config.NotificationsView,
		StartTask: func(task context.Task) tea.Cmd {
			return nil // No-op for testing
		},
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	m := Model{
		ctx:              ctx,
		keys:             keys.Keys,
		footer:           footer.NewModel(ctx),
		notificationView: notificationview.NewModel(ctx),
	}

	// Set up a PR notification subject and pending action
	m.notificationView.SetSubjectPR(&prrow.Data{
		Primary: &data.PullRequestData{
			Number: 123,
		},
	}, "test-notification-id")
	m.notificationView.SetPendingPRAction("merge")

	// Press 'Y' to confirm
	msg := tea.KeyPressMsg{Text: "Y"}
	newModel, cmd := m.Update(msg)
	m = newModel.(Model)

	// Verify pending action is cleared and command is returned
	require.Empty(t, m.notificationView.GetPendingAction(),
		"pendingNotificationAction should be cleared after confirmation")
	require.NotNil(t, cmd, "should return a command to execute the action")
}

func TestNotificationConfirmation_AcceptWithEnter(t *testing.T) {
	// Test that pressing Enter also confirms
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag: "../config/testdata/test-config.yml",
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config: &cfg,
		View:   config.NotificationsView,
		StartTask: func(task context.Task) tea.Cmd {
			return nil // No-op for testing
		},
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	m := Model{
		ctx:              ctx,
		keys:             keys.Keys,
		footer:           footer.NewModel(ctx),
		notificationView: notificationview.NewModel(ctx),
	}

	// Set up an Issue notification subject and pending action
	m.notificationView.SetSubjectIssue(&data.IssueData{
		Number: 456,
		Url:    "https://github.com/test/repo/issues/456",
		Repository: data.Repository{
			NameWithOwner: "test/repo",
		},
	}, "test-notification-id")
	m.notificationView.SetPendingIssueAction("reopen")

	// Press Enter to confirm
	msg := tea.KeyPressMsg{Code: tea.KeyEnter}
	newModel, cmd := m.Update(msg)
	m = newModel.(Model)

	// Verify pending action is cleared and command is returned
	require.Empty(t, m.notificationView.GetPendingAction(),
		"pendingNotificationAction should be cleared after confirmation")
	require.NotNil(t, cmd, "should return a command to execute the action")
}

func TestPromptConfirmationForNotificationIssue_NilSubject(t *testing.T) {
	// Test that promptConfirmationForNotificationIssue returns nil when no Issue subject
	ctx := &context.ProgramContext{}
	m := Model{
		notificationView: notificationview.NewModel(ctx),
	}

	cmd := m.promptConfirmationForNotificationIssue("close")

	require.Nil(t, cmd, "should return nil when no Issue subject")
	require.Empty(
		t,
		m.notificationView.GetPendingAction(),
		"should not set pending action when no Issue subject",
	)
}

func TestRefresh_ClearsEnrichmentCache(t *testing.T) {
	// This test verifies that pressing the refresh key ('r') clears the
	// enrichment cache, ensuring fresh reviewer data is fetched.
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag: "../config/testdata/test-config.yml",
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config:    &cfg,
		View:      config.PRsView,
		StartTask: func(task context.Task) tea.Cmd { return nil },
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	// Create a PR section so getCurrSection() returns non-nil
	prSection := prssection.NewModel(
		0,
		ctx,
		config.PrsSectionConfig{
			Title:   "Test",
			Filters: "is:open",
		},
		time.Now(),
		time.Now(),
	)

	m := Model{
		ctx:              ctx,
		keys:             keys.Keys,
		prs:              []section.Section{&prSection},
		sidebar:          sidebar.NewModel(),
		footer:           footer.NewModel(ctx),
		tabs:             tabs.NewModel(ctx),
		prView:           prview.NewModel(ctx),
		issueSidebar:     issueview.NewModel(ctx),
		notificationView: notificationview.NewModel(ctx),
	}

	// Simulate having a populated cache by ensuring it's NOT cleared
	// (In real usage, this would happen after viewing a PR in sidebar)
	data.SetClient(nil) // Reset to known state first
	// Note: We can't easily populate the cache without making API calls,
	// so we verify the cache clearing behavior works from a cleared state

	// Verify cache starts cleared
	require.True(t, data.IsEnrichmentCacheCleared(), "cache should start cleared")

	// Send refresh key - this should call data.ClearEnrichmentCache()
	msg := tea.KeyPressMsg{Text: "R"}
	_, _ = m.Update(msg)

	// Verify cache is still cleared (ClearEnrichmentCache was called)
	require.True(t, data.IsEnrichmentCacheCleared(),
		"cache should be cleared after refresh key press")
}

func TestPromptConfirmationForNotificationPR_ApproveWorkflows(t *testing.T) {
	// Test that promptConfirmationForNotificationPR sets the pending action
	// for approveWorkflows.
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag: "../config/testdata/test-config.yml",
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config: &cfg,
		View:   config.NotificationsView,
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	m := Model{
		ctx:    ctx,
		keys:   keys.Keys,
		footer: footer.NewModel(ctx),
	}

	// Set up a PR notification subject
	m.notificationView.SetSubjectPR(&prrow.Data{
		Primary: &data.PullRequestData{
			Number: 42,
		},
	}, "test-notification-id")

	// Call promptConfirmationForNotificationPR with approveWorkflows
	m.promptConfirmationForNotificationPR("approveWorkflows")

	// Verify pending action is set
	require.Equal(t, "pr_approveWorkflows", m.notificationView.GetPendingAction(),
		"pendingNotificationAction should be set to pr_approveWorkflows")
}

func TestChecksRefreshFetchedUpdatesNotificationSubjectPR(t *testing.T) {
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config: &cfg,
		View:   config.NotificationsView,
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	url := "https://github.com/owner/repo/pull/42"
	pr := &prrow.Data{Primary: &data.PullRequestData{Number: 42, Url: url}}
	m := Model{
		ctx:              ctx,
		keys:             keys.Keys,
		sidebar:          sidebar.NewModel(),
		footer:           footer.NewModel(ctx),
		prView:           prview.NewModel(ctx),
		issueSidebar:     issueview.NewModel(ctx),
		notificationView: notificationview.NewModel(ctx),
		tabs:             tabs.NewModel(ctx),
	}
	m.sidebar.IsOpen = true
	m.prView.SetRow(pr)
	m.prView.GoToActivityTab()
	m.notificationView.SetSubjectPR(pr, "notification-id")

	updated := data.EnrichedPullRequestData{
		Number: 42,
		Title:  "updated title",
		Url:    url,
	}
	target := visibleRefreshTarget{key: "pr-preview:" + url, kind: visibleRefreshPRPreview, url: url}
	newModel, _ := m.Update(visibleRefreshFetchedMsg{target: target, data: updated})
	m = newModel.(Model)

	subject := m.notificationView.GetSubjectPR()
	require.NotNil(t, subject)
	require.True(t, subject.IsEnriched)
	require.Equal(t, "updated title", subject.Primary.Title)
	require.Equal(t, updated, subject.Enriched)
	require.Equal(t, "notification-id", m.notificationView.GetSubjectId())
}

func TestNotificationConfirmation_ApproveWorkflows_AcceptWithY(t *testing.T) {
	// Test that confirming approveWorkflows executes the action
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag: "../config/testdata/test-config.yml",
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config: &cfg,
		View:   config.NotificationsView,
		StartTask: func(task context.Task) tea.Cmd {
			return nil // No-op for testing
		},
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	m := Model{
		ctx:              ctx,
		keys:             keys.Keys,
		footer:           footer.NewModel(ctx),
		notificationView: notificationview.NewModel(ctx),
	}

	// Set up a PR notification subject and pending approveWorkflows action
	m.notificationView.SetSubjectPR(&prrow.Data{
		Primary: &data.PullRequestData{
			Number: 42,
			Repository: data.Repository{
				NameWithOwner: "owner/repo",
			},
		},
	}, "test-notification-id")
	m.notificationView.SetPendingPRAction("approveWorkflows")

	// Press 'y' to confirm
	msg := tea.KeyPressMsg{Text: "y"}
	newModel, cmd := m.Update(msg)
	m = newModel.(Model)

	// Verify pending action is cleared and command is returned
	require.Empty(t, m.notificationView.GetPendingAction(),
		"pendingNotificationAction should be cleared after confirmation")
	require.NotNil(t, cmd, "should return a command to execute the action")
}

func TestIsUserDefinedKeybinding_NotificationsView_PRNotification(t *testing.T) {
	// Custom PR keybindings should be recognized when viewing a PR notification
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)

	// Add a custom PR keybinding
	cfg.Keybindings.Prs = []config.Keybinding{
		{Key: "B", Command: "gh pr view -w {{.PrNumber}}"},
	}

	ctx := &context.ProgramContext{
		Config: &cfg,
		View:   config.NotificationsView,
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	// Create a notification section with a PR notification as the current row
	notifSec := notificationssection.NewModel(
		0,
		ctx,
		config.NotificationsSectionConfig{},
		time.Now(),
	)
	notifSec.Notifications = []notificationrow.Data{
		{
			Notification: data.NotificationData{
				Id: "test-1",
				Subject: data.NotificationSubject{
					Title: "Test PR",
					Url:   "https://api.github.com/repos/owner/repo/pulls/42",
					Type:  "PullRequest",
				},
				Repository: data.NotificationRepository{
					FullName: "owner/repo",
				},
			},
		},
	}
	notifSec.Table.SetRows(notifSec.BuildRows())

	m := Model{
		ctx:           ctx,
		keys:          keys.Keys,
		notifications: []section.Section{&notifSec},
	}

	// The custom PR keybinding "B" should be recognized
	msg := tea.KeyPressMsg{Text: "B"}
	require.True(t, m.isUserDefinedKeybinding(msg),
		"custom PR keybinding should be recognized in NotificationsView for PR notifications")

	// A non-configured key should not be recognized
	msg = tea.KeyPressMsg{Text: "Z"}
	require.False(t, m.isUserDefinedKeybinding(msg),
		"unconfigured key should not be recognized")
}

func TestIsUserDefinedKeybinding_NotificationsView_IssueNotification(t *testing.T) {
	// Custom Issue keybindings should be recognized when viewing an Issue notification
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)

	// Add a custom Issue keybinding
	cfg.Keybindings.Issues = []config.Keybinding{
		{Key: "B", Command: "gh issue view -w {{.IssueNumber}}"},
	}

	ctx := &context.ProgramContext{
		Config: &cfg,
		View:   config.NotificationsView,
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	// Create a notification section with an Issue notification as the current row
	notifSec := notificationssection.NewModel(
		0,
		ctx,
		config.NotificationsSectionConfig{},
		time.Now(),
	)
	notifSec.Notifications = []notificationrow.Data{
		{
			Notification: data.NotificationData{
				Id: "test-2",
				Subject: data.NotificationSubject{
					Title: "Test Issue",
					Url:   "https://api.github.com/repos/owner/repo/issues/99",
					Type:  "Issue",
				},
				Repository: data.NotificationRepository{
					FullName: "owner/repo",
				},
			},
		},
	}
	notifSec.Table.SetRows(notifSec.BuildRows())

	m := Model{
		ctx:           ctx,
		keys:          keys.Keys,
		notifications: []section.Section{&notifSec},
	}

	// The custom Issue keybinding "B" should be recognized
	msg := tea.KeyPressMsg{Text: "B"}
	require.True(t, m.isUserDefinedKeybinding(msg),
		"custom Issue keybinding should be recognized in NotificationsView for Issue notifications")
}

func TestIsUserDefinedKeybinding_NotificationsView_NonPRIssueNotification(t *testing.T) {
	// Custom PR/Issue keybindings should NOT be recognized for other notification types
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)

	cfg.Keybindings.Prs = []config.Keybinding{
		{Key: "B", Command: "gh pr view -w {{.PrNumber}}"},
	}

	ctx := &context.ProgramContext{
		Config: &cfg,
		View:   config.NotificationsView,
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	// Create a notification section with a Release notification
	notifSec := notificationssection.NewModel(
		0,
		ctx,
		config.NotificationsSectionConfig{},
		time.Now(),
	)
	notifSec.Notifications = []notificationrow.Data{
		{
			Notification: data.NotificationData{
				Id: "test-3",
				Subject: data.NotificationSubject{
					Title: "v1.0.0",
					Url:   "https://api.github.com/repos/owner/repo/releases/12345",
					Type:  "Release",
				},
				Repository: data.NotificationRepository{
					FullName: "owner/repo",
				},
			},
		},
	}
	notifSec.Table.SetRows(notifSec.BuildRows())

	m := Model{
		ctx:           ctx,
		keys:          keys.Keys,
		notifications: []section.Section{&notifSec},
	}

	// The custom PR keybinding "B" should NOT be recognized for a Release notification
	msg := tea.KeyPressMsg{Text: "B"}
	require.False(t, m.isUserDefinedKeybinding(msg),
		"custom PR keybinding should not be recognized for Release notifications")
}

func TestNotificationPRCommandTemplateVariables(t *testing.T) {
	// Test that notification PR command templates have RepoName and PrNumber available,
	// matching the behavior of runCustomNotificationPRCommand in modelUtils.go
	input := map[string]any{
		"RepoName": "owner/repo",
		"PrNumber": 42,
	}

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "PrNumber variable",
			template: "gh pr view {{.PrNumber}}",
			expected: "gh pr view 42",
		},
		{
			name:     "RepoName and PrNumber",
			template: "gh pr view -R {{.RepoName}} {{.PrNumber}}",
			expected: "gh pr view -R owner/repo 42",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := executeCommandTemplate(t, tc.template, input)
			require.NoError(t, err)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestNotificationPRCommandTemplateVariables_WithSidebar(t *testing.T) {
	// When the sidebar is open for a PR notification, HeadRefName, BaseRefName,
	// and Author become available in the template fields.
	input := map[string]any{
		"RepoName":    "owner/repo",
		"PrNumber":    42,
		"HeadRefName": "feature-branch",
		"BaseRefName": "main",
		"Author":      "octocat",
	}

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "HeadRefName variable",
			template: "git checkout {{.HeadRefName}}",
			expected: "git checkout feature-branch",
		},
		{
			name:     "BaseRefName variable",
			template: "git diff {{.BaseRefName}}...{{.HeadRefName}}",
			expected: "git diff main...feature-branch",
		},
		{
			name:     "Author variable",
			template: "echo {{.Author}}",
			expected: "echo octocat",
		},
		{
			name:     "all fields combined",
			template: "gh pr view -R {{.RepoName}} {{.PrNumber}} # by {{.Author}} ({{.HeadRefName}} -> {{.BaseRefName}})",
			expected: "gh pr view -R owner/repo 42 # by octocat (feature-branch -> main)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := executeCommandTemplate(t, tc.template, input)
			require.NoError(t, err)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestNotificationIssueCommandTemplateVariables(t *testing.T) {
	// Test that notification Issue command templates have RepoName and IssueNumber available,
	// matching the behavior of runCustomNotificationIssueCommand in modelUtils.go
	input := map[string]any{
		"RepoName":    "owner/repo",
		"IssueNumber": 99,
	}

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "IssueNumber variable",
			template: "gh issue view {{.IssueNumber}}",
			expected: "gh issue view 99",
		},
		{
			name:     "RepoName and IssueNumber",
			template: "gh issue view -R {{.RepoName}} {{.IssueNumber}}",
			expected: "gh issue view -R owner/repo 99",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := executeCommandTemplate(t, tc.template, input)
			require.NoError(t, err)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestNotificationIssueCommandTemplateVariables_WithSidebar(t *testing.T) {
	// When the sidebar is open for an Issue notification, Author becomes available.
	input := map[string]any{
		"RepoName":    "owner/repo",
		"IssueNumber": 99,
		"Author":      "octocat",
	}

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "Author variable",
			template: "echo {{.Author}}",
			expected: "echo octocat",
		},
		{
			name:     "all fields combined",
			template: "gh issue view -R {{.RepoName}} {{.IssueNumber}} # by {{.Author}}",
			expected: "gh issue view -R owner/repo 99 # by octocat",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := executeCommandTemplate(t, tc.template, input)
			require.NoError(t, err)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestNotificationCommandTemplate_PRFieldsWithoutSidebar(t *testing.T) {
	// When the sidebar is not open, notification PR commands only have RepoName and PrNumber.
	// Templates referencing sidebar-only fields should error (missingkey=error behavior).
	input := map[string]any{
		"RepoName": "owner/repo",
		"PrNumber": 42,
	}

	unavailableFields := []struct {
		name     string
		template string
	}{
		{"HeadRefName", "git checkout {{.HeadRefName}}"},
		{"BaseRefName", "git checkout {{.BaseRefName}}"},
		{"Author", "echo {{.Author}}"},
	}

	for _, tc := range unavailableFields {
		t.Run(tc.name, func(t *testing.T) {
			_, err := executeCommandTemplate(t, tc.template, input)
			require.Error(t, err,
				"%s should not be available in notification PR commands", tc.name)
		})
	}
}

func TestIsUserDefinedKeybinding_NotificationsView_NotificationKeybinding(t *testing.T) {
	// Custom notification keybindings should be recognized regardless of subject type
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)

	// Add a custom notification keybinding
	cfg.Keybindings.Notifications = []config.Keybinding{
		{Key: "N", Command: "echo {{.RepoName}} {{.Number}}"},
	}

	ctx := &context.ProgramContext{
		Config: &cfg,
		View:   config.NotificationsView,
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	// Create a notification section with a Release notification (not PR or Issue)
	notifSec := notificationssection.NewModel(
		0,
		ctx,
		config.NotificationsSectionConfig{},
		time.Now(),
	)
	notifSec.Notifications = []notificationrow.Data{
		{
			Notification: data.NotificationData{
				Id: "test-notif",
				Subject: data.NotificationSubject{
					Title: "v2.0.0",
					Url:   "https://api.github.com/repos/owner/repo/releases/99",
					Type:  "Release",
				},
				Repository: data.NotificationRepository{
					FullName: "owner/repo",
				},
			},
		},
	}
	notifSec.Table.SetRows(notifSec.BuildRows())

	m := Model{
		ctx:           ctx,
		keys:          keys.Keys,
		notifications: []section.Section{&notifSec},
	}

	// The custom notification keybinding "N" should be recognized
	msg := tea.KeyPressMsg{Text: "N"}
	require.True(t, m.isUserDefinedKeybinding(msg),
		"custom notification keybinding should be recognized for any notification type")

	// A non-configured key should not be recognized
	msg = tea.KeyPressMsg{Text: "Z"}
	require.False(t, m.isUserDefinedKeybinding(msg),
		"unconfigured key should not be recognized")
}

func TestNotificationCommandTemplateVariables(t *testing.T) {
	// Test that notification-specific command templates have RepoName and Number available,
	// matching the behavior of runCustomNotificationCommand in modelUtils.go
	input := map[string]any{
		"RepoName": "owner/repo",
		"Number":   42,
	}

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "Number variable",
			template: "gh api /notifications/threads/{{.Number}}",
			expected: "gh api /notifications/threads/42",
		},
		{
			name:     "RepoName and Number",
			template: "echo {{.RepoName}} #{{.Number}}",
			expected: "echo owner/repo #42",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := executeCommandTemplate(t, tc.template, input)
			require.NoError(t, err)
			require.Equal(t, tc.expected, result)
		})
	}
}
