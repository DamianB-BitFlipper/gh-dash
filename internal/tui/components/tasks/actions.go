package tasks

import (
	"fmt"
	"os/exec"

	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dehub/v4/internal/data"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/context"
)

type UpdateActionMsg struct {
	RunID      int
	Repo       string
	Status     *string
	Conclusion *string
}

func buildActionTaskId(prefix string, runID int) string {
	return fmt.Sprintf("actions_%s_%d", prefix, runID)
}

func RerunWorkflowRun(ctx *context.ProgramContext, section SectionIdentifier, run data.RowData) tea.Cmd {
	runID := run.GetNumber()
	repo := run.GetRepoNameWithOwner()
	return fireTask(ctx, GitHubTask{
		Id: buildActionTaskId("rerun", runID),
		Args: []string{
			"run",
			"rerun",
			fmt.Sprint(runID),
			"-R",
			repo,
		},
		Section:      section,
		StartText:    fmt.Sprintf("Rerunning workflow run %d", runID),
		FinishedText: fmt.Sprintf("Workflow run %d has been rerun", runID),
		Msg: func(c *exec.Cmd, err error) tea.Msg {
			status := "queued"
			return UpdateActionMsg{RunID: runID, Repo: repo, Status: &status}
		},
	})
}

func RerunFailedJobs(ctx *context.ProgramContext, section SectionIdentifier, run data.RowData) tea.Cmd {
	runID := run.GetNumber()
	repo := run.GetRepoNameWithOwner()
	return fireTask(ctx, GitHubTask{
		Id: buildActionTaskId("rerun_failed", runID),
		Args: []string{
			"run",
			"rerun",
			fmt.Sprint(runID),
			"--failed",
			"-R",
			repo,
		},
		Section:      section,
		StartText:    fmt.Sprintf("Rerunning failed jobs for workflow run %d", runID),
		FinishedText: fmt.Sprintf("Failed jobs for workflow run %d have been rerun", runID),
		Msg: func(c *exec.Cmd, err error) tea.Msg {
			status := "queued"
			return UpdateActionMsg{RunID: runID, Repo: repo, Status: &status}
		},
	})
}

func CancelWorkflowRun(ctx *context.ProgramContext, section SectionIdentifier, run data.RowData) tea.Cmd {
	runID := run.GetNumber()
	repo := run.GetRepoNameWithOwner()
	return fireTask(ctx, GitHubTask{
		Id: buildActionTaskId("cancel", runID),
		Args: []string{
			"run",
			"cancel",
			fmt.Sprint(runID),
			"-R",
			repo,
		},
		Section:      section,
		StartText:    fmt.Sprintf("Cancelling workflow run %d", runID),
		FinishedText: fmt.Sprintf("Workflow run %d has been cancelled", runID),
		Msg: func(c *exec.Cmd, err error) tea.Msg {
			status := "completed"
			conclusion := "cancelled"
			return UpdateActionMsg{RunID: runID, Repo: repo, Status: &status, Conclusion: &conclusion}
		},
	})
}

func EnableWorkflow(ctx *context.ProgramContext, section SectionIdentifier, repo string, workflowID string) tea.Cmd {
	return workflowStateTask(ctx, section, repo, workflowID, "enable")
}

func DisableWorkflow(ctx *context.ProgramContext, section SectionIdentifier, repo string, workflowID string) tea.Cmd {
	return workflowStateTask(ctx, section, repo, workflowID, "disable")
}

func workflowStateTask(
	ctx *context.ProgramContext,
	section SectionIdentifier,
	repo string,
	workflowID string,
	action string,
) tea.Cmd {
	return fireTask(ctx, GitHubTask{
		Id: fmt.Sprintf("actions_workflow_%s_%s_%s", action, repo, workflowID),
		Args: []string{
			"workflow",
			action,
			workflowID,
			"-R",
			repo,
		},
		Section:      section,
		StartText:    fmt.Sprintf("%s workflow %s", actionTitle(action), workflowID),
		FinishedText: fmt.Sprintf("Workflow %s has been %sd", workflowID, action),
		Msg: func(c *exec.Cmd, err error) tea.Msg {
			return UpdateActionMsg{Repo: repo}
		},
	})
}

func actionTitle(action string) string {
	switch action {
	case "enable":
		return "Enabling"
	case "disable":
		return "Disabling"
	default:
		return action
	}
}
