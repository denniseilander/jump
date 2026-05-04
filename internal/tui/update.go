package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/denniseilander/jump/internal/clip"
)

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// ── action panel is open ──────────────────────────────────────────────
		if m.showPanel {
			switch msg.Type {
			case tea.KeyCtrlC:
				return m, tea.Quit

			case tea.KeyEsc, tea.KeyTab:
				m.showPanel = false
				m.panelHost = nil
				return m, nil

			case tea.KeyUp:
				if m.panelCursor > 0 {
					m.panelCursor--
				}
				return m, nil

			case tea.KeyDown:
				if m.panelCursor < len(hostMenuItems)-1 {
					m.panelCursor++
				}
				return m, nil

			case tea.KeyEnter:
				if m.panelHost == nil {
					return m, nil
				}
				item := hostMenuItems[m.panelCursor]
				switch item.action {
				case actionCopy:
					cmd := m.panelHost.Command()
					if err := clip.Copy(cmd); err != nil {
						m.statusMsg = "copy failed: " + err.Error()
						m.statusIsErr = true
					} else {
						m.statusMsg = "Copied: " + cmd
						m.statusIsErr = false
					}
					m.showPanel = false
					m.panelHost = nil
					return m, nil
				default:
					m.action = item.action
					m.actionAlias = m.panelHost.Alias
					return m, tea.Quit
				}
			}
			// swallow all other keys while panel is open
			return m, nil
		}

		// ── normal list mode ──────────────────────────────────────────────────
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit

		case tea.KeyEsc:
			if m.input.Value() != "" {
				m.input.SetValue("")
				m.refilter()
				m.cursor = 0
				m.statusMsg = ""
				return m, nil
			}
			return m, tea.Quit

		case tea.KeyEnter:
			if m.cursor < len(m.rows) {
				row := m.rows[m.cursor]
				if row.IsCommand {
					m.action = row.Action
					m.actionAlias = row.Host.Alias
				} else {
					m.action = ActionConnect
					m.actionAlias = row.Host.Alias
				}
			}
			return m, tea.Quit

		case tea.KeyUp:
			for m.cursor > 0 {
				m.cursor--
				if !m.rows[m.cursor].IsGroup {
					break
				}
			}
			m.statusMsg = ""
			return m, nil

		case tea.KeyDown:
			for m.cursor < len(m.rows)-1 {
				m.cursor++
				if !m.rows[m.cursor].IsGroup {
					break
				}
			}
			m.statusMsg = ""
			return m, nil

		case tea.KeyTab:
			// open action panel for the selected host row
			if host := m.selectedHostView(); host != nil && !m.rows[m.cursor].IsCommand {
				h := *host
				m.panelHost = &h
				m.panelCursor = 0
				m.showPanel = true
			}
			return m, nil
		}

		// q quits when search is empty; all other single chars type freely
		if msg.String() == "q" && m.input.Value() == "" {
			return m, tea.Quit
		}
	}

	// pass all remaining messages to the text input
	prevVal := m.input.Value()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.input.Value() != prevVal {
		m.refilter()
		m.cursor = m.firstHostCursor()
		m.statusMsg = ""
		m.showPanel = false // close panel when query changes
		m.panelHost = nil
	}
	return m, cmd
}
