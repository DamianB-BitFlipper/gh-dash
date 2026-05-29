package section

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"text/template"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/log/v2"
	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/go-sprout/sprout"
	timeregistry "github.com/go-sprout/sprout/registry/time"

	"github.com/dlvhdr/gh-dehub/v4/internal/config"
	"github.com/dlvhdr/gh-dehub/v4/internal/data"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/common"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/prompt"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/search"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/selection"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/table"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dehub/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dehub/v4/internal/utils"
)

type BaseModel struct {
	Id                        int
	Config                    config.SectionConfig
	Ctx                       *context.ProgramContext
	Spinner                   spinner.Model
	SearchBar                 search.Model
	IsSearching               bool
	SearchValue               string
	LocalSearchBar            search.Model
	IsLocalSearching          bool
	LocalSearchValue          string
	Table                     table.Model
	Type                      string
	SingularForm              string
	PluralForm                string
	Columns                   []table.Column
	TotalCount                int
	PageInfo                  *data.PageInfo
	PromptConfirmationBox     prompt.Model
	IsPromptConfirmationShown bool
	PromptConfirmationAction  string
	LastFetchTaskId           string
	IsSearchSupported         bool
	ShowAuthorIcon            bool
	IsFilteredByCurrentRemote bool
	IsLoading                 bool
	SortOrder                 data.SearchSort
}

type NewSectionOptions struct {
	Id          int
	Config      config.SectionConfig
	Ctx         *context.ProgramContext
	Type        string
	Columns     []table.Column
	Singular    string
	Plural      string
	LastUpdated time.Time
	CreatedAt   time.Time
}

func (options NewSectionOptions) GetConfigFiltersWithCurrentRemoteAdded(
	ctx *context.ProgramContext,
) string {
	searchValue := options.Config.Filters
	if !ctx.Config.SmartFilteringAtLaunch {
		return searchValue
	}
	repo, err := repository.Current()
	if err != nil {
		return searchValue
	}
	for token := range strings.FieldsSeq(searchValue) {
		if strings.HasPrefix(token, "repo:") {
			return searchValue
		}
	}
	return fmt.Sprintf("repo:%s/%s %s", repo.Owner, repo.Name, searchValue)
}

func NewModel(
	ctx *context.ProgramContext,
	options NewSectionOptions,
) BaseModel {
	filters := options.GetConfigFiltersWithCurrentRemoteAdded(ctx)
	isFilteredByCurrentRemote := false
	repo, err := repository.Current()
	if err == nil {
		currentCloneFilter := fmt.Sprintf("repo:%s/%s", repo.Owner, repo.Name)
		for token := range strings.FieldsSeq(filters) {
			if token == currentCloneFilter {
				isFilteredByCurrentRemote = true
				break
			}
		}
	}
	m := BaseModel{
		Ctx:          ctx,
		Id:           options.Id,
		Type:         options.Type,
		Config:       options.Config,
		Spinner:      spinner.Model{Spinner: spinner.Dot},
		Columns:      options.Columns,
		SingularForm: options.Singular,
		PluralForm:   options.Plural,
		SearchBar: search.NewModel(ctx, search.SearchOptions{
			Prefix:       fmt.Sprintf("is:%s", options.Type),
			InitialValue: filters,
		}),
		LocalSearchBar: search.NewModel(ctx, search.SearchOptions{
			Prefix:      "local",
			Placeholder: "filter loaded rows",
			// The local search filters loaded rows by substring; the
			// autocomplete contexts (is:open, repo:, author:, ...) are
			// not meaningful here. Disabling them also keeps the search
			// box's rounded border closed at the bottom instead of
			// being drawn open as if a popup were attached below.
			DisableCompletions: true,
		}),
		SearchValue:               filters,
		IsSearching:               false,
		IsFilteredByCurrentRemote: isFilteredByCurrentRemote,
		TotalCount:                0,
		PageInfo:                  nil,
		PromptConfirmationBox:     prompt.NewModel(ctx),
		ShowAuthorIcon:            ctx.Config.ShowAuthorIcons,
		SortOrder:                 data.SearchSortUpdated,
	}
	m.Table = table.NewModel(
		*ctx,
		m.GetDimensions(),
		options.LastUpdated,
		options.CreatedAt,
		m.Columns,
		nil,
		m.SingularForm,
		utils.StringPtr(m.Ctx.Styles.Section.EmptyStateStyle.Render(
			fmt.Sprintf(
				"No %s were found that match the given filters",
				m.PluralForm,
			),
		)),
		"Loading...",
		false,
	)
	return m
}

