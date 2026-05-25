package tasks

import (
	"fmt"
	"os/exec"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
)

func TestApproveWorkflows_TaskConfiguration(t *testing.T) {
	var capturedTask context.Task

	ctx := &context.ProgramContext{
		StartTask: func(task context.Task) tea.Cmd {
			capturedTask = task
			return nil
		},
	}
	section := SectionIdentifier{Id: 2, Type: "pr"}
	pr := mockIssue{
		number:   42,
		repoName: "owner/repo",
	}

	_ = ApproveWorkflows(ctx, section, pr)

	require.Equal(t, "pr_approve_workflows_42", capturedTask.Id)
	require.Equal(t, "Approving workflows for PR #42", capturedTask.StartText)
	require.Equal(t, "Workflows for PR #42 have been approved", capturedTask.FinishedText)
	require.Equal(t, context.TaskStart, capturedTask.State)
	require.Nil(t, capturedTask.Error)
}

func TestAssignPR_TaskConfiguration(t *testing.T) {
	section := SectionIdentifier{Id: 2, Type: "pr"}
	pr := mockIssue{
		number:   42,
		repoName: "owner/repo",
	}

	task := buildAssignPRTask(section, pr, []string{"alice"}, []string{"bob"})

	require.Equal(t, "pr_assign_42", task.Id)
	require.Equal(t, []string{
		"pr", "edit", "42", "-R", "owner/repo",
		"--add-assignee", "alice",
		"--remove-assignee", "bob",
	}, task.Args)
	require.Equal(t, section, task.Section)
	require.Equal(t, "Updating assignees for pr #42", task.StartText)
	require.Equal(t, "Assignees for pr #42 have been updated", task.FinishedText)
}

func TestAssignPR_UpdateMessage(t *testing.T) {
	pr := mockIssue{number: 42, repoName: "owner/repo"}
	task := buildAssignPRTask(SectionIdentifier{Id: 2, Type: "pr"}, pr, []string{"alice"}, []string{"bob"})
	msg := task.Msg(&exec.Cmd{}, nil).(UpdatePRMsg)

	require.Equal(t, 42, msg.PrNumber)
	require.Equal(t, "alice", msg.AddedAssignees.Nodes[0].Login)
	require.Equal(t, "bob", msg.RemovedAssignees.Nodes[0].Login)
}

func TestRequestReviewPR_TaskConfiguration(t *testing.T) {
	section := SectionIdentifier{Id: 2, Type: "pr"}
	pr := mockIssue{
		number:   42,
		repoName: "owner/repo",
	}

	task := buildRequestReviewPRTask(section, pr, []string{"alice"}, []string{"bob"})

	require.Equal(t, "pr_request_review_42", task.Id)
	require.Equal(t, []string{
		"pr", "edit", "42", "-R", "owner/repo",
		"--add-reviewer", "alice",
		"--remove-reviewer", "bob",
	}, task.Args)
	require.Equal(t, section, task.Section)
	require.Equal(t, "Updating review requests for pr #42", task.StartText)
	require.Equal(t, "Review requests for pr #42 have been updated", task.FinishedText)
}

func TestRequestReviewPR_UpdateMessage(t *testing.T) {
	pr := mockIssue{number: 42, repoName: "owner/repo"}
	task := buildRequestReviewPRTask(SectionIdentifier{Id: 2, Type: "pr"}, pr, []string{"alice"}, []string{"bob"})
	msg := task.Msg(&exec.Cmd{}, nil).(UpdatePRMsg)

	require.Equal(t, 42, msg.PrNumber)
	require.Equal(t, "alice", msg.AddedReviewers.Nodes[0].GetReviewerDisplayName())
	require.Equal(t, "bob", msg.RemovedReviewers.Nodes[0].GetReviewerDisplayName())
}

func TestTogglePRDraft_TaskConfiguration(t *testing.T) {
	tests := []struct {
		name         string
		isDraft      bool
		expectedArgs []string
		startText    string
		finishedText string
		isDraftMsg   bool
	}{
		{
			name:         "draft PR is marked ready",
			isDraft:      true,
			expectedArgs: []string{"pr", "ready", "42", "-R", "owner/repo"},
			startText:    "Marking PR #42 as ready for review",
			finishedText: "PR #42 has been marked as ready for review",
			isDraftMsg:   false,
		},
		{
			name:         "ready PR is converted to draft",
			isDraft:      false,
			expectedArgs: []string{"pr", "ready", "--undo", "42", "-R", "owner/repo"},
			startText:    "Converting PR #42 to draft",
			finishedText: "PR #42 has been converted to draft",
			isDraftMsg:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := mockIssue{number: 42, repoName: "owner/repo", isDraft: tt.isDraft}

			task := buildTogglePRDraftTask(SectionIdentifier{Id: 2, Type: "pr"}, pr)
			msg := task.Msg(&exec.Cmd{}, nil).(UpdatePRMsg)

			require.Equal(t, "pr_toggle_draft_42", task.Id)
			require.Equal(t, tt.expectedArgs, task.Args)
			require.Equal(t, tt.startText, task.StartText)
			require.Equal(t, tt.finishedText, task.FinishedText)
			require.NotNil(t, msg.IsDraft)
			require.Equal(t, tt.isDraftMsg, *msg.IsDraft)
		})
	}
}

