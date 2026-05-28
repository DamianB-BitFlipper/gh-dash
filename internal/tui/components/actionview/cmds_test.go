package actionview

import (
	"errors"
	"os"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	data "github.com/dlvhdr/gh-dehub/v4/internal/data/actions"
	api "github.com/dlvhdr/gh-dehub/v4/internal/data/actionsapi"
	graphql "github.com/hasura/go-graphql-client"
)

func TestMergingOfSameWorkflowJobs(t *testing.T) {
	d, err := os.ReadFile("./testdata/fetchPROneContext.json")
	if err != nil {
		t.Errorf("failed reading mock data file %v", err)
	}

	res := struct{ Data api.PRCheckRunsQuery }{}
	err = graphql.UnmarshalGraphQL(d, &res)
	if err != nil {
		t.Error(err)
	}

	wfr := makeWorkflowRun(
		res.Data.Resource.PullRequest.Commits.Nodes[0].Commit.StatusCheckRollup.Contexts.Nodes[0].CheckRun,
	)

	m := NewModel("dlvhdr/gh-dehub", "1", ModelOpts{})
	m.prWithChecks = res.Data.Resource.PullRequest

	runs := makeWorkflowRuns(
		m.prWithChecks.Commits.Nodes[0].Commit.StatusCheckRollup.Contexts.Nodes,
	)
	msg1 := workflowRunsFetchedMsg{runs: runs, pr: m.prWithChecks}
	m.mergeWorkflowRuns(msg1)

	if len(m.workflowRuns) != 1 {
		t.Fatalf(`expected workflow runs to have length of 1, got: %d`, len(m.workflowRuns))
	}

	if len(m.workflowRuns[0].Jobs) != 1 {
		t.Logf("%+v", m.workflowRuns[0].Jobs)
		t.Fatalf(`expected jobs to have length of 1, got: %d`, len(m.workflowRuns[0].Jobs))
	}

	next := []data.WorkflowRun{
		{
			Id:        wfr.Id,
			Name:      wfr.Name,
			Link:      wfr.Link,
			Workflow:  wfr.Workflow,
			Event:     wfr.Event,
			RunNumber: wfr.RunNumber,
			Jobs: []data.WorkflowJob{
				{
					RunNumber:  wfr.RunNumber,
					Id:         "job2",
					State:      api.StatusCompleted,
					Conclusion: api.ConclusionSuccess,
					Name:       "job2",
					Title:      "job2",
					Workflow:   wfr.Name,
					Event:      "pull_request",
					Logs:       []data.LogsWithTime{},
					Link:       "https://github.com/dlvhdr/gh-dehub/actions/runs/19991547923/job/57332991075",
					Bucket:     data.CheckBucketPass,
					Kind:       data.JobKindCheckRun,
				},
			},
			Bucket: wfr.Bucket,
		},
	}
	msg2 := workflowRunsFetchedMsg{runs: next}
	m.mergeWorkflowRuns(msg2)

	if len(m.workflowRuns) != 1 {
		t.Fatalf(
			`expected workflow runs to have length of 1, got: %d, %+v`,
			len(m.workflowRuns),
			m.workflowRuns,
		)
	}

	if len(m.workflowRuns[0].Jobs) != 2 {
		t.Fatalf(`expected jobs to have length of 2, got: %d`, len(m.workflowRuns[0].Jobs))
	}
}

func TestMakeWorkflowRunsIncludesStatusContexts(t *testing.T) {
	runs := makeWorkflowRuns([]api.ContextNode{
		{
			Typename: "StatusContext",
			StatusContext: api.StatusContext{
				Context:     "vercel/deployment",
				Description: "Preview deployment failed",
				State:       api.ConclusionFailure,
				TargetUrl:   "https://example.com/deployment",
			},
		},
	})

	if len(runs) != 1 {
		t.Fatalf("expected one synthetic status context run, got %d", len(runs))
	}
	if runs[0].Name != "Status checks" {
		t.Fatalf("expected status checks run, got %q", runs[0].Name)
	}
	if len(runs[0].Jobs) != 1 {
		t.Fatalf("expected one status context job, got %d", len(runs[0].Jobs))
	}

	job := runs[0].Jobs[0]
	if job.Kind != data.JobKindStatusContext {
		t.Fatalf("expected status context job kind, got %v", job.Kind)
	}
	if job.Name != "vercel/deployment" {
		t.Fatalf("expected status context name, got %q", job.Name)
	}
	if job.Bucket != data.CheckBucketFail {
		t.Fatalf("expected failed bucket, got %v", job.Bucket)
	}
}