type Section interface {
	Identifier
	Component
	Table
	Search
	PromptConfirmation
	GetConfig() config.SectionConfig
	UpdateProgramContext(ctx *context.ProgramContext)
	MakeSectionCmd(cmd tea.Cmd) tea.Cmd
	GetPagerContent() string
	GetItemSingularForm() string
	GetItemPluralForm() string
	GetTotalCount() int
	RowsSelectionScroll(contentTop int) selection.Scroll
}

type Identifier interface {
	GetId() int
	GetType() string
}

type Component interface {
	Update(msg tea.Msg) (Section, tea.Cmd)
	View() string
}

type Table interface {
	NumRows() int
	GetCurrRow() data.RowData
	CurrRow() int
	NextRow() int
	PrevRow() int
	PageDown() int
	PageUp() int
	FirstItem() int
	LastItem() int
	FetchNextPageSectionRows() []tea.Cmd
	BuildRows() []table.Row
	ResetRows()
	GetIsLoading() bool
	SetIsLoading(val bool)
}

type Search interface {
	SetIsSearching(val bool) tea.Cmd
	IsSearchFocused() bool
	SetIsLocalSearching(val bool) tea.Cmd
	IsLocalSearchFocused() bool
	ViewCompletions() string
	HeaderSearchView() string
	ResetFilters()
	GetFilters() string
	ResetPageInfo()
}

type PromptConfirmation interface {
	SetIsPromptConfirmationShown(val bool) tea.Cmd
	IsPromptConfirmationFocused() bool
	SetPromptConfirmationAction(action string)
	GetPromptConfirmationAction() string
	GetPromptConfirmation() string
}

func (m *BaseModel) GetDimensions() constants.Dimensions {
	height := m.Ctx.MainContentHeight
	if m.bodySearchVisible() {
		height -= common.SearchHeight
	}
	return constants.Dimensions{
		Width: max(
			0,
			m.Ctx.MainContentWidth-m.Ctx.Styles.Section.ContainerStyle.GetHorizontalPadding(),
		),
		Height: max(0, height),
	}
}

func (m *BaseModel) localSearchVisible() bool {
	return m.IsLocalSearching || m.LocalSearchValue != ""
}

func (m *BaseModel) bodySearchVisible() bool {
	return m.localSearchVisible() || m.IsSearching
}

func (m *BaseModel) syncTableDimensions() {
	if m.Ctx == nil {
		return
	}
	newDimensions := m.GetDimensions()
	m.Table.SetDimensions(constants.Dimensions{
		Height: max(0, newDimensions.Height-2),
		Width:  max(0, newDimensions.Width),
	})
}

func (m *BaseModel) GetConfig() config.SectionConfig {
	return m.Config
}

func (m *BaseModel) HasRepoNameInConfiguredFilter() bool {
	filters := m.SearchValue
	for token := range strings.FieldsSeq(filters) {
		if strings.HasPrefix(token, "repo:") {
			return true
		}
	}
	return false
}

func (m *BaseModel) HasCurrentRepoNameInConfiguredFilter() bool {
	filters := m.SearchValue
	repo, err := repository.Current()
	if err != nil {
		return false
	}
	currentCloneFilter := fmt.Sprintf("repo:%s/%s", repo.Owner, repo.Name)
	for token := range strings.FieldsSeq(filters) {
		if token == currentCloneFilter {
			return true
		}
	}
	return false
}

func (m *BaseModel) SyncSmartFilterWithSearchValue() {
	m.IsFilteredByCurrentRemote = m.HasCurrentRepoNameInConfiguredFilter()
}

