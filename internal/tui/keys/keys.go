package keys

import (
	"fmt"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	log "charm.land/log/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
)

// NotificationSubjectType indicates what type of content is being viewed in the notifications view
type NotificationSubjectType int

const (
	NotificationSubjectNone NotificationSubjectType = iota
	NotificationSubjectPR
	NotificationSubjectIssue
)

// notificationSubject tracks the current notification subject type for help display
var notificationSubject NotificationSubjectType

type PRPreviewContext int

const (
	PRPreviewContextNone PRPreviewContext = iota
	PRPreviewContextActivity
	PRPreviewContextChecks
)

var prPreviewContext PRPreviewContext

// SetNotificationSubject sets the current notification subject type for help display
func SetNotificationSubject(subjectType NotificationSubjectType) {
	notificationSubject = subjectType
}

func SetPRPreviewContext(context PRPreviewContext) {
	prPreviewContext = context
}

type KeyMap struct {
	viewType      config.ViewType
	Up            key.Binding
	Down          key.Binding
	FirstLine     key.Binding
	LastLine      key.Binding
	CyclePreview  key.Binding
	OpenGithub    key.Binding
	Refresh       key.Binding
	Redraw        key.Binding
	PageDown      key.Binding
	PageUp        key.Binding
	PreviewTop    key.Binding
	PreviewBottom key.Binding
	FocusMain     key.Binding
	FocusPreview  key.Binding
	NextView      key.Binding
	PrevView      key.Binding
	NextSection   key.Binding
	PrevSection   key.Binding
	Search        key.Binding
	LocalSearch   key.Binding
	CopyUrl       key.Binding
	CopyNumber    key.Binding
	Help          key.Binding
	Quit          key.Binding
}

func CreateKeyMapForView(viewType config.ViewType) help.KeyMap {
	Keys.viewType = viewType
	return Keys
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	var additionalKeys []key.Binding
	var customKeys []key.Binding

	if len(CustomUniversalBindings) > 0 {
		customKeys = append(customKeys, CustomUniversalBindings...)
	}

	switch k.viewType {
	case config.PRsView:
		additionalKeys = PRFullHelp()
		customKeys = append(customKeys, CustomPRBindings...)
	case config.NotificationsView:
		additionalKeys = NotificationFullHelp()
		customKeys = append(customKeys, CustomNotificationBindings...)
		// Include PR or Issue keys when viewing that subject type
		switch notificationSubject {
		case NotificationSubjectPR:
			additionalKeys = append(additionalKeys, PRFullHelp()...)
			customKeys = append(customKeys, CustomPRBindings...)
		case NotificationSubjectIssue:
			additionalKeys = append(additionalKeys, IssueFullHelp()...)
			customKeys = append(customKeys, CustomIssueBindings...)
		}
	case config.ActionsView:
		additionalKeys = ActionsFullHelp()
		customKeys = append(customKeys, CustomActionBindings...)
	default:
		additionalKeys = IssueFullHelp()
		customKeys = append(customKeys, CustomIssueBindings...)
	}

	keyGroups := [][]key.Binding{
		k.NavigationKeys(),
		k.AppKeys(),
		additionalKeys,
	}

	if len(customKeys) > 0 {
		keyGroups = append(keyGroups, customKeys)
	}

	keyGroups = append(keyGroups, k.QuitAndHelpKeys())

	return buildBalancedHelpColumns(keyGroups...)
}

func buildBalancedHelpColumns(keyGroups ...[]key.Binding) [][]key.Binding {
	var bindings []key.Binding
	for _, group := range keyGroups {
		bindings = append(bindings, group...)
	}
	if len(keyGroups) == 0 || len(bindings) == 0 {
		return keyGroups
	}

	baseColumnSize := len(bindings) / len(keyGroups)
	extraBindings := len(bindings) % len(keyGroups)
	columns := make([][]key.Binding, 0, len(keyGroups))
	for i := range keyGroups {
		count := baseColumnSize
		if i < extraBindings {
			count++
		}
		if count == 0 {
			continue
		}
		columns = append(columns, bindings[:count])
		bindings = bindings[count:]
	}

	return columns
}

func enabledBindings(bindings ...key.Binding) []key.Binding {
	enabled := make([]key.Binding, 0, len(bindings))
	for _, binding := range bindings {
		if len(binding.Keys()) > 0 {
			enabled = append(enabled, binding)
		}
	}
	return enabled
}

func (k KeyMap) NavigationKeys() []key.Binding {
	return []key.Binding{
		k.Up,
		k.Down,
		k.PrevSection,
		k.NextSection,
		k.FirstLine,
		k.LastLine,
		k.PageDown,
		k.PageUp,
		k.PreviewTop,
		k.PreviewBottom,
		k.FocusMain,
		k.FocusPreview,
		k.NextView,
		k.PrevView,
	}
}

func (k KeyMap) AppKeys() []key.Binding {
	localSearch := k.LocalSearch
	if prPreviewContext == PRPreviewContextChecks {
		localSearch = key.NewBinding(
			key.WithKeys(k.LocalSearch.Keys()...),
			key.WithHelp(k.LocalSearch.Help().Key, "logs search"),
		)
	}

	return []key.Binding{
		k.Refresh,
		k.CyclePreview,
		k.OpenGithub,
		k.CopyNumber,
		k.CopyUrl,
		k.Search,
		localSearch,
	}
}

func (k KeyMap) QuitAndHelpKeys() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

