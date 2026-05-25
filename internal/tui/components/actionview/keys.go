package actionview

import (
	"charm.land/bubbles/v2/key"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
)

var (
	openUrlKey = key.NewBinding(
		key.WithKeys("o"),
		key.WithHelp("o", "open in browser"),
	)

	openPRKey = key.NewBinding(
		key.WithKeys("O"),
		key.WithHelp("O", "open PR"),
	)

	quitKey = key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "quit"),
	)

	nextRowKey = key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp("j/↓", "next row"),
	)

	prevRowKey = key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp("k/↑", "previous row"),
	)

	zoomPaneKey = key.NewBinding(
		key.WithKeys("z"),
		key.WithHelp("z", "zoom pane"),
	)

	nextPaneKey = key.NewBinding(
		key.WithKeys("l"),
		key.WithHelp("l", "next pane"),
	)

	prevPaneKey = key.NewBinding(
		key.WithKeys("h"),
		key.WithHelp("h", "previous pane"),
	)

	gotoTopKey = key.NewBinding(
		key.WithKeys("g"),
		key.WithHelp("g", "go to top"),
	)

	gotoBottomKey = key.NewBinding(
		key.WithKeys("shift+g", "G"),
		key.WithHelp("G", "go to bottom"),
	)

	rightKey = key.NewBinding(
		key.WithKeys("right"),
		key.WithHelp("→", "move right"),
	)

	leftKey = key.NewBinding(
		key.WithKeys("left"),
		key.WithHelp("←", "move left"),
	)

	searchKey = key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "search in pane"),
	)

	modeKey = key.NewBinding(
		key.WithKeys("m"),
		key.WithHelp("m", "switch display mode"),
	)

	cancelSearchKey = key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel search"),
	)

	applySearchKey = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "apply search"),
	)

	nextSearchMatchKey = key.NewBinding(
		key.WithKeys("n", "ctrl+n"),
		key.WithHelp("ctrl+n", "next match"),
	)

	prevSearchMatchKey = key.NewBinding(
		key.WithKeys("N", "ctrl+p"),
		key.WithHelp("ctrl+p", "prev match"),
	)

	refreshAllKey = key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("R", "refresh all"),
	)

	rerunKey = key.NewBinding(
		key.WithKeys("ctrl+r"),
		key.WithHelp("ctrl+r", "rerun"),
	)
)

func RebindActionsKeybindings(bindings []config.Keybinding) error {
	for _, kb := range bindings {
		if kb.Builtin == "" {
			continue
		}

		switch kb.Builtin {
		case "openUrl":
			openUrlKey = kb.NewBinding(&openUrlKey)
		case "openPR":
			openPRKey = kb.NewBinding(&openPRKey)
		case "quit":
			quitKey = kb.NewBinding(&quitKey)
		case "nextRow":
			nextRowKey = kb.NewBinding(&nextRowKey)
		case "prevRow":
			prevRowKey = kb.NewBinding(&prevRowKey)
		case "zoomPane":
			zoomPaneKey = kb.NewBinding(&zoomPaneKey)
		case "nextPane":
			nextPaneKey = kb.NewBinding(&nextPaneKey)
		case "prevPane":
			prevPaneKey = kb.NewBinding(&prevPaneKey)
		case "gotoTop":
			gotoTopKey = kb.NewBinding(&gotoTopKey)
		case "gotoBottom":
			gotoBottomKey = kb.NewBinding(&gotoBottomKey)
		case "right":
			rightKey = kb.NewBinding(&rightKey)
		case "left":
			leftKey = kb.NewBinding(&leftKey)
		case "search":
			searchKey = kb.NewBinding(&searchKey)
		case "mode":
			modeKey = kb.NewBinding(&modeKey)
		case "cancelSearch":
			cancelSearchKey = kb.NewBinding(&cancelSearchKey)
		case "applySearch":
			applySearchKey = kb.NewBinding(&applySearchKey)
		case "nextSearchMatch":
			nextSearchMatchKey = kb.NewBinding(&nextSearchMatchKey)
		case "prevSearchMatch":
			prevSearchMatchKey = kb.NewBinding(&prevSearchMatchKey)
		case "refreshAll":
			refreshAllKey = kb.NewBinding(&refreshAllKey)
		case "rerun":
			rerunKey = kb.NewBinding(&rerunKey)
		default:
			continue
		}
	}

	return nil
}
