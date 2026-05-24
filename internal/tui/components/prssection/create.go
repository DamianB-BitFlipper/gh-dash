package prssection

import (
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/tui/common"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/fuzzyselect"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
)

type createPRCreatedMsg struct{}

type createPRBranchesFetchedMsg struct {
	RepoName  string
	RequestID uint64
	Branches  []fuzzyselect.Suggestion
	Head      string
	Base      string
	Err       error
}

type RepoBranches struct {
	RepoName string
	Branches []fuzzyselect.Suggestion
	Head     string
	Base     string
	Err      error
}

type RefreshRepoBranchesMsg struct {
	SectionId int
	RepoName  string
}

var runCreatePRRepoCommand = common.RunRepoCommand

func (m *Model) validateCanCreatePR() error {
	repoName, ok := m.repoFromFilters()
	if !ok {
		return errors.New("current PR section must have exactly one repo:owner/name filter to create a PR")
	}

	if _, ok := common.GetRepoLocalPath(repoName, m.Ctx.Config.RepoPaths); !ok {
		return errors.New(
			"local path to repo not specified, set one in your config.yml under repoPaths",
		)
	}
	return nil
}

func (m *Model) prepareCreatePRForm() (tea.Cmd, error) {
	return m.PrepareCreatePRForm(nil)
}

func (m *Model) PrepareCreatePRForm(branches *RepoBranches) (tea.Cmd, error) {
	if err := m.validateCanCreatePR(); err != nil {
		return nil, err
	}
	repoName, _ := m.repoFromFilters()
	m.createPRBranchRequestID++
	if branches != nil && branches.RepoName == repoName {
		if branches.Err != nil {
			m.CreatePRForm.SetBranchesError(branches.Err)
		} else {
			m.CreatePRForm.SetBranches(branches.Branches, branches.Head, branches.Base)
		}
	} else {
		m.CreatePRForm.SetBranchesLoading()
	}

	return func() tea.Msg {
		return RefreshRepoBranchesMsg{SectionId: m.Id, RepoName: repoName}
	}, nil
}

func (m *Model) ApplyCreatePRBranches(branches RepoBranches) bool {
	repoName, ok := m.repoFromFilters()
	if !m.IsPromptConfirmationShown || m.GetPromptConfirmationAction() != "create_pr" ||
		!ok || branches.RepoName != repoName {
		return false
	}
	if branches.Err != nil {
		m.CreatePRForm.SetBranchesError(branches.Err)
		m.Ctx.Error = branches.Err
		return true
	}
	m.CreatePRForm.SetBranches(branches.Branches, branches.Head, branches.Base)
	return true
}

func (m *Model) createPR(title string, body string, head string, base string) (tea.Cmd, error) {
	if m.CreatePRForm.BranchesLoading() {
		return nil, errors.New("branches are still loading")
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, errors.New("PR title is required")
	}

	repoName, _ := m.repoFromFilters()
	repoPath, _ := common.GetRepoLocalPath(repoName, m.Ctx.Config.RepoPaths)

	taskId := fmt.Sprintf("create_pr_%s_%d", strings.ReplaceAll(repoName, "/", "_"), time.Now().Unix())
	task := context.Task{
		Id:           taskId,
		StartText:    fmt.Sprintf("Creating PR in %s", repoName),
		FinishedText: fmt.Sprintf("PR created in %s", repoName),
		State:        context.TaskStart,
		Error:        nil,
	}
	startCmd := m.Ctx.StartTask(task)
	return tea.Batch(startCmd, func() tea.Msg {
		args := []string{"gh", "pr", "create", "--title", title, "--body", body}
		if head != "" {
			args = append(args, "--head", head)
		}
		if base != "" {
			args = append(args, "--base", base)
		}
		err := runCreatePRRepoCommand(repoPath, args...)
		return constants.TaskFinishedMsg{
			SectionId:   m.Id,
			SectionType: SectionType,
			TaskId:      taskId,
			Err:         err,
			Msg:         createPRCreatedMsg{},
		}
	}), nil
}
