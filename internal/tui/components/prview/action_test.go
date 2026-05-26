package prview

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prrow"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/keys"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/theme"
)

func newTestModelForAction(t *testing.T) Model {
	t.Helper()
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../../../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	thm := theme.ParseTheme(&cfg)
	ctx := &context.ProgramContext{
		Config: &cfg,
		Theme:  thm,
		Styles: context.InitStyles(thm),
	}

	m := NewModel(ctx)
	m.ctx = ctx
	m.pr = &prrow.PullRequest{
		Ctx: ctx,
		Data: &prrow.Data{
			Primary:    &data.PullRequestData{},
			IsEnriched: true,
		},
	}
	return m
}

func TestMsgToActionReturnsCorrectActions(t *testing.T) {
	testCases := []struct {
		name           string
		msg            tea.KeyPressMsg
		expectedAction PRActionType
	}{
		{"approve key", tea.KeyPressMsg{Code: 'v'}, PRActionApprove},
		{"assign key", tea.KeyPressMsg{Code: 'a'}, PRActionAssign},
		{"request review key", tea.KeyPressMsg{Code: 'r'}, PRActionRequestReview},
		{"comment key", tea.KeyPressMsg{Code: 'c'}, PRActionComment},
		{"diff key", tea.KeyPressMsg{Code: 'd'}, PRActionDiff},
		{"checkout key C", tea.KeyPressMsg{Code: 'C'}, PRActionCheckout},
		{"ready key", tea.KeyPressMsg{Code: 'W'}, PRActionReady},
		{"toggle open/close key", tea.KeyPressMsg{Code: 'X'}, PRActionReopen},
		{"merge key", tea.KeyPressMsg{Code: 'm'}, PRActionMerge},
		{"update key", tea.KeyPressMsg{Code: 'u'}, PRActionUpdate},
		{"summary view more key", tea.KeyPressMsg{Code: 'e'}, PRActionSummaryViewMore},
		{"approve workflows key", tea.KeyPressMsg{Code: 'V'}, PRActionApproveWorkflows},
		{"previous review thread key", tea.KeyPressMsg{Code: ','}, PRActionPrevReviewThread},
		{"next review thread key", tea.KeyPressMsg{Code: '.'}, PRActionNextReviewThread},
		{"previous step key", tea.KeyPressMsg{Code: ',', Mod: tea.ModCtrl}, PRActionPrevStep},
		{"next step key", tea.KeyPressMsg{Code: '.', Mod: tea.ModCtrl}, PRActionNextStep},
		{"toggle review thread key", tea.KeyPressMsg{Code: 'z'}, PRActionToggleReviewThread},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			action := MsgToAction(tc.msg)

			require.NotNil(t, action, "expected action for key %q", tc.msg.String())
			require.Equal(
				t,
				tc.expectedAction,
				action.Type,
				"expected action type %v for key %q, got %v",
				tc.expectedAction,
				tc.msg.String(),
				action.Type,
			)
		})
	}
}

func TestMsgToActionReturnsNilForUnknownKeys(t *testing.T) {
	testCases := []struct {
		name string
		msg  tea.KeyPressMsg
	}{
		{"unknown key", tea.KeyPressMsg{Text: "q"}},
		{"freed close key", tea.KeyPressMsg{Code: 'x'}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			action := MsgToAction(tc.msg)

			require.Nil(t, action, "expected nil action for unknown key")
		})
	}
}

func TestIsTextInputBoxFocusedWhenCommenting(t *testing.T) {
	m := newTestModelForAction(t)
	cmd := m.SetIsCommenting(true)

	require.NotNil(t, cmd)
	require.True(
		t,
		m.IsTextInputBoxFocused(),
		"expected text input box focused when in commenting mode",
	)
}

func TestIsTextInputBoxFocusedWhenApproving(t *testing.T) {
	m := newTestModelForAction(t)
	cmd := m.SetIsApproving(true)

	require.NotNil(t, cmd)
	require.True(
		t,
		m.IsTextInputBoxFocused(),
		"expected text input box focused when in approving mode",
	)
}

func TestIsTextInputBoxFocusedWhenAssigning(t *testing.T) {
	m := newTestModelForAction(t)
	cmd := m.SetIsAssigning(true)

	require.NotNil(t, cmd)
	require.True(
		t,
		m.IsTextInputBoxFocused(),
		"expected text input box focused when in assigning mode",
	)
}

