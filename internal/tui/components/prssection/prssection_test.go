package prssection

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dehub/v4/internal/config"
	"github.com/dlvhdr/gh-dehub/v4/internal/data"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/fuzzyselect"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/prompt"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/prrow"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/section"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/tasks"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/theme"
)

// newTestModel creates a minimal Model with the prompt confirmation box
// focused and a single PR row so that GetCurrRow returns non-nil.
func newTestModel(action string) Model {
	ctx := &context.ProgramContext{
		Theme: *theme.DefaultTheme,
		StartTask: func(task context.Task) tea.Cmd {
			return func() tea.Msg { return nil }
		},
	}
	m := Model{
		BaseModel: section.BaseModel{
			Ctx:                       ctx,
			IsPromptConfirmationShown: true,
			PromptConfirmationAction:  action,
			PromptConfirmationBox:     prompt.NewModel(ctx),
		},
		Prs: []prrow.Data{
			{Primary: &data.PullRequestData{Number: 42}},
		},
		CreatePRForm: newCreatePRForm(ctx),
	}
	m.PromptConfirmationBox.Focus()
	return m
}

func TestCreatePRBranchesFetchedIgnoresStaleResult(t *testing.T) {
	m := newTestModel("create_pr")
	m.SearchValue = "repo:owner/current is:open"
	m.createPRBranchRequestID = 2
	m.CreatePRForm.SetBranchesLoading()

	_, _ = m.Update(createPRBranchesFetchedMsg{
		RepoName:  "owner/old",
		RequestID: 1,
		Branches:  []fuzzyselect.Suggestion{{Value: "old-branch"}},
		Head:      "old-head",
		Base:      "old-base",
	})

	require.True(t, m.CreatePRForm.BranchesLoading())
	require.Empty(t, m.CreatePRForm.Head())
	require.Empty(t, m.CreatePRForm.Base())
}

func TestCreatePRFormShowsHeadBeforeBase(t *testing.T) {
	f := newTestModel("create_pr").CreatePRForm
	f.SetWidth(80)
	f.SetBranches([]fuzzyselect.Suggestion{{Value: "feature"}, {Value: "main"}}, "feature", "main")

	view := ansi.Strip(f.View())
	headIdx := strings.Index(view, "Head branch")
	baseIdx := strings.Index(view, "Base branch")

	require.NotEqual(t, -1, headIdx)
	require.NotEqual(t, -1, baseIdx)
	require.Less(t, headIdx, baseIdx)
	require.NotContains(t, view, "←")
}

func TestCreatePRFormTabsThroughFieldsInVisualOrder(t *testing.T) {
	f := newTestModel("create_pr").CreatePRForm

	f, _ = f.Update(tea.KeyPressMsg{Text: "tab"})
	require.Equal(t, 1, f.active)

	f, _ = f.Update(tea.KeyPressMsg{Text: "tab"})
	require.Equal(t, 2, f.active)

	f, _ = f.Update(tea.KeyPressMsg{Text: "tab"})
	require.Equal(t, 3, f.active)

	f, _ = f.Update(tea.KeyPressMsg{Text: "shift+tab"})
	require.Equal(t, 2, f.active)
}

func TestEditPRFormHidesHeadBranchAndTabsThroughEditableFields(t *testing.T) {
	f := newTestModel("edit_pr").CreatePRForm
	f.SetWidth(80)
	f.SetEditPR("My PR", "body", "feature", "main")
	f.SetBranchSuggestions([]fuzzyselect.Suggestion{{Value: "main"}, {Value: "develop"}})

	view := ansi.Strip(f.View())
	require.NotContains(t, view, "Head branch")
	require.Contains(t, view, "Base branch")
	require.Contains(t, view, "My PR")

	f, _ = f.Update(tea.KeyPressMsg{Text: "tab"})
	require.Equal(t, 1, f.active)

	f, _ = f.Update(tea.KeyPressMsg{Text: "tab"})
	require.Equal(t, 2, f.active)

	f, _ = f.Update(tea.KeyPressMsg{Text: "tab"})
	require.Equal(t, 0, f.active)
}

