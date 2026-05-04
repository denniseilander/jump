package output

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/denniseilander/jump/internal/search"
	"github.com/denniseilander/jump/internal/sshconfig"
	"github.com/denniseilander/jump/internal/store"
)

// Plain disables all styling when set to true (--plain / --no-color / non-TTY).
var Plain bool

var (
	headingStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00B4D8"))
	headerStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00B4D8"))
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	aliasStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#80FFDB"))
	labelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD166"))
	hintStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#80FFDB"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFB700")).Bold(true)
	okStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#80FFDB"))
	failStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B"))
)

func heading(s string) string {
	if Plain {
		return s
	}
	return headingStyle.Render(s)
}

func dim(s string) string {
	if Plain {
		return s
	}
	return dimStyle.Render(s)
}

func label(s string) string {
	if Plain {
		return s
	}
	return labelStyle.Render(s)
}

func alias(s string) string {
	if Plain {
		return s
	}
	return aliasStyle.Render(s)
}

func hint(s string) string {
	if Plain {
		return s
	}
	return hintStyle.Render(s)
}

// ScanSummary prints the jump scan output.
func ScanSummary(hosts []sshconfig.Host, includedFiles []string, managedPath, metaPath, historyPath string) {
	managed := 0
	for _, h := range hosts {
		if h.SourceFile == managedPath {
			managed++
		}
	}

	fmt.Fprintln(os.Stdout, heading("Jump scan complete"))
	fmt.Fprintln(os.Stdout)

	rows := [][2]string{
		{"SSH config", sshconfig.DefaultConfigPath()},
		{"Includes", fmt.Sprintf("%d files", len(includedFiles))},
		{"Hosts found", fmt.Sprintf("%d", len(hosts))},
		{"Managed hosts", fmt.Sprintf("%d", managed)},
		{"Metadata", metaPath},
		{"History", historyPath},
	}

	labelW := 0
	for _, r := range rows {
		if len(r[0]) > labelW {
			labelW = len(r[0])
		}
	}

	for _, r := range rows {
		fmt.Fprintf(os.Stdout, "  %-*s  %s\n", labelW, label(r[0]), r[1])
	}
}

// Hosts prints a table of SSH hosts, grouped by client when metadata is present.
func Hosts(results []search.Result) {
	fmt.Fprintln(os.Stdout, heading("JUMP SSH HOSTS"))
	fmt.Fprintln(os.Stdout)

	// check if any result has app metadata → grouped view
	hasGroups := false
	for _, r := range results {
		if r.Host.Meta["app"] != "" {
			hasGroups = true
			break
		}
	}

	if !hasGroups {
		hostsFlat(results)
		return
	}

	// group by app code, preserve insertion order via ordered keys
	type group struct {
		app    string
		client string
		items  []search.Result
	}
	groupMap := map[string]*group{}
	var order []string
	for _, r := range results {
		app := r.Host.Meta["app"]
		if app == "" {
			app = "~other"
		}
		if _, ok := groupMap[app]; !ok {
			clientName := r.Host.Meta["client"]
			groupMap[app] = &group{app: app, client: clientName}
			order = append(order, app)
		}
		if groupMap[app].client == "" && r.Host.Meta["client"] != "" {
			groupMap[app].client = r.Host.Meta["client"]
		}
		groupMap[app].items = append(groupMap[app].items, r)
	}
	sort.Slice(order, func(i, j int) bool {
		return order[i] < order[j]
	})

	for _, app := range order {
		g := groupMap[app]
		header := strings.ToUpper(app)
		if g.client != "" {
			header = g.client + " " + dim("("+app+")")
		}
		fmt.Fprintf(os.Stdout, "%s\n\n", heading(header))

		t := NewTable("Alias", "User", "Hostname", "Port", "Tags")
		for _, r := range g.items {
			h := r.Host
			port := h.Port
			if port == "" {
				port = "22"
			}
			t.Row(alias(h.Alias), h.User, h.HostName, port, tagsFrom(h))
		}
		t.Render(os.Stdout)
		fmt.Fprintln(os.Stdout)
	}

	fmt.Fprintf(os.Stdout, "%s\n", dim(fmt.Sprintf("%d hosts found", len(results))))
}

func hostsFlat(results []search.Result) {
	t := NewTable("Alias", "User", "Hostname", "Port", "Tags")
	for _, r := range results {
		h := r.Host
		port := h.Port
		if port == "" {
			port = "22"
		}
		t.Row(alias(h.Alias), h.User, h.HostName, port, tagsFrom(h))
	}
	t.Render(os.Stdout)
	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(os.Stdout, "%s\n", dim(fmt.Sprintf("%d hosts found", len(results))))
}

