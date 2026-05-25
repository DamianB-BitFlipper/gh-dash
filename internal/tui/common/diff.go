package common

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/diffviewer"
)

// DiffPR opens a diff view for a PR using the gh CLI.
// The env parameter should be the result of Config.GetFullScreenDiffPagerEnv().
func DiffPR(prNumber int, repoName string, prURL string, diffPager string, runInBackground bool, env []string) tea.Cmd {
	if runInBackground || diffviewer.IsBuiltInPager(diffPager) {
		return openDiffInBackground(prNumber, repoName, prURL, diffPager)
	}

	c := exec.Command(
		"gh",
		"pr",
		"diff",
		fmt.Sprint(prNumber),
		"-R",
		repoName,
	)
	c.Env = env

	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return constants.ErrMsg{Err: err}
		}
		return nil
	})
}

func openDiffInBackground(prNumber int, repoName string, prURL string, diffPager string) tea.Cmd {
	return func() tea.Msg {
		if strings.TrimSpace(diffPager) == "" {
			diffPager = "less"
		}

		diff, err := fetchPRDiff(prNumber, repoName)
		if err != nil {
			return constants.ErrMsg{Err: err}
		}

		if diffviewer.IsBuiltInPager(diffPager) {
			if err := diffviewer.Open(context.Background(), diffviewer.Options{Diff: diff, SourceURL: prURL}); err != nil {
				return constants.ErrMsg{Err: err}
			}
			return nil
		}

		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "sh"
		}
		pagerCmd := exec.Command(shell, "-c", diffPager)
		pagerCmd.Stdin = bytes.NewReader(diff)
		pagerCmd.Env = append(os.Environ(),
			fmt.Sprintf("GH_DASH_PR_NUMBER=%d", prNumber),
			fmt.Sprintf("GH_DASH_PR_REPO=%s", repoName),
			fmt.Sprintf("GH_DASH_PR_URL=%s", prURL),
		)

		if err := pagerCmd.Run(); err != nil {
			return constants.ErrMsg{Err: err}
		}

		return nil
	}
}

func fetchPRDiff(prNumber int, repoName string) ([]byte, error) {
	ghCmd := exec.Command(
		"gh",
		"pr",
		"diff",
		fmt.Sprint(prNumber),
		"-R",
		repoName,
	)

	var ghErr bytes.Buffer
	ghCmd.Stderr = &ghErr

	diff, err := ghCmd.Output()
	if err != nil {
		if ghErr.Len() > 0 {
			return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(ghErr.String()))
		}
		return nil, err
	}

	return diff, nil
}
