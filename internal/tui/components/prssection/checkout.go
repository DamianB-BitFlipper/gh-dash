package prssection

import (
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dehub/v4/internal/tui/common"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/context"
)

func (m *Model) checkout() (tea.Cmd, error) {
	pr := m.GetCurrRow()
	if pr == nil {
		return nil, errors.New("no pr selected")
	}

	repoName := pr.GetRepoNameWithOwner()
	repoPath, ok := common.GetRepoLocalPath(repoName, m.Ctx.Config.RepoPaths)

	if !ok {
		return nil, errors.New(
			"local path to repo not specified, set one in your config.yml under repoPaths",
		)
	}

	prNumber := pr.GetNumber()
	taskId := fmt.Sprintf("checkout_%d", prNumber)
	task := context.Task{
		Id:           taskId,
		StartText:    fmt.Sprintf("Checking out PR #%d", prNumber),
		FinishedText: fmt.Sprintf("PR #%d has been checked out at %s", prNumber, repoPath),
		State:        context.TaskStart,
		Error:        nil,
	}
	startCmd := m.Ctx.StartTask(task)
	return tea.Batch(startCmd, func() tea.Msg {
		err := common.RunRepoCommand(
			repoPath,
			"gh",
			"pr",
			"checkout",
			fmt.Sprint(m.GetCurrRow().GetNumber()),
		)
		return constants.TaskFinishedMsg{TaskId: taskId, Err: err}
	}), nil
}
