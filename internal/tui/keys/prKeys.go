package keys

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	log "charm.land/log/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
)

type PRKeyMap struct {
	PrevSidebarTab       key.Binding
	NextSidebarTab       key.Binding
	Approve              key.Binding
	Assign               key.Binding
	RequestReview        key.Binding
	Label                key.Binding
	Comment              key.Binding
	Diff                 key.Binding
	Create               key.Binding
	CopyBranch           key.Binding
	Checkout             key.Binding
	Close                key.Binding
	SummaryViewMore      key.Binding
	Ready                key.Binding
	Reopen               key.Binding
	Merge                key.Binding
	Update               key.Binding
	ApproveWorkflows     key.Binding
	PrevReviewThread     key.Binding
	NextReviewThread     key.Binding
	ToggleReviewThread   key.Binding
	ToggleSmartFiltering key.Binding
	SortOrder            key.Binding
	ViewIssues           key.Binding
}

var PRKeys = PRKeyMap{
	PrevSidebarTab: key.NewBinding(
		key.WithKeys("ctrl+left"),
		key.WithHelp("Ctrl+←", "previous sidebar tab"),
	),
	NextSidebarTab: key.NewBinding(
		key.WithKeys("ctrl+right"),
		key.WithHelp("Ctrl+→", "next sidebar tab"),
	),
	Approve: key.NewBinding(
		key.WithKeys("v"),
		key.WithHelp("v", "approve"),
	),
	Assign: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "edit assignees"),
	),
	RequestReview: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "request review"),
	),
	Label: key.NewBinding(
		key.WithKeys("L"),
		key.WithHelp("L", "label"),
	),
	Comment: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "comment"),
	),
	Diff: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "diff"),
	),
	Create: key.NewBinding(
		key.WithKeys("N"),
		key.WithHelp("N", "create PR"),
	),
	CopyBranch: key.NewBinding(
		key.WithKeys("b"),
		key.WithHelp("b", "copy branch"),
	),
	Checkout: key.NewBinding(
		key.WithKeys("C"),
		key.WithHelp("C", "checkout"),
	),
	Close: key.NewBinding(
		key.WithKeys("x"),
		key.WithHelp("x", "close"),
	),
	SummaryViewMore: key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "expand description"),
	),
	Reopen: key.NewBinding(
		key.WithKeys("X"),
		key.WithHelp("X", "reopen"),
	),
	Ready: key.NewBinding(
		key.WithKeys("W"),
		key.WithHelp("W", "toggle draft/ready"),
	),
	Merge: key.NewBinding(
		key.WithKeys("m"),
		key.WithHelp("m", "merge"),
	),
	Update: key.NewBinding(
		key.WithKeys("u"),
		key.WithHelp("u", "update pr from base branch"),
	),
	ApproveWorkflows: key.NewBinding(
		key.WithKeys("V"),
		key.WithHelp("V", "approve all workflows"),
	),
	PrevReviewThread: key.NewBinding(
		key.WithKeys("["),
		key.WithHelp("[", "previous review thread"),
	),
	NextReviewThread: key.NewBinding(
		key.WithKeys("]"),
		key.WithHelp("]", "next review thread"),
	),
	ToggleReviewThread: key.NewBinding(
		key.WithKeys("z"),
		key.WithHelp("z", "resolve/unresolve thread"),
	),
	ToggleSmartFiltering: key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "toggle smart filtering"),
	),
	SortOrder: key.NewBinding(
		key.WithKeys("S"),
		key.WithHelp("S", "sort order"),
	),
	ViewIssues: key.NewBinding(
		key.WithHelp("", "switch to issues"),
	),
}

func PRFullHelp() []key.Binding {
	return enabledBindings(
		PRKeys.CopyBranch,
		PRKeys.PrevSidebarTab,
		PRKeys.NextSidebarTab,
		PRKeys.Approve,
		PRKeys.Assign,
		PRKeys.RequestReview,
		PRKeys.Label,
		PRKeys.Comment,
		PRKeys.Diff,
		PRKeys.Create,
		PRKeys.Checkout,
		PRKeys.Close,
		PRKeys.Ready,
		PRKeys.Reopen,
		PRKeys.Merge,
		PRKeys.Update,
		PRKeys.ApproveWorkflows,
		PRKeys.PrevReviewThread,
		PRKeys.NextReviewThread,
		PRKeys.ToggleReviewThread,
		PRKeys.ToggleSmartFiltering,
		PRKeys.SortOrder,
		PRKeys.ViewIssues,
	)
}

func rebindPRKeys(keys []config.Keybinding) error {
	CustomPRBindings = []key.Binding{}

	for _, prKey := range keys {
		if prKey.Builtin == "" {
			// Handle custom commands
			if prKey.Command != "" {
				name := prKey.Name
				if prKey.Name == "" {
					name = config.TruncateCommand(prKey.Command)
				}

				customBinding := key.NewBinding(
					key.WithKeys(prKey.Key),
					key.WithHelp(prKey.Key, name),
				)

				CustomPRBindings = append(CustomPRBindings, customBinding)
			}
			continue
		}

		log.Debug("Rebinding PR key", "builtin", prKey.Builtin, "key", prKey.Key)

		var key *key.Binding

		switch prKey.Builtin {
		case "prevSidebarTab":
			key = &PRKeys.PrevSidebarTab
		case "nextSidebarTab":
			key = &PRKeys.NextSidebarTab
		case "approve":
			key = &PRKeys.Approve
		case "assign":
			key = &PRKeys.Assign
		case "requestReview":
			key = &PRKeys.RequestReview
		case "label":
			key = &PRKeys.Label
		case "comment":
			key = &PRKeys.Comment
		case "diff":
			key = &PRKeys.Diff
		case "createPr":
			key = &PRKeys.Create
		case "copyBranch":
			key = &PRKeys.CopyBranch
		case "checkout":
			key = &PRKeys.Checkout
		case "close":
			key = &PRKeys.Close
		case "ready":
			key = &PRKeys.Ready
		case "reopen":
			key = &PRKeys.Reopen
		case "merge":
			key = &PRKeys.Merge
		case "update":
			key = &PRKeys.Update
		case "approveWorkflows":
			key = &PRKeys.ApproveWorkflows
		case "prevReviewThread":
			key = &PRKeys.PrevReviewThread
		case "nextReviewThread":
			key = &PRKeys.NextReviewThread
		case "toggleReviewThread":
			key = &PRKeys.ToggleReviewThread
		case "sortOrder":
			key = &PRKeys.SortOrder
		case "viewIssues":
			key = &PRKeys.ViewIssues
		case "summaryViewMore":
			key = &PRKeys.SummaryViewMore
		default:
			return fmt.Errorf("unknown built-in pr key: '%s'", prKey.Builtin)
		}

		key.SetKeys(prKey.Key)

		helpDesc := key.Help().Desc
		if prKey.Name != "" {
			helpDesc = prKey.Name
		}
		key.SetHelp(prKey.Key, helpDesc)
	}

	return nil
}
