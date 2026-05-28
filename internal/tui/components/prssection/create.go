package prssection

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dehub/v4/internal/data"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/common"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/fuzzyselect"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/prrow"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/tasks"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/context"
)

type createPRCreatedMsg struct {
	PR *prrow.Data
}

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

var (
	runCreatePRRepoCommand = runCreatePRCommand
	fetchCreatedPR         = data.FetchPullRequest
)

func runCreatePRCommand(repoPath string, args ...string) (string, error) {
	if len(args) == 0 {
		return "", nil
	}
	repoPath = common.ExpandRepoPath(repoPath)

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	out := strings.TrimSpace(string(output))
	if err == nil {
		return out, nil
	}
	if out == "" {
		out = err.Error()
	}
	return out, fmt.Errorf("%s failed in %s: %s", strings.Join(args, " "), repoPath, out)
}

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
	m.CreatePRForm.SetCreateMode()
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

func (m *Model) PrepareEditPRForm(pr *prrow.Data, branches *RepoBranches) (tea.Cmd, error) {
	if pr == nil || pr.Primary == nil {
		return nil, errors.New("current selection isn't associated with a PR")
	}

	repoName := pr.Primary.Repository.NameWithOwner
	m.createPRBranchRequestID++
	m.CreatePRForm.SetEditPR(pr.Primary.Title, pr.Primary.Body, pr.Primary.HeadRefName, pr.Primary.BaseRefName)
	if branches != nil && branches.RepoName == repoName {
		if branches.Err != nil {
			m.CreatePRForm.SetBranchesError(branches.Err)
		} else {
			m.CreatePRForm.SetBranchSuggestions(branches.Branches)
		}
	} else if m.Ctx != nil && m.Ctx.Config != nil {
		if _, ok := common.GetRepoLocalPath(repoName, m.Ctx.Config.RepoPaths); ok {
			m.CreatePRForm.SetBranchesLoading()
		} else {
			m.CreatePRForm.SetBranchSuggestions([]fuzzyselect.Suggestion{{Value: pr.Primary.BaseRefName}})
		}
	} else {
		m.CreatePRForm.SetBranchSuggestions([]fuzzyselect.Suggestion{{Value: pr.Primary.BaseRefName}})
	}

	return func() tea.Msg {
		return RefreshRepoBranchesMsg{SectionId: m.Id, RepoName: repoName}
	}, nil
}

func (m *Model) ApplyCreatePRBranches(branches RepoBranches) bool {
	action := m.GetPromptConfirmationAction()
	repoName, ok := m.repoFromFilters()
	if action == "edit_pr" {
		if pr, ok := m.GetCurrRow().(*prrow.Data); ok && pr != nil && pr.Primary != nil {
			repoName = pr.Primary.Repository.NameWithOwner
			ok = true
		}
	}
	if !m.IsPromptConfirmationShown || (action != "create_pr" && action != "edit_pr") || !ok || branches.RepoName != repoName {
		return false
	}
	if branches.Err != nil {
		m.CreatePRForm.SetBranchesError(branches.Err)
		m.Ctx.Error = branches.Err
		return true
	}
	if action == "edit_pr" {
		m.CreatePRForm.SetBranchSuggestions(branches.Branches)
	} else {
		m.CreatePRForm.SetBranches(branches.Branches, branches.Head, branches.Base)
	}
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
		output, err := runCreatePRRepoCommand(repoPath, args...)
		var createdPR *prrow.Data
		if err == nil {
			createdPR = fetchCreatedPRData(output)
		}
		return constants.TaskFinishedMsg{
			SectionId:   m.Id,
			SectionType: SectionType,
			TaskId:      taskId,
			Err:         err,
			Msg:         createPRCreatedMsg{PR: createdPR},
		}
	}), nil
}

func (m *Model) editPR(title string, body string, base string) (tea.Cmd, error) {
	if m.CreatePRForm.BranchesLoading() {
		return nil, errors.New("branches are still loading")
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, errors.New("PR title is required")
	}
	pr, ok := m.GetCurrRow().(*prrow.Data)
	if !ok || pr == nil || pr.Primary == nil {
		return nil, errors.New("current selection isn't associated with a PR")
	}

	sid := tasks.SectionIdentifier{Id: m.Id, Type: SectionType}
	return tasks.EditPR(m.Ctx, sid, pr, title, body, strings.TrimSpace(base)), nil
}

func fetchCreatedPRData(output string) *prrow.Data {
	url := createdPRURL(output)
	if url == "" {
		return nil
	}
	pr, err := fetchCreatedPR(url)
	if err != nil {
		return nil
	}
	primary := pr.ToPullRequestData()
	return &prrow.Data{Primary: &primary, Enriched: pr, IsEnriched: true}
}

func createdPRURL(output string) string {
	for _, field := range strings.Fields(output) {
		field = strings.Trim(field, "\t\n\r .,;()[]{}<>")
		if strings.HasPrefix(field, "https://github.com/") && strings.Contains(field, "/pull/") {
			return field
		}
	}
	return ""
}