func (m *BaseModel) GetSearchValue() string {
	searchValue := m.enrichSearchWithTemplateVars()
	repo, err := repository.Current()
	if err != nil {
		return searchValue
	}

	currentCloneFilter := fmt.Sprintf("repo:%s/%s", repo.Owner, repo.Name)
	var searchValueWithoutCurrentCloneFilter []string
	for token := range strings.FieldsSeq(searchValue) {
		if token != currentCloneFilter {
			searchValueWithoutCurrentCloneFilter = append(
				searchValueWithoutCurrentCloneFilter,
				token,
			)
		}
	}
	if m.IsFilteredByCurrentRemote {
		return fmt.Sprintf("%s %s", currentCloneFilter,
			strings.Join(searchValueWithoutCurrentCloneFilter, " "))
	}
	return strings.Join(searchValueWithoutCurrentCloneFilter, " ")
}

func (m *BaseModel) enrichSearchWithTemplateVars() string {
	searchValue := m.SearchValue
	searchVars := struct{ Now time.Time }{
		Now: time.Now(),
	}
	sl := slog.New(log.Default())
	handler := sprout.New(
		sprout.WithRegistries(timeregistry.NewRegistry(), utils.NewRegistry()),
		sprout.WithLogger(sl),
	)
	funcs := handler.Build()

	tmpl, err := template.New("search").Funcs(funcs).Parse(searchValue)
	if err != nil {
		log.Error("bad template", "err", err)
		return searchValue
	}
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, searchVars)
	if err != nil {
		return searchValue
	}

	return buf.String()
}

func (m *BaseModel) UpdateProgramContext(ctx *context.ProgramContext) {
	m.Ctx = ctx
	m.syncTableDimensions()
	m.Table.UpdateProgramContext(ctx)
	m.Table.SyncViewPortContent()
	m.SearchBar.UpdateProgramContext(ctx)
	m.LocalSearchBar.UpdateProgramContext(ctx)
}

type SectionRowsFetchedMsg struct {
	SectionId int
	Issues    []data.RowData
}

func (msg SectionRowsFetchedMsg) GetSectionId() int {
	return msg.SectionId
}

func (m *BaseModel) GetId() int {
	return m.Id
}

func (m *BaseModel) GetType() string {
	return m.Type
}

func (m *BaseModel) CurrRow() int {
	return m.Table.GetCurrItem()
}

func (m *BaseModel) NextRow() int {
	return m.Table.NextItem()
}

func (m *BaseModel) PrevRow() int {
	return m.Table.PrevItem()
}

func (m *BaseModel) PageDown() int {
	return m.Table.PageDown()
}

func (m *BaseModel) PageUp() int {
	return m.Table.PageUp()
}

func (m *BaseModel) FirstItem() int {
	return m.Table.FirstItem()
}

func (m *BaseModel) LastItem() int {
	return m.Table.LastItem()
}

func (m *BaseModel) IsSearchFocused() bool {
	return m.IsSearching
}

func (m *BaseModel) IsLocalSearchFocused() bool {
	return m.IsLocalSearching
}

func (m *BaseModel) GetIsLoading() bool {
	return m.IsLoading
}

func (m *BaseModel) SetIsSearching(val bool) tea.Cmd {
	m.IsSearching = val
	if val {
		cmds := make([]tea.Cmd, 0)
		cmd := m.SearchBar.Focus()
		cmds = append(cmds, cmd)
		m.SearchBar.CursorEnd()
		m.SearchBar, cmd = m.SearchBar.Update(nil)
		cmds = append(cmds, cmd)
		return tea.Sequence(cmds...)
	} else {
		m.SearchBar.Blur()
		return nil
	}
}

func (m *BaseModel) SetIsLocalSearching(val bool) tea.Cmd {
	m.IsLocalSearching = val
	defer m.syncTableDimensions()
	if val {
		cmds := make([]tea.Cmd, 0)
		cmd := m.LocalSearchBar.Focus()
		cmds = append(cmds, cmd)
		m.LocalSearchBar.CursorEnd()
		m.LocalSearchBar, cmd = m.LocalSearchBar.Update(nil)
		cmds = append(cmds, cmd)
		return tea.Sequence(cmds...)
	}
	m.LocalSearchBar.Blur()
	return nil
}

func (m *BaseModel) LocalSearchQuery() string {
	return strings.ToLower(strings.TrimSpace(m.LocalSearchValue))
}

