package context

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dehub/v4/internal/config"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/common"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/theme"
)

type State = int

const (
	TaskStart State = iota
	TaskFinished
	TaskError
)

type Task struct {
	Id           string
	StartText    string
	FinishedText string
	State        State
	Error        error
	StartTime    time.Time
	FinishedTime *time.Time
}

type ProgramContext struct {
	RepoPath             string
	RepoUrl              string
	User                 string
	ScreenHeight         int
	ScreenWidth          int
	MainContentWidth     int
	MainContentHeight    int
	DynamicPreviewWidth  int
	DynamicPreviewHeight int    // calculated preview height for bottom mode
	PreviewPosition      string // resolved "right" or "bottom"
	SidebarOpen          bool
	Config               *config.Config
	ConfigFlag           string
	Version              string
	View                 config.ViewType
	ActivePane           string
	Error                error
	StartTask            func(task Task) tea.Cmd
	Theme                theme.Theme
	Styles               Styles
}

func (ctx *ProgramContext) GetViewSectionsConfig() []config.SectionConfig {
	var configs []config.SectionConfig
	switch ctx.View {
	case config.NotificationsView:
		for _, cfg := range ctx.Config.NotificationsSections {
			configs = append(configs, cfg.ToSectionConfig())
		}
	case config.PRsView:
		for _, cfg := range ctx.Config.PRSections {
			configs = append(configs, cfg.ToSectionConfig())
		}
	case config.IssuesView:
		for _, cfg := range ctx.Config.IssuesSections {
			configs = append(configs, cfg.ToSectionConfig())
		}
	case config.ActionsView:
		for _, cfg := range ctx.actionSectionConfigs() {
			configs = append(configs, cfg.ToSectionConfig())
		}
	}

	return append([]config.SectionConfig{{Title: ""}}, configs...)
}

func (ctx *ProgramContext) actionSectionConfigs() []config.ActionsSectionConfig {
	configured := make([]config.ActionsSectionConfig, 0, len(ctx.Config.ActionsSections))
	for _, cfg := range ctx.Config.ActionsSections {
		if actionsRepoFromFilters(cfg.Filters) != "" {
			configured = append(configured, cfg)
		}
	}
	if len(configured) > 0 {
		return configured
	}

	repos := common.ExpandRepoPaths(ctx.Config.RepoPaths)
	configured = make([]config.ActionsSectionConfig, 0, len(repos))
	for _, repo := range repos {
		configured = append(configured, config.ActionsSectionConfig{Title: repo.Name, Filters: "repo:" + repo.Name})
	}
	return configured
}

func actionsRepoFromFilters(filters string) string {
	for _, token := range strings.Fields(filters) {
		value, ok := strings.CutPrefix(token, "repo:")
		if ok && value != "" {
			return value
		}
	}
	return ""
}

func (ctx *ProgramContext) PreviewCursorPosition() tea.Position {
	if ctx.PreviewPosition == "right" {
		return tea.Position{
			X: ctx.MainContentWidth,
			Y: ctx.Styles.Pager.Height,
		}
	}

	return tea.Position{
		X: 0,
		Y: ctx.MainContentHeight,
	}
}