func TestIsTextInputBoxFocusedWhenRequestingReview(t *testing.T) {
	m := newTestModelForAction(t)
	cmd := m.SetIsRequestingReview(true)

	require.NotNil(t, cmd)
	require.True(
		t,
		m.IsTextInputBoxFocused(),
		"expected text input box focused when requesting review",
	)
}

func TestUpdateHandlesSidebarTabNavigation(t *testing.T) {
	t.Run("prev sidebar tab", func(t *testing.T) {
		m := newTestModelForAction(t)
		// Move to a non-first tab first
		m.carousel.MoveRight()
		initialTab := m.carousel.SelectedItem()

		msg := tea.KeyPressMsg{Code: tea.KeyLeft}
		m, _ = m.Update(msg)

		require.NotEqual(t, initialTab, m.carousel.SelectedItem(),
			"carousel should have moved to previous tab")
	})

	t.Run("next sidebar tab", func(t *testing.T) {
		m := newTestModelForAction(t)
		initialTab := m.carousel.SelectedItem()

		msg := tea.KeyPressMsg{Code: tea.KeyRight}
		m, _ = m.Update(msg)

		require.NotEqual(t, initialTab, m.carousel.SelectedItem(),
			"carousel should have moved to next tab")
	})
}

func TestPRActionTypes(t *testing.T) {
	// Verify all action types are distinct
	actionTypes := []PRActionType{
		PRActionNone,
		PRActionApprove,
		PRActionAssign,
		PRActionLabel,
		PRActionComment,
		PRActionDiff,
		PRActionCheckout,
		PRActionClose,
		PRActionReady,
		PRActionReopen,
		PRActionMerge,
		PRActionUpdate,
		PRActionSummaryViewMore,
		PRActionApproveWorkflows,
		PRActionPrevReviewThread,
		PRActionNextReviewThread,
		PRActionPrevStep,
		PRActionNextStep,
		PRActionToggleReviewThread,
	}

	seen := make(map[PRActionType]bool)
	for _, at := range actionTypes {
		require.False(t, seen[at], "duplicate action type value: %v", at)
		seen[at] = true
	}

	// Verify PRActionNone is zero value
	require.Equal(t, PRActionType(0), PRActionNone, "PRActionNone should be zero value")
}

func TestAssigneeChanges(t *testing.T) {
	m := newTestModelForAction(t)
	m.pr.Data.Primary.Assignees.Nodes = []data.Assignee{
		{Login: "alice"},
		{Login: "bob"},
	}

	added, removed := m.assigneeChanges([]string{"alice", "carol", "carol"})

	require.Equal(t, []string{"carol"}, added)
	require.Equal(t, []string{"bob"}, removed)
}

func TestMsgToActionWithReboundKeys(t *testing.T) {
	// Save original key bindings
	originalApproveKeys := keys.PRKeys.Approve.Keys()

	// Rebind approve key to "V" (uppercase)
	keys.PRKeys.Approve.SetKeys("V")
	defer func() {
		// Restore original bindings
		keys.PRKeys.Approve.SetKeys(originalApproveKeys...)
	}()

	msg := tea.KeyPressMsg{Text: "V"}

	action := MsgToAction(msg)

	require.NotNil(t, action, "expected action for rebound key")
	require.Equal(t, PRActionApprove, action.Type, "expected approve action for rebound key")
}

func TestIsTextInputBoxFocusedWhenLabeling(t *testing.T) {
	m := newTestModelForAction(t)
	cmd := m.SetIsLabeling(true)

	require.NotNil(t, cmd)
	require.True(
		t,
		m.IsTextInputBoxFocused(),
		"expected text input box focused when in labeling mode",
	)
}

func TestGetIsLabeling(t *testing.T) {
	t.Run("returns false initially", func(t *testing.T) {
		m := newTestModelForAction(t)
		require.False(t, m.GetIsLabeling(), "expected GetIsLabeling to return false initially")
	})

	t.Run("returns true when labeling", func(t *testing.T) {
		m := newTestModelForAction(t)
		cmd := m.SetIsLabeling(true)
		require.NotNil(t, cmd)
		require.True(t, m.GetIsLabeling(), "expected GetIsLabeling to return true when labeling")
	})
}

func TestSetIsLabelingWithNilPR(t *testing.T) {
	m := newTestModelForAction(t)
	m.pr = nil

	cmd := m.SetIsLabeling(true)

	require.Nil(t, cmd, "expected nil command when PR is nil")
}
