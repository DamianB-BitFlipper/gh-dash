package prssection

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/fuzzyselect"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/context"
)

type createPRForm struct {
	ctx      *context.ProgramContext
	mode     createPRFormMode
	title    textinput.Model
	head     textinput.Model
	base     textinput.Model
	body     textarea.Model
	branches []fuzzyselect.Suggestion
	loading  bool
	err      error
	headList []fuzzyselect.Suggestion
	baseList []fuzzyselect.Suggestion
	headIdx  int
	baseIdx  int
	active   int
	width    int
}

type createPRFormMode int

const (
	createPRFormModeCreate createPRFormMode = iota
	createPRFormModeEdit
)

func newCreatePRForm(ctx *context.ProgramContext) createPRForm {
	title := newCreatePRTextInput(ctx, "PR title")
	head := newCreatePRTextInput(ctx, "current branch")
	base := newCreatePRTextInput(ctx, "default branch")
	body := textarea.New()
	body.Prompt = ""
	body.Placeholder = "PR body (optional)"
	body.ShowLineNumbers = false
	body.SetHeight(5)
	body.CharLimit = 65536
	if ctx != nil {
		baseStyle := lipgloss.NewStyle().Background(ctx.Theme.SelectedBackground)
		body.SetStyles(textarea.Styles{
			Focused: textarea.StyleState{
				Base:             baseStyle,
				Text:             baseStyle.Foreground(ctx.Theme.PrimaryText),
				CursorLine:       baseStyle.Foreground(ctx.Theme.PrimaryText),
				Placeholder:      baseStyle.Foreground(ctx.Theme.FaintText),
				EndOfBuffer:      baseStyle.Foreground(ctx.Theme.FaintText),
				LineNumber:       baseStyle.Foreground(ctx.Theme.FaintText),
				CursorLineNumber: baseStyle.Foreground(ctx.Theme.FaintText),
			},
			Blurred: textarea.StyleState{
				Base:        baseStyle,
				Text:        baseStyle.Foreground(ctx.Theme.PrimaryText),
				Placeholder: baseStyle.Foreground(ctx.Theme.FaintText),
				EndOfBuffer: baseStyle.Foreground(ctx.Theme.FaintText),
			},
			Cursor: textarea.CursorStyle{Color: ctx.Theme.FaintText, Shape: tea.CursorBar, Blink: true},
		})
	}

	f := createPRForm{ctx: ctx, mode: createPRFormModeCreate, title: title, head: head, base: base, body: body}
	f.focusActive()
	return f
}

func newCreatePRTextInput(ctx *context.ProgramContext, placeholder string) textinput.Model {
	input := textinput.New()
	input.Prompt = ""
	input.Placeholder = placeholder
	input.CharLimit = 256
	if ctx != nil {
		base := lipgloss.NewStyle().Background(ctx.Theme.SelectedBackground)
		input.SetStyles(textinput.Styles{
			Focused: textinput.StyleState{
				Text:        base.Foreground(ctx.Theme.PrimaryText),
				Placeholder: base.Foreground(ctx.Theme.FaintText),
			},
			Blurred: textinput.StyleState{
				Text:        base.Foreground(ctx.Theme.PrimaryText),
				Placeholder: base.Foreground(ctx.Theme.FaintText),
			},
			Cursor: textinput.CursorStyle{Color: ctx.Theme.FaintText, Shape: tea.CursorBar, Blink: true},
		})
	}
	return input
}

func (f createPRForm) popupBgStyle() lipgloss.Style {
	if f.ctx == nil {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().Background(f.ctx.Theme.SelectedBackground)
}

func (f createPRForm) renderField(width int, content string) string {
	return f.popupBgStyle().Width(width).Render(content)
}

func (f createPRForm) Update(msg tea.Msg) (createPRForm, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "tab", "shift+tab":
			if keyMsg.String() == "shift+tab" {
				f.active = (f.active + f.fieldCount() - 1) % f.fieldCount()
			} else {
				f.active = (f.active + 1) % f.fieldCount()
			}
			return f, f.focusActive()
		case "down", "ctrl+n":
			if f.isHeadActive() {
				f.headIdx = nextBranchIndex(f.headIdx, len(f.headList))
				return f, nil
			}
			if f.isBaseActive() {
				f.baseIdx = nextBranchIndex(f.baseIdx, len(f.baseList))
				return f, nil
			}
		case "up", "ctrl+p":
			if f.isHeadActive() {
				f.headIdx = prevBranchIndex(f.headIdx, len(f.headList))
				return f, nil
			}
			if f.isBaseActive() {
				f.baseIdx = prevBranchIndex(f.baseIdx, len(f.baseList))
				return f, nil
			}
		case "enter", "ctrl+y":
			if f.isHeadActive() {
				if selected := selectedBranch(f.headList, f.headIdx); selected != "" {
					f.head.SetValue(selected)
					f.head.CursorEnd()
					return f, nil
				}
			}
			if f.isBaseActive() {
				if selected := selectedBranch(f.baseList, f.baseIdx); selected != "" {
					f.base.SetValue(selected)
					f.base.CursorEnd()
					return f, nil
				}
			}
		}
	}

	var cmd tea.Cmd
	switch f.active {
	case 0:
		f.title, cmd = f.title.Update(msg)
	case f.headFieldIndex():
		f.head, cmd = f.head.Update(msg)
		f.headList = filterBranches(f.branches, f.head.Value())
		f.headIdx = clampBranchIndex(f.headIdx, len(f.headList))
	case f.baseFieldIndex():
		f.base, cmd = f.base.Update(msg)
		f.baseList = filterBranches(f.branches, f.base.Value())
		f.baseIdx = clampBranchIndex(f.baseIdx, len(f.baseList))
	default:
		f.body, cmd = f.body.Update(msg)
	}
	return f, cmd
}

