package data

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type ActionsWorkflowRunsResponse struct {
	TotalCount   int           `json:"total_count"`
	WorkflowRuns []WorkflowRun `json:"workflow_runs"`
	PageInfo     PageInfo
}

func FetchActionsWorkflowRuns(filters string, limit int, _ *PageInfo) (ActionsWorkflowRunsResponse, error) {
	repo, params := parseActionsWorkflowRunFilters(filters)
	if repo == "" {
		return ActionsWorkflowRunsResponse{}, fmt.Errorf("actions sections require a repo:<owner>/<name> filter")
	}

	if limit <= 0 {
		limit = 20
	}
	params.Set("per_page", strconv.Itoa(limit))

	client, err := getRESTClient()
	if err != nil {
		return ActionsWorkflowRunsResponse{}, err
	}

	path := fmt.Sprintf("repos/%s/actions/runs?%s", repo, params.Encode())
	var response ActionsWorkflowRunsResponse
	if err := client.Get(path, &response); err != nil {
		return response, err
	}
	response.PageInfo = PageInfo{HasNextPage: false}
	return response, nil
}

func parseActionsWorkflowRunFilters(filters string) (string, url.Values) {
	params := url.Values{}
	var repo string
	for _, token := range strings.Fields(filters) {
		key, value, ok := strings.Cut(token, ":")
		if !ok || value == "" {
			continue
		}
		switch key {
		case "repo":
			repo = value
		case "branch":
			params.Set("branch", value)
		case "event":
			params.Set("event", value)
		case "actor":
			if value != "@me" {
				params.Set("actor", value)
			}
		case "status", "is":
			params.Set("status", normalizeActionsStatus(value))
		case "workflow":
			params.Set("workflow", value)
		}
	}
	return repo, params
}

func normalizeActionsStatus(status string) string {
	switch status {
	case "failure", "failed":
		return "failure"
	case "cancelled", "canceled":
		return "cancelled"
	case "in-progress":
		return "in_progress"
	default:
		return status
	}
}
