package prssection

import (
	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dehub/v4/internal/tui/common"
)

func (m Model) diff() tea.Cmd {
	currRowData := m.GetCurrRow()
	if currRowData == nil {
		return nil
	}

	return common.DiffPR(
		currRowData.GetNumber(),
		currRowData.GetRepoNameWithOwner(),
		currRowData.GetTitle(),
		currRowData.GetUrl(),
		m.Ctx.Config.Pager.Diff,
		m.Ctx.Config.RunDiffPagerInBackground(),
		m.Ctx.Config.GetFullScreenDiffPagerEnv(),
	)
}
