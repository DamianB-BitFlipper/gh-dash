package search

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/cmpcontroller"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/fuzzyselect"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/inputbox"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/context"
)

type Model struct {
	ctx                *context.ProgramContext
	initialValue       string
	cmpctl             *cmpcontroller.Controller
	disableCompletions bool
}

type SearchOptions struct {
	Prefix       string
	InitialValue string
	Placeholder  string
	// DisableCompletions suppresses the autocomplete popup for this
	// search input. Used by the local row-filter search where the
	// autocomplete contexts (is:open, author:, repo:, ...) are not
	// meaningful and the popup would otherwise leave the search box's
	// border drawn open at the bottom even though no popup is rendered
	// (BaseModel.ViewCompletions only exposes the global search bar's
	// completions to the floating-layer renderer in ui.go).
	DisableCompletions bool
}

func NewModel(ctx *context.ProgramContext, opts SearchOptions) Model {
	ti := textinput.New()
	ti.Placeholder = opts.Placeholder
	base := lipgloss.NewStyle()
	ti.SetStyles(textinput.Styles{
		Focused: textinput.StyleState{
			Placeholder: lipgloss.NewStyle().Foreground(ctx.Theme.FaintText),
			Prompt:      base.Foreground(ctx.Theme.SecondaryText),
			Text:        base.Foreground(ctx.Theme.PrimaryText),
		},
		Blurred: textinput.StyleState{
			Placeholder: lipgloss.NewStyle().Foreground(ctx.Theme.FaintText),
			Prompt:      base.Foreground(ctx.Theme.SecondaryText),
			Text:        lipgloss.NewStyle().Foreground(ctx.Theme.PrimaryText),
		},
		Cursor: textinput.CursorStyle{
			Color: ctx.Theme.FaintText,
			Shape: tea.CursorBar,
			Blink: true,
		},
	})
	ti.Prompt = fmt.Sprintf(" %s ", opts.Prefix)

	ti.Blur()
	ti.SetValue(opts.InitialValue)
	ti.CursorStart()

	ctl := cmpcontroller.New(
		ctx,
		inputbox.ModelOpts{TextInput: &ti},
	)
	selectStyles := ctx.Styles.Select
	selectStyles.PopupStyle = ctx.Styles.Select.PopupStyle.BorderTop(false).BorderForeground(
		ctx.Styles.Colors.OpenIssue,
	)
	ctl.SetSelectStyles(selectStyles)

	m := Model{
		ctx:                ctx,
		initialValue:       opts.InitialValue,
		cmpctl:             &ctl,
		disableCompletions: opts.DisableCompletions,
	}

	m.cmpctl.Exit()

	return m
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	cmd, _ := m.cmpctl.Update(msg)
	return m, cmd
}

func (m Model) View(ctx *context.ProgramContext) string {
	return m.view(ctx, m.ctx.Styles.Colors.OpenIssue)
}

func (m Model) ViewWithFocusedBorder(ctx *context.ProgramContext, focusedBorder color.Color) string {
	return m.view(ctx, focusedBorder)
}

func (m Model) view(ctx *context.ProgramContext, focusedBorder color.Color) string {
	s := m.ctx.Styles.Search.Root
	// Only modify the bottom border to "open into" the completions
	// popup when the popup will actually be rendered. Inputs that
	// disable completions must keep their closed rounded box;
	// otherwise the border would extend below the input even though
	// nothing is drawn there (see SearchOptions.DisableCompletions).
	if cmp := m.ViewCompletions(); cmp != "" {
		b := lipgloss.RoundedBorder()
		b.BottomLeft = lipgloss.RoundedBorder().MiddleLeft
		b.BottomRight = lipgloss.RoundedBorder().MiddleRight
		s = s.Border(b, true)
	}
	if m.cmpctl.Focused() {
		s = s.BorderForeground(focusedBorder)
	}
	return s.Render(m.cmpctl.View())
}

// ViewInline returns the raw input view without the surrounding rounded
// border so the caller can place the search field inside another row (for
// example, on the tabs row beside the section tabs).
func (m Model) ViewInline() string {
	return m.cmpctl.View()
}

func (m Model) ViewCompletions() string {
	if m.disableCompletions {
		return ""
	}
	return m.cmpctl.ViewCompletions()
}

func (m *Model) CursorEnd() {
	m.cmpctl.CursorEnd()
}

func (m *Model) Repo() (cmpcontroller.RepoRef, bool) {
	for token := range strings.FieldsSeq(m.Value()) {
		if strings.HasPrefix(token, "repo:") {
			repo, found := strings.CutPrefix(token, "repo:")
			parts := strings.Split(repo, "/")
			if len(parts) < 2 {
				return cmpcontroller.RepoRef{}, false
			}
			return cmpcontroller.RepoRef{
				NameWithOwner: repo,
				Owner:         parts[0],
				Name:          parts[1],
			}, found
		}
	}
	return cmpcontroller.RepoRef{}, false
}

func (m *Model) Focus() tea.Cmd {
	repo, _ := m.Repo()
	m.cmpctl.SetAutocompleteSource(&fuzzyselect.SearchQuerySource{})
	cmd := m.cmpctl.Enter(cmpcontroller.EnterOptions{
		Mode:                             cmpcontroller.ModeSearch,
		Prompt:                           "",
		Repo:                             repo,
		EnterFetch:                       cmpcontroller.FetchWithLoading,
		ConfirmDiscardOnCancel:           false,
		HideAutocompleteWhenContextEmpty: m.disableCompletions,
		InitialValue:                     m.cmpctl.Value(),
	})
	// Searches that disable completions (local row filter) must not
	// force the popup open; only the global search bar shows the
	// autocomplete contexts (is:open, repo:, author:, ...).
	if !m.disableCompletions {
		m.cmpctl.ShowCompletions()
	}
	return cmd
}

func (m *Model) Blur() {
	m.cmpctl.Exit()
}

func (m *Model) SetValue(val string) {
	m.cmpctl.SetValue(val)
}

func (m *Model) UpdateProgramContext(ctx *context.ProgramContext) {
	oldWidth := m.cmpctl.Width()
	newWidth := m.getInputWidth(ctx)
	m.cmpctl.SetWidth(newWidth)
	if newWidth != oldWidth {
		m.cmpctl.CursorEnd()
	}
}

func (m *Model) getInputWidth(ctx *context.ProgramContext) int {
	return max(
		2,
		ctx.MainContentWidth-4,
	)
}

func (m Model) Value() string {
	return m.cmpctl.Value()
}