func TestEmbeddedFlatChecksShowsLoadingBeforeFirstFetch(t *testing.T) {
	m := NewModel("dlvhdr/gh-dehub", "1", ModelOpts{Flat: true})
	m.SetSize(80, 20)

	view := m.EmbeddedView()
	if !strings.Contains(view, "Loading checks") {
		t.Fatalf("expected loading checks message, got %q", view)
	}
}

func TestEmbeddedFlatChecksShowsRateLimitError(t *testing.T) {
	m := NewModel("dlvhdr/gh-dehub", "1", ModelOpts{Flat: true, Embedded: true})
	m.SetSize(80, 20)

	err := errors.New("rate limit exceeded")
	next, _ := m.Update(workflowRunsFetchedMsg{err: err})

	view := next.EmbeddedView()
	if strings.Contains(view, "Loading checks") {
		t.Fatalf("expected loading checks message to stop after error, got %q", view)
	}
	if !strings.Contains(view, err.Error()) {
		t.Fatalf("expected embedded view to show fetch error, got %q", view)
	}
}

func TestEmbeddedModelUsesFullProvidedHeight(t *testing.T) {
	m := NewModel("dlvhdr/gh-dehub", "1", ModelOpts{Flat: true, Embedded: true})
	m.SetSize(80, 20)

	if got := m.getMainContentHeight(); got != 20 {
		t.Fatalf("expected embedded checks to use full parent height, got %d", got)
	}
}

func TestLogsCopySelectionContentUsesVisibleUnstyledLogs(t *testing.T) {
	m := NewModel("dlvhdr/gh-dehub", "1", ModelOpts{Flat: true, Embedded: true})
	m.SetSize(80, 10)
	check := NewCheckItem(data.WorkflowJob{
		Id:         "1",
		Name:       "check",
		State:      api.StatusCompleted,
		Conclusion: api.ConclusionSuccess,
		Bucket:     data.CheckBucketPass,
		Kind:       data.JobKindGithubActions,
	}, m.styles)
	check.unstyledLogs = []string{"one", "two", "three"}
	check.renderedLogs = []string{"one", "two", "three"}
	m.checksList.SetItems([]list.Item{&check})
	m.logsViewport.SetContentLines(check.renderedLogs)
	m.logsViewport.SetHeight(2)
	m.logsViewport.SetYOffset(1)

	content := m.LogsCopySelectionContent()
	if strings.Contains(content, "one") {
		t.Fatalf("expected copy content to start at visible viewport offset, got %q", content)
	}
	if !strings.Contains(content, "    2 │ two") || !strings.Contains(content, "    3 │ three") {
		t.Fatalf("expected copy content to include visible log lines with gutters, got %q", content)
	}
}

func TestRenderLogsStripsANSIEscapes(t *testing.T) {
	m := NewModel("dlvhdr/gh-dehub", "1", ModelOpts{})
	m.SetSize(80, 20)
	ji := &jobItem{
		logs: []data.LogsWithTime{
			{Log: "\x1b[36mcolored output\x1b[0m"},
			{Log: "^[[31mcaret escaped output^[[0m"},
		},
	}

	rendered, unstyled := m.renderLogs(ji)

	if got := rendered[0]; got != "colored output" {
		t.Fatalf("expected rendered logs to strip ANSI escapes, got %q", got)
	}
	if got := unstyled[0]; got != "colored output" {
		t.Fatalf("expected unstyled logs to strip ANSI escapes, got %q", got)
	}
	if got := rendered[1]; got != "caret escaped output" {
		t.Fatalf("expected rendered logs to strip caret ANSI escapes, got %q", got)
	}
	if got := unstyled[1]; got != "caret escaped output" {
		t.Fatalf("expected unstyled logs to strip caret ANSI escapes, got %q", got)
	}
}

