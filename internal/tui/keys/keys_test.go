package keys

import (
	"testing"

	"charm.land/bubbles/v2/key"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
)

func TestSetNotificationSubject(t *testing.T) {
	tests := []struct {
		name     string
		subject  NotificationSubjectType
		expected NotificationSubjectType
	}{
		{"none", NotificationSubjectNone, NotificationSubjectNone},
		{"pr", NotificationSubjectPR, NotificationSubjectPR},
		{"issue", NotificationSubjectIssue, NotificationSubjectIssue},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetNotificationSubject(tt.subject)
			if notificationSubject != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, notificationSubject)
			}
		})
	}
}

func TestSetPRPreviewContext(t *testing.T) {
	defer SetPRPreviewContext(PRPreviewContextNone)

	tests := []struct {
		name     string
		context  PRPreviewContext
		expected PRPreviewContext
	}{
		{"none", PRPreviewContextNone, PRPreviewContextNone},
		{"activity", PRPreviewContextActivity, PRPreviewContextActivity},
		{"checks", PRPreviewContextChecks, PRPreviewContextChecks},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetPRPreviewContext(tt.context)
			if prPreviewContext != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, prPreviewContext)
			}
		})
	}
}

func TestFullHelpIncludesPRKeysForPRSubject(t *testing.T) {
	// Set up for notifications view with PR subject
	keymap := CreateKeyMapForView(config.NotificationsView)
	SetNotificationSubject(NotificationSubjectPR)

	help := keymap.FullHelp()

	// Flatten all sections to check for keys
	var allKeys []key.Binding
	for _, section := range help {
		allKeys = append(allKeys, section...)
	}

	// Check that notification keys are present
	found := findKeyByHelp(allKeys, "mark as done")
	if !found {
		t.Error("expected notification key 'mark as done' to be present")
	}

	// Check that PR keys are present
	found = findKeyByHelp(allKeys, "diff")
	if !found {
		t.Error("expected PR key 'diff' to be present when viewing PR notification")
	}

	found = findKeyByHelp(allKeys, "approve")
	if !found {
		t.Error("expected PR key 'approve' to be present when viewing PR notification")
	}

	found = findKeyByHelp(allKeys, "approve all workflows")
	if !found {
		t.Error(
			"expected PR key 'approve all workflows' to be present when viewing PR notification",
		)
	}

	found = findKeyByHelp(allKeys, "toggle open/close")
	if !found {
		t.Error("expected PR key 'toggle open/close' to be present when viewing PR notification")
	}

	// Clean up
	SetNotificationSubject(NotificationSubjectNone)
}

func TestFullHelpIncludesIssueKeysForIssueSubject(t *testing.T) {
	// Set up for notifications view with Issue subject
	keymap := CreateKeyMapForView(config.NotificationsView)
	SetNotificationSubject(NotificationSubjectIssue)

	help := keymap.FullHelp()

	// Flatten all sections to check for keys
	var allKeys []key.Binding
	for _, section := range help {
		allKeys = append(allKeys, section...)
	}

	// Check that notification keys are present
	found := findKeyByHelp(allKeys, "mark as done")
	if !found {
		t.Error("expected notification key 'mark as done' to be present")
	}

	// Check that Issue keys are present
	found = findKeyByHelp(allKeys, "label")
	if !found {
		t.Error("expected Issue key 'label' to be present when viewing Issue notification")
	}

	found = findKeyByHelp(allKeys, "checkout")
	if !found {
		t.Error("expected Issue key 'checkout' to be present when viewing Issue notification")
	}

	found = findKeyByHelp(allKeys, "toggle open/close")
	if !found {
		t.Error("expected Issue key 'toggle open/close' to be present when viewing Issue notification")
	}

	// Clean up
	SetNotificationSubject(NotificationSubjectNone)
}

func TestFullHelpExcludesPRKeysForNoSubject(t *testing.T) {
	// Set up for notifications view with no subject
	keymap := CreateKeyMapForView(config.NotificationsView)
	SetNotificationSubject(NotificationSubjectNone)

	help := keymap.FullHelp()

	// Flatten all sections to check for keys
	var allKeys []key.Binding
	for _, section := range help {
		allKeys = append(allKeys, section...)
	}

	// Check that notification keys are present
	found := findKeyByHelp(allKeys, "mark as done")
	if !found {
		t.Error("expected notification key 'mark as done' to be present")
	}

	// Check that PR keys are NOT present
	found = findKeyByHelp(allKeys, "diff")
	if found {
		t.Error("expected PR key 'diff' to NOT be present when no subject is selected")
	}

	// Check that Issue keys are NOT present
	found = findKeyByHelp(allKeys, "label")
	if found {
		t.Error("expected Issue key 'label' to NOT be present when no subject is selected")
	}
}

