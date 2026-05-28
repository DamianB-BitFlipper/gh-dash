package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/tasks"
)

type mergePopupTestRow struct{}

func (mergePopupTestRow) GetRepoNameWithOwner() string { return "owner/repo" }
func (mergePopupTestRow) GetTitle() string             { return "Add popup merge" }
func (mergePopupTestRow) GetNumber() int               { return 42 }
func (mergePopupTestRow) GetUrl() string               { return "https://github.com/owner/repo/pull/42" }
func (mergePopupTestRow) GetUpdatedAt() time.Time      { return time.Time{} }

func TestMergePRPopupRendersDefaults(t *testing.T) {
	m := newMessagePopupTestModel(t)
	m.openMergePRPopup(tasks.SectionIdentifier{Id: 2, Type: "pr"}, mergePopupTestRow{})

	view := ansi.Strip(m.renderMergePRPopup())

	require.Contains(t, view, "Merge PR")
	require.Contains(t, view, "#42 Add popup merge")
	require.Contains(t, view, "> Squash and merge")
	require.Contains(t, view, "[ ] Enable auto-merge")
	require.Contains(t, view, "[x] Delete branch after merge")
}

func TestMergePRPopupUpdatesOptions(t *testing.T) {
	m := newMessagePopupTestModel(t)
	m.openMergePRPopup(tasks.SectionIdentifier{Id: 2, Type: "pr"}, mergePopupTestRow{})

	cmd := m.updateMergePRPopup(tea.KeyPressMsg{Text: "r"})
	require.Nil(t, cmd)
	cmd = m.updateMergePRPopup(tea.KeyPressMsg{Text: "a"})
	require.Nil(t, cmd)
	cmd = m.updateMergePRPopup(tea.KeyPressMsg{Text: "d"})
	require.Nil(t, cmd)

	require.Equal(t, tasks.MergeMethodRebase, m.mergePRPopup.options().Method)
	require.True(t, m.mergePRPopup.options().Auto)
	require.False(t, m.mergePRPopup.options().DeleteBranch)
}

func TestMergePRPopupDismissesOnEscape(t *testing.T) {
	m := newMessagePopupTestModel(t)
	m.openMergePRPopup(tasks.SectionIdentifier{Id: 2, Type: "pr"}, mergePopupTestRow{})

	updated, cmd := m.Update(tea.KeyPressMsg{Text: "esc"})
	m = updated.(Model)

	require.Nil(t, cmd)
	require.Nil(t, m.mergePRPopup)
}
