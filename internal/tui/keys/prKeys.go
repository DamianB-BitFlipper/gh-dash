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
	PrevStep             key.Binding
	NextStep             key.Binding
	ToggleReviewThread   key.Binding
	ToggleSmartFiltering key.Binding
	SortOrder            key.Binding
	ViewIssues           key.Binding
}

var PRKeys = PRKeyMap{
	PrevSidebarTab: key.NewBinding(
		key.WithKeys("left"),
		key.WithHelp("←", "previous sidebar tab"),
	),
	NextSidebarTab: key.NewBinding(
		key.WithKeys("right"),
		key.WithHelp("→", "next sidebar tab"),
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
		key.WithHelp("", "close"),
	),
	SummaryViewMore: key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "expand description"),
	),
	Reopen: key.NewBinding(
		key.WithKeys("X"),
		key.WithHelp("X", "toggle open/close"),
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
		key.WithHelp("u", "edit PR"),
	),
	ApproveWorkflows: key.NewBinding(
		key.WithKeys("V"),
		key.WithHelp("V", "approve all workflows"),
	),
	PrevReviewThread: key.NewBinding(
		key.WithKeys(","),
		key.WithHelp(",", "previous review thread"),
	),
	NextReviewThread: key.NewBinding(
		key.WithKeys("."),
		key.WithHelp(".", "next review thread"),
	),
	PrevStep: key.NewBinding(
		key.WithKeys("ctrl+,"),
		key.WithHelp("ctrl+,", "previous step"),
	),
	NextStep: key.NewBinding(
		key.WithKeys("ctrl+."),
		key.WithHelp("ctrl+.", "next step"),
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
	previewBindings := []key.Binding{}
	switch prPreviewContext {
	case PRPreviewContextActivity:
		previewBindings = append(
			previewBindings,
			PRKeys.PrevReviewThread,
			PRKeys.NextReviewThread,
			key.NewBinding(
				key.WithKeys(PRKeys.SummaryViewMore.Keys()...),
				key.WithHelp(PRKeys.SummaryViewMore.Help().Key, "expand/collapse snippets"),
			),
		)
	case PRPreviewContextChecks:
		previewBindings = append(
			previewBindings,
			key.NewBinding(
				key.WithKeys(PRKeys.PrevReviewThread.Keys()...),
				key.WithHelp(PRKeys.PrevReviewThread.Help().Key, "previous check"),
			),
			key.NewBinding(
				key.WithKeys(PRKeys.NextReviewThread.Keys()...),
				key.WithHelp(PRKeys.NextReviewThread.Help().Key, "next check"),
			),
			key.NewBinding(
				key.WithKeys(PRKeys.PrevStep.Keys()...),
				key.WithHelp(PRKeys.PrevStep.Help().Key, "previous step"),
			),
			key.NewBinding(
				key.WithKeys(PRKeys.NextStep.Keys()...),
				key.WithHelp(PRKeys.NextStep.Help().Key, "next step"),
			),
		)
	}

	bindings := enabledBindings(
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
	)
	bindings = append(bindings, previewBindings...)
	bindings = append(bindings, enabledBindings(
		PRKeys.ToggleReviewThread,
		PRKeys.ToggleSmartFiltering,
		PRKeys.SortOrder,
		PRKeys.ViewIssues,
	)...)
	return bindings
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
		case "prevStep":
			key = &PRKeys.PrevStep
		case "nextStep":
			key = &PRKeys.NextStep
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