func TestCreatePRFormLongBranchesDoNotOverflowNarrowWidth(t *testing.T) {
	f := newTestModel("create_pr").CreatePRForm
	f.SetWidth(80)
	f.active = 1
	f.SetBranches([]fuzzyselect.Suggestion{{Value: strings.Repeat("microservice/", 12)}}, strings.Repeat("microservice/", 12), "main")

	view := ansi.Strip(f.View())
	for _, line := range strings.Split(view, "\n") {
		require.LessOrEqual(t, lipgloss.Width(line), 80, "line overflows form width: %q", line)
	}
}

func TestResetRowsClearsLastFetchTaskID(t *testing.T) {
	m := Model{
		BaseModel: section.BaseModel{LastFetchTaskId: "fetching_prs_1_previous"},
		Prs:       []prrow.Data{{Primary: &data.PullRequestData{Number: 42}}},
	}

	m.ResetRows()

	require.Empty(t, m.LastFetchTaskId)
	require.Nil(t, m.Prs)
}

func TestSortPRsUsesLoadedRows(t *testing.T) {
	oldCreated := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	newCreated := oldCreated.Add(time.Hour)
	oldUpdated := oldCreated.Add(2 * time.Hour)
	newUpdated := oldCreated.Add(3 * time.Hour)
	m := Model{
		BaseModel: section.BaseModel{SortOrder: data.SearchSortUpdated},
		Prs: []prrow.Data{
			{Primary: &data.PullRequestData{Number: 1, CreatedAt: newCreated, UpdatedAt: oldUpdated}},
			{Primary: &data.PullRequestData{Number: 2, CreatedAt: oldCreated, UpdatedAt: newUpdated}},
		},
	}

	m.sortPRs()
	require.Equal(t, 2, m.Prs[0].Primary.Number)

	m.ToggleSortOrder()
	m.sortPRs()
	require.Equal(t, 1, m.Prs[0].Primary.Number)
}

func TestMergeRefreshedPRsPreservesEnrichedData(t *testing.T) {
	m := Model{
		Prs: []prrow.Data{
			{
				Primary:    &data.PullRequestData{Number: 1, Url: "https://github.com/owner/repo/pull/1"},
				IsEnriched: true,
				Enriched: data.EnrichedPullRequestData{
					Number: 1,
					Url:    "https://github.com/owner/repo/pull/1",
					Title:  "enriched title",
				},
			},
		},
	}

	m.mergeRefreshedPRs([]prrow.Data{
		{Primary: &data.PullRequestData{Number: 1, Url: "https://github.com/owner/repo/pull/1", Title: "fresh title"}},
	})

	require.Len(t, m.Prs, 1)
	require.Equal(t, "fresh title", m.Prs[0].Primary.Title)
	require.True(t, m.Prs[0].IsEnriched)
	require.Equal(t, "enriched title", m.Prs[0].Enriched.Title)
}

func TestMergeRefreshedPRsDedupesDuplicateRows(t *testing.T) {
	url := "https://github.com/owner/repo/pull/1"
	m := Model{}

	m.mergeRefreshedPRs([]prrow.Data{
		{Primary: &data.PullRequestData{Number: 1, Url: url, Title: "first"}},
		{Primary: &data.PullRequestData{Number: 1, Url: url, Title: "duplicate"}},
	})

	require.Len(t, m.Prs, 1)
	require.Equal(t, "first", m.Prs[0].Primary.Title)
}

func TestUniquePRsDedupesByRepoAndNumberWithoutURL(t *testing.T) {
	prs := uniquePRs([]prrow.Data{
		{Primary: &data.PullRequestData{Number: 1, Repository: data.Repository{NameWithOwner: "owner/repo"}, Title: "first"}},
		{Primary: &data.PullRequestData{Number: 1, Repository: data.Repository{NameWithOwner: "owner/repo"}, Title: "duplicate"}},
		{Primary: &data.PullRequestData{Number: 1, Repository: data.Repository{NameWithOwner: "other/repo"}, Title: "other repo"}},
	})

	require.Len(t, prs, 2)
	require.Equal(t, "first", prs[0].Primary.Title)
	require.Equal(t, "other repo", prs[1].Primary.Title)
}

func TestAppendUniquePRsSkipsExistingRows(t *testing.T) {
	existing := []prrow.Data{
		{Primary: &data.PullRequestData{Number: 1, Url: "https://github.com/owner/repo/pull/1", Title: "existing"}},
	}
	incoming := []prrow.Data{
		{Primary: &data.PullRequestData{Number: 1, Url: "https://github.com/owner/repo/pull/1", Title: "duplicate"}},
		{Primary: &data.PullRequestData{Number: 2, Url: "https://github.com/owner/repo/pull/2", Title: "new"}},
	}

	prs := appendUniquePRs(existing, incoming...)

	require.Len(t, prs, 2)
	require.Equal(t, "existing", prs[0].Primary.Title)
	require.Equal(t, "new", prs[1].Primary.Title)
}

