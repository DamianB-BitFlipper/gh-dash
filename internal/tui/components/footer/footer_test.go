package footer

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dehub/v4/internal/config"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/theme"
)

func TestLogoOnlyRendersInExpandedHelp(t *testing.T) {
	ctx := newFooterTestContext(t)
	m := NewModel(ctx)
	logoFirstLine := strings.Split(constants.Logo, "\n")[0]

	require.NotContains(t, ansi.Strip(m.View()), logoFirstLine)

	m.ShowAll = true
	view := ansi.Strip(m.View())
	require.Contains(t, view, logoFirstLine)
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, logoFirstLine) {
			require.Contains(t, line, "move up")
			require.Greater(t, strings.Index(line, logoFirstLine), ctx.ScreenWidth/2)
			return
		}
	}
	t.Fatal("logo first line not found")
}

func newFooterTestContext(t *testing.T) *context.ProgramContext {
	t.Helper()
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../../../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)
	thm := theme.ParseTheme(&cfg)
	return &context.ProgramContext{
		Config:       &cfg,
		ScreenWidth:  120,
		ScreenHeight: 40,
		Theme:        thm,
		Styles:       context.InitStyles(thm),
		View:         config.PRsView,
	}
}