func (f *createPRForm) focusActive() tea.Cmd {
	f.title.Blur()
	f.head.Blur()
	f.base.Blur()
	f.body.Blur()
	switch f.active {
	case 0:
		return f.title.Focus()
	case f.headFieldIndex():
		cmd := f.head.Focus()
		f.headList = filterBranches(f.branches, f.head.Value())
		f.headIdx = clampBranchIndex(f.headIdx, len(f.headList))
		return cmd
	case f.baseFieldIndex():
		cmd := f.base.Focus()
		f.baseList = filterBranches(f.branches, f.base.Value())
		f.baseIdx = clampBranchIndex(f.baseIdx, len(f.baseList))
		return cmd
	default:
		return f.body.Focus()
	}
}

func (f createPRForm) View() string {
	label := func(active bool, text string) string {
		style := f.popupBgStyle().Bold(active)
		if active && f.ctx != nil {
			style = style.Foreground(f.ctx.Theme.PrimaryText)
		} else if f.ctx != nil {
			style = style.Foreground(f.ctx.Theme.FaintText)
		}
		return style.Render(text)
	}

	width := f.width
	if width <= 0 {
		width = 30
	}

	headBranchList := ""
	baseBranchList := ""
	if f.isHeadActive() {
		headBranchList = f.branchListView(f.headList, f.headIdx, width)
	} else if f.isBaseActive() {
		baseBranchList = f.branchListView(f.baseList, f.baseIdx, width)
	}

	helpStyle := f.popupBgStyle()
	if f.ctx != nil {
		helpStyle = helpStyle.Foreground(f.ctx.Theme.FaintText)
	}

	parts := []string{
		f.renderField(width, label(f.active == 0, "Title")),
		f.renderField(width, f.title.View()),
		f.renderField(width, ""),
	}
	if f.mode == createPRFormModeCreate {
		parts = append(
			parts,
			f.renderField(width, label(f.isHeadActive(), "Head branch")),
			f.renderField(width, f.head.View()),
			f.renderField(width, headBranchList),
			f.renderField(width, ""),
		)
	}
	parts = append(
		parts,
		f.renderField(width, label(f.isBaseActive(), "Base branch")),
		f.renderField(width, f.base.View()),
		f.renderField(width, baseBranchList),
		f.renderField(width, ""),
		f.renderField(width, label(f.isBodyActive(), "Body")),
		f.renderField(width, f.body.View()),
		f.renderField(width, ""),
		f.renderField(width, helpStyle.Render("tab switch field · ↑/↓ choose branch · enter select · ctrl+d submit · esc cancel")),
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		parts...,
	)
}

func (f createPRForm) fieldCount() int {
	if f.mode == createPRFormModeEdit {
		return 3
	}
	return 4
}

func (f createPRForm) headFieldIndex() int {
	if f.mode == createPRFormModeEdit {
		return -1
	}
	return 1
}

func (f createPRForm) baseFieldIndex() int {
	if f.mode == createPRFormModeEdit {
		return 1
	}
	return 2
}

func (f createPRForm) bodyFieldIndex() int {
	if f.mode == createPRFormModeEdit {
		return 2
	}
	return 3
}

func (f createPRForm) isHeadActive() bool { return f.active == f.headFieldIndex() }
func (f createPRForm) isBaseActive() bool { return f.active == f.baseFieldIndex() }
func (f createPRForm) isBodyActive() bool { return f.active == f.bodyFieldIndex() }