// Matches prints a numbered picker table for multiple search results.
func Matches(results []search.Result, limit int) {
	if limit <= 0 || limit > len(results) {
		limit = len(results)
	}
	fmt.Fprintln(os.Stdout, heading("Multiple matches found"))
	fmt.Fprintln(os.Stdout)

	t := NewTable("#", "Alias", "Target", "Tags")
	for i := 0; i < limit; i++ {
		h := results[i].Host
		t.Row(
			fmt.Sprintf("%d", i+1),
			alias(h.Alias),
			target(h),
			tagsFrom(h),
		)
	}
	t.Render(os.Stdout)
}

// BestMatch prints a summary of the chosen host before connecting.
func BestMatch(result search.Result) {
	h := result.Host
	fmt.Fprintf(os.Stdout, "%s %s  %s %s\n", heading("Connecting to"), alias(h.Alias), dim("→"), hint(target(h)))
	if desc := h.Meta["description"]; desc != "" {
		fmt.Fprintf(os.Stdout, "%s\n", dim(desc))
	}
	fmt.Fprintf(os.Stdout, "%s %s\n", dim("↳"), dim("ssh "+h.Alias))
}

// PrintMatch prints full details for --print mode.
func PrintMatch(result search.Result) {
	h := result.Host

	fmt.Fprintln(os.Stdout, heading("Best match"))
	fmt.Fprintln(os.Stdout)

	fields := [][2]string{
		{"Alias", h.Alias},
		{"User", h.User},
		{"Host", h.HostName},
		{"Port", portOrDefault(h.Port)},
		{"Tags", tagsFrom(h)},
		{"Score", fmt.Sprintf("%d", result.Score)},
	}
	printFields(fields)

	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, heading("Command"))
	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(os.Stdout, "  %s\n", hint("ssh "+h.Alias))
}

// PrintMatchTable prints a table of matches for --print mode with multiple results.
func PrintMatchTable(results []search.Result, limit int) {
	if limit <= 0 || limit > len(results) {
		limit = len(results)
	}
	fmt.Fprintln(os.Stdout, heading("Multiple matches found"))
	fmt.Fprintln(os.Stdout)

	t := NewTable("#", "Alias", "Target", "Tags")
	for i := 0; i < limit; i++ {
		h := results[i].Host
		t.Row(
			fmt.Sprintf("%d", i+1),
			alias(h.Alias),
			target(h),
			tagsFrom(h),
		)
	}
	t.Render(os.Stdout)
	fmt.Fprintln(os.Stdout)

	fmt.Fprintln(os.Stdout, hint("Run one of:"))
	fmt.Fprintln(os.Stdout)
	for i := 0; i < limit && i < 3; i++ {
		fmt.Fprintf(os.Stdout, "  jump %s\n", results[i].Host.Alias)
	}
}

// NoMatches prints an actionable error for zero search results.
func NoMatches(query string) {
	if query == "" {
		fmt.Fprintln(os.Stderr, "jump: no SSH hosts found")
	} else {
		fmt.Fprintf(os.Stderr, "jump: no hosts matched: %s\n", query)
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, hint("Try:"))
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "  jump list")
	if query != "" {
		fmt.Fprintln(os.Stderr, "  jump scan")
	}
	os.Exit(1)
}

// NoSSHConfig prints a helpful error when no SSH config is found.
func NoSSHConfig() {
	fmt.Fprintln(os.Stderr, "jump: no SSH config found at ~/.ssh/config")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, hint("Run:"))
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "  jump init")
	os.Exit(1)
}

// ShowDetails prints host details for jump show.
func ShowDetails(h sshconfig.Host) {
	fields := [][2]string{
		{"Alias", h.Alias},
		{"HostName", h.HostName},
		{"User", h.User},
		{"Port", portOrDefault(h.Port)},
		{"Identity", h.Identity},
		{"Source", fmt.Sprintf("%s:%d", h.SourceFile, h.Line)},
	}
	if len(h.Meta) > 0 {
		fields = append(fields,
			[2]string{"App", h.Meta["app"]},
			[2]string{"Env", h.Meta["env"]},
			[2]string{"Tags", tagsFrom(h)},
			[2]string{"Description", h.Meta["description"]},
		)
	}
	printFields(fields)
}

