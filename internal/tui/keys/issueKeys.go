package keys

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	log "charm.land/log/v2"

	"github.com/dlvhdr/gh-dehub/v4/internal/config"
)

type IssueKeyMap struct {
	Label                key.Binding
	Assign               key.Binding
	Unassign             key.Binding
	Comment              key.Binding
	Checkout             key.Binding
	Close                key.Binding
	Reopen               key.Binding
	ToggleSmartFiltering key.Binding
	SortOrder            key.Binding
	ViewPRs              key.Binding
}

var IssueKeys = IssueKeyMap{
	Label: key.NewBinding(
		key.WithKeys("L"),
		key.WithHelp("L", "label"),
	),
	Assign: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "assign"),
	),
	Unassign: key.NewBinding(
		key.WithKeys("A"),
		key.WithHelp("A", "unassign"),
	),
	Comment: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "comment"),
	),
	Checkout: key.NewBinding(
		key.WithKeys("C"),
		key.WithHelp("C", "checkout"),
	),
	Close: key.NewBinding(
		key.WithHelp("", "close"),
	),
	Reopen: key.NewBinding(
		key.WithKeys("X"),
		key.WithHelp("X", "toggle open/close"),
	),
	ToggleSmartFiltering: key.NewBinding(
		key.WithHelp("", "toggle smart filtering"),
	),
	SortOrder: key.NewBinding(
		key.WithKeys("S"),
		key.WithHelp("S", "sort order"),
	),
	ViewPRs: key.NewBinding(
		key.WithHelp("", "switch to notifications"),
	),
}

func IssueFullHelp() []key.Binding {
	return enabledBindings(
		IssueKeys.Label,
		IssueKeys.Assign,
		IssueKeys.Unassign,
		IssueKeys.Comment,
		IssueKeys.Checkout,
		IssueKeys.Close,
		IssueKeys.Reopen,
		IssueKeys.ToggleSmartFiltering,
		IssueKeys.SortOrder,
		IssueKeys.ViewPRs,
	)
}

func rebindIssueKeys(keys []config.Keybinding) error {
	CustomIssueBindings = []key.Binding{}

	for _, issueKey := range keys {
		if issueKey.Builtin == "" {
			// Handle custom commands
			if issueKey.Command != "" {
				name := issueKey.Name
				if issueKey.Name == "" {
					name = config.TruncateCommand(issueKey.Command)
				}

				customBinding := key.NewBinding(
					key.WithKeys(issueKey.Key),
					key.WithHelp(issueKey.Key, name),
				)

				CustomIssueBindings = append(CustomIssueBindings, customBinding)
			}
			continue
		}

		log.Debug("Rebinding issue key", "builtin", issueKey.Builtin, "key", issueKey.Key)

		var key *key.Binding

		switch issueKey.Builtin {
		case "label":
			key = &IssueKeys.Label
		case "assign":
			key = &IssueKeys.Assign
		case "unassign":
			key = &IssueKeys.Unassign
		case "comment":
			key = &IssueKeys.Comment
		case "checkout":
			key = &IssueKeys.Checkout
		case "close":
			key = &IssueKeys.Close
		case "reopen":
			key = &IssueKeys.Reopen
		case "sortOrder":
			key = &IssueKeys.SortOrder
		case "viewPrs":
			key = &IssueKeys.ViewPRs
		default:
			return fmt.Errorf("unknown built-in issue key: '%s'", issueKey.Builtin)
		}

		key.SetKeys(issueKey.Key)

		helpDesc := key.Help().Desc
		if issueKey.Name != "" {
			helpDesc = issueKey.Name
		}
		key.SetHelp(issueKey.Key, helpDesc)
	}

	return nil
}
