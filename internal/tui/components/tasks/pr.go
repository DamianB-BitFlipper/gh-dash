package tasks

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/log/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dash/v4/internal/utils"
)

type SectionIdentifier struct {
	Id   int
	Type string
}

type UpdatePRMsg struct {
	PrNumber         int
	IsClosed         *bool
	NewComment       *data.Comment
	ThreadReply      *ReviewThreadReply
	ThreadResolved   *ReviewThreadResolved
	ReadyForReview   *bool
	IsDraft          *bool
	IsMerged         *bool
	AddedAssignees   *data.Assignees
	RemovedAssignees *data.Assignees
	Labels           *data.PRLabels
}

type ReviewThreadReply struct {
	ThreadId string
	Comment  data.ReviewComment
}

type ReviewThreadResolved struct {
	ThreadId   string
	IsResolved bool
}

type DraftablePRData interface {
	data.RowData
	GetIsDraft() bool
}

type MergeMethod string

const (
	MergeMethodMerge  MergeMethod = "merge"
	MergeMethodSquash MergeMethod = "squash"
	MergeMethodRebase MergeMethod = "rebase"
)

type MergePROptions struct {
	Method       MergeMethod
	Auto         bool
	DeleteBranch bool
}

type UpdateBranchMsg struct {
	Name      string
	IsCreated *bool
	NewPr     *data.PullRequestData
}

func buildTaskId(prefix string, prNumber int) string {
	return fmt.Sprintf("%s_%d", prefix, prNumber)
}

type GitHubTask struct {
	Id           string
	Args         []string
	Section      SectionIdentifier
	StartText    string
	FinishedText string
	Msg          func(c *exec.Cmd, err error) tea.Msg
}

func fireTask(ctx *context.ProgramContext, task GitHubTask) tea.Cmd {
	start := context.Task{
		Id:           task.Id,
		StartText:    task.StartText,
		FinishedText: task.FinishedText,
		State:        context.TaskStart,
		Error:        nil,
	}

	startCmd := ctx.StartTask(start)
	return tea.Batch(startCmd, func() tea.Msg {
		log.Info("Running task", "cmd", "gh "+strings.Join(task.Args, " "))
		c := exec.Command("gh", task.Args...)

		err := c.Run()
		return constants.TaskFinishedMsg{
			TaskId:      task.Id,
			SectionId:   task.Section.Id,
			SectionType: task.Section.Type,
			Err:         err,
			Msg:         task.Msg(c, err),
		}
	})
}

func OpenBranchPR(ctx *context.ProgramContext, section SectionIdentifier, branch string) tea.Cmd {
	return fireTask(ctx, GitHubTask{
		Id: fmt.Sprintf("branch_open_%s", branch),
		Args: []string{
			"pr",
			"view",
			"--web",
			branch,
			"-R",
			ctx.RepoUrl,
		},
		Section:      section,
		StartText:    fmt.Sprintf("Opening PR for branch %s", branch),
		FinishedText: fmt.Sprintf("PR for branch %s has been opened", branch),
		Msg: func(c *exec.Cmd, err error) tea.Msg {
			return UpdatePRMsg{}
		},
	})
}

func ReopenPR(ctx *context.ProgramContext, section SectionIdentifier, pr data.RowData) tea.Cmd {
	prNumber := pr.GetNumber()
	return fireTask(ctx, GitHubTask{
		Id: buildTaskId("pr_reopen", prNumber),
		Args: []string{
			"pr",
			"reopen",
			fmt.Sprint(prNumber),
			"-R",
			pr.GetRepoNameWithOwner(),
		},
		Section:      section,
		StartText:    fmt.Sprintf("Reopening PR #%d", prNumber),
		FinishedText: fmt.Sprintf("PR #%d has been reopened", prNumber),
		Msg: func(c *exec.Cmd, err error) tea.Msg {
			return UpdatePRMsg{
				PrNumber: prNumber,
				IsClosed: utils.BoolPtr(false),
			}
		},
	})
}

