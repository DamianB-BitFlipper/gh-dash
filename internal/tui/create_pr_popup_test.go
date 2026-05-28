package tui

import (
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dehub/v4/internal/config"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/prssection"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/section"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/theme"
)

func TestRenderCreatePRPopup(t *testing.T) {
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
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	prSection := prssection.NewModel(0, ctx, config.PrsSectionConfig{
		Title:   "Repo PRs",
		Filters: "repo:owner/name is:open",
	}, time.Now(), time.Now())
	prSection.SetPromptConfirmationAction("create_pr")
	_ = prSection.SetIsPromptConfirmationShown(true)

	m := NewModel(config.Location{})
	m.ctx = ctx
	m.prs = []section.Section{&prSection}
	m.currSectionId = 0

	view := ansi.Strip(m.renderCreatePRPopup())

	require.Contains(t, view, "Create PR")
	require.Contains(t, view, "Repository: owner/name")
	require.Contains(t, view, "Title")
	require.Contains(t, view, "Head branch")
	require.Contains(t, view, "Base branch")
	require.Contains(t, view, "Body")
	require.Contains(t, view, "ctrl+d submit")
	require.Empty(t, prSection.GetPromptConfirmation())
}
