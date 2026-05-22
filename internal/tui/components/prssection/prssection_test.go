package prssection

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prompt"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prrow"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/section"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
)

// newTestModel creates a minimal Model with the prompt confirmation box
// focused and a single PR row so that GetCurrRow returns non-nil.
func newTestModel(action string) Model {
	ctx := &context.ProgramContext{
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
	}
	m.PromptConfirmationBox.Focus()
	return m
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

	cmd, err := m.createPR()

	require.Nil(t, cmd)
	require.EqualError(t, err, "current PR section must have exactly one repo:owner/name filter to create a PR")
}

func TestCreatePRRequiresConfiguredRepoPath(t *testing.T) {
	m := newTestModel("")
	m.SearchValue = "repo:owner/name is:open"
	m.Ctx.Config = &config.Config{RepoPaths: map[string]string{}}

	cmd, err := m.createPR()

	require.Nil(t, cmd)
	require.EqualError(t, err, "local path to repo not specified, set one in your config.yml under repoPaths")
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

	cmd, err := m.createPR()

	require.NoError(t, err)
	require.NotNil(t, cmd)
	require.Contains(t, started.Id, "create_pr_owner_name")
	require.Equal(t, "Creating PR in owner/name", started.StartText)
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
