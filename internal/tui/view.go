package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

const wideThreshold = 100

// View renders the full TUI frame.
func (m Model) View() string {
	innerW := m.width - 2
	if innerW < 20 {
		innerW = 20
	}

	// fixed rows: header(1)+sep(1)+search(1)+sep(1)+sep(1)+preview(1)+sep(1)+footer(1) = 8
	// border top+bottom = 2 → total fixed = 10
	listH := m.height - 10
	if listH < 3 {
		listH = 3
	}

	sep := dimStyle.Render(strings.Repeat("─", innerW))

	sections := []string{
		m.renderHeader(innerW),
		sep,
		m.renderSearch(innerW),
		sep,
		m.renderBody(innerW, listH),
		sep,
		m.renderPreviewLine(innerW),
		sep,
		m.renderFooter(innerW),
	}

	return borderStyle.Width(innerW).Render(strings.Join(sections, "\n"))
}

func (m Model) renderHeader(w int) string {
	title := titleStyle.Render("⚡ Jump SSH")
	stats := dimStyle.Render(fmt.Sprintf("%d hosts", len(m.hosts)))
	gap := w - lipgloss.Width(title) - lipgloss.Width(stats)
	if gap < 1 {
		gap = 1
	}
	return title + strings.Repeat(" ", gap) + stats
}

func (m Model) renderSearch(w int) string {
	prompt := labelStyle.Render("Search:")
	line := prompt + " " + m.input.View()
	if lipgloss.Width(line) > w {
		line = truncStr(line, w)
	}
	return line
}

func (m Model) renderBody(w, h int) string {
	if m.showPanel && m.panelHost != nil {
		if m.width >= wideThreshold {
			return m.renderWideBody(w, h)
		}
		return m.renderPanel(w, h)
	}
	return m.renderList(w, h)
}

// renderWideBody shows the result list on the left and action panel on the right.
func (m Model) renderWideBody(w, h int) string {
	listW := w * 58 / 100
	detailW := w - listW - 3 // 3 = " │ "

	listLines := strings.Split(m.renderList(listW, h), "\n")
	panelLines := strings.Split(m.renderPanel(detailW, h), "\n")

	for len(listLines) < h {
		listLines = append(listLines, "")
	}
	for len(panelLines) < h {
		panelLines = append(panelLines, "")
	}

	divider := dimStyle.Render("│")
	rows := make([]string, h)
	for i := 0; i < h; i++ {
		ll := padTo(listLines[i], listW)
		dl := ""
		if i < len(panelLines) {
			dl = panelLines[i]
		}
		rows[i] = ll + " " + divider + " " + dl
	}
	return strings.Join(rows, "\n")
}