// Explain prints a search score breakdown.
func Explain(results []search.DetailedResult, query string) {
	if len(results) == 0 {
		fmt.Fprintf(os.Stderr, "jump: no hosts matched: %s\n", query)
		return
	}

	top := results[0]
	fmt.Fprintf(os.Stdout, "%s %s\n", heading("Best match:"), alias(top.Host.Alias))
	fmt.Fprintln(os.Stdout)

	if len(top.Reasons) > 0 {
		fmt.Fprintln(os.Stdout, heading("Matched query tokens"))
		fmt.Fprintln(os.Stdout)
		t := NewTable("Token", "Field", "Score")
		for _, r := range top.Reasons {
			t.Row(r.Token, r.Field, fmt.Sprintf("+%d", r.Score))
		}
		t.Render(os.Stdout)
		fmt.Fprintln(os.Stdout)
	}

	fmt.Fprintf(os.Stdout, "  Total score: %s\n", heading(fmt.Sprintf("%d", top.Score)))
}

// History prints recently used hosts.
func History(entries []store.HistoryEntry, results []search.Result) {
	fmt.Fprintln(os.Stdout, heading("RECENT CONNECTIONS"))
	fmt.Fprintln(os.Stdout)

	t := NewTable("#", "Alias", "Target", "Last used", "Count")
	for i, e := range entries {
		var tgt string
		if i < len(results) {
			tgt = target(results[i].Host)
		}
		t.Row(
			fmt.Sprintf("%d", i+1),
			alias(e.Alias),
			tgt,
			formatAge(e.LastUsedAt),
			fmt.Sprintf("%d", e.ConnectCount),
		)
	}
	t.Render(os.Stdout)
	fmt.Fprintln(os.Stdout)
}

func formatAge(t time.Time) string {
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

// helpers

func printFields(fields [][2]string) {
	labelW := 0
	for _, f := range fields {
		if len(f[0]) > labelW {
			labelW = len(f[0])
		}
	}
	for _, f := range fields {
		if f[1] != "" {
			fmt.Fprintf(os.Stdout, "  %-*s  %s\n", labelW, label(f[0]), f[1])
		}
	}
}

func tagsFrom(h sshconfig.Host) string {
	raw := h.Meta["tags"]
	if raw == "" {
		return ""
	}
	parts := strings.Split(raw, ",")
	var tags []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			tags = append(tags, p)
		}
	}
	return strings.Join(tags, ", ")
}

func target(h sshconfig.Host) string {
	hn := h.HostName
	if hn == "" {
		hn = h.Alias
	}
	if h.User != "" {
		return h.User + "@" + hn
	}
	return hn
}

func portOrDefault(p string) string {
	if p == "" {
		return "22"
	}
	return p
}

// Success prints a styled success line.
func Success(msg string) {
	if Plain {
		fmt.Fprintln(os.Stdout, msg)
		return
	}
	fmt.Fprintln(os.Stdout, successStyle.Render(msg))
}

// Section prints a styled section header.
func Section(msg string) {
	if Plain {
		fmt.Fprintln(os.Stdout, msg)
		return
	}
	fmt.Fprintln(os.Stdout, headingStyle.Render(msg))
}

// DocCheck prints a single doctor check line with colored ✓/✗.
func DocCheck(lbl string, ok bool, msg string) {
	if Plain {
		icon := "✓"
		if !ok {
			icon = "✗"
		}
		line := fmt.Sprintf("  %s  %s", icon, lbl)
		if msg != "" && !ok {
			line += ": " + msg
		}
		fmt.Fprintln(os.Stdout, line)
		return
	}
	var icon string
	if ok {
		icon = okStyle.Render("✓")
	} else {
		icon = failStyle.Render("✗")
	}
	line := fmt.Sprintf("  %s  %s", icon, lbl)
	if msg != "" && !ok {
		line += dim(": "+msg)
	}
	fmt.Fprintln(os.Stdout, line)
}

// CopySuccess prints a styled clipboard-copy confirmation.
func CopySuccess(cmd string) {
	if Plain {
		fmt.Printf("Copied to clipboard:\n\n  %s\n", cmd)
		return
	}
	fmt.Fprintln(os.Stdout, heading("Copied to clipboard"))
	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(os.Stdout, "  %s\n", hint(cmd))
}

// PingReachable prints a styled ping success line.
func PingReachable(addr string, ms int64) {
	if Plain {
		fmt.Printf("reachable  %s  %dms\n", addr, ms)
		return
	}
	fmt.Fprintf(os.Stdout, "%s  %s  %s\n",
		okStyle.Render("reachable"),
		label(addr),
		dim(fmt.Sprintf("%dms", ms)),
	)
}

// PingTarget prints "Pinging alias (addr)..." with styling.
func PingTarget(al, addr string) {
	if Plain {
		fmt.Printf("Pinging %s (%s)...\n", al, addr)
		return
	}
	fmt.Fprintf(os.Stdout, "%s %s %s\n",
		dim("Pinging"),
		alias(al),
		dim("("+addr+")..."),
	)
}
