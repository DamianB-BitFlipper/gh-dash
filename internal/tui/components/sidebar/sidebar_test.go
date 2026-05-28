package sidebar

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dehub/v4/internal/config"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/theme"
)

func TestFixedHeaderRendersOutsideScrollableContent(t *testing.T) {
	cfg := &config.Config{}
	thm := *theme.DefaultTheme
	ctx := &context.ProgramContext{
		Config:              cfg,
		Theme:               thm,
		Styles:              context.InitStyles(thm),
		MainContentHeight:   6,
		DynamicPreviewWidth: 40,
	}

	m := NewModel()
	m.IsOpen = true
	m.UpdateProgramContext(ctx)
	m.SetHeader("fixed header")
	m.SetContent(strings.Repeat("body line\n", 20))
	m.ScrollToBottom()

	view := m.View()
	require.Contains(t, view, "fixed header")
	require.Greater(t, m.YOffset(), 0, "test setup should scroll body content")
}