// renderPanel renders the host action menu + host details below.
func (m Model) renderPanel(w, h int) string {
	if m.panelHost == nil {
		return strings.Repeat("\n", h-1)
	}
	host := m.panelHost
	lines := make([]string, 0, h)

	// ── action menu items ────────────────────────────────────────────────────
	for i, item := range hostMenuItems {
		selected := i == m.panelCursor
		cursor := "  "
		if selected {
			cursor = cursorGlyph + " "
		}

		var st lipgloss.Style
		switch item.action {
		case ActionConnect:
			st = selectedAliasStyle
		case actionCopy:
			st = previewCmdStyle
		case ActionEdit:
			st = cmdEditStyle
		case ActionDelete:
			st = cmdDeleteStyle
		default:
			st = dimStyle
		}

		label := item.icon + "  " + item.label
		if selected {
			label = st.Render(label)
		} else {
			label = dimStyle.Render(label)
		}
		lines = append(lines, cursor+label)
	}

	// ── separator ────────────────────────────────────────────────────────────
	lines = append(lines, dimStyle.Render(strings.Repeat("─", w)))

	// ── host detail fields ───────────────────────────────────────────────────
	type kv struct{ k, v string }
	fields := []kv{
		{"Alias", host.Alias},
		{"User", host.User},
		{"Host", host.HostName},
		{"Port", portOrDefault(host.Port)},
		{"Env", host.Env},
		{"Tags", host.TagString()},
		{"Desc", host.Description},
		{"Last used", formatAge(host.LastUsedAt)},
		{"Managed", boolStr(host.Managed)},
		{"Command", host.Command()},
	}

	labelW := 0
	for _, f := range fields {
		if len(f.k) > labelW {
			labelW = len(f.k)
		}
	}

	for _, f := range fields {
		if len(lines) >= h {
			break
		}
		if f.v == "" || f.v == "0" || f.v == "never" {
			continue
		}
		lines = append(lines, fmt.Sprintf("  %s  %s",
			labelStyle.Render(padRight(f.k, labelW)),
			truncStr(f.v, w-labelW-4),
		))
	}

	for len(lines) < h {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// renderList renders the scrollable result list, padded to exactly h rows.
func (m Model) renderList(w, h int) string {
	if len(m.rows) == 0 {
		return m.renderEmpty(w, h)
	}

	start := m.cursor - h + 1
	if start < 0 {
		start = 0
	}
	end := start + h
	if end > len(m.rows) {
		end = len(m.rows)
		start = end - h
		if start < 0 {
			start = 0
		}
	}

	lines := make([]string, 0, h)
	for i := start; i < end; i++ {
		lines = append(lines, m.renderListRow(m.rows[i], i == m.cursor, w))
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderListRow(row ListRow, selected bool, w int) string {
	if row.IsGroup {
		return renderGroupRow(row.GroupName, w)
	}
	if row.IsCommand {
		return m.renderCommandRow(row, selected, w)
	}
	return m.renderHostRow(row.Host, selected, w)
}

func renderGroupRow(name string, w int) string {
	prefix := "  " + name + "  "
	fill := w - len(prefix)
	if fill < 1 {
		fill = 1
	}
	return dimStyle.Render(prefix + strings.Repeat("─", fill))
}

func (m Model) renderCommandRow(row ListRow, selected bool, w int) string {
	cursor := "  "
	if selected {
		cursor = cursorGlyph + " "
	}

	var icon string
	var st lipgloss.Style
	switch row.Action {
	case ActionAdd:
		icon = "⊕"
		st = cmdAddStyle
	case ActionEdit:
		icon = "✎"
		st = cmdEditStyle
	case ActionDelete:
		icon = "✖"
		st = cmdDeleteStyle
	default:
		icon = "→"
		st = dimStyle
	}

	label := icon + "  " + row.Label
	if selected {
		label = st.Render(label)
	} else {
		label = dimStyle.Render(label)
	}
	return cursor + label
}

func (m Model) renderHostRow(h HostView, selected bool, w int) string {
	const aliasMaxW = 18
	const targetMaxW = 36
	const envMaxW = 6

	cursor := "  "
	if selected {
		cursor = cursorGlyph + " "
	}

	aliasStr := truncStr(h.Alias, aliasMaxW)
	if selected {
		aliasStr = selectedAliasStyle.Render(aliasStr)
	} else {
		aliasStr = dimStyle.Render(aliasStr)
	}
	aliasCol := padTo(aliasStr, aliasMaxW)

	used := 2 + aliasMaxW
	var cols []string
	cols = append(cols, cursor+aliasCol)

	if w-used >= targetMaxW+2 {
		tgt := truncStr(h.Target(), targetMaxW)
		cols = append(cols, padTo(dimStyle.Render(tgt), targetMaxW))
		used += 2 + targetMaxW
	}

	if w-used >= envMaxW+2 {
		cols = append(cols, padTo(renderEnv(h.Env), envMaxW))
	}

	row := strings.Join(cols, "  ")
	if selected {
		row = selectedRowBg.Render(padTo(row, w))
	}
	return row
}

func (m Model) renderEmpty(w, h int) string {
	var lines []string
	if m.input.Value() != "" {
		lines = append(lines,
			"",
			"  "+dimStyle.Render("No results for: "+m.input.Value()),
			"",
			"  "+dimStyle.Render("Commands: add · edit <query> · delete <query>"),
		)
	} else {
		lines = append(lines,
			"",
			"  "+dimStyle.Render("Start typing to search SSH hosts"),
			"",
			"  "+dimStyle.Render("Commands"),
			"  "+dimStyle.Render("  add              add a new host"),
			"  "+dimStyle.Render("  edit <query>     edit a host"),
			"  "+dimStyle.Render("  delete <query>   delete a host"),
		)
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return strings.Join(lines[:h], "\n")
}

func (m Model) renderPreviewLine(w int) string {
	if m.statusMsg != "" {
		if m.statusIsErr {
			return statusErrStyle.Render(truncStr(m.statusMsg, w))
		}
		return statusOkStyle.Render(truncStr(m.statusMsg, w))
	}
	if m.showPanel && m.panelHost != nil {
		item := hostMenuItems[m.panelCursor]
		return labelStyle.Render("Action: ") + previewCmdStyle.Render(item.icon+" "+item.label+" "+m.panelHost.Alias)
	}
	if m.cursor < len(m.rows) {
		row := m.rows[m.cursor]
		if row.IsCommand {
			return labelStyle.Render("Action: ") + previewCmdStyle.Render(row.Label)
		}
		return labelStyle.Render("Command: ") + previewCmdStyle.Render("$ "+truncStr(row.Host.Command(), w-11))
	}
	return dimStyle.Render("Command: —")
}

func (m Model) renderFooter(w int) string {
	var shortcuts []string
	if m.showPanel {
		shortcuts = []string{"↑↓ navigate", "↵ select", "Esc / Tab close"}
	} else {
		shortcuts = []string{"↵ connect", "Tab actions", "↑↓ navigate", "Esc quit"}
	}
	_ = w
	return dimStyle.Render(strings.Join(shortcuts, "   ·   "))
}

// ── helpers ──────────────────────────────────────────────────────────────────

func renderEnv(env string) string {
	switch env {
	case "prod", "production", "prd", "productie":
		return envProdStyle.Render("prod")
	case "acc", "acceptance", "acceptatie", "staging":
		return envAccStyle.Render("acc ")
	case "dev", "development", "ontwikkeling":
		return envDevStyle.Render("dev ")
	case "test", "tst":
		return envTestStyle.Render("tst ")
	default:
		return dimStyle.Render(env)
	}
}

func padRight(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}

func padTo(s string, w int) string {
	vw := lipgloss.Width(s)
	if vw >= w {
		return s
	}
	return s + strings.Repeat(" ", w-vw)
}

func truncStr(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxW {
		return s
	}
	if maxW <= 1 {
		return string(runes[:maxW])
	}
	return string(runes[:maxW-1]) + "…"
}

func portOrDefault(p string) string {
	if p == "" {
		return "22"
	}
	return p
}

func boolStr(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func formatAge(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Format("2006-01-02")
	}
}