func (m *BaseModel) HandleLocalSearchKey(msg tea.KeyMsg, rows func() []table.Row) (bool, tea.Cmd) {
	if !m.IsLocalSearching {
		return false, nil
	}
	switch msg.String() {
	case "esc":
		m.LocalSearchValue = ""
		m.LocalSearchBar.SetValue("")
		cmd := m.SetIsLocalSearching(false)
		m.Table.ResetCurrItem()
		m.Table.SetRows(rows())
		return true, cmd
	case "ctrl+c", "enter":
		return true, m.SetIsLocalSearching(false)
	}

	var cmd tea.Cmd
	m.LocalSearchBar, cmd = m.LocalSearchBar.Update(msg)
	m.LocalSearchValue = m.LocalSearchBar.Value()
	m.Table.ResetCurrItem()
	m.Table.SetRows(rows())
	return true, cmd
}

func (m *BaseModel) ResetFilters() {
	m.SearchBar.SetValue(m.GetSearchValue())
	m.LocalSearchBar.SetValue("")
	m.LocalSearchValue = ""
	m.syncTableDimensions()
}

func (m *BaseModel) ViewCompletions() string {
	return m.SearchBar.ViewCompletions()
}

// HeaderSearchView intentionally returns no search input. Search inputs are
// scoped to the active section body so they do not escape into the tabs row or
// preview/sidebar zones.
func (m *BaseModel) HeaderSearchView() string {
	return ""
}

func (m *BaseModel) ResetPageInfo() {
	m.PageInfo = nil
}

func (m *BaseModel) ToggleSortOrder() data.SearchSort {
	if m.SortOrder == data.SearchSortUpdated || m.SortOrder == "" {
		m.SortOrder = data.SearchSortCreated
	} else {
		m.SortOrder = data.SearchSortUpdated
	}
	return m.SortOrder
}

func (m *BaseModel) GetSortOrder() data.SearchSort {
	if m.SortOrder == "" {
		return data.SearchSortUpdated
	}
	return m.SortOrder
}

func (m *BaseModel) SortOrderLabel() string {
	if m.GetSortOrder() == data.SearchSortCreated {
		return "Newest created"
	}
	return "Recently updated"
}

func (m *BaseModel) SetColumnTitle(index int, title string) {
	if index >= 0 && index < len(m.Columns) {
		m.Columns[index].Title = title
	}
	if index >= 0 && index < len(m.Table.Columns) {
		m.Table.Columns[index].Title = title
	}
}

func (m *BaseModel) IsPromptConfirmationFocused() bool {
	return m.IsPromptConfirmationShown
}

func (m *BaseModel) SetIsPromptConfirmationShown(val bool) tea.Cmd {
	m.IsPromptConfirmationShown = val
	if val {
		m.PromptConfirmationBox.Focus()
		return m.PromptConfirmationBox.Init()
	}

	m.PromptConfirmationBox.Blur()
	return nil
}

func (m *BaseModel) SetPromptConfirmationAction(action string) {
	m.PromptConfirmationAction = action
}

func (m *BaseModel) GetPromptConfirmationAction() string {
	return m.PromptConfirmationAction
}

type SectionMsg struct {
	Id          int
	Type        string
	InternalMsg tea.Msg
}

func (m *BaseModel) MakeSectionCmd(cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}

	return func() tea.Msg {
		internalMsg := cmd()
		return SectionMsg{
			Id:          m.Id,
			Type:        m.Type,
			InternalMsg: internalMsg,
		}
	}
}

func (m *BaseModel) GetFilters() string {
	return m.GetSearchValue()
}

func (m *BaseModel) GetMainContent() string {
	if m.Table.Rows == nil {
		d := m.GetDimensions()
		return lipgloss.Place(
			d.Width,
			d.Height,
			lipgloss.Center,
			lipgloss.Center,

			fmt.Sprintf(
				"%s you can change the search query by pressing %s and submitting it with %s",
				lipgloss.NewStyle().Bold(true).Render(" Tip:"),
				m.Ctx.Styles.Section.KeyStyle.Render("/"),
				m.Ctx.Styles.Section.KeyStyle.Render("Enter"),
			),
		)
	} else {
		return m.Table.View()
	}
}

