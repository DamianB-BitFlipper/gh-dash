package common

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
)

// DiffPR opens a diff view for a PR using the gh CLI.
// The env parameter should be the result of Config.GetFullScreenDiffPagerEnv().
func DiffPR(prNumber int, repoName string, diffPager string, runInBackground bool, env []string) tea.Cmd {
	if runInBackground {
		return openDiffInBackground(prNumber, repoName, diffPager)
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

func openDiffInBackground(prNumber int, repoName string, diffPager string) tea.Cmd {
	return func() tea.Msg {
		if strings.TrimSpace(diffPager) == "" {
			diffPager = "less"
		}

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
				return constants.ErrMsg{Err: fmt.Errorf("%w: %s", err, strings.TrimSpace(ghErr.String()))}
			}
			return constants.ErrMsg{Err: err}
		}

		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "sh"
		}
		pagerCmd := exec.Command(shell, "-c", diffPager)
		pagerCmd.Stdin = bytes.NewReader(diff)

		if err := pagerCmd.Run(); err != nil {
			return constants.ErrMsg{Err: err}
		}

		return nil
	}
}