var Keys = &KeyMap{
	Up: key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "move down"),
	),
	FirstLine: key.NewBinding(
		key.WithKeys("<", "home"),
		key.WithHelp("</home", "first item"),
	),
	LastLine: key.NewBinding(
		key.WithKeys(">", "end"),
		key.WithHelp(">/end", "last item"),
	),
	CyclePreview: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "cycle preview"),
	),
	OpenGithub: key.NewBinding(
		key.WithKeys("o"),
		key.WithHelp("o", "open in GitHub"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("R", "refresh"),
	),
	PageDown: key.NewBinding(
		key.WithKeys("ctrl+down"),
		key.WithHelp("Ctrl+↓", "preview page down"),
	),
	PageUp: key.NewBinding(
		key.WithKeys("ctrl+up"),
		key.WithHelp("Ctrl+↑", "preview page up"),
	),
	PreviewTop: key.NewBinding(
		key.WithKeys("ctrl+h"),
		key.WithHelp("Ctrl+h", "preview top"),
	),
	PreviewBottom: key.NewBinding(
		key.WithKeys("ctrl+g"),
		key.WithHelp("Ctrl+g", "preview bottom"),
	),
	FocusMain: key.NewBinding(
		key.WithKeys("ctrl+left"),
		key.WithHelp("Ctrl+←", "focus main"),
	),
	FocusPreview: key.NewBinding(
		key.WithKeys("ctrl+right"),
		key.WithHelp("Ctrl+→", "focus preview"),
	),
	NextView: key.NewBinding(
		key.WithKeys("}"),
		key.WithHelp("}", "next view"),
	),
	PrevView: key.NewBinding(
		key.WithKeys("{"),
		key.WithHelp("{", "previous view"),
	),
	NextSection: key.NewBinding(
		key.WithKeys("]"),
		key.WithHelp("]", "next section"),
	),
	PrevSection: key.NewBinding(
		key.WithKeys("["),
		key.WithHelp("[", "previous section"),
	),
	Search: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "search"),
	),
	LocalSearch: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "filter rows"),
	),
	CopyNumber: key.NewBinding(
		key.WithKeys("y"),
		key.WithHelp("y", "copy number"),
	),
	CopyUrl: key.NewBinding(
		key.WithKeys("Y"),
		key.WithHelp("Y", "copy url"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("Q", "ctrl+c"),
		key.WithHelp("Q", "quit"),
	),
}

// Rebind will update our saved keybindings from configuration values.
func Rebind(universal, issueKeys, prKeys, notificationKeys []config.Keybinding) error {
	err := rebindUniversal(universal)
	if err != nil {
		return err
	}

	err = rebindPRKeys(prKeys)
	if err != nil {
		return err
	}

	err = rebindIssueKeys(issueKeys)
	if err != nil {
		return err
	}

	return rebindNotificationKeys(notificationKeys)
}

// CustomBindings stores custom keybindings that don't have built-in equivalents
var (
	CustomUniversalBindings    []key.Binding
	CustomPRBindings           []key.Binding
	CustomIssueBindings        []key.Binding
	CustomNotificationBindings []key.Binding
	CustomActionBindings       []key.Binding
)

func rebindUniversal(universal []config.Keybinding) error {
	log.Debug("Rebinding universal keys", "keys", universal)

	CustomUniversalBindings = []key.Binding{}

	for _, kb := range universal {
		if kb.Builtin == "" {
			// Handle custom commands
			if kb.Command != "" {
				name := kb.Name
				if kb.Name == "" {
					name = config.TruncateCommand(kb.Command)
				}

				customBinding := key.NewBinding(
					key.WithKeys(kb.Key),
					key.WithHelp(kb.Key, name),
				)

				CustomUniversalBindings = append(CustomUniversalBindings, customBinding)
			}
			continue
		}

		log.Debug("Rebinding universal key", "builtin", kb.Builtin, "key", kb.Key)

		var key *key.Binding

		switch kb.Builtin {
		case "up":
			key = &Keys.Up
		case "down":
			key = &Keys.Down
		case "firstLine":
			key = &Keys.FirstLine
		case "lastLine":
			key = &Keys.LastLine
		case "cyclePreview":
			key = &Keys.CyclePreview
		case "openGithub":
			key = &Keys.OpenGithub
		case "refresh":
			key = &Keys.Refresh
		case "redraw":
			key = &Keys.Redraw
		case "pageDown":
			key = &Keys.PageDown
		case "pageUp":
			key = &Keys.PageUp
		case "previewTop":
			key = &Keys.PreviewTop
		case "previewBottom":
			key = &Keys.PreviewBottom
		case "focusMain":
			key = &Keys.FocusMain
		case "focusPreview":
			key = &Keys.FocusPreview
		case "nextView":
			key = &Keys.NextView
		case "prevView":
			key = &Keys.PrevView
		case "nextSection":
			key = &Keys.NextSection
		case "prevSection":
			key = &Keys.PrevSection
		case "search":
			key = &Keys.Search
		case "localSearch":
			key = &Keys.LocalSearch
		case "copyurl":
			key = &Keys.CopyUrl
		case "copyNumber":
			key = &Keys.CopyNumber
		case "help":
			key = &Keys.Help
		case "quit":
			key = &Keys.Quit
		default:
			return fmt.Errorf("unknown built-in universal key: '%s'", kb.Builtin)
		}

		key.SetKeys(kb.Key)

		helpDesc := key.Help().Desc
		if kb.Name != "" {
			helpDesc = kb.Name
		} else if kb.Command != "" {
			helpDesc = kb.Command
		}
		key.SetHelp(kb.Key, helpDesc)
	}

	return nil
}