func TestFullHelpForPRViewDoesNotIncludeNotificationKeys(t *testing.T) {
	// Set up for PR view (not notifications)
	keymap := CreateKeyMapForView(config.PRsView)
	SetNotificationSubject(NotificationSubjectNone)

	help := keymap.FullHelp()

	// Flatten all sections to check for keys
	var allKeys []key.Binding
	for _, section := range help {
		allKeys = append(allKeys, section...)
	}

	// Check that PR keys are present
	found := findKeyByHelp(allKeys, "diff")
	if !found {
		t.Error("expected PR key 'diff' to be present in PR view")
	}
	found = findKeyByHelp(allKeys, "create PR")
	if !found {
		t.Error("expected PR key 'create PR' to be present in PR view")
	}
	found = findKeyByHelp(allKeys, "open PR URL")
	if !found {
		t.Error("expected PR key 'open PR URL' to be present in PR view")
	}
	found = findKeyByHelp(allKeys, "watch checks")
	if found {
		t.Error("expected PR key 'watch checks' to be removed from PR view")
	}

	// Check that notification-specific keys are NOT present
	found = findKeyByHelp(allKeys, "toggle bookmark")
	if found {
		t.Error("expected notification key 'toggle bookmark' to NOT be present in PR view")
	}
}

func TestPRPreviewHelpChangesWithPreviewContext(t *testing.T) {
	keymap := CreateKeyMapForView(config.PRsView)
	SetNotificationSubject(NotificationSubjectNone)
	defer SetPRPreviewContext(PRPreviewContextNone)

	SetPRPreviewContext(PRPreviewContextActivity)
	activityKeys := flattenHelp(keymap.FullHelp())
	if !findKeyByHelp(activityKeys, "previous review thread") {
		t.Fatal("expected activity help to include previous review thread")
	}
	if findKeyByHelp(activityKeys, "previous check") {
		t.Fatal("did not expect activity help to include previous check")
	}

	SetPRPreviewContext(PRPreviewContextChecks)
	checksKeys := flattenHelp(keymap.FullHelp())
	if !findKeyByHelp(checksKeys, "previous check") {
		t.Fatal("expected checks help to include previous check")
	}
	if !findKeyByHelp(checksKeys, "logs search") {
		t.Fatal("expected checks help to include logs search")
	}
	if findKeyByHelp(checksKeys, "local search") {
		t.Fatal("did not expect checks help to include local search")
	}
	if findKeyByHelp(checksKeys, "previous review thread") {
		t.Fatal("did not expect checks help to include previous review thread")
	}

	SetPRPreviewContext(PRPreviewContextNone)
	noneKeys := flattenHelp(keymap.FullHelp())
	if findKeyByHelp(noneKeys, "previous check") || findKeyByHelp(noneKeys, "previous review thread") {
		t.Fatal("did not expect tab-specific comma help without a PR preview context")
	}
}

func TestNotificationPRHelpUsesPRPreviewContext(t *testing.T) {
	keymap := CreateKeyMapForView(config.NotificationsView)
	SetNotificationSubject(NotificationSubjectPR)
	SetPRPreviewContext(PRPreviewContextChecks)
	defer SetNotificationSubject(NotificationSubjectNone)
	defer SetPRPreviewContext(PRPreviewContextNone)

	allKeys := flattenHelp(keymap.FullHelp())
	if !findKeyByHelp(allKeys, "previous check") {
		t.Fatal("expected PR notification checks help to include previous check")
	}
	if findKeyByHelp(allKeys, "previous review thread") {
		t.Fatal("did not expect PR notification checks help to include previous review thread")
	}
}