func (f createPRForm) branchListView(branches []fuzzyselect.Suggestion, selected int, width int) string {
	if f.ctx == nil {
		return ""
	}
	width = max(1, width)
	listStyle := f.popupBgStyle().Width(width).MaxWidth(width).MaxHeight(4)
	if f.loading {
		return listStyle.
			Foreground(f.ctx.Theme.FaintText).
			Render("Loading...")
	}
	if f.err != nil {
		return listStyle.
			Foreground(f.ctx.Theme.ErrorText).
			Render("Failed loading branches")
	}
	if len(branches) == 0 {
		return listStyle.
			Foreground(f.ctx.Theme.FaintText).
			Render("No matching branches")
	}
	limit := min(4, len(branches))
	rows := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		prefix := "  "
		style := f.popupBgStyle().Foreground(f.ctx.Theme.PrimaryText)
		if i == selected {
			prefix = constants.SelectionIcon + " "
			style = style.Bold(true)
		}
		detail := ""
		if branches[i].Detail != "" {
			detail = f.popupBgStyle().Foreground(f.ctx.Theme.FaintText).Render(" " + branches[i].Detail)
		}
		rows = append(rows, f.popupBgStyle().Width(width).MaxWidth(width).MaxHeight(1).Render(style.Render(prefix+branches[i].Value)+detail))
	}
	return listStyle.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func (f *createPRForm) SetWidth(width int) {
	width = max(20, width)
	f.width = width
	f.title.SetWidth(width)
	f.head.SetWidth(width)
	f.base.SetWidth(width)
	f.body.SetWidth(width)
}

func (f *createPRForm) SetBranches(branches []fuzzyselect.Suggestion, head string, base string) {
	f.loading = false
	f.err = nil
	f.branches = branches
	f.head.SetValue(head)
	f.base.SetValue(base)
	f.head.CursorEnd()
	f.base.CursorEnd()
	f.headList = filterBranches(f.branches, f.head.Value())
	f.baseList = filterBranches(f.branches, f.base.Value())
	f.headIdx = 0
	f.baseIdx = 0
}

func (f *createPRForm) SetBranchSuggestions(branches []fuzzyselect.Suggestion) {
	f.loading = false
	f.err = nil
	f.branches = branches
	f.headList = filterBranches(f.branches, f.head.Value())
	f.baseList = filterBranches(f.branches, f.base.Value())
	f.headIdx = 0
	f.baseIdx = 0
}

func (f *createPRForm) SetEditPR(title string, body string, head string, base string) {
	f.mode = createPRFormModeEdit
	f.title.SetValue(title)
	f.body.SetValue(body)
	f.head.SetValue(head)
	f.base.SetValue(base)
	f.title.CursorEnd()
	f.base.CursorEnd()
	f.active = 0
	f.focusActive()
}

func (f *createPRForm) SetCreateMode() {
	f.mode = createPRFormModeCreate
}

func (f *createPRForm) SetBranchesLoading() {
	f.loading = true
	f.err = nil
	f.branches = nil
	f.headList = nil
	f.baseList = nil
	f.headIdx = 0
	f.baseIdx = 0
}

func (f *createPRForm) SetBranchesError(err error) {
	f.loading = false
	f.err = err
}

func (f createPRForm) BranchesLoading() bool {
	return f.loading
}

func filterBranches(branches []fuzzyselect.Suggestion, query string) []fuzzyselect.Suggestion {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return branches
	}
	filtered := make([]fuzzyselect.Suggestion, 0, len(branches))
	for _, branch := range branches {
		if strings.Contains(strings.ToLower(branch.Value), query) {
			filtered = append(filtered, branch)
		}
	}
	return filtered
}

func selectedBranch(branches []fuzzyselect.Suggestion, index int) string {
	if index < 0 || index >= len(branches) {
		return ""
	}
	return branches[index].Value
}

func nextBranchIndex(index int, length int) int {
	if length == 0 {
		return 0
	}
	return (index + 1) % length
}

func prevBranchIndex(index int, length int) int {
	if length == 0 {
		return 0
	}
	index--
	if index < 0 {
		return length - 1
	}
	return index
}

func clampBranchIndex(index int, length int) int {
	if length == 0 || index < 0 {
		return 0
	}
	if index >= length {
		return length - 1
	}
	return index
}

func (f createPRForm) Title() string {
	return strings.TrimSpace(f.title.Value())
}

func (f createPRForm) Body() string {
	return f.body.Value()
}

func (f createPRForm) Head() string {
	return strings.TrimSpace(f.head.Value())
}

func (f createPRForm) Base() string {
	return strings.TrimSpace(f.base.Value())
}

func (f *createPRForm) Reset() {
	f.mode = createPRFormModeCreate
	f.title.Reset()
	f.head.Reset()
	f.base.Reset()
	f.body.Reset()
	f.headIdx = 0
	f.baseIdx = 0
	f.active = 0
	f.focusActive()
}
