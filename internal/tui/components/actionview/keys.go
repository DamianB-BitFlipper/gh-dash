package actionview

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dehub/v4/internal/config"
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
		key.WithKeys("down"),
		key.WithHelp("↓", "next row"),
	)

	prevRowKey = key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "previous row"),
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

	prevStepKey = key.NewBinding(
		key.WithKeys(","),
		key.WithHelp(",", "previous step"),
	)

	nextStepKey = key.NewBinding(
		key.WithKeys("."),
		key.WithHelp(".", "next step"),
	)

	scrollLogsUpKey = key.NewBinding(
		key.WithKeys("ctrl+,"),
		key.WithHelp("ctrl+,", "scroll logs up"),
	)

	scrollLogsDownKey = key.NewBinding(
		key.WithKeys("ctrl+."),
		key.WithHelp("ctrl+.", "scroll logs down"),
	)
)

// allLocalKeys lists every key.Binding that is "actionview-local" in the
// sense that the actionview's own Update is the authoritative handler for
// it. Both embedding sites (the PR Checks tab via prview.UpdateEmbedded,
// and the dashboard Actions view's Details pane) consult IsLocalKey to
// decide whether to forward a key into the actionview or handle it
// themselves. Keeping the list here makes actionview the single source of
// truth for "what is an actionview key" so feature additions only need to
// touch this package.
func allLocalKeys() []key.Binding {
	return []key.Binding{
		openUrlKey,
		openPRKey,
		nextRowKey,
		prevRowKey,
		zoomPaneKey,
		nextPaneKey,
		prevPaneKey,
		gotoTopKey,
		gotoBottomKey,
		rightKey,
		leftKey,
		searchKey,
		modeKey,
		cancelSearchKey,
		applySearchKey,
		nextSearchMatchKey,
		prevSearchMatchKey,
		refreshAllKey,
		rerunKey,
		prevStepKey,
		nextStepKey,
		scrollLogsUpKey,
		scrollLogsDownKey,
	}
}

// IsLocalKey reports whether msg matches any actionview-local key binding.
// Embedding sites use this to gate forwarding of keys into the actionview's
// Update; non-key messages return false.
func IsLocalKey(msg tea.KeyMsg) bool {
	for _, b := range allLocalKeys() {
		if key.Matches(msg, b) {
			return true
		}
	}
	return false
}

// RebindActionsKeybindings applies user overrides to the embedded
// actionview's local keybindings. These bindings are only consulted when the
// embedded view has focus (the Details pane of the Actions view, or when
// logs search is focused). User-configured keys MUST NOT collide with
// universal parent keybindings (Quit, NextSection, Refresh, etc.) — the
// parent's main Update loop matches universal keys first and will swallow
// any conflicting binding before the embedded view ever sees it.
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
		case "prevStep":
			prevStepKey = kb.NewBinding(&prevStepKey)
		case "nextStep":
			nextStepKey = kb.NewBinding(&nextStepKey)
		case "scrollLogsUp":
			scrollLogsUpKey = kb.NewBinding(&scrollLogsUpKey)
		case "scrollLogsDown":
			scrollLogsDownKey = kb.NewBinding(&scrollLogsDownKey)
		default:
			continue
		}
	}

	return nil
}