func ClosePR(ctx *context.ProgramContext, section SectionIdentifier, pr data.RowData) tea.Cmd {
	prNumber := pr.GetNumber()
	return fireTask(ctx, GitHubTask{
		Id: buildTaskId("pr_close", prNumber),
		Args: []string{
			"pr",
			"close",
			fmt.Sprint(prNumber),
			"-R",
			pr.GetRepoNameWithOwner(),
		},
		Section:      section,
		StartText:    fmt.Sprintf("Closing PR #%d", prNumber),
		FinishedText: fmt.Sprintf("PR #%d has been closed", prNumber),
		Msg: func(c *exec.Cmd, err error) tea.Msg {
			return UpdatePRMsg{
				PrNumber: prNumber,
				IsClosed: utils.BoolPtr(true),
			}
		},
	})
}

func PRReady(ctx *context.ProgramContext, section SectionIdentifier, pr data.RowData) tea.Cmd {
	prNumber := pr.GetNumber()
	return fireTask(ctx, GitHubTask{
		Id: buildTaskId("pr_ready", prNumber),
		Args: []string{
			"pr",
			"ready",
			fmt.Sprint(prNumber),
			"-R",
			pr.GetRepoNameWithOwner(),
		},
		Section:      section,
		StartText:    fmt.Sprintf("Marking PR #%d as ready for review", prNumber),
		FinishedText: fmt.Sprintf("PR #%d has been marked as ready for review", prNumber),
		Msg: func(c *exec.Cmd, err error) tea.Msg {
			return UpdatePRMsg{
				PrNumber:       prNumber,
				ReadyForReview: utils.BoolPtr(true),
			}
		},
	})
}

func TogglePRDraft(ctx *context.ProgramContext, section SectionIdentifier, pr DraftablePRData) tea.Cmd {
	return fireTask(ctx, buildTogglePRDraftTask(section, pr))
}

func buildTogglePRDraftTask(section SectionIdentifier, pr DraftablePRData) GitHubTask {
	prNumber := pr.GetNumber()
	isDraft := pr.GetIsDraft()
	args := []string{
		"pr",
		"ready",
	}
	if !isDraft {
		args = append(args, "--undo")
	}
	args = append(
		args,
		fmt.Sprint(prNumber),
		"-R",
		pr.GetRepoNameWithOwner(),
	)

	newIsDraft := !isDraft
	startText := fmt.Sprintf("Marking PR #%d as ready for review", prNumber)
	finishedText := fmt.Sprintf("PR #%d has been marked as ready for review", prNumber)
	if newIsDraft {
		startText = fmt.Sprintf("Converting PR #%d to draft", prNumber)
		finishedText = fmt.Sprintf("PR #%d has been converted to draft", prNumber)
	}

	return GitHubTask{
		Id:           buildTaskId("pr_toggle_draft", prNumber),
		Args:         args,
		Section:      section,
		StartText:    startText,
		FinishedText: finishedText,
		Msg: func(c *exec.Cmd, err error) tea.Msg {
			return UpdatePRMsg{
				PrNumber: prNumber,
				IsDraft:  utils.BoolPtr(newIsDraft),
			}
		},
	}
}

func MergePR(ctx *context.ProgramContext, section SectionIdentifier, pr data.RowData) tea.Cmd {
	return MergePRWithOptions(ctx, section, pr, MergePROptions{Method: MergeMethodMerge})
}

func MergePRWithOptions(
	ctx *context.ProgramContext,
	section SectionIdentifier,
	pr data.RowData,
	options MergePROptions,
) tea.Cmd {
	return fireTask(ctx, buildMergePRTask(section, pr, options))
}

func buildMergePRTask(section SectionIdentifier, pr data.RowData, options MergePROptions) GitHubTask {
	prNumber := pr.GetNumber()
	method := options.Method
	if method == "" {
		method = MergeMethodMerge
	}

	args := []string{"pr", "merge", fmt.Sprint(prNumber), "-R", pr.GetRepoNameWithOwner(), "--" + string(method)}
	if options.Auto {
		args = append(args, "--auto")
	}
	if options.DeleteBranch {
		args = append(args, "--delete-branch")
	}

	return GitHubTask{
		Id:           fmt.Sprintf("merge_%d", prNumber),
		Args:         args,
		Section:      section,
		StartText:    fmt.Sprintf("Merging PR #%d", prNumber),
		FinishedText: fmt.Sprintf("PR #%d has been merged", prNumber),
		Msg: func(c *exec.Cmd, err error) tea.Msg {
			isMerged := err == nil
			return UpdatePRMsg{
				PrNumber: prNumber,
				IsMerged: &isMerged,
			}
		},
	}
}

