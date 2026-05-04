package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/denniseilander/jump/internal/sshconfig"
	"github.com/denniseilander/jump/internal/store"
)

// Action identifies what the user wants to do after the TUI closes.
type Action string

const (
	ActionConnect Action = "connect"
	ActionAdd     Action = "add"
	ActionEdit    Action = "edit"
	ActionDelete  Action = "delete"
	ActionQuit    Action = ""

	// actionCopy is handled internally (no TUI exit needed).
	actionCopy Action = "_copy"
)

// Result is returned by Run after the TUI exits.
type Result struct {
	Action Action
	Alias  string // set for connect / edit / delete
	Query  string // current search input — preserved for loop re-open
}

// Run opens the full-screen TUI picker and returns the user's chosen action.
func Run(hosts []sshconfig.Host, history store.History, managedPath, initialQuery string) (Result, error) {
	m := New(hosts, history, managedPath, initialQuery)
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return Result{}, err
	}
	return final.(Model).Result(), nil
}
