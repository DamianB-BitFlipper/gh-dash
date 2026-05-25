package keys

import (
	"charm.land/bubbles/v2/key"
	log "charm.land/log/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
)

type ActionsKeyMap struct {
	ToggleSmartFiltering key.Binding
	SortOrder            key.Binding
}

var ActionsKeys = ActionsKeyMap{
	ToggleSmartFiltering: key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "toggle smart filtering"),
	),
	SortOrder: key.NewBinding(
		key.WithKeys("S"),
		key.WithHelp("S", "sort order"),
	),
}

func ActionsFullHelp() []key.Binding {
	return enabledBindings(
		ActionsKeys.ToggleSmartFiltering,
		ActionsKeys.SortOrder,
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
		default:
			continue
		}
	}

	return nil
}