func CreatePR(
	ctx *context.ProgramContext,
	section SectionIdentifier,
	branchName string,
	title string,
) tea.Cmd {
	c := exec.Command(
		"gh",
		"pr",
		"create",
		"--title",
		title,
		"-R",
		ctx.RepoUrl,
	)

	taskId := fmt.Sprintf("create_pr_%s", title)
	task := context.Task{
		Id:           taskId,
		StartText:    fmt.Sprintf(`Creating PR "%s"`, title),
		FinishedText: fmt.Sprintf(`PR "%s" has been created`, title),
		State:        context.TaskStart,
		Error:        nil,
	}
	startCmd := ctx.StartTask(task)

	return tea.Batch(startCmd, tea.ExecProcess(c, func(err error) tea.Msg {
		isCreated := err == nil && c.ProcessState.ExitCode() == 0

		return constants.TaskFinishedMsg{
			SectionId:   section.Id,
			SectionType: section.Type,
			TaskId:      taskId,
			Err:         nil,
			Msg:         UpdateBranchMsg{Name: branchName, IsCreated: &isCreated},
		}
	}))
}

func UpdatePR(ctx *context.ProgramContext, section SectionIdentifier, pr data.RowData) tea.Cmd {
	prNumber := pr.GetNumber()
	return fireTask(ctx, GitHubTask{
		Id: buildTaskId("pr_update", prNumber),
		Args: []string{
			"pr",
			"update-branch",
			fmt.Sprint(prNumber),
			"-R",
			pr.GetRepoNameWithOwner(),
		},
		Section:      section,
		StartText:    fmt.Sprintf("Updating PR #%d", prNumber),
		FinishedText: fmt.Sprintf("PR #%d has been updated", prNumber),
		Msg: func(c *exec.Cmd, err error) tea.Msg {
			return UpdatePRMsg{
				PrNumber: prNumber,
				IsClosed: utils.BoolPtr(true),
			}
		},
	})
}

func AssignPR(
	ctx *context.ProgramContext,
	section SectionIdentifier,
	pr data.RowData,
	added []string,
	removed []string,
) tea.Cmd {
	return fireTask(ctx, buildAssignPRTask(section, pr, added, removed))
}

func buildAssignPRTask(
	section SectionIdentifier,
	pr data.RowData,
	added []string,
	removed []string,
) GitHubTask {
	prNumber := pr.GetNumber()
	args := []string{
		"pr",
		"edit",
		fmt.Sprint(prNumber),
		"-R",
		pr.GetRepoNameWithOwner(),
	}
	for _, assignee := range added {
		args = append(args, "--add-assignee", assignee)
	}
	for _, assignee := range removed {
		args = append(args, "--remove-assignee", assignee)
	}
	return GitHubTask{
		Id:           buildTaskId("pr_assign", prNumber),
		Args:         args,
		Section:      section,
		StartText:    fmt.Sprintf("Updating assignees for pr #%d", prNumber),
		FinishedText: fmt.Sprintf("Assignees for pr #%d have been updated", prNumber),
		Msg: func(c *exec.Cmd, err error) tea.Msg {
			addedAssignees := data.Assignees{Nodes: []data.Assignee{}}
			for _, assignee := range added {
				addedAssignees.Nodes = append(
					addedAssignees.Nodes,
					data.Assignee{Login: assignee},
				)
			}
			removedAssignees := data.Assignees{Nodes: []data.Assignee{}}
			for _, assignee := range removed {
				removedAssignees.Nodes = append(
					removedAssignees.Nodes,
					data.Assignee{Login: assignee},
				)
			}
			return UpdatePRMsg{
				PrNumber:         prNumber,
				AddedAssignees:   &addedAssignees,
				RemovedAssignees: &removedAssignees,
			}
		},
	}
}

