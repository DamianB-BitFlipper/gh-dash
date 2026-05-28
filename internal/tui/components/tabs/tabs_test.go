package tabs

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	// "charm.land/x/exp/teatest"

	"github.com/dlvhdr/gh-dehub/v4/internal/config"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/common"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/section"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/tabs/testdata"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/theme"
)

func TestViewSectionTabsDoesNotExceedAvailableWidth(t *testing.T) {
	ctx := newTabsTestContext(t)
	m := NewModel(ctx)
	m.SetSections([]section.Section{
		&testdata.TestSection{Config: config.SectionConfig{Title: ""}},
		&testdata.TestSection{Config: config.SectionConfig{Title: "platform"}},
		&testdata.TestSection{Config: config.SectionConfig{Title: "iac"}},
		&testdata.TestSection{Config: config.SectionConfig{Title: "prime"}},
		&testdata.TestSection{Config: config.SectionConfig{Title: "My Pull Requests"}},
		&testdata.TestSection{Config: config.SectionConfig{Title: "Needs My Review"}},
		&testdata.TestSection{Config: config.SectionConfig{Title: "Involved"}},
	})
	m.SetCurrSectionId(1)

	view := m.viewSectionTabs(50)
	plain := ansi.Strip(view)

	require.NotContains(t, view, "\n")
	require.LessOrEqual(t, lipgloss.Width(view), 50)
	require.Less(t, strings.Index(plain, ""), strings.Index(plain, "platform"))
}

func TestViewSectionTabsReservesWidthForInlineSearch(t *testing.T) {
	ctx := newTabsTestContext(t)
	m := NewModel(ctx)
	m.SetSections([]section.Section{
		&testdata.TestSection{Config: config.SectionConfig{Title: ""}},
		&testdata.TestSection{Config: config.SectionConfig{Title: "platform"}, HeaderSearch: " repo:owner/repo is:open"},
		&testdata.TestSection{Config: config.SectionConfig{Title: "Very Long Section Name"}},
		&testdata.TestSection{Config: config.SectionConfig{Title: "Another Long Section Name"}},
	})
	m.SetCurrSectionId(1)

	view := m.viewSectionTabs(55)
	plain := ansi.Strip(view)

	require.NotContains(t, view, "\n")
	require.LessOrEqual(t, lipgloss.Width(view), 55)
	require.True(t, strings.Contains(plain, "repo:owner/repo"))
	require.Less(t, strings.Index(plain, "repo:owner/repo"), strings.Index(plain, "platform"))
}

func TestViewDoesNotRenderLogo(t *testing.T) {
	ctx := newTabsTestContext(t)
	m := NewModel(ctx)
	m.SetSections([]section.Section{
		&testdata.TestSection{Config: config.SectionConfig{Title: "Mine"}},
	})

	view := ansi.Strip(m.View())

	require.NotContains(t, view, strings.Split(constants.Logo, "\n")[0])
}

func TestViewHeightMatchesTabsHeight(t *testing.T) {
	ctx := newTabsTestContext(t)
	m := NewModel(ctx)
	m.SetSections([]section.Section{
		&testdata.TestSection{Config: config.SectionConfig{Title: "Mine"}},
	})

	require.Equal(t, common.TabsHeight, lipgloss.Height(m.View()))
}

func newTabsTestContext(t *testing.T) *context.ProgramContext {
	t.Helper()
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../../../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)
	thm := theme.ParseTheme(&cfg)
	return &context.ProgramContext{
		Config:       &cfg,
		ScreenWidth:  80,
		ScreenHeight: 30,
		Theme:        thm,
		Styles:       context.InitStyles(thm),
		View:         config.PRsView,
	}
}

