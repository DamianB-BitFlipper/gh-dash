package actionrow

import (
	"testing"
	"time"

	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
)

func TestStatusIcon(t *testing.T) {
	tests := []struct {
		name string
		run  data.WorkflowRun
		want string
	}{
		{name: "success", run: data.WorkflowRun{Conclusion: "success"}, want: constants.SuccessIcon},
		{name: "failure", run: data.WorkflowRun{Conclusion: "failure"}, want: constants.FailureIcon},
		{name: "cancelled", run: data.WorkflowRun{Conclusion: "cancelled"}, want: constants.FailureIcon},
		{name: "skipped", run: data.WorkflowRun{Conclusion: "skipped"}, want: constants.SkippedIcon},
		{name: "neutral", run: data.WorkflowRun{Conclusion: "neutral"}, want: constants.NeutralIcon},
		{name: "pending", run: data.WorkflowRun{Status: "in_progress"}, want: constants.WaitingIcon},
		{name: "unknown", run: data.WorkflowRun{}, want: constants.OpenCircleIcon},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StatusIcon(tt.run); got != tt.want {
				t.Fatalf("StatusIcon() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuild(t *testing.T) {
	run := data.WorkflowRun{
		Name:         "CI",
		DisplayTitle: "Run tests",
		HeadBranch:   "main",
		Event:        "push",
		Status:       "in_progress",
		UpdatedAt:    time.Now().Add(-time.Minute),
		CreatedAt:    time.Now().Add(-2 * time.Minute),
	}
	run.Repository.FullName = "owner/repo"
	run.Actor.Login = "octocat"

	row := Build(run)
	if len(row) != 9 {
		t.Fatalf("len(row) = %d, want 9", len(row))
	}
	if row[0] != constants.WaitingIcon || row[1] != "owner/repo" || row[2] != "CI" || row[3] != "Run tests" {
		t.Fatalf("unexpected row: %#v", row)
	}
}