func TestFlatChecksCanSelectChecksImperatively(t *testing.T) {
	m := NewModel("dlvhdr/gh-dehub", "1", ModelOpts{Flat: true})
	m.SetSize(80, 20)
	first := NewCheckItem(data.WorkflowJob{
		Id:         "1",
		Name:       "first",
		State:      api.StatusCompleted,
		Conclusion: api.ConclusionSuccess,
		Bucket:     data.CheckBucketPass,
		Kind:       data.JobKindStatusContext,
	}, m.styles)
	second := NewCheckItem(data.WorkflowJob{
		Id:         "2",
		Name:       "second",
		State:      api.StatusCompleted,
		Conclusion: api.ConclusionSuccess,
		Bucket:     data.CheckBucketPass,
		Kind:       data.JobKindStatusContext,
	}, m.styles)
	m.checksList.SetItems([]list.Item{&first, &second})

	moved, _ := m.SelectNextCheck()
	if !moved {
		t.Fatal("expected SelectNextCheck to move selection")
	}
	selected := m.getSelectedCheckItem()
	if selected == nil || selected.job.Id != "2" {
		t.Fatalf("expected SelectNextCheck to select second check, got %#v", selected)
	}

	moved, _ = m.SelectPrevCheck()
	if !moved {
		t.Fatal("expected SelectPrevCheck to move selection")
	}
	selected = m.getSelectedCheckItem()
	if selected == nil || selected.job.Id != "1" {
		t.Fatalf("expected SelectPrevCheck to select first check, got %#v", selected)
	}
}

func TestActionViewDoesNotBindCommaDotForCheckNavigation(t *testing.T) {
	m := NewModel("dlvhdr/gh-dehub", "1", ModelOpts{Flat: true})

	for _, key := range m.checksList.KeyMap.CursorUp.Keys() {
		if key == "," {
			t.Fatal("actions checks list must not bind comma directly")
		}
	}
	for _, key := range m.checksList.KeyMap.CursorDown.Keys() {
		if key == "." {
			t.Fatal("actions checks list must not bind dot directly")
		}
	}
}

func TestEmbeddedActionViewDoesNotHandleKeyPresses(t *testing.T) {
	m := NewModel("dlvhdr/gh-dehub", "1", ModelOpts{Flat: true})
	first := NewCheckItem(data.WorkflowJob{
		Id:         "1",
		Name:       "first",
		State:      api.StatusCompleted,
		Conclusion: api.ConclusionSuccess,
		Bucket:     data.CheckBucketPass,
		Kind:       data.JobKindStatusContext,
	}, m.styles)
	second := NewCheckItem(data.WorkflowJob{
		Id:         "2",
		Name:       "second",
		State:      api.StatusCompleted,
		Conclusion: api.ConclusionSuccess,
		Bucket:     data.CheckBucketPass,
		Kind:       data.JobKindStatusContext,
	}, m.styles)
	m.checksList.SetItems([]list.Item{&first, &second})

	next, cmd := m.UpdateEmbedded(tea.KeyPressMsg{Code: 'j'})
	if cmd != nil {
		t.Fatal("expected embedded keypress to return no command")
	}
	selected := next.getSelectedCheckItem()
	if selected == nil || selected.job.Id != "1" {
		t.Fatalf("expected embedded keypress not to move selection, got %#v", selected)
	}
}

func TestFocusLogsSearchAllowsEmbeddedSearchInputKeys(t *testing.T) {
	m := NewModel("dlvhdr/gh-dehub", "1", ModelOpts{Flat: true})
	m.FocusLogsSearch()
	if !m.logsInput.Focused() {
		t.Fatal("expected logs input to be focused")
	}

	next, _ := m.UpdateEmbedded(tea.KeyPressMsg{Text: "x", Code: 'x'})
	if next.logsInput.Value() != "x" {
		t.Fatalf("expected embedded keypress to update focused logs search, got %q", next.logsInput.Value())
	}
}

func TestEscClearsFocusedLogsSearch(t *testing.T) {
	m := NewModel("dlvhdr/gh-dehub", "1", ModelOpts{Flat: true})
	m.FocusLogsSearch()

	next, _ := m.UpdateEmbedded(tea.KeyPressMsg{Text: "x", Code: 'x'})
	next, _ = next.UpdateEmbedded(tea.KeyPressMsg{Code: tea.KeyEscape})

	if next.logsInput.Focused() {
		t.Fatal("expected esc to blur logs search")
	}
	if next.logsInput.Value() != "" {
		t.Fatalf("expected esc to clear logs search, got %q", next.logsInput.Value())
	}
}