// func TestTabs(t *testing.T) {
// 	t.Parallel()
//
// 	t.Run("Should display loading tabs", func(t *testing.T) {
// 		t.Parallel()
// 		cfg, err := config.ParseConfig(config.Location{
// 			ConfigFlag:       "../../../config/testdata/test-config.yml",
// 			SkipGlobalConfig: true,
// 		})
// 		if err != nil {
// 			t.Error(err)
// 		}
// 		m := newTestModel(t, cfg)
// 		tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 30))
//
// 		testutils.WaitForText(t, tm, "  |  Mine   ⣻   |  Review   ⣻   |  All   ⣻")
// 		tm.Quit()
//
// 		fm := tm.FinalModel(t)
// 		teatest.RequireEqualOutput(t, []byte(fm.View()))
// 	})
//
// 	t.Run("Should display tab counts", func(t *testing.T) {
// 		t.Parallel()
// 		cfg, err := config.ParseConfig(config.Location{
// 			ConfigFlag:       "../../../config/testdata/test-config.yml",
// 			SkipGlobalConfig: true,
// 		})
// 		if err != nil {
// 			t.Error(err)
// 		}
// 		m := newTestModel(t, cfg)
// 		tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 30))
//
// 		testutils.WaitForText(t, tm, "  |  Mine   ⣻   |  Review   ⣻   |  All   ⣻")
// 		tm.Send(dataFetchedMsg{})
// 		testutils.WaitForText(t, tm, "  |  Mine (10)  |  Review (10)  |  All (10)")
// 		tm.Quit()
//
// 		fm := tm.FinalModel(t)
// 		teatest.RequireEqualOutput(t, []byte(fm.View()))
// 	})
//
// 	t.Run("Should allow setting new tabs", func(t *testing.T) {
// 		t.Parallel()
// 		cfg, err := config.ParseConfig(config.Location{
// 			ConfigFlag:       "../../../config/testdata/test-config.yml",
// 			SkipGlobalConfig: true,
// 		})
// 		if err != nil {
// 			t.Error(err)
// 		}
// 		m := newTestModel(t, cfg)
// 		tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 30))
//
// 		testutils.WaitForText(t, tm, "  |  Mine   ⣻   |  Review   ⣻   |  All   ⣻")
// 		tm.Send(dataFetchedMsg{})
// 		testutils.WaitForText(t, tm, "  |  Mine (10)  |  Review (10)  |  All (10)")
//
// 		tm.Send(changeTabsMsg{})
// 		testutils.WaitForText(t, tm, "  |  Mine New   ⣻   |  Review New   ⣻   |  All New   ⣻")
//
// 		tm.Quit()
//
// 		fm := tm.FinalModel(t)
// 		teatest.RequireEqualOutput(t, []byte(fm.View()))
// 	})
//
// 	t.Run("Should show overflow symbol", func(t *testing.T) {
// 		t.Parallel()
//
// 		baseCfg, err := config.ParseConfig(config.Location{
// 			ConfigFlag:       "../../../config/testdata/test-config.yml",
// 			SkipGlobalConfig: true,
// 		})
// 		if err != nil {
// 			t.Error(err)
// 		}
//
// 		m := newTestModel(t, config.Config{
// 			PRSections: []config.PrsSectionConfig{
// 				{Title: "1. Very long title"},
// 				{Title: "2. Title"},
// 				{Title: "3. Title"},
// 				{Title: "4. Very long title"},
// 				{Title: "5. Title"},
// 				{Title: "6. Title"},
// 				{Title: "7. Very long title"},
// 			},
// 			Theme: baseCfg.Theme,
// 		})
//
// 		tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 30))
//
// 		testutils.WaitForText(t, tm, "  |  1. Very long title   ⣻   |  2. Title   ⣻   |  3. Title   ⣻   |  … →")
// 		tm.Send(dataFetchedMsg{})
// 		testutils.WaitForText(t, tm, "  |  1. Very long title (10)  |  2. Title (10)  |  3. Title (10)  |  … →")
// 		for i := 0; i < len(m.ctx.Config.PRSections); i++ {
// 			tm.Send(tea.KeyMsg{
// 				Type:  tea.KeyRunes,
// 				Runes: []rune("l"),
// 			})
// 		}
// 		testutils.WaitForText(t, tm, "← … (10)  |  5. Title (10)  |  6. Title (10)  |  7. Very long title (10)")
// 		tm.Quit()
//
// 		fm := tm.FinalModel(t)
// 		teatest.RequireEqualOutput(t, []byte(fm.View()))
// 	})
// }

// func init() {
// 	lipgloss.SetColorProfile(termenv.Ascii)
// 	if d := os.Getenv("DEBUG"); d != "" {
// 		log.SetLevel(log.DebugLevel)
// 	}
// }

type testModel struct {
	ctx  *context.ProgramContext
	tabs Model
}

// func newTestModel(t *testing.T, cfg config.Config) testModel {
// 	t.Helper()
// 	ctx := &context.ProgramContext{
// 		Config:       &cfg,
// 		ScreenWidth:  90,
// 		ScreenHeight: 30,
// 		View:         config.PRsView,
// 	}
//
// 	ctx.Theme = theme.ParseTheme(ctx.Config)
// 	ctx.Styles = context.InitStyles(ctx.Theme)
//
// 	return testModel{
// 		ctx:  ctx,
// 		tabs: NewModel(ctx),
// 	}
// }

type (
	initMsg        struct{}
	dataFetchedMsg struct{}
	changeTabsMsg  struct{}
)

func (m testModel) Init() tea.Cmd {
	return func() tea.Msg { return initMsg{} }
}

func (m testModel) Update(msg tea.Msg) (testModel, tea.Cmd) {
	var cmd tea.Cmd
	cmds := make([]tea.Cmd, 0)
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "l" || msg.String() == "right" {
			m.tabs.SetCurrSectionId(m.tabs.CurrSectionId() + 1)
		}
		if msg.String() == "h" || msg.String() == "left" {
			m.tabs.SetCurrSectionId(m.tabs.CurrSectionId() - 1)
		}

		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case initMsg:
		sections := make([]section.Section, 0)
		search := testdata.TestSection{Config: config.SectionConfig{Title: ""}}
		sections = append(sections, &search)
		for _, cfg := range m.ctx.Config.PRSections {
			s := testdata.TestSection{Config: config.SectionConfig{Title: cfg.Title}}
			s.SetIsLoading(true)
			sections = append(sections, &s)
		}
		m.tabs.SetSections(sections)
		cmds = append(cmds, m.tabs.SetAllLoading()...)

	case dataFetchedMsg:
		for _, tab := range m.tabs.sectionTabs {
			tab.section.SetIsLoading(false)
		}

	case changeTabsMsg:
		sections := make([]section.Section, 0)
		search := testdata.TestSection{Config: config.SectionConfig{Title: ""}}
		sections = append(sections, &search)
		for _, cfg := range m.ctx.Config.PRSections {
			s := testdata.TestSection{Config: config.SectionConfig{Title: cfg.Title + " New"}}
			s.SetIsLoading(true)
			sections = append(sections, &s)
		}
		m.tabs.SetSections(sections)
		cmds = append(cmds, m.tabs.SetAllLoading()...)
	}

	tm, cmd := m.tabs.Update(msg)
	m.tabs = tm
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m testModel) View() string {
	return m.tabs.View()
}