func TestDefaultArrowKeybindings(t *testing.T) {
	requireKeys(t, Keys.Up, "up")
	requireKeys(t, Keys.Down, "down")
	requireKeys(t, Keys.FirstLine, "<", "home")
	requireKeys(t, Keys.LastLine, ">", "end")
	requireKeys(t, Keys.PrevSection, "[")
	requireKeys(t, Keys.NextSection, "]")
	requireKeys(t, Keys.PageUp, "ctrl+up")
	requireKeys(t, Keys.PageDown, "ctrl+down")
	requireKeys(t, Keys.PreviewTop, "ctrl+h")
	requireKeys(t, Keys.PreviewBottom, "ctrl+g")
	requireKeys(t, Keys.FocusMain, "ctrl+left")
	requireKeys(t, Keys.FocusPreview, "ctrl+right")
	requireKeys(t, Keys.NextView, "}")
	requireKeys(t, Keys.PrevView, "{")
	requireKeys(t, PRKeys.PrevSidebarTab, "left")
	requireKeys(t, PRKeys.NextSidebarTab, "right")
	requireKeys(t, PRKeys.PrevReviewThread, ",")
	requireKeys(t, PRKeys.NextReviewThread, ".")
	requireKeys(t, PRKeys.PrevStep, "ctrl+,")
	requireKeys(t, PRKeys.NextStep, "ctrl+.")
	requireKeys(t, Keys.Refresh, "R")
	requireKeys(t, PRKeys.RequestReview, "r")
	requireKeys(t, PRKeys.SortOrder, "S")
	requireKeys(t, PRKeys.OpenURL, "O")
	requireKeys(t, IssueKeys.SortOrder, "S")
	requireKeys(t, ActionsKeys.SortOrder, "S")
	requireKeys(t, ActionsKeys.Rerun, "ctrl+r")
	requireKeys(t, ActionsKeys.RerunFailed, "ctrl+f")
	requireKeys(t, ActionsKeys.Cancel, "x")
	requireKeys(t, Keys.LocalSearch, "s")
}

func TestDefaultViewSwitchKeybindingsAreUnbound(t *testing.T) {
	if len(PRKeys.ViewIssues.Keys()) != 0 {
		t.Fatalf("expected PR view switch key to be unbound by default, got %v", PRKeys.ViewIssues.Keys())
	}
	if len(IssueKeys.ViewPRs.Keys()) != 0 {
		t.Fatalf("expected issue view switch key to be unbound by default, got %v", IssueKeys.ViewPRs.Keys())
	}
	if len(NotificationKeys.SwitchToPRs.Keys()) != 0 {
		t.Fatalf("expected notification view switch key to be unbound by default, got %v", NotificationKeys.SwitchToPRs.Keys())
	}
}

func TestDefaultSmartFilteringKeybindingsAreUnbound(t *testing.T) {
	if len(PRKeys.ToggleSmartFiltering.Keys()) != 0 {
		t.Fatalf("expected PR smart filtering key to be unbound by default, got %v", PRKeys.ToggleSmartFiltering.Keys())
	}
	if len(IssueKeys.ToggleSmartFiltering.Keys()) != 0 {
		t.Fatalf("expected issue smart filtering key to be unbound by default, got %v", IssueKeys.ToggleSmartFiltering.Keys())
	}
	if len(ActionsKeys.ToggleSmartFiltering.Keys()) != 0 {
		t.Fatalf("expected actions smart filtering key to be unbound by default, got %v", ActionsKeys.ToggleSmartFiltering.Keys())
	}
	if len(NotificationKeys.ToggleSmartFiltering.Keys()) != 0 {
		t.Fatalf("expected notification smart filtering key to be unbound by default, got %v", NotificationKeys.ToggleSmartFiltering.Keys())
	}
}

func TestFullHelpColumnsAreBalanced(t *testing.T) {
	keymap := CreateKeyMapForView(config.PRsView)
	help := keymap.FullHelp()

	minLen := len(help[0])
	maxLen := len(help[0])
	for _, section := range help {
		if len(section) < minLen {
			minLen = len(section)
		}
		if len(section) > maxLen {
			maxLen = len(section)
		}
	}

	if maxLen-minLen > 1 {
		t.Fatalf("expected balanced help columns, got lengths min=%d max=%d", minLen, maxLen)
	}
}

func requireKeys(t *testing.T, binding key.Binding, want ...string) {
	t.Helper()
	got := binding.Keys()
	if len(got) != len(want) {
		t.Fatalf("expected keys %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected keys %v, got %v", want, got)
		}
	}
}

func flattenHelp(help [][]key.Binding) []key.Binding {
	var allKeys []key.Binding
	for _, section := range help {
		allKeys = append(allKeys, section...)
	}
	return allKeys
}

// findKeyByHelp searches for a key binding by its help description
func findKeyByHelp(keys []key.Binding, helpDesc string) bool {
	for _, k := range keys {
		if k.Help().Desc == helpDesc {
			return true
		}
	}
	return false
}

