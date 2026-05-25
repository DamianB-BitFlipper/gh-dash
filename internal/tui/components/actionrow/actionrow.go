package actionrow

import (
	"strings"

	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/table"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dash/v4/internal/utils"
)

func Build(run data.WorkflowRun) table.Row {
	return table.Row{
		StatusIcon(run),
		run.GetRepoNameWithOwner(),
		run.Name,
		run.GetTitle(),
		run.HeadBranch,
		run.Event,
		run.Actor.Login,
		utils.TimeElapsed(run.UpdatedAt),
		utils.TimeElapsed(run.CreatedAt),
	}
}

func StatusIcon(run data.WorkflowRun) string {
	switch strings.ToLower(run.Conclusion) {
	case "success":
		return constants.SuccessIcon
	case "failure", "startup_failure", "timed_out", "action_required", "cancelled":
		return constants.FailureIcon
	case "skipped":
		return constants.SkippedIcon
	case "neutral":
		return constants.NeutralIcon
	}

	switch strings.ToLower(run.Status) {
	case "in_progress", "queued", "requested", "waiting", "pending":
		return constants.WaitingIcon
	}

	return constants.OpenCircleIcon
}