func CommentOnPR(
	ctx *context.ProgramContext,
	section SectionIdentifier,
	pr data.RowData,
	body string,
) tea.Cmd {
	prNumber := pr.GetNumber()
	return fireTask(ctx, GitHubTask{
		Id: buildTaskId("pr_comment", prNumber),
		Args: []string{
			"pr",
			"comment",
			fmt.Sprint(prNumber),
			"-R",
			pr.GetRepoNameWithOwner(),
			"-b",
			body,
		},
		Section:      section,
		StartText:    fmt.Sprintf("Commenting on PR #%d", prNumber),
		FinishedText: fmt.Sprintf("Commented on PR #%d", prNumber),
		Msg: func(c *exec.Cmd, err error) tea.Msg {
			return UpdatePRMsg{
				PrNumber: prNumber,
				NewComment: &data.Comment{
					Author:    struct{ Login string }{Login: ctx.User},
					Body:      body,
					UpdatedAt: time.Now(),
				},
			}
		},
	})
}

func ReplyToReviewThread(
	ctx *context.ProgramContext,
	section SectionIdentifier,
	pr data.RowData,
	threadId string,
	body string,
) tea.Cmd {
	prNumber := pr.GetNumber()
	return fireTask(ctx, GitHubTask{
		Id: buildTaskId("pr_thread_reply", prNumber),
		Args: []string{
			"api",
			"graphql",
			"-f",
			`query=mutation($thread:ID!,$body:String!){addPullRequestReviewThreadReply(input:{pullRequestReviewThreadId:$thread,body:$body}){comment{id}}}`,
			"-F",
			"thread=" + threadId,
			"-f",
			"body=" + body,
		},
		Section:      section,
		StartText:    fmt.Sprintf("Replying to review thread on PR #%d", prNumber),
		FinishedText: fmt.Sprintf("Replied to review thread on PR #%d", prNumber),
		Msg: func(c *exec.Cmd, err error) tea.Msg {
			if err != nil {
				return UpdatePRMsg{PrNumber: prNumber}
			}

			return UpdatePRMsg{
				PrNumber: prNumber,
				ThreadReply: &ReviewThreadReply{
					ThreadId: threadId,
					Comment: data.ReviewComment{
						Author:    struct{ Login string }{Login: ctx.User},
						Body:      body,
						UpdatedAt: time.Now(),
					},
				},
			}
		},
	})
}

func ToggleReviewThreadResolved(
	ctx *context.ProgramContext,
	section SectionIdentifier,
	pr data.RowData,
	threadId string,
	isResolved bool,
) tea.Cmd {
	prNumber := pr.GetNumber()
	mutation := `query=mutation($thread:ID!){resolveReviewThread(input:{threadId:$thread}){thread{id isResolved}}}`
	startText := fmt.Sprintf("Resolving review thread on PR #%d", prNumber)
	finishedText := fmt.Sprintf("Resolved review thread on PR #%d", prNumber)
	if isResolved {
		mutation = `query=mutation($thread:ID!){unresolveReviewThread(input:{threadId:$thread}){thread{id isResolved}}}`
		startText = fmt.Sprintf("Unresolving review thread on PR #%d", prNumber)
		finishedText = fmt.Sprintf("Unresolved review thread on PR #%d", prNumber)
	}

	return fireTask(ctx, GitHubTask{
		Id: buildTaskId("pr_thread_resolve", prNumber),
		Args: []string{
			"api",
			"graphql",
			"-f",
			mutation,
			"-F",
			"thread=" + threadId,
		},
		Section:      section,
		StartText:    startText,
		FinishedText: finishedText,
		Msg: func(c *exec.Cmd, err error) tea.Msg {
			if err != nil {
				return UpdatePRMsg{PrNumber: prNumber}
			}

			return UpdatePRMsg{
				PrNumber: prNumber,
				ThreadResolved: &ReviewThreadResolved{
					ThreadId:   threadId,
					IsResolved: !isResolved,
				},
			}
		},
	})
}