func TestRebindPRKeys_CopyBranchBuiltin(t *testing.T) {
	origKeys := PRKeys.CopyBranch.Keys()
	origHelp := PRKeys.CopyBranch.Help().Desc
	defer func() {
		PRKeys.CopyBranch.SetKeys(origKeys...)
		PRKeys.CopyBranch.SetHelp(origKeys[0], origHelp)
	}()

	err := rebindPRKeys([]config.Keybinding{
		{Builtin: "copyBranch", Key: "B", Name: "copy head branch"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	keys := PRKeys.CopyBranch.Keys()
	if len(keys) != 1 || keys[0] != "B" {
		t.Errorf("expected key to be rebound to B, got %v", keys)
	}
	if PRKeys.CopyBranch.Help().Desc != "copy head branch" {
		t.Errorf("expected help to be updated, got %q", PRKeys.CopyBranch.Help().Desc)
	}
}

func TestRebindPRKeys_CreatePrBuiltin(t *testing.T) {
	origKeys := PRKeys.Create.Keys()
	origHelp := PRKeys.Create.Help().Desc
	defer func() {
		PRKeys.Create.SetKeys(origKeys...)
		PRKeys.Create.SetHelp(origKeys[0], origHelp)
	}()

	err := rebindPRKeys([]config.Keybinding{
		{Builtin: "createPr", Key: "O", Name: "open create PR"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	keys := PRKeys.Create.Keys()
	if len(keys) != 1 || keys[0] != "O" {
		t.Errorf("expected key to be rebound to O, got %v", keys)
	}
	if PRKeys.Create.Help().Desc != "open create PR" {
		t.Errorf("expected help to be updated, got %q", PRKeys.Create.Help().Desc)
	}
}

func TestRebindPRKeys_OpenPrUrlBuiltin(t *testing.T) {
	origKeys := PRKeys.OpenURL.Keys()
	origHelp := PRKeys.OpenURL.Help().Desc
	defer func() {
		PRKeys.OpenURL.SetKeys(origKeys...)
		PRKeys.OpenURL.SetHelp(origKeys[0], origHelp)
	}()

	err := rebindPRKeys([]config.Keybinding{
		{Builtin: "openPrUrl", Key: "P", Name: "open pasted PR"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	keys := PRKeys.OpenURL.Keys()
	if len(keys) != 1 || keys[0] != "P" {
		t.Errorf("expected key to be rebound to P, got %v", keys)
	}
	if PRKeys.OpenURL.Help().Desc != "open pasted PR" {
		t.Errorf("expected help to be updated, got %q", PRKeys.OpenURL.Help().Desc)
	}
}

func TestRebindPRKeys_WatchChecksBuiltinRemoved(t *testing.T) {
	err := rebindPRKeys([]config.Keybinding{
		{Builtin: "watchChecks", Key: "w"},
	})
	if err == nil {
		t.Fatal("expected watchChecks builtin to be removed")
	}
}

func TestRebindNotificationKeys_Builtin(t *testing.T) {
	// Save original key and restore after test
	origKey := NotificationKeys.MarkAsDone.Keys()
	origHelp := NotificationKeys.MarkAsDone.Help().Desc
	defer func() {
		NotificationKeys.MarkAsDone.SetKeys(origKey...)
		NotificationKeys.MarkAsDone.SetHelp(origKey[0], origHelp)
	}()

	err := rebindNotificationKeys([]config.Keybinding{
		{Builtin: "markAsDone", Key: "X"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	keys := NotificationKeys.MarkAsDone.Keys()
	if len(keys) != 1 || keys[0] != "X" {
		t.Errorf("expected key to be rebound to X, got %v", keys)
	}
}

func TestRebindNotificationKeys_UnknownBuiltin(t *testing.T) {
	err := rebindNotificationKeys([]config.Keybinding{
		{Builtin: "nonExistent", Key: "Z"},
	})
	if err == nil {
		t.Error("expected error for unknown builtin, got nil")
	}
}

func TestRebindNotificationKeys_CustomCommand(t *testing.T) {
	// Clear any previous custom bindings
	CustomNotificationBindings = nil

	err := rebindNotificationKeys([]config.Keybinding{
		{Key: "N", Command: "echo hello"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(CustomNotificationBindings) != 1 {
		t.Fatalf("expected 1 custom binding, got %d", len(CustomNotificationBindings))
	}
	if CustomNotificationBindings[0].Keys()[0] != "N" {
		t.Errorf("expected custom binding key N, got %s", CustomNotificationBindings[0].Keys()[0])
	}
}

func TestFullHelpIncludesCustomNotificationBindings(t *testing.T) {
	// Set up custom notification bindings
	CustomNotificationBindings = []key.Binding{
		key.NewBinding(key.WithKeys("N"), key.WithHelp("N", "custom notif action")),
	}
	defer func() { CustomNotificationBindings = nil }()

	keymap := CreateKeyMapForView(config.NotificationsView)
	SetNotificationSubject(NotificationSubjectNone)

	help := keymap.FullHelp()

	var allKeys []key.Binding
	for _, section := range help {
		allKeys = append(allKeys, section...)
	}

	found := findKeyByHelp(allKeys, "custom notif action")
	if !found {
		t.Error("expected custom notification binding to appear in help")
	}

	// Clean up
	SetNotificationSubject(NotificationSubjectNone)
}
