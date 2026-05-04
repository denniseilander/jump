package output

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	bannerTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFB700"))

	bannerTaglineStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888"))

	bannerBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#FFB700")).
				Padding(0, 2)

	sectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00B4D8"))

	cmdStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#80FFDB")).
			Bold(true)

	flagStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD166"))

	exampleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))
)

// PrintUsage prints the styled help output.
func PrintUsage() {
	if Plain {
		printUsagePlain()
		return
	}

	// Banner
	title := bannerTitleStyle.Render("⚡ jump")
	tagline := bannerTaglineStyle.Render("native SSH launcher")
	banner := bannerBorderStyle.Render(title + "\n" + tagline)
	fmt.Fprintln(os.Stdout, banner)
	fmt.Fprintln(os.Stdout)

	section := func(s string) string { return sectionStyle.Render(s) }
	cmd := func(s string) string { return cmdStyle.Render(s) }
	flag := func(s string) string { return flagStyle.Render(s) }
	ex := func(s string) string { return "  " + exampleStyle.Render(s) }
	row := func(name, desc string, nameW int) string {
		return fmt.Sprintf("  %-*s  %s", nameW, name, desc)
	}

	// USAGE
	fmt.Fprintln(os.Stdout, section("USAGE"))
	fmt.Fprintln(os.Stdout, row(cmd("jump")+" [--print] [--json] [query...]", "search and connect", 0))
	fmt.Fprintln(os.Stdout)

	// COMMANDS — search & inspect
	fmt.Fprintln(os.Stdout, section("COMMANDS — search & inspect"))
	cmds := [][2]string{
		{"list [--json]", "list all known hosts, grouped by client"},
		{"show <alias>", "show host details"},
		{"scan [--json]", "scan SSH config files"},
		{"explain <query>", "explain search score"},
		{"aliases [--env e] [--tag t]", "print host aliases (scripting)"},
		{"ping <query>", "check TCP reachability"},
	}
	printCmdTable(cmds, cmd)

	// COMMANDS — connect
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, section("COMMANDS — connect"))
	cmds = [][2]string{
		{"-", "reconnect to last used host"},
		{"recent [n]", "list recent hosts; connect to nth"},
		{"history [--limit n]", "show full history"},
		{"copy <query>", "copy SSH command to clipboard"},
	}
	printCmdTable(cmds, cmd)

	// COMMANDS — manage
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, section("COMMANDS — manage"))
	cmds = [][2]string{
		{"init", "initialize managed SSH config"},
		{"add", "add a new managed SSH host"},
		{"bulk-add", "add multiple hosts from a template"},
		{"edit <alias>", "edit a managed host"},
		{"rename <old> <new>", "rename an alias"},
		{"delete <alias>", "delete a managed host"},
		{"tag <alias> <tag...>", "add tags to a host"},
		{"describe <alias> <text>", "set host description"},
		{"set-client <code> <name>", "set client name for all hosts with matching code"},
	}
	printCmdTable(cmds, cmd)

	// COMMANDS — config & tools
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, section("COMMANDS — config & tools"))
	cmds = [][2]string{
		{"config", "view and edit jump preferences"},
		{"doctor", "validate jump and SSH setup"},
		{"open-config [--managed|--ssh|...]", "open a config file in editor"},
	}
	printCmdTable(cmds, cmd)

	// FLAGS
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, section("FLAGS"))
	flags := [][2]string{
		{"--print", "print SSH command without executing"},
		{"--json", "machine-readable JSON output"},
		{"--plain", "disable colors and styling"},
		{"--no-color", "disable colors"},
		{"--limit <n>", "limit number of results shown"},
	}
	w := maxWidth(flags)
	for _, f := range flags {
		fmt.Fprintf(os.Stdout, "  %-*s  %s\n", w, flag(f[0]), f[1])
	}

	// EXAMPLES
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, section("EXAMPLES"))
	examples := []string{
		"jump myapp production        connect to myapp in production",
		"jump gateway acc             connect to acceptance gateway",
		"jump -                       reconnect to last used host",
		"jump copy myapp prod         copy SSH command to clipboard",
		"jump ping myapp-web-prod     check if host is reachable",
		"jump list --json             machine-readable host list",
		"jump doctor                  validate your setup",
	}
	for _, e := range examples {
		fmt.Fprintln(os.Stdout, ex(e))
	}
	fmt.Fprintln(os.Stdout)
}

func printCmdTable(cmds [][2]string, style func(string) string) {
	w := maxWidth(cmds)
	for _, c := range cmds {
		fmt.Fprintf(os.Stdout, "  %-*s  %s\n", w, style(c[0]), c[1])
	}
}

func maxWidth(rows [][2]string) int {
	w := 0
	for _, r := range rows {
		if len(r[0]) > w {
			w = len(r[0])
		}
	}
	return w
}

func printUsagePlain() {
	fmt.Fprint(os.Stdout, strings.TrimSpace(`
jump — native SSH launcher

USAGE
  jump [--print] [--json] [query terms...]   search and connect

COMMANDS
  list [--json]                         list all known hosts
  show <alias>                          show host details
  scan [--json]                         scan SSH config
  explain <query>                       explain search score
  aliases [--env e] [--tag t]           list host aliases
  ping <query>                          check TCP reachability
  - (dash)                              reconnect to last used host
  recent [n]                            list recent hosts; connect to nth
  history [--limit n]                   show full history
  copy <query>                          copy SSH command to clipboard
  init                                  initialize managed config
  config                                view and edit preferences
  add                                   add a new managed SSH host
  bulk-add                              add multiple hosts from a template
  edit <alias>                          edit a managed host
  rename <old> <new>                    rename an alias
  delete <alias>                        delete a managed host
  tag <alias> <tag...>                  add tags to a host
  describe <alias> <text>               set description
  set-client <code> <name>             set client name
  doctor                                validate setup
  open-config [--managed|--ssh|...]    open a config file

FLAGS
  --print      print SSH command without executing
  --json       machine-readable JSON output
  --plain      disable colors and styling
  --no-color   disable colors
  --limit <n>  limit number of results
`))
	fmt.Fprintln(os.Stdout)
}
