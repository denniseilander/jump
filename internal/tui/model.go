package tui

import (
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/denniseilander/jump/internal/search"
	"github.com/denniseilander/jump/internal/sshconfig"
	"github.com/denniseilander/jump/internal/store"
)

// HostView is a display-ready view model for a single SSH host.
type HostView struct {
	Alias       string
	HostName    string
	User        string
	Port        string
	Env         string
	App         string
	Client      string
	Tags        []string
	Description string
	LastUsedAt  time.Time
	UseCount    int
	Score       int
	Source      string
	Managed     bool
}

func (h HostView) Command() string { return "ssh " + h.Alias }

func (h HostView) Target() string {
	hn := h.HostName
	if hn == "" {
		hn = h.Alias
	}
	if h.User != "" {
		return h.User + "@" + hn
	}
	return hn
}

func (h HostView) TagString() string { return strings.Join(h.Tags, ", ") }

// ListRow is a single navigable entry — either a group header, a command, or an SSH host.
type ListRow struct {
	IsCommand bool
	IsGroup   bool
	GroupName string
	Action    Action
	Host      HostView
	Label     string
}

// menuItem is an entry in the host action panel.
type menuItem struct {
	icon   string
	label  string
	action Action
}

var hostMenuItems = []menuItem{
	{"↵", "Connect", ActionConnect},
	{"⎘", "Copy SSH command", actionCopy},
	{"✎", "Edit", ActionEdit},
	{"✖", "Delete", ActionDelete},
}

// Model is the bubbletea TUI model.
type Model struct {
	hosts       []sshconfig.Host
	history     store.History
	managedPath string

	input  textinput.Model
	rows   []ListRow
	cursor int

	// action panel (opened with Tab on a host row)
	showPanel   bool
	panelCursor int
	panelHost   *HostView

	width       int
	height      int
	statusMsg   string
	statusIsErr bool

	action      Action
	actionAlias string
}

// Result returns what the user chose when the TUI exited.
func (m Model) Result() Result {
	return Result{
		Action: m.action,
		Alias:  m.actionAlias,
		Query:  m.input.Value(),
	}
}

// firstHostCursor returns the index of the first selectable host row.
func (m *Model) firstHostCursor() int {
	for i, r := range m.rows {
		if !r.IsCommand && !r.IsGroup {
			return i
		}
	}
	return 0
}

// selectedHostView returns the HostView for the currently highlighted list row.
func (m Model) selectedHostView() *HostView {
	if m.cursor >= len(m.rows) {
		return nil
	}
	h := &m.rows[m.cursor].Host
	if h.Alias == "" {
		return nil
	}
	return h
}

// New creates an initial TUI model ready to run.
func New(hosts []sshconfig.Host, history store.History, managedPath, initialQuery string) Model {
	ti := textinput.New()
	ti.Placeholder = "type to search…"
	ti.CharLimit = 80
	ti.Focus()
	if initialQuery != "" {
		ti.SetValue(initialQuery)
	}

	m := Model{
		hosts:       hosts,
		history:     history,
		managedPath: managedPath,
		input:       ti,
		width:       80,
		height:      24,
	}
	m.refilter()
	m.cursor = m.firstHostCursor()
	return m
}

// parsedQuery holds the result of command-prefix detection.
type parsedQuery struct {
	action    Action
	hostQuery string
}

// detectCommand checks whether the query starts with a command keyword (or a
// recognisable prefix of ≥3 characters) and splits it into action + host fragment.
func detectCommand(raw string) (parsedQuery, bool) {
	q := strings.TrimSpace(strings.ToLower(raw))
	if len(q) < 2 {
		return parsedQuery{}, false
	}

	type entry struct {
		keywords []string
		action   Action
	}
	commands := []entry{
		{[]string{"add"}, ActionAdd},
		{[]string{"edit", "edi"}, ActionEdit},
		{[]string{"delete", "delet", "dele", "del"}, ActionDelete},
		{[]string{"rm"}, ActionDelete},
	}

	for _, cmd := range commands {
		for _, kw := range cmd.keywords {
			if q == kw {
				return parsedQuery{cmd.action, ""}, true
			}
			if strings.HasPrefix(q, kw+" ") {
				hostQ := strings.TrimSpace(raw[len(kw)+1:])
				return parsedQuery{cmd.action, hostQ}, true
			}
		}
	}

	return parsedQuery{}, false
}

func (m *Model) refilter() {
	raw := m.input.Value()
	rows := []ListRow{{IsCommand: true, Action: ActionAdd, Label: "Add new host"}}

	var views []HostView
	if cmd, ok := detectCommand(raw); ok {
		switch cmd.action {
		case ActionEdit, ActionDelete:
			views = toHostViews(search.Find(m.hosts, cmd.hostQuery), m.history, m.managedPath)
		}
	} else {
		views = toHostViews(search.Find(m.hosts, raw), m.history, m.managedPath)
	}

	rows = append(rows, groupAndSort(views)...)

	m.rows = rows
	if m.cursor >= len(m.rows) {
		if len(m.rows) > 0 {
			m.cursor = len(m.rows) - 1
		} else {
			m.cursor = 0
		}
	}
}

// clientOf returns the client name for grouping: client meta → app meta → alias prefix.
func clientOf(h HostView) string {
	if h.Client != "" {
		return h.Client
	}
	if h.App != "" {
		return h.App
	}
	if idx := strings.IndexByte(h.Alias, '-'); idx > 0 {
		return h.Alias[:idx]
	}
	return h.Alias
}

// envOrder maps environment names to a sort key (dev < tst < acc < prod).
func envOrder(env string) int {
	switch env {
	case "dev", "development", "ontwikkeling":
		return 0
	case "test", "tst":
		return 1
	case "acc", "acceptance", "acceptatie", "staging":
		return 2
	case "prod", "production", "prd", "productie":
		return 3
	default:
		return 4
	}
}

// groupAndSort sorts views by (client, env, alias) and inserts group header rows.
func groupAndSort(views []HostView) []ListRow {
	if len(views) == 0 {
		return nil
	}
	sort.Slice(views, func(i, j int) bool {
		ci, cj := clientOf(views[i]), clientOf(views[j])
		if ci != cj {
			return ci < cj
		}
		ei, ej := envOrder(views[i].Env), envOrder(views[j].Env)
		if ei != ej {
			return ei < ej
		}
		return views[i].Alias < views[j].Alias
	})

	var rows []ListRow
	var lastClient string
	for _, v := range views {
		c := clientOf(v)
		if c != lastClient {
			rows = append(rows, ListRow{IsGroup: true, GroupName: c})
			lastClient = c
		}
		rows = append(rows, ListRow{Host: v})
	}
	return rows
}

func toHostViews(results []search.Result, history store.History, managedPath string) []HostView {
	views := make([]HostView, len(results))
	for i, r := range results {
		h := r.Host
		e := history[h.Alias]
		views[i] = HostView{
			Alias:       h.Alias,
			HostName:    h.HostName,
			User:        h.User,
			Port:        h.Port,
			Env:         h.Meta["env"],
			App:         h.Meta["app"],
			Client:      h.Meta["client"],
			Tags:        parseTags(h.Meta["tags"]),
			Description: h.Meta["description"],
			LastUsedAt:  e.LastUsedAt,
			UseCount:    e.ConnectCount,
			Score:       r.Score,
			Source:      h.SourceFile,
			Managed:     h.SourceFile == managedPath,
		}
	}
	return views
}

func parseTags(raw string) []string {
	var out []string
	for _, t := range strings.Split(raw, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}