func TestFocusLogsSearchKeepsChecksPaneVisible(t *testing.T) {
	m := NewModel("dlvhdr/gh-dehub", "1", ModelOpts{Flat: true})
	m.SetSize(100, 30)
	check := NewCheckItem(data.WorkflowJob{
		Id:         "1",
		Name:       "first check",
		State:      api.StatusCompleted,
		Conclusion: api.ConclusionSuccess,
		Bucket:     data.CheckBucketPass,
		Kind:       data.JobKindStatusContext,
	}, m.styles)
	m.checksList.SetItems([]list.Item{&check})

	m.FocusLogsSearch()

	if m.focusedPane != PaneChecks {
		t.Fatalf("expected focused pane to remain checks, got %v", m.focusedPane)
	}
	if !strings.Contains(m.EmbeddedView(), "first check") {
		t.Fatalf("expected checks pane to remain visible while logs search is focused")
	}
}

func TestMergingOfDifferentWorkflowJobs(t *testing.T) {
	d, err := os.ReadFile("./testdata/fetchPROneContext.json")
	if err != nil {
		t.Fatalf("failed reading mock data file %v", err)
	}

	res := struct{ Data api.PRCheckRunsQuery }{}
	err = graphql.UnmarshalGraphQL(d, &res)
	if err != nil {
		t.Error(err)
	}

	wfr := makeWorkflowRun(
		res.Data.Resource.PullRequest.Commits.Nodes[0].Commit.StatusCheckRollup.Contexts.Nodes[0].CheckRun,
	)

	m := NewModel("dlvhdr/gh-dehub", "1", ModelOpts{})
	m.prWithChecks = res.Data.Resource.PullRequest

	runs := makeWorkflowRuns(
		m.prWithChecks.Commits.Nodes[0].Commit.StatusCheckRollup.Contexts.Nodes,
	)
	msg1 := workflowRunsFetchedMsg{runs: runs, pr: m.prWithChecks}
	m.mergeWorkflowRuns(msg1)

	if len(m.workflowRuns) != 1 {
		t.Fatalf(`expected workflow runs to have length of 1, got: %d`, len(m.workflowRuns))
	}

	if len(m.workflowRuns[0].Jobs) != 1 {
		t.Logf("%+v", m.workflowRuns[0].Jobs)
		t.Fatalf(`expected jobs to have length of 1, got: %d`, len(m.workflowRuns[0].Jobs))
	}

	next := []data.WorkflowRun{
		{
			Id:       "CR_kwDOAPphoM8AAAAKdilagx",
			Name:     "some-other-id",
			Link:     "https://github.com/neovim/neovim/runs/11111111111",
			Workflow: m.workflowRuns[0].Workflow,
			Event:    m.workflowRuns[0].Event,
			Jobs: []data.WorkflowJob{
				{
					Id:         "job1",
					State:      api.StatusCompleted,
					Conclusion: api.ConclusionSuccess,
					Name:       m.workflowRuns[0].Jobs[0].Name,
					Title:      m.workflowRuns[0].Jobs[0].Name,
					Workflow:   "some-other-workflow",
					Event:      "pull_request",
					Logs:       []data.LogsWithTime{},
					Link:       "https://github.com/neovim/neovim/actions/runs/15928656163/job/44932094595",
					Bucket:     data.CheckBucketPass,
					Kind:       data.JobKindCheckRun,
				},
			},
			Bucket: wfr.Bucket,
		},
	}
	msg2 := workflowRunsFetchedMsg{runs: next}
	m.mergeWorkflowRuns(msg2)

	if len(m.workflowRuns) != 2 {
		t.Errorf(`expected workflow runs to have length of 2, got: %d`, len(m.workflowRuns))
	}

	if len(m.workflowRuns[0].Jobs) != 1 {
		t.Errorf(`expected jobs to have length of 1, got: %d`, len(m.workflowRuns[0].Jobs))
	}
	if len(m.workflowRuns[1].Jobs) != 1 {
		t.Errorf(`expected jobs to have length of 1, got: %d`, len(m.workflowRuns[1].Jobs))
	}
}
