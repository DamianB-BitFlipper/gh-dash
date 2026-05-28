package actionview

import (
	"fmt"
	"io"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/log/v2"

	data "github.com/dlvhdr/gh-dehub/v4/internal/data/actions"
)

type runItem struct {
	meta      itemMeta
	run       *data.WorkflowRun
	jobsItems []*jobItem
	loading   bool
	spinner   spinner.Model
}

// Title implements /charm.land/bubbles.list.DefaultItem.Title
func (i *runItem) Title() string {
	return i.meta.renderTitleWithStatus(i.viewStatus(), i.run.Name)
}

// Description implements /charm.land/bubbles.list.DefaultItem.Description
func (i *runItem) Description() string {
	if i.run.Event == "" {
		if i.run.Workflow == "" {
			return "status check"
		}
		return i.run.Workflow
	}

	return fmt.Sprintf("on: %s", i.run.Event)
}

// FilterValue implements /charm.land/bubbles.list.Item.FilterValue
func (i *runItem) FilterValue() string { return i.run.Name }

func (i *runItem) IsInProgress() bool {
	numPending := 0
	for _, ji := range i.jobsItems {
		if ji.isStatusInProgress() {
			numPending++
		}
	}
	return numPending > 0
}

func (i *runItem) viewStatus() string {
	s := i.meta.TitleStyle()

	if i.IsInProgress() {
		return i.spinner.View()
	}

	return bucketToIcon(i.run.Bucket, s, i.meta.styles)
}

func (ri *runItem) Tick() tea.Cmd {
	if ri.IsInProgress() {
		return ri.spinner.Tick
	}

	return nil
}

// runsDelegate implements list.ItemDelegate
type runsDelegate struct {
	commonDelegate
}

func (d *runsDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	ri, ok := item.(*runItem)
	if !ok {
		return
	}

	d.commonDelegate.Render(w, m, index, ri, &ri.meta)
}

// Update implements charm.land/bubbles.list.ItemDelegate.Update
func (d *runsDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	selected, ok := m.SelectedItem().(*runItem)
	if !ok {
		return nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		log.Info("key pressed on run", "key", msg.Text)
		switch {
		case key.Matches(msg, openUrlKey):
			return makeOpenUrlCmd(selected.run.Link)
		}
	}

	return nil
}

func newRunItemDelegate(styles styles) list.ItemDelegate {
	// The Runs pane follows the conventional pattern: its selection is
	// rendered prominently only when the pane has focus. setFocusedPaneStyles
	// flips prominentSelection at runtime as the user navigates between
	// panes.
	d := runsDelegate{commonDelegate{styles: styles}}
	return &d
}

func NewRunItem(run data.WorkflowRun, styles styles) runItem {
	jobs := make([]*jobItem, 0)
	for _, job := range run.Jobs {
		ji := NewJobItem(job, styles)
		jobs = append(jobs, &ji)
	}

	return runItem{
		meta:      itemMeta{styles: styles},
		run:       &run,
		jobsItems: jobs,
		loading:   true,
		spinner:   NewClockSpinner(styles),
	}
}
