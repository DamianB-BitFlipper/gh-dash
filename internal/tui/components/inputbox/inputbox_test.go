package inputbox

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/fuzzyselect"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/theme"
)

func newTestInputBox() Model {
	thm := *theme.DefaultTheme
	ctx := &context.ProgramContext{
		Theme:  thm,
		Styles: context.InitStyles(thm),
	}
	input := DefaultTextInput(ctx)
	fzfSelect := fuzzyselect.NewModel(ctx, &fuzzyselect.ListSource{Options: []fuzzyselect.Suggestion{
		{Value: "alice"},
	}})
	m := NewModel(ctx, ModelOpts{TextInput: &input})
	m.SetAutocomplete(&fzfSelect)
	m.fzfSelect.Filter("", fuzzyselect.Context{}, nil)
	m.fzfSelect.Show()
	return m
}

func TestEnterSelectsAutocompleteSuggestion(t *testing.T) {
	m := newTestInputBox()

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	require.Equal(t, "alice", m.Value())
}

func TestCtrlYDoesNotSelectAutocompleteSuggestion(t *testing.T) {
	m := newTestInputBox()

	m, _ = m.Update(tea.KeyPressMsg{Code: 'y', Mod: tea.ModCtrl})

	require.Empty(t, m.Value())
}