func TestMergePRTaskConfiguration(t *testing.T) {
	section := SectionIdentifier{Id: 2, Type: "pr"}
	pr := mockIssue{number: 42, repoName: "owner/repo"}

	task := buildMergePRTask(section, pr, MergePROptions{
		Method:       MergeMethodSquash,
		Auto:         true,
		DeleteBranch: true,
	})
	msg := task.Msg(&exec.Cmd{}, nil).(UpdatePRMsg)

	require.Equal(t, "merge_42", task.Id)
	require.Equal(t, []string{
		"pr", "merge", "42", "-R", "owner/repo", "--squash", "--auto", "--delete-branch",
	}, task.Args)
	require.Equal(t, section, task.Section)
	require.Equal(t, "Merging PR #42", task.StartText)
	require.Equal(t, "PR #42 has been merged", task.FinishedText)
	require.Equal(t, 42, msg.PrNumber)
	require.NotNil(t, msg.IsMerged)
	require.True(t, *msg.IsMerged)
}

func TestMergePRTaskDefaultsToMergeMethod(t *testing.T) {
	pr := mockIssue{number: 42, repoName: "owner/repo"}

	task := buildMergePRTask(SectionIdentifier{}, pr, MergePROptions{})

	require.Equal(t, []string{"pr", "merge", "42", "-R", "owner/repo", "--merge"}, task.Args)
}

func TestApproveWorkflows_ReturnsNonNilCommand(t *testing.T) {
	tests := []struct {
		name     string
		prNumber int
		repoName string
	}{
		{
			name:     "standard PR",
			prNumber: 123,
			repoName: "owner/repo",
		},
		{
			name:     "large PR number",
			prNumber: 99999,
			repoName: "my-org/my-project",
		},
		{
			name:     "hyphenated repo name",
			prNumber: 1,
			repoName: "some-owner/some-repo-name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &context.ProgramContext{
				StartTask: noopStartTask,
			}
			section := SectionIdentifier{Id: 1, Type: "pr"}
			pr := mockIssue{
				number:   tt.prNumber,
				repoName: tt.repoName,
			}

			cmd := ApproveWorkflows(ctx, section, pr)

			require.NotNil(t, cmd, "ApproveWorkflows should return a non-nil command")
		})
	}
}

func TestApproveWorkflows_UsesCorrectPRNumber(t *testing.T) {
	prNumbers := []int{1, 100, 12345, 999999}

	for _, num := range prNumbers {
		t.Run(fmt.Sprintf("pr_%d", num), func(t *testing.T) {
			var capturedTask context.Task
			ctx := &context.ProgramContext{
				StartTask: func(task context.Task) tea.Cmd {
					capturedTask = task
					return nil
				},
			}
			pr := mockIssue{number: num, repoName: "o/r"}

			ApproveWorkflows(ctx, SectionIdentifier{}, pr)

			expectedId := fmt.Sprintf("pr_approve_workflows_%d", num)
			require.Equal(t, expectedId, capturedTask.Id)
			require.Contains(t, capturedTask.StartText, fmt.Sprintf("#%d", num))
			require.Contains(t, capturedTask.FinishedText, fmt.Sprintf("#%d", num))
		})
	}
}

func TestApproveWorkflows_SectionIdentifierPropagation(t *testing.T) {
	tests := []struct {
		name        string
		sectionId   int
		sectionType string
	}{
		{
			name:        "pr section type",
			sectionId:   1,
			sectionType: "pr",
		},
		{
			name:        "notification section type",
			sectionId:   10,
			sectionType: "notification",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &context.ProgramContext{
				StartTask: noopStartTask,
			}
			section := SectionIdentifier{Id: tt.sectionId, Type: tt.sectionType}
			pr := mockIssue{number: 1, repoName: "o/r"}

			cmd := ApproveWorkflows(ctx, section, pr)

			require.NotNil(t, cmd)
		})
	}
}
