package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/theme"
)

func newMessagePopupTestModel(t *testing.T) Model {
	t.Helper()
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)

	ctx := &context.ProgramContext{
		Config:       &cfg,
		ScreenWidth:  100,
		ScreenHeight: 30,
		View:         config.PRsView,
		StartTask:    func(task context.Task) tea.Cmd { return nil },
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	m := NewModel(config.Location{})
	m.ctx = ctx
	m.footer.UpdateProgramContext(ctx)
	m.prView.UpdateProgramContext(ctx)
	m.issueSidebar.UpdateProgramContext(ctx)
	m.branchSidebar.UpdateProgramContext(ctx)
	m.notificationView.UpdateProgramContext(ctx)
	m.tabs.UpdateProgramContext(ctx)
	m.sidebar.UpdateProgramContext(ctx)
	return m
}

func TestMessagePopupRendersErrorAndHint(t *testing.T) {
	m := newMessagePopupTestModel(t)
	m.messagePopup = newErrorMessagePopup(errors.New("checkout failed with useful details"))

	view := ansi.Strip(m.renderMessagePopup())

	require.Contains(t, view, "Error")
	require.Contains(t, view, "checkout failed with useful details")
	require.Contains(t, view, "esc/enter to dismiss")
}

func TestMessagePopupDismissesOnEscape(t *testing.T) {
	m := newMessagePopupTestModel(t)
	m.messagePopup = newErrorMessagePopup(errors.New("boom"))

	updated, cmd := m.Update(tea.KeyPressMsg{Text: "esc"})
	m = updated.(Model)

	require.Nil(t, cmd)
	require.Nil(t, m.messagePopup)
}

func TestTaskErrorOpensPersistentMessagePopup(t *testing.T) {
	m := newMessagePopupTestModel(t)
	m.tasks = map[string]context.Task{
		"task": {Id: "task", State: context.TaskStart},
	}

	updated, _ := m.Update(constants.TaskFinishedMsg{TaskId: "task", Err: errors.New("exit status 1")})
	m = updated.(Model)
	require.NotNil(t, m.messagePopup)
	require.Equal(t, "exit status 1", m.messagePopup.body)

	updated, _ = m.Update(constants.ClearTaskMsg{TaskId: "task"})
	m = updated.(Model)
	require.NotNil(t, m.messagePopup)
	require.Equal(t, "exit status 1", m.messagePopup.body)
}

func TestClampPopupBodyAddsTruncationHint(t *testing.T) {
	body := strings.Join([]string{"a", "b", "c", "d"}, "\n")

	got := clampPopupBody(body, 3)

	require.Equal(t, "a\nb\n… truncated", got)
}
