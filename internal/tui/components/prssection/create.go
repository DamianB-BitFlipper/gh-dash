package prssection

import (
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/tui/common"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
)

func (m *Model) createPR() (tea.Cmd, error) {
	repoName, ok := m.repoFromFilters()
	if !ok {
		return nil, errors.New("current PR section must have exactly one repo:owner/name filter to create a PR")
	}

	repoPath, ok := common.GetRepoLocalPath(repoName, m.Ctx.Config.RepoPaths)
	if !ok {
		return nil, errors.New(
			"local path to repo not specified, set one in your config.yml under repoPaths",
		)
	}

	taskId := fmt.Sprintf("create_pr_%s_%d", strings.ReplaceAll(repoName, "/", "_"), time.Now().Unix())
	task := context.Task{
		Id:           taskId,
		StartText:    fmt.Sprintf("Creating PR in %s", repoName),
		FinishedText: fmt.Sprintf("PR create opened for %s", repoName),
		State:        context.TaskStart,
		Error:        nil,
	}
	startCmd := m.Ctx.StartTask(task)
	return tea.Batch(startCmd, func() tea.Msg {
		err := common.RunRepoCommand(repoPath, "gh", "pr", "create", "--web")
		return constants.TaskFinishedMsg{TaskId: taskId, Err: err}
	}), nil
}