func TestMergeRefreshedPRsInvalidatesStaleEnrichedState(t *testing.T) {
	url := "https://github.com/owner/repo/pull/1"
	m := Model{
		Prs: []prrow.Data{
			{
				Primary:    &data.PullRequestData{Number: 1, Url: url, State: "OPEN"},
				IsEnriched: true,
				Enriched: data.EnrichedPullRequestData{
					Number: 1,
					Url:    url,
					State:  "OPEN",
				},
			},
		},
	}

	m.mergeRefreshedPRs([]prrow.Data{
		{Primary: &data.PullRequestData{Number: 1, Url: url, State: "MERGED"}},
	})

	require.Len(t, m.Prs, 1)
	require.Equal(t, "MERGED", m.Prs[0].Primary.State)
	require.False(t, m.Prs[0].IsEnriched)
}

func TestMergeRefreshedPRsInvalidatesStaleEnrichedReviewDecision(t *testing.T) {
	url := "https://github.com/owner/repo/pull/1"
	m := Model{
		Prs: []prrow.Data{
			{
				Primary:    &data.PullRequestData{Number: 1, Url: url, ReviewDecision: "REVIEW_REQUIRED"},
				IsEnriched: true,
				Enriched: data.EnrichedPullRequestData{
					Number:         1,
					Url:            url,
					ReviewDecision: "REVIEW_REQUIRED",
				},
			},
		},
	}

	m.mergeRefreshedPRs([]prrow.Data{
		{Primary: &data.PullRequestData{Number: 1, Url: url, ReviewDecision: "APPROVED"}},
	})

	require.Len(t, m.Prs, 1)
	require.Equal(t, "APPROVED", m.Prs[0].Primary.ReviewDecision)
	require.False(t, m.Prs[0].IsEnriched)
}

func TestEnrichPRUpdatesPrimaryStateAndReviewDecision(t *testing.T) {
	url := "https://github.com/owner/repo/pull/1"
	m := Model{
		Prs: []prrow.Data{
			{Primary: &data.PullRequestData{Number: 1, Url: url, State: "OPEN", ReviewDecision: "REVIEW_REQUIRED"}},
		},
	}

	m.EnrichPR(data.EnrichedPullRequestData{
		Number:         1,
		Url:            url,
		State:          "MERGED",
		ReviewDecision: "APPROVED",
	})

	require.True(t, m.Prs[0].IsEnriched)
	require.Equal(t, "MERGED", m.Prs[0].Primary.State)
	require.Equal(t, "APPROVED", m.Prs[0].Primary.ReviewDecision)
}

func TestLocalSearchFiltersPRsByTitleNumberAndBranch(t *testing.T) {
	first := data.PullRequestData{Number: 123, Title: "Add math tests", HeadRefName: "feature/math", BaseRefName: "main"}
	first.Repository.Name = "repo"
	first.Repository.NameWithOwner = "owner/repo"
	first.Author.Login = "alice"
	second := data.PullRequestData{Number: 456, Title: "Update docs", HeadRefName: "docs", BaseRefName: "main"}
	second.Repository.Name = "repo"
	second.Repository.NameWithOwner = "owner/repo"
	second.Author.Login = "bob"
	m := Model{Prs: []prrow.Data{{Primary: &first}, {Primary: &second}}}

	m.LocalSearchValue = "math"
	require.Len(t, m.filteredPRs(), 1)
	require.Equal(t, 123, m.GetCurrRow().(*prrow.Data).Primary.Number)

	m.LocalSearchValue = "#456"
	require.Len(t, m.filteredPRs(), 1)
	require.Equal(t, 456, m.filteredPRs()[0].Primary.Number)

	m.LocalSearchValue = "feature/math"
	require.Len(t, m.filteredPRs(), 1)
	require.Equal(t, 123, m.filteredPRs()[0].Primary.Number)
}

