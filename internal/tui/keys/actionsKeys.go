package keys

import (
	"charm.land/bubbles/v2/key"
	log "charm.land/log/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
)

type ActionsKeyMap struct {
	ToggleSmartFiltering key.Binding
	SortOrder            key.Binding
	Rerun                key.Binding
	RerunFailed          key.Binding
	Cancel               key.Binding
	FocusNextPane        key.Binding
	FocusPrevPane        key.Binding
}

var ActionsKeys = ActionsKeyMap{
	ToggleSmartFiltering: key.NewBinding(
		key.WithHelp("", "toggle smart filtering"),
	),
	SortOrder: key.NewBinding(
		key.WithKeys("S"),
		key.WithHelp("S", "sort order"),
	),
	Rerun: key.NewBinding(
		key.WithKeys("ctrl+r"),
		key.WithHelp("ctrl+r", "rerun workflow"),
	),
	RerunFailed: key.NewBinding(
		key.WithKeys("ctrl+f"),
		key.WithHelp("ctrl+f", "rerun failed jobs"),
	),
	Cancel: key.NewBinding(
		key.WithKeys("x"),
		key.WithHelp("x", "cancel workflow"),
	),
	FocusNextPane: key.NewBinding(
		key.WithKeys("ctrl+right"),
		key.WithHelp("Ctrl+→", "focus next pane"),
	),
	FocusPrevPane: key.NewBinding(
		key.WithKeys("ctrl+left"),
		key.WithHelp("Ctrl+←", "focus prev pane"),
	),
}

func ActionsFullHelp() []key.Binding {
	return enabledBindings(
		ActionsKeys.ToggleSmartFiltering,
		ActionsKeys.SortOrder,
		ActionsKeys.Rerun,
		ActionsKeys.RerunFailed,
		ActionsKeys.Cancel,
		ActionsKeys.FocusNextPane,
		ActionsKeys.FocusPrevPane,
	)
}

func RebindActionsKeys(keys []config.Keybinding) error {
	log.Debug("Rebinding actions keys", "keys", keys)
	CustomActionBindings = []key.Binding{}

	for _, actionsKey := range keys {
		if actionsKey.Builtin == "" {
			if actionsKey.Command != "" {
				name := actionsKey.Name
				if actionsKey.Name == "" {
					name = config.TruncateCommand(actionsKey.Command)
				}

				CustomActionBindings = append(CustomActionBindings, key.NewBinding(
					key.WithKeys(actionsKey.Key),
					key.WithHelp(actionsKey.Key, name),
				))
			}
			continue
		}

		switch actionsKey.Builtin {
		case "toggleSmartFiltering":
			ActionsKeys.ToggleSmartFiltering = actionsKey.NewBinding(&ActionsKeys.ToggleSmartFiltering)
		case "sortOrder":
			ActionsKeys.SortOrder = actionsKey.NewBinding(&ActionsKeys.SortOrder)
		case "rerun":
			ActionsKeys.Rerun = actionsKey.NewBinding(&ActionsKeys.Rerun)
		case "rerunFailed":
			ActionsKeys.RerunFailed = actionsKey.NewBinding(&ActionsKeys.RerunFailed)
		case "cancel":
			ActionsKeys.Cancel = actionsKey.NewBinding(&ActionsKeys.Cancel)
		case "focusNextPane":
			ActionsKeys.FocusNextPane = actionsKey.NewBinding(&ActionsKeys.FocusNextPane)
		case "focusPrevPane":
			ActionsKeys.FocusPrevPane = actionsKey.NewBinding(&ActionsKeys.FocusPrevPane)
		default:
			continue
		}
	}

	return nil
}