func ApprovePR(
	ctx *context.ProgramContext,
	section SectionIdentifier,
	pr data.RowData,
	comment string,
) tea.Cmd {
	prNumber := pr.GetNumber()
	args := []string{
		"pr",
		"review",
		"-R",
		pr.GetRepoNameWithOwner(),
		fmt.Sprint(prNumber),
		"--approve",
	}
	if comment != "" {
		args = append(args, "--body", comment)
	}
	return fireTask(ctx, GitHubTask{
		Id:           buildTaskId("pr_approve", prNumber),
		Args:         args,
		Section:      section,
		StartText:    fmt.Sprintf("Approving pr #%d", prNumber),
		FinishedText: fmt.Sprintf("pr #%d has been approved", prNumber),
		Msg: func(c *exec.Cmd, err error) tea.Msg {
			return UpdatePRMsg{
				PrNumber: prNumber,
			}
		},
	})
}

func ApproveWorkflows(
	ctx *context.ProgramContext,
	section SectionIdentifier,
	pr data.RowData,
) tea.Cmd {
	prNumber := pr.GetNumber()
	repo := pr.GetRepoNameWithOwner()
	taskId := buildTaskId("pr_approve_workflows", prNumber)

	task := context.Task{
		Id:           taskId,
		StartText:    fmt.Sprintf("Approving workflows for PR #%d", prNumber),
		FinishedText: fmt.Sprintf("Workflows for PR #%d have been approved", prNumber),
		State:        context.TaskStart,
		Error:        nil,
	}
	startCmd := ctx.StartTask(task)

	return tea.Batch(startCmd, func() tea.Msg {
		// Step 1: Get head SHA
		shaCmd := exec.Command("gh", "pr", "view", fmt.Sprint(prNumber),
			"-R", repo, "--json", "headRefOid", "--jq", ".headRefOid")
		shaOut, err := shaCmd.Output()
		if err != nil {
			return constants.TaskFinishedMsg{
				TaskId:      taskId,
				SectionId:   section.Id,
				SectionType: section.Type,
				Err:         fmt.Errorf("failed to get head SHA: %w", err),
				Msg:         UpdatePRMsg{PrNumber: prNumber},
			}
		}
		sha := strings.TrimSpace(string(shaOut))

		// Step 2: Get workflow run IDs awaiting approval
		runsCmd := exec.Command("gh", "api",
			fmt.Sprintf("repos/%s/actions/runs?status=action_required&head_sha=%s", repo, sha),
			"--jq", ".workflow_runs[].id")
		runsOut, err := runsCmd.Output()
		if err != nil {
			return constants.TaskFinishedMsg{
				TaskId:      taskId,
				SectionId:   section.Id,
				SectionType: section.Type,
				Err:         fmt.Errorf("failed to get workflow runs: %w", err),
				Msg:         UpdatePRMsg{PrNumber: prNumber},
			}
		}

		runIds := strings.Fields(strings.TrimSpace(string(runsOut)))
		if len(runIds) == 0 {
			return constants.TaskFinishedMsg{
				TaskId:      taskId,
				SectionId:   section.Id,
				SectionType: section.Type,
				Err:         fmt.Errorf("no workflows awaiting approval"),
				Msg:         UpdatePRMsg{PrNumber: prNumber},
			}
		}

		// Step 3: Approve each run (best-effort)
		var lastErr error
		approved := 0
		for _, runId := range runIds {
			log.Info("Approving workflow run", "runId", runId, "pr", prNumber)
			approveCmd := exec.Command("gh", "api", "-X", "POST",
				fmt.Sprintf("repos/%s/actions/runs/%s/approve", repo, runId))
			output, err := approveCmd.CombinedOutput()
			if err != nil {
				outStr := string(output)
				if strings.Contains(outStr, "not from a fork pull request") {
					lastErr = fmt.Errorf(
						"workflow not approvable via API (only fork PR workflows can be approved)",
					)
				} else {
					lastErr = fmt.Errorf("failed to approve run %s: %w", runId, err)
				}
			} else {
				approved++
			}
		}

		return constants.TaskFinishedMsg{
			TaskId:      taskId,
			SectionId:   section.Id,
			SectionType: section.Type,
			Err:         lastErr,
			Msg:         UpdatePRMsg{PrNumber: prNumber},
		}
	})
}
