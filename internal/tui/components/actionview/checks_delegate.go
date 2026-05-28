package actionview

import (
	"io"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/log/v2"

	data "github.com/dlvhdr/gh-dehub/v4/internal/data/actions"
)

type checkItem struct {
	jobItem
}

// Title implements github.com/charmbracelet/bubbles.list.DefaultItem.Title
func (ci *checkItem) Title() string {
	return ci.jobItem.Title()
}

// Description implements /github.com/charmbracelet/bubbles.list.DefaultItem.Description
func (ci *checkItem) Description() string {
	return ci.jobItem.Description()
}

// FilterValue implements /github.com/charmbracelet/bubbles.list.Item.FilterValue
func (ci *checkItem) FilterValue() string { return ci.job.Name }

// checksDelegate implements list.ItemDelegate
type checksDelegate struct {
	commonDelegate
}

func (d *checksDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	ri, ok := item.(*checkItem)
	if !ok {
		return
	}

	d.commonDelegate.Render(w, m, index, ri, &ri.meta)
}

// Update implements github.com/charmbracelet/bubbles.list.ItemDelegate.Update
func (d *checksDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	selected, ok := m.SelectedItem().(*checkItem)
	if !ok {
		return nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		log.Info("key pressed on check", "key", msg.Text)
		switch {
		case key.Matches(msg, openUrlKey):
			return makeOpenUrlCmd(selected.job.Link)
		}
	}

	return nil
}

func newCheckItemDelegate(styles styles) list.ItemDelegate {
	// The Checks pane intentionally keeps its selection rendered prominently
	// even when blurred, so the user can see which check's logs they are
	// reading. See itemMeta for the full rationale.
	d := checksDelegate{commonDelegate{styles: styles, prominentSelection: true}}
	return &d
}

func NewCheckItem(job data.WorkflowJob, styles styles) checkItem {
	return checkItem{
		jobItem: NewJobItem(job, styles),
	}
}