func (m *BaseModel) View() string {
	if m.localSearchVisible() {
		return m.Ctx.Styles.Section.ContainerStyle.
			Width(m.Ctx.MainContentWidth).
			Render(
				lipgloss.JoinVertical(
					lipgloss.Left,
					m.LocalSearchBar.View(m.Ctx),
					m.GetMainContent(),
				),
			)
	}
	if m.IsSearching {
		return m.Ctx.Styles.Section.ContainerStyle.
			Width(m.Ctx.MainContentWidth).
			Render(
				lipgloss.JoinVertical(
					lipgloss.Left,
					m.SearchBar.View(m.Ctx),
					m.GetMainContent(),
				),
			)
	}

	return m.Ctx.Styles.Section.ContainerStyle.
		Width(m.Ctx.MainContentWidth).
		Render(m.GetMainContent())
}

func (m *BaseModel) ResetRows() {
	m.Table.Rows = nil
	m.ResetPageInfo()
	m.Table.ResetCurrItem()
}

// RowsSelectionScroll describes the section's row list as a scrollable selection
// area, so each visible row can be selected independently. contentTop is the
// screen Y of the first content row of the section (top of the section view,
// below the tabs bar).
func (m *BaseModel) RowsSelectionScroll(contentTop int) selection.Scroll {
	originX := m.Ctx.Styles.Section.ContainerStyle.GetPaddingLeft()
	// The table renders its column header above the rows viewport, and a search
	// bar may be shown above that.
	headerY := contentTop + common.TableHeaderHeight
	if m.bodySearchVisible() {
		headerY += common.SearchHeight
	}
	return selection.Scroll{
		OriginX:       originX,
		OriginY:       headerY,
		Width:         m.Table.RowsWidth(),
		VisibleHeight: m.Table.RowsViewportHeight(),
		YOffset:       m.Table.RowsViewportYOffset(),
		Blocks:        m.Table.SelectionBlocks(),
	}
}

func (m *BaseModel) LastUpdated() time.Time {
	return m.Table.LastUpdated()
}

func (m *BaseModel) CreatedAt() time.Time {
	return m.Table.CreatedAt()
}

func (m *BaseModel) UpdateTotalItemsCount(count int) {
	m.Table.UpdateTotalItemsCount(count)
}

func (m *BaseModel) GetPromptConfirmation() string {
	if m.IsPromptConfirmationShown {
		var prompt string
		switch {
		case m.PromptConfirmationAction == "close" && m.Ctx.View == config.PRsView:
			prompt = "Are you sure you want to close this PR? (Y/n) "

		case m.PromptConfirmationAction == "reopen" && m.Ctx.View == config.PRsView:
			prompt = "Are you sure you want to reopen this PR? (Y/n) "

		case m.PromptConfirmationAction == "ready" && m.Ctx.View == config.PRsView:
			prompt = "Are you sure you want to mark this PR as ready? (Y/n) "

		case m.PromptConfirmationAction == "merge" && m.Ctx.View == config.PRsView:
			prompt = "Are you sure you want to merge this PR? (Y/n) "

		case m.PromptConfirmationAction == "update" && m.Ctx.View == config.PRsView:
			prompt = "Are you sure you want to update this PR? (Y/n) "

		case m.PromptConfirmationAction == "approveWorkflows" && m.Ctx.View == config.PRsView:
			prompt = "Are you sure you want to approve all workflows? (Y/n) "

		case m.PromptConfirmationAction == "close" && m.Ctx.View == config.IssuesView:
			prompt = "Are you sure you want to close this issue? (Y/n) "

		case m.PromptConfirmationAction == "reopen" && m.Ctx.View == config.IssuesView:
			prompt = "Are you sure you want to reopen this issue? (Y/n) "
		case m.PromptConfirmationAction == "done_all" && m.Ctx.View == config.NotificationsView:
			prompt = "Are you sure you want to mark all as done? (Y/n) "
		}

		m.PromptConfirmationBox.SetPrompt(prompt)

		return m.Ctx.Styles.ListViewPort.PagerStyle.Render(m.PromptConfirmationBox.View())
	}

	return ""
}