func TestRepoFromFilters(t *testing.T) {
	tests := []struct {
		name    string
		filters string
		want    string
		ok      bool
	}{
		{name: "single repo", filters: "is:open repo:owner/name author:@me", want: "owner/name", ok: true},
		{name: "no repo", filters: "is:open author:@me", ok: false},
		{name: "empty repo", filters: "repo: is:open", ok: false},
		{name: "multiple repos", filters: "repo:owner/one repo:owner/two", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := repoFromFilters(tt.filters)
			require.Equal(t, tt.ok, ok)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestCreatePRRequiresSingleRepoFilter(t *testing.T) {
	m := newTestModel("")
	m.SearchValue = "is:open"

	err := m.validateCanCreatePR()

	require.EqualError(t, err, "current PR section must have exactly one repo:owner/name filter to create a PR")
}

func TestCreatePRRequiresConfiguredRepoPath(t *testing.T) {
	m := newTestModel("")
	m.SearchValue = "repo:owner/name is:open"
	m.Ctx.Config = &config.Config{RepoPaths: map[string]string{}}

	err := m.validateCanCreatePR()

	require.EqualError(t, err, "local path to repo not specified, set one in your config.yml under repoPaths")
}

func TestCreatePRRequiresTitle(t *testing.T) {
	m := newTestModel("")
	m.SearchValue = "repo:owner/name is:open"
	m.Ctx.Config = &config.Config{RepoPaths: map[string]string{"owner/name": "/tmp/name"}}

	cmd, err := m.createPR(" ", "body", "feature", "main")

	require.Nil(t, cmd)
	require.EqualError(t, err, "PR title is required")
}

func TestCreatePRStartsTaskWhenRepoScoped(t *testing.T) {
	var started context.Task
	m := newTestModel("")
	m.SearchValue = "repo:owner/name is:open"
	m.Ctx.Config = &config.Config{RepoPaths: map[string]string{"owner/name": "/tmp/name"}}
	m.Ctx.StartTask = func(task context.Task) tea.Cmd {
		started = task
		return func() tea.Msg { return nil }
	}

	cmd, err := m.createPR("My PR", "body", "feature", "main")

	require.NoError(t, err)
	require.NotNil(t, cmd)
	require.Contains(t, started.Id, "create_pr_owner_name")
	require.Equal(t, "Creating PR in owner/name", started.StartText)
}

func TestPrepareCreatePRFormUsesCachedBranchesAndRequestsRefresh(t *testing.T) {
	m := newTestModel("")
	m.SearchValue = "repo:owner/name is:open"
	m.Ctx.Config = &config.Config{RepoPaths: map[string]string{"owner/name": "/tmp/name"}}

	cmd, err := m.PrepareCreatePRForm(&RepoBranches{
		RepoName: "owner/name",
		Branches: []fuzzyselect.Suggestion{{Value: "feature"}, {Value: "main"}},
		Head:     "feature",
		Base:     "main",
	})

	require.NoError(t, err)
	require.NotNil(t, cmd)
	require.False(t, m.CreatePRForm.BranchesLoading())
	require.Equal(t, "feature", m.CreatePRForm.Head())
	require.Equal(t, "main", m.CreatePRForm.Base())
	require.Equal(t, RefreshRepoBranchesMsg{SectionId: m.Id, RepoName: "owner/name"}, cmd())
}

func TestPrepareEditPRFormPrefillsCurrentPR(t *testing.T) {
	m := newTestModel("")
	m.Ctx.Config = &config.Config{}
	pr := data.PullRequestData{
		Number:      42,
		Title:       "Old title",
		Body:        "Old body",
		HeadRefName: "feature",
		BaseRefName: "main",
	}
	pr.Repository.NameWithOwner = "owner/name"
	m.Prs = []prrow.Data{{Primary: &pr}}

	cmd, err := m.PrepareEditPRForm(&m.Prs[0], &RepoBranches{
		RepoName: "owner/name",
		Branches: []fuzzyselect.Suggestion{{Value: "main"}, {Value: "develop"}},
		Head:     "ignored-local-head",
		Base:     "ignored-default-base",
	})

	require.NoError(t, err)
	require.NotNil(t, cmd)
	require.Equal(t, "Old title", m.CreatePRForm.Title())
	require.Equal(t, "Old body", m.CreatePRForm.Body())
	require.Equal(t, "feature", m.CreatePRForm.Head())
	require.Equal(t, "main", m.CreatePRForm.Base())
	require.Equal(t, RefreshRepoBranchesMsg{SectionId: m.Id, RepoName: "owner/name"}, cmd())
}

func TestCreatePRRunsGhCreateWithoutWeb(t *testing.T) {
	orig := runCreatePRRepoCommand
	defer func() { runCreatePRRepoCommand = orig }()

	var gotPath string
	var gotArgs []string
	runCreatePRRepoCommand = func(repoPath string, args ...string) (string, error) {
		gotPath = repoPath
		gotArgs = args
		return "", nil
	}

	m := newTestModel("")
	m.SearchValue = "repo:owner/name is:open"
	m.Ctx.Config = &config.Config{RepoPaths: map[string]string{"owner/name": "/tmp/name"}}
	m.Ctx.StartTask = func(task context.Task) tea.Cmd { return nil }

	cmd, err := m.createPR("My PR", "body", "feature", "main")
	require.NoError(t, err)
	require.NotNil(t, cmd)

	msg := cmd()
	require.IsType(t, constants.TaskFinishedMsg{}, msg)
	require.Equal(t, "/tmp/name", gotPath)
	require.Equal(t, []string{"gh", "pr", "create", "--title", "My PR", "--body", "body", "--head", "feature", "--base", "main"}, gotArgs)
	require.NotContains(t, gotArgs, "--web")
}

func TestCreatePRFetchesCreatedPRFromOutputURL(t *testing.T) {
	origRun := runCreatePRRepoCommand
	origFetch := fetchCreatedPR
	defer func() {
		runCreatePRRepoCommand = origRun
		fetchCreatedPR = origFetch
	}()

	runCreatePRRepoCommand = func(repoPath string, args ...string) (string, error) {
		return "https://github.com/owner/name/pull/123\n", nil
	}
	fetchCreatedPR = func(url string) (data.EnrichedPullRequestData, error) {
		require.Equal(t, "https://github.com/owner/name/pull/123", url)
		pr := data.EnrichedPullRequestData{Number: 123, Title: "My PR", Url: url, State: "OPEN"}
		pr.Repository.NameWithOwner = "owner/name"
		return pr, nil
	}

	m := newTestModel("")
	m.SearchValue = "repo:owner/name is:open"
	m.Ctx.Config = &config.Config{RepoPaths: map[string]string{"owner/name": "/tmp/name"}}
	m.Ctx.StartTask = func(task context.Task) tea.Cmd { return nil }

	cmd, err := m.createPR("My PR", "body", "feature", "main")
	require.NoError(t, err)
	require.NotNil(t, cmd)

	msg := cmd().(constants.TaskFinishedMsg)
	created := msg.Msg.(createPRCreatedMsg)
	require.NotNil(t, created.PR)
	require.Equal(t, 123, created.PR.Primary.Number)
	require.True(t, created.PR.IsEnriched)
}

func TestCreatedPRURLParsesGitHubPullURL(t *testing.T) {
	require.Equal(
		t,
		"https://github.com/owner/repo/pull/123",
		createdPRURL("Creating pull request...\nhttps://github.com/owner/repo/pull/123\n"),
	)
	require.Empty(t, createdPRURL("no url"))
}

func TestUpsertCreatedPRInsertsPR(t *testing.T) {
	m := newTestModel("")
	existing := data.PullRequestData{Number: 1, Title: "old", Url: "https://github.com/owner/name/pull/1", UpdatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}
	created := data.PullRequestData{Number: 2, Title: "new", Url: "https://github.com/owner/name/pull/2", UpdatedAt: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)}
	m.Prs = []prrow.Data{{Primary: &existing}}

	m.upsertCreatedPR(prrow.Data{Primary: &created})
	m.sortPRs()

	require.Len(t, m.Prs, 2)
	require.Equal(t, 2, m.Prs[0].Primary.Number)
}

func TestUpsertCreatedPRDedupesExistingPR(t *testing.T) {
	m := newTestModel("")
	old := data.PullRequestData{Number: 2, Title: "old", Url: "https://github.com/owner/name/pull/2"}
	created := data.PullRequestData{Number: 2, Title: "new", Url: "https://github.com/owner/name/pull/2"}
	m.Prs = []prrow.Data{{Primary: &old}}

	m.upsertCreatedPR(prrow.Data{Primary: &created})

	require.Len(t, m.Prs, 1)
	require.Equal(t, "new", m.Prs[0].Primary.Title)
}

func TestUpdatePRMsgAppliesEditedFields(t *testing.T) {
	m := newTestModel("")
	pr := data.PullRequestData{Number: 42, Title: "old", Body: "old body", BaseRefName: "main"}
	m.Prs = []prrow.Data{{Primary: &pr, Enriched: data.EnrichedPullRequestData{Number: 42, Title: "old", Body: "old body", BaseRefName: "main"}, IsEnriched: true}}
	title := "new"
	body := "new body"
	base := "develop"

	require.True(t, m.applyUpdatePRMsg(tasks.UpdatePRMsg{PrNumber: 42, Title: &title, Body: &body, BaseRefName: &base}))

	require.Equal(t, "new", m.Prs[0].Primary.Title)
	require.Equal(t, "new body", m.Prs[0].Primary.Body)
	require.Equal(t, "develop", m.Prs[0].Primary.BaseRefName)
	require.Equal(t, "new", m.Prs[0].Enriched.Title)
	require.Equal(t, "new body", m.Prs[0].Enriched.Body)
	require.Equal(t, "develop", m.Prs[0].Enriched.BaseRefName)
}

func TestConfirmation_AcceptWithEmptyInput(t *testing.T) {
	// Pressing Enter without typing anything should confirm, since the
	// prompt says (Y/n) indicating Y is the default.
	m := newTestModel("close")

	msg := tea.KeyPressMsg{Code: tea.KeyEnter}
	_, cmd := m.Update(msg)

	require.NotNil(t, cmd, "empty input (default Y) should execute the action")
	require.False(t, m.IsPromptConfirmationShown,
		"confirmation prompt should be dismissed")
}

func TestConfirmation_AcceptWithLowercaseY(t *testing.T) {
	m := newTestModel("merge")
	m.PromptConfirmationBox.SetValue("y")

	msg := tea.KeyPressMsg{Code: tea.KeyEnter}
	_, cmd := m.Update(msg)

	require.NotNil(t, cmd, "lowercase y should execute the action")
}

func TestConfirmation_AcceptWithUppercaseY(t *testing.T) {
	m := newTestModel("reopen")
	m.PromptConfirmationBox.SetValue("Y")

	msg := tea.KeyPressMsg{Code: tea.KeyEnter}
	_, cmd := m.Update(msg)

	require.NotNil(t, cmd, "uppercase Y should execute the action")
}

func TestConfirmation_RejectWithN(t *testing.T) {
	m := newTestModel("close")
	m.PromptConfirmationBox.SetValue("n")

	msg := tea.KeyPressMsg{Code: tea.KeyEnter}
	_, cmd := m.Update(msg)

	// cmd is a batch of (nil, blinkCmd) -- the nil means no action was taken.
	// We verify the prompt is dismissed regardless.
	require.False(t, m.IsPromptConfirmationShown,
		"confirmation prompt should be dismissed on rejection")
	_ = cmd
}

func TestConfirmation_CancelWithEsc(t *testing.T) {
	m := newTestModel("merge")

	msg := tea.KeyPressMsg{Code: tea.KeyEsc}
	_, cmd := m.Update(msg)

	require.False(t, m.IsPromptConfirmationShown,
		"Esc should dismiss the confirmation prompt")
	_ = cmd
}

func TestConfirmation_CancelWithCtrlC(t *testing.T) {
	m := newTestModel("update")

	msg := tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	_, cmd := m.Update(msg)

	require.False(t, m.IsPromptConfirmationShown,
		"Ctrl+C should dismiss the confirmation prompt")
	_ = cmd
}

func TestConfirmation_AllActions(t *testing.T) {
	actions := []string{"close", "reopen", "ready", "merge", "update", "approveWorkflows"}

	for _, action := range actions {
		t.Run(action+"_empty_input", func(t *testing.T) {
			m := newTestModel(action)

			msg := tea.KeyPressMsg{Code: tea.KeyEnter}
			_, cmd := m.Update(msg)

			require.NotNil(t, cmd,
				"empty input should confirm for action %q", action)
		})

		t.Run(action+"_explicit_y", func(t *testing.T) {
			m := newTestModel(action)
			m.PromptConfirmationBox.SetValue("y")

			msg := tea.KeyPressMsg{Code: tea.KeyEnter}
			_, cmd := m.Update(msg)

			require.NotNil(t, cmd,
				"explicit y should confirm for action %q", action)
		})
	}
}
