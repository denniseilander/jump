package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/denniseilander/jump/internal/clip"
	"github.com/denniseilander/jump/internal/output"
	"github.com/denniseilander/jump/internal/runner"
	"github.com/denniseilander/jump/internal/search"
	"github.com/denniseilander/jump/internal/sshconfig"
	"github.com/denniseilander/jump/internal/store"
	"github.com/denniseilander/jump/internal/tui"
)

var stdin = bufio.NewReader(os.Stdin)

func main() {
	printOnly := flag.Bool("print", false, "print the ssh command instead of executing it")
	limit := flag.Int("limit", 20, "maximum number of results to show")
	pick := flag.Bool("pick", false, "always open the TUI picker")
	noTUI := flag.Bool("no-tui", false, "disable TUI, use classic CLI mode")
	// json/plain/no-color are also scanned directly from os.Args so they work
	// after a subcommand (e.g. "jump list --json").
	flag.Bool("json", false, "machine-readable JSON output")
	flag.Bool("plain", false, "disable colors and styling")
	flag.Bool("no-color", false, "disable colors")
	if hasFlag("plain") || hasFlag("no-color") {
		output.Plain = true
	}
	flag.Usage = usage
	flag.Parse()

	jsonOut := hasFlag("json")

	args := flag.Args()
	if len(args) == 1 && args[0] == "-" {
		mustLast(*printOnly)
		return
	}
	if len(args) > 0 {
		switch args[0] {
		case "tui":
			mustTUI(args[1:], *printOnly)
			return
		case "list":
			mustList(*limit, jsonOut)
			return
		case "scan":
			mustScan(jsonOut)
			return
		case "show":
			if len(args) < 2 {
				fatal("usage: jump show <alias>")
			}
			mustShow(args[1])
			return
		case "explain":
			if len(args) < 2 {
				fatal("usage: jump explain <query>")
			}
			mustExplain(strings.Join(args[1:], " "))
			return
		case "init":
			mustInit()
			return
		case "config":
			mustConfig()
			return
		case "add":
			mustAdd()
			return
		case "bulk-add":
			mustBulkAdd()
			return
		case "set-client":
			if len(args) < 3 {
				fatal("usage: jump set-client <code> <name>")
			}
			mustSetClient(args[1], strings.Join(args[2:], " "))
			return
		case "rename":
			if len(args) < 3 {
				fatal("usage: jump rename <old> <new>")
			}
			mustRename(args[1], args[2])
			return
		case "history":
			mustHistory(*limit, jsonOut)
			return
		case "recent":
			mustRecent(args[1:], *printOnly, jsonOut)
			return
		case "doctor":
			mustDoctor()
			return
		case "copy":
			mustCopy(strings.Join(args[1:], " "), *limit)
			return
		case "aliases":
			mustAliases(args[1:], jsonOut)
			return
		case "open-config":
			mustOpenConfig(args[1:])
			return
		case "ping":
			if len(args) < 2 {
				fatal("usage: jump ping <alias>")
			}
			mustPing(args[1])
			return
		case "edit":
			if len(args) < 2 {
				fatal("usage: jump edit <alias>")
			}
			mustEdit(args[1])
			return
		case "delete", "rm", "remove":
			if len(args) < 2 {
				fatal("usage: jump delete <alias>")
			}
			mustDelete(args[1])
			return
		case "tag":
			if len(args) < 3 {
				fatal("usage: jump tag <alias> <tag> [tag...]")
			}
			mustTag(args[1], args[2:])
			return
		case "describe":
			if len(args) < 3 {
				fatal("usage: jump describe <alias> <description>")
			}
			mustDescribe(args[1], strings.Join(args[2:], " "))
			return
		case "help", "--help", "-h":
			usage()
			return
		}
	}

	query := strings.Join(args, " ")
	mustJump(query, *limit, *printOnly, jsonOut, *pick, *noTUI)
}

func mustJump(query string, limit int, printOnly bool, jsonOut bool, pick bool, noTUI bool) {
	hosts, err := sshconfig.ParseDefault()
	if err != nil {
		fatal(err.Error())
	}
	results := search.Find(hosts, query)

	if jsonOut {
		_ = output.HostsJSON(results, query)
		return
	}
	if len(results) == 0 && noTUI {
		output.NoMatches(query)
	}

	if printOnly {
		if len(results) == 1 || (len(results) > 1 && results[0].Score-results[1].Score >= 40) {
			output.PrintMatch(results[0])
		} else {
			output.PrintMatchTable(results, limit)
		}
		return
	}

	// strong single match without --pick: connect directly
	strongMatch := len(results) == 1 ||
		(len(results) > 1 && results[0].Score-results[1].Score >= 40)
	if strongMatch && !pick && query != "" {
		connectToAlias(results[0].Host.Alias, hosts, false)
		return
	}

	// open TUI unless --no-tui
	if !noTUI {
		runTUILoop(query, printOnly)
		return
	}

	// classic CLI fallback (--no-tui)
	if len(results) == 0 {
		output.NoMatches(query)
	}
	chosen := promptPicker(results, limit)
	connectToAlias(chosen.Host.Alias, hosts, false)
}

// connectToAlias records history, prints best-match, and runs SSH.
func connectToAlias(alias string, hosts []sshconfig.Host, printOnly bool) {
	if err := store.Record(alias); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not update history: %v\n", err)
	}
	cfg := store.LoadConfig()
	timeout := cfg.ConnectTimeout
	if timeout <= 0 {
		timeout = store.DefaultConnectTimeout
	}
	for _, h := range hosts {
		if h.Alias == alias {
			output.BestMatch(search.Result{Host: h})
			break
		}
	}
	if err := runner.SSH(alias, printOnly, timeout); err != nil {
		fatal(err.Error())
	}
}

// mustTUI opens the TUI directly, optionally with a prefilled query.
func mustTUI(args []string, printOnly bool) {
	runTUILoop(strings.Join(args, " "), printOnly)
}

// runTUILoop runs the TUI and handles the returned action, looping back into
// the TUI after management commands so the user stays in context.
func runTUILoop(initialQuery string, printOnly bool) {
	query := initialQuery
	for {
		hosts, err := sshconfig.ParseDefault()
		if err != nil {
			fatal(err.Error())
		}
		history := store.Load()
		managedPath, _ := sshconfig.ManagedConfigPath()

		result, err := tui.Run(hosts, history, managedPath, query)
		if err != nil {
			fatal(err.Error())
		}

		query = result.Query // preserve search across iterations

		switch result.Action {
		case tui.ActionConnect:
			connectToAlias(result.Alias, hosts, printOnly)
			return

		case tui.ActionAdd:
			fmt.Println()
			mustAdd()

		case tui.ActionEdit:
			fmt.Println()
			mustEdit(result.Alias)

		case tui.ActionDelete:
			fmt.Println()
			mustDelete(result.Alias)

		default: // ActionQuit
			return
		}
		// after a management action, loop back and reopen the TUI
	}
}

func mustList(limit int, jsonOut bool) {
	hosts, err := sshconfig.ParseDefault()
	if err != nil {
		fatal(err.Error())
	}
	results := search.Find(hosts, "")
	if limit > 0 && limit < len(results) {
		results = results[:limit]
	}

	if jsonOut {
		_ = output.HostsJSON(results, "")
		return
	}
	output.Hosts(results)
}

func mustScan(jsonOut bool) {
	result, err := sshconfig.ScanDefault()
	if err != nil {
		fatal(err.Error())
	}

	managedPath, _ := sshconfig.ManagedConfigPath()
	metaPath, _ := metadataPathStr()
	historyPath, _ := store.Path()

	if jsonOut {
		_ = output.ScanJSON(result.Hosts, result.IncludedFiles, managedPath, metaPath, historyPath)
		return
	}
	output.ScanSummary(result.Hosts, result.IncludedFiles, managedPath, metaPath, historyPath)
}

func mustShow(alias string) {
	hosts, err := sshconfig.ParseDefault()
	if err != nil {
		fatal(err.Error())
	}
	for _, h := range hosts {
		if h.Alias == alias {
			output.ShowDetails(h)
			return
		}
	}
	fatal("alias not found")
}

func mustExplain(query string) {
	hosts, err := sshconfig.ParseDefault()
	if err != nil {
		fatal(err.Error())
	}
	results := search.FindWithReasons(hosts, query)
	output.Explain(results, query)
}

func mustInit() {
	if err := sshconfig.InitConfig(); err != nil {
		fatal(err.Error())
	}
	output.Success("jump: managed config initialized")
}

func mustConfig() {
	cfg := store.LoadConfig()

	output.Section("jump config — current settings")
	fmt.Println("(press enter to keep current value)")
	fmt.Println()

	cfg.DefaultIdentity = promptString(fmt.Sprintf("Default IdentityFile [%s]: ", cfg.DefaultIdentity), cfg.DefaultIdentity)
	cfg.DefaultUser = promptString(fmt.Sprintf("Default User [%s]: ", cfg.DefaultUser), cfg.DefaultUser)
	defaultPort := cfg.DefaultPort
	if defaultPort == "" {
		defaultPort = "22"
	}
	cfg.DefaultPort = promptString(fmt.Sprintf("Default Port [%s]: ", defaultPort), defaultPort)
	if cfg.DefaultPort == "22" {
		cfg.DefaultPort = ""
	}
	currentTimeout := cfg.ConnectTimeout
	if currentTimeout <= 0 {
		currentTimeout = store.DefaultConnectTimeout
	}
	cfg.ConnectTimeout = promptInt(fmt.Sprintf("Connect timeout in seconds [%d]: ", currentTimeout), currentTimeout)
	if cfg.ConnectTimeout == store.DefaultConnectTimeout {
		cfg.ConnectTimeout = 0
	}

	if err := store.SaveConfig(cfg); err != nil {
		fatal(err.Error())
	}
	fmt.Println()
	output.Success("jump: config saved")
}

func mustAdd() {
	output.Section("Add a new SSH host to jump's managed config.")
	fmt.Println()

	clientCode := strings.ToLower(promptRequired("App/project code (e.g. myapp, api): "))
	clientName := promptString("Project name (optional, e.g. My Project): ", "")
	env := promptEnum("Environment [prod/acc/dev/test]: ", []string{"prod", "acc", "dev", "test"})
	service := strings.ToLower(promptString("Service/role (optional, e.g. gateway, web, db): ", ""))

	alias, _, autoDesc, meta := buildHostParts(clientCode, clientName, env, service)
	fmt.Printf("  → alias: %q\n\n", alias)

	hosts, err := sshconfig.ParseDefault()
	if err != nil {
		fatal(err.Error())
	}
	for _, h := range hosts {
		if h.Alias == alias {
			fatal(fmt.Sprintf("alias %q already exists in %s", alias, h.SourceFile))
		}
	}

	cfg := store.LoadConfig()

	hostname := promptRequired("HostName: ")
	user := promptString(fmt.Sprintf("User [%s]: ", cfg.DefaultUser), cfg.DefaultUser)
	defaultPort := cfg.DefaultPort
	if defaultPort == "" {
		defaultPort = "22"
	}
	port := promptString(fmt.Sprintf("Port [%s]: ", defaultPort), defaultPort)
	identity := promptString(fmt.Sprintf("IdentityFile [%s]: ", cfg.DefaultIdentity), cfg.DefaultIdentity)
	meta["description"] = promptString(fmt.Sprintf("Description [%s]: ", autoDesc), autoDesc)

	h := sshconfig.Host{
		Alias:    alias,
		HostName: hostname,
		User:     user,
		Port:     port,
		Identity: identity,
		Meta:     meta,
	}

	if err := sshconfig.WriteHost(h); err != nil {
		fatal(err.Error())
	}
	output.Success(fmt.Sprintf("jump: added host %q", alias))
}

func mustBulkAdd() {
	output.Section("Bulk-add SSH hosts for multiple environments.")
	fmt.Println()

	clientCode := strings.ToLower(promptRequired("App/project code (e.g. myapp, api): "))
	clientName := promptString("Project name (optional, e.g. My Project): ", "")
	service := strings.ToLower(promptString("Service/role (optional, e.g. web, db, api): ", ""))

	cfg := store.LoadConfig()
	defaultPort := cfg.DefaultPort
	if defaultPort == "" {
		defaultPort = "22"
	}
	port := promptString(fmt.Sprintf("Port [%s]: ", defaultPort), defaultPort)
	identity := promptString(fmt.Sprintf("IdentityFile [%s]: ", cfg.DefaultIdentity), cfg.DefaultIdentity)

	fmt.Println()
	fmt.Println("Templates: use {env} as placeholder.")
	userTpl := promptRequired("Username template (e.g. deploy_{env}): ")
	hostTpl := promptRequired("Hostname template (e.g. {env}-01.client.internal): ")

	fmt.Println()
	envsRaw := promptString("Environments [prod acc test]: ", "prod acc test")
	envs := strings.Fields(envsRaw)
	if len(envs) == 0 {
		fatal("no environments specified")
	}

	// Build candidate hosts
	type candidate struct {
		host   sshconfig.Host
		status string // "new", "managed", "unmanaged"
	}

	allHosts, err := sshconfig.ParseDefault()
	if err != nil {
		fatal(err.Error())
	}
	managedPath, err := sshconfig.ManagedConfigPath()
	if err != nil {
		fatal(err.Error())
	}
	existingByAlias := map[string]sshconfig.Host{}
	for _, h := range allHosts {
		existingByAlias[h.Alias] = h
	}

	candidates := make([]candidate, 0, len(envs))
	for _, env := range envs {
		user := strings.ReplaceAll(userTpl, "{env}", env)
		hostname := strings.ReplaceAll(hostTpl, "{env}", env)

		alias, _, _, meta := buildHostParts(clientCode, clientName, env, service)
		h := sshconfig.Host{
			Alias:    alias,
			HostName: hostname,
			User:     user,
			Port:     port,
			Identity: identity,
			Meta:     meta,
		}

		status := "new"
		if existing, ok := existingByAlias[alias]; ok {
			if existing.SourceFile == managedPath {
				status = "managed"
			} else {
				status = "unmanaged"
			}
		}
		candidates = append(candidates, candidate{host: h, status: status})
	}

	// Preview
	fmt.Println()
	fmt.Println("Preview:")
	fmt.Println()
	conflictCount := 0
	labelW := 0
	for _, c := range candidates {
		if len(c.host.Alias) > labelW {
			labelW = len(c.host.Alias)
		}
	}
	for _, c := range candidates {
		target := c.host.User + "@" + c.host.HostName
		tag := ""
		switch c.status {
		case "managed":
			tag = "  [exists — managed, can overwrite]"
			conflictCount++
		case "unmanaged":
			tag = "  [exists — unmanaged, will skip]"
			conflictCount++
		}
		fmt.Printf("  %-*s  %s%s\n", labelW, c.host.Alias, target, tag)
	}
	fmt.Println()

	// Conflict resolution
	overwriteManaged := false
	if conflictCount > 0 {
		fmt.Printf("%d conflict(s) found.\n", conflictCount)
		choice := promptEnum("Resolve: [o]verwrite managed / [s]kip conflicts / [a]bort: ",
			[]string{"o", "s", "a", "overwrite", "skip", "abort"})
		switch choice {
		case "a", "abort":
			fmt.Println("aborted")
			return
		case "o", "overwrite":
			overwriteManaged = true
		}
	}

	// Confirm
	toCreate := 0
	for _, c := range candidates {
		if c.status == "new" || (c.status == "managed" && overwriteManaged) {
			toCreate++
		}
	}
	if toCreate == 0 {
		fmt.Println("nothing to add")
		return
	}
	confirm := promptString(fmt.Sprintf("Add %d host(s)? [Y/n]: ", toCreate), "y")
	if strings.ToLower(confirm) != "y" && strings.ToLower(confirm) != "yes" {
		fmt.Println("aborted")
		return
	}

	// Write
	for _, c := range candidates {
		switch c.status {
		case "unmanaged":
			fmt.Printf("  skip %s (exists in unmanaged config)\n", c.host.Alias)
			continue
		case "managed":
			if !overwriteManaged {
				fmt.Printf("  skip %s (exists)\n", c.host.Alias)
				continue
			}
		}
		if err := sshconfig.WriteHost(c.host); err != nil {
			fmt.Fprintf(os.Stderr, "  error %s: %v\n", c.host.Alias, err)
			continue
		}
		action := "added"
		if c.status == "managed" {
			action = "updated"
		}
		fmt.Printf("  %s %s\n", action, c.host.Alias)
	}
	fmt.Println()
	output.Success("jump: bulk-add complete")
}

func mustSetClient(code, name string) {
	code = strings.ToLower(code)
	n, err := sshconfig.UpdateHostMetaByApp(code, map[string]string{"client": name})
	if err != nil {
		fatal(err.Error())
	}
	if n == 0 {
		fatal(fmt.Sprintf("no managed hosts found with app=%q", code))
	}
	output.Success(fmt.Sprintf("jump: set client=%q on %d host(s) with app=%q", name, n, code))
}

func mustRename(oldAlias, newAlias string) {
	hosts, err := sshconfig.ParseDefault()
	if err != nil {
		fatal(err.Error())
	}
	managedPath, err := sshconfig.ManagedConfigPath()
	if err != nil {
		fatal(err.Error())
	}
	var found *sshconfig.Host
	for i := range hosts {
		if hosts[i].Alias == oldAlias {
			found = &hosts[i]
		}
		if hosts[i].Alias == newAlias {
			fatal(fmt.Sprintf("alias %q already exists", newAlias))
		}
	}
	if found == nil {
		fatal(fmt.Sprintf("alias %q not found", oldAlias))
	}
	if found.SourceFile != managedPath {
		fatal(fmt.Sprintf("host %q is not managed by jump (source: %s)", oldAlias, found.SourceFile))
	}
	if err := sshconfig.RenameAlias(oldAlias, newAlias); err != nil {
		fatal(err.Error())
	}
	if err := store.RenameHistory(oldAlias, newAlias); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not update history: %v\n", err)
	}
	output.Success(fmt.Sprintf("jump: renamed %q → %q", oldAlias, newAlias))
}

func mustLast(printOnly bool) {
	h := store.Load()
	if len(h) == 0 {
		fatal("no history yet")
	}
	var last store.HistoryEntry
	for _, e := range h {
		if last.Alias == "" || e.LastUsedAt.After(last.LastUsedAt) {
			last = e
		}
	}
	hosts, err := sshconfig.ParseDefault()
	if err != nil {
		fatal(err.Error())
	}
	for _, host := range hosts {
		if host.Alias == last.Alias {
			connectToAlias(last.Alias, hosts, printOnly)
			return
		}
	}
	fatal(fmt.Sprintf("last host %q no longer exists in config", last.Alias))
}

func mustRecent(args []string, printOnly bool, jsonOut bool) {
	h := store.Load()
	if len(h) == 0 {
		fmt.Println("jump: no history yet")
		return
	}
	entries := make([]store.HistoryEntry, 0, len(h))
	for _, e := range h {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].LastUsedAt.After(entries[j].LastUsedAt)
	})

	// jump recent <n> → connect to nth entry
	if len(args) > 0 {
		var n int
		if _, err := fmt.Sscanf(args[0], "%d", &n); err != nil || n < 1 || n > len(entries) {
			fatal(fmt.Sprintf("invalid index %q (1-%d)", args[0], len(entries)))
		}
		target := entries[n-1]
		hosts, err := sshconfig.ParseDefault()
		if err != nil {
			fatal(err.Error())
		}
		for _, host := range hosts {
			if host.Alias == target.Alias {
				connectToAlias(target.Alias, hosts, printOnly)
				return
			}
		}
		fatal(fmt.Sprintf("host %q no longer exists in config", target.Alias))
		return
	}

	// no arg → show list
	mustHistory(20, jsonOut)
}

func mustDoctor() {
	output.Section("Jump doctor")
	fmt.Println()

	home, _ := os.UserHomeDir()
	sshConfig := filepath.Join(home, ".ssh", "config")
	sshConfigD := filepath.Join(home, ".ssh", "config.d")
	managedPath, _ := sshconfig.ManagedConfigPath()

	type check struct {
		label string
		ok    bool
		msg   string
	}
	var checks []check

	add := func(label string, ok bool, msg string) {
		checks = append(checks, check{label, ok, msg})
	}

	// SSH config exists
	_, err := os.Stat(sshConfig)
	add("~/.ssh/config exists", err == nil, sshConfig)

	// Include line present
	if err == nil {
		data, _ := os.ReadFile(sshConfig)
		hasInclude := strings.Contains(string(data), "Include ~/.ssh/config.d/*.conf")
		add("Include ~/.ssh/config.d/*.conf found", hasInclude, "")
	}

	// config.d dir
	_, err = os.Stat(sshConfigD)
	add("~/.ssh/config.d/ exists", err == nil, sshConfigD)

	// jump.conf exists
	_, err = os.Stat(managedPath)
	add("jump.conf exists", err == nil, managedPath)

	// permissions
	checkPerm := func(path string, want os.FileMode) (bool, string) {
		info, err := os.Stat(path)
		if err != nil {
			return false, err.Error()
		}
		got := info.Mode().Perm()
		if got&0o077 != 0 {
			return false, fmt.Sprintf("%s has permissions %o (should be %o)", filepath.Base(path), got, want)
		}
		return true, ""
	}
	ok, msg := checkPerm(sshConfig, 0o600)
	add("~/.ssh/config permissions safe", ok, msg)
	ok, msg = checkPerm(managedPath, 0o600)
	add("jump.conf permissions safe", ok, msg)

	// SSH binary
	_, err = exec.LookPath("ssh")
	add("ssh binary found", err == nil, "")

	// parse managed hosts
	hosts, parseErr := sshconfig.ParseDefault()
	add("SSH config parses cleanly", parseErr == nil, func() string {
		if parseErr != nil {
			return parseErr.Error()
		}
		return ""
	}())

	if parseErr == nil {
		// duplicate aliases
		seen := map[string]int{}
		for _, h := range hosts {
			seen[h.Alias]++
		}
		var dups []string
		for alias, n := range seen {
			if n > 1 {
				dups = append(dups, alias)
			}
		}
		add(fmt.Sprintf("%d hosts indexed", len(hosts)), true, "")
		add("no duplicate aliases", len(dups) == 0, strings.Join(dups, ", "))
	}

	// print results
	allOk := true
	for _, c := range checks {
		if !c.ok {
			allOk = false
		}
		output.DocCheck(c.label, c.ok, c.msg)
	}
	fmt.Println()
	if allOk {
		output.Success("Everything looks healthy.")
	} else {
		fmt.Fprintln(os.Stderr, "Some issues found — see above.")
	}
}

func mustCopy(query string, limit int) {
	if query == "" {
		fatal("usage: jump copy <query>")
	}
	hosts, err := sshconfig.ParseDefault()
	if err != nil {
		fatal(err.Error())
	}
	results := search.Find(hosts, query)
	if len(results) == 0 {
		output.NoMatches(query)
	}
	chosen := results[0]
	if len(results) > 1 && results[0].Score-results[1].Score < 40 {
		chosen = promptPicker(results, limit)
	}

	cmd := "ssh " + chosen.Host.Alias
	if err := clip.Copy(cmd); err != nil {
		fatal("clipboard unavailable: " + err.Error())
	}
	output.CopySuccess(cmd)
}

func mustAliases(args []string, jsonOut bool) {
	hosts, err := sshconfig.ParseDefault()
	if err != nil {
		fatal(err.Error())
	}

	envFilter := hasArgVal(args, "--env")
	tagFilter := hasArgVal(args, "--tag")

	var out []string
	for _, h := range hosts {
		if envFilter != "" && h.Meta["env"] != envFilter {
			continue
		}
		if tagFilter != "" {
			found := false
			for _, t := range strings.Split(h.Meta["tags"], ",") {
				if strings.TrimSpace(t) == tagFilter {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		out = append(out, h.Alias)
	}

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		_ = enc.Encode(out)
		return
	}
	for _, a := range out {
		fmt.Println(a)
	}
}

func mustOpenConfig(args []string) {
	home, _ := os.UserHomeDir()
	managed, _ := sshconfig.ManagedConfigPath()

	files := map[string]string{
		"--managed":  managed,
		"--ssh":      filepath.Join(home, ".ssh", "config"),
		"--metadata": filepath.Join(home, ".config", "jump", "index.json"),
		"--history":  filepath.Join(home, ".config", "jump", "history.json"),
		"--config":   filepath.Join(home, ".config", "jump", "config.json"),
	}

	target := "--managed"
	if len(args) > 0 {
		target = args[0]
	}
	path, ok := files[target]
	if !ok {
		fatal(fmt.Sprintf("unknown target %q — use: --managed --ssh --metadata --history --config", target))
	}
	fmt.Printf("Opening %s\n", path)
	var openCmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		openCmd = exec.Command("open", path)
	case "windows":
		openCmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	default:
		openCmd = exec.Command("xdg-open", path)
	}
	if err := openCmd.Run(); err != nil {
		fatal("could not open file: " + err.Error())
	}
}

func hasArgVal(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(a, flag+"=") {
			return strings.TrimPrefix(a, flag+"=")
		}
	}
	return ""
}

func mustHistory(limit int, jsonOut bool) {
	h := store.Load()
	if len(h) == 0 {
		fmt.Println("jump: no history yet")
		return
	}

	entries := make([]store.HistoryEntry, 0, len(h))
	for _, e := range h {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].LastUsedAt.After(entries[j].LastUsedAt)
	})
	if limit > 0 && limit < len(entries) {
		entries = entries[:limit]
	}

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		_ = enc.Encode(entries)
		return
	}

	results := make([]search.Result, 0, len(entries))
	hosts, err := sshconfig.ParseDefault()
	if err != nil {
		fatal(err.Error())
	}
	hostByAlias := map[string]sshconfig.Host{}
	for _, h := range hosts {
		hostByAlias[h.Alias] = h
	}
	for _, e := range entries {
		h := hostByAlias[e.Alias]
		if h.Alias == "" {
			h.Alias = e.Alias
		}
		results = append(results, search.Result{Host: h})
	}

	output.History(entries, results)
}

func mustPing(query string) {
	hosts, err := sshconfig.ParseDefault()
	if err != nil {
		fatal(err.Error())
	}
	results := search.Find(hosts, query)
	if len(results) == 0 {
		output.NoMatches(query)
	}

	chosen := results[0]
	if len(results) > 1 && results[0].Score-results[1].Score < 40 {
		chosen = promptPicker(results, 20)
	}
	target := &chosen.Host

	hostname := target.HostName
	if hostname == "" {
		hostname = target.Alias
	}
	port := target.Port
	if port == "" {
		port = "22"
	}
	addr := hostname + ":" + port

	output.PingTarget(target.Alias, addr)

	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	elapsed := time.Since(start)

	if err != nil {
		fmt.Fprintf(os.Stderr, "unreachable: %v\n", err)
		if hint := pingHint(err.Error()); hint != "" {
			fmt.Fprintf(os.Stderr, "hint: %s\n", hint)
		}
		os.Exit(1)
	}
	conn.Close()
	output.PingReachable(addr, elapsed.Milliseconds())
}

func pingHint(errMsg string) string {
	s := strings.ToLower(errMsg)
	switch {
	case strings.Contains(s, "no route to host"), strings.Contains(s, "network is unreachable"):
		return "host unreachable — check VPN or network connection"
	case strings.Contains(s, "i/o timeout"), strings.Contains(s, "timed out"):
		return "connection timed out — check VPN or firewall rules"
	case strings.Contains(s, "connection refused"):
		return "connection refused — SSH service may not be running on port " + s[strings.LastIndex(s, ":")+1:]
	case strings.Contains(s, "no such host"), strings.Contains(s, "name or service not known"):
		return "hostname not found — check DNS or VPN"
	}
	return ""
}

func mustEdit(alias string) {
	hosts, err := sshconfig.ParseDefault()
	if err != nil {
		fatal(err.Error())
	}

	var target *sshconfig.Host
	for i := range hosts {
		if hosts[i].Alias == alias {
			target = &hosts[i]
			break
		}
	}
	if target == nil {
		fatal("alias not found")
	}

	managedPath, err := sshconfig.ManagedConfigPath()
	if err != nil {
		fatal(err.Error())
	}
	if target.SourceFile != managedPath {
		fatal(fmt.Sprintf("host %q is not managed by jump (source: %s)", alias, target.SourceFile))
	}

	output.Section(fmt.Sprintf("Editing host %q", alias))
	fmt.Println("(press enter to keep current value)")
	fmt.Println()

	target.HostName = promptString(fmt.Sprintf("HostName [%s]: ", target.HostName), target.HostName)
	target.User = promptString(fmt.Sprintf("User [%s]: ", target.User), target.User)
	target.Port = promptString(fmt.Sprintf("Port [%s]: ", target.Port), target.Port)
	target.Identity = promptString(fmt.Sprintf("IdentityFile [%s]: ", target.Identity), target.Identity)

	if target.Meta == nil {
		target.Meta = map[string]string{}
	}
	target.Meta["app"] = promptString(fmt.Sprintf("Application [%s]: ", target.Meta["app"]), target.Meta["app"])
	target.Meta["client"] = promptString(fmt.Sprintf("Client name [%s]: ", target.Meta["client"]), target.Meta["client"])
	target.Meta["env"] = promptString(fmt.Sprintf("Environment [%s]: ", target.Meta["env"]), target.Meta["env"])
	newTags := promptString(fmt.Sprintf("Tags [%s]: ", target.Meta["tags"]), target.Meta["tags"])
	target.Meta["tags"] = normalizeTags(newTags)
	target.Meta["description"] = promptString(fmt.Sprintf("Description [%s]: ", target.Meta["description"]), target.Meta["description"])

	if err := sshconfig.WriteHost(*target); err != nil {
		fatal(err.Error())
	}
	output.Success(fmt.Sprintf("jump: updated host %q", alias))
}

func mustDelete(alias string) {
	hosts, err := sshconfig.ParseDefault()
	if err != nil {
		fatal(err.Error())
	}

	var target *sshconfig.Host
	for i := range hosts {
		if hosts[i].Alias == alias {
			target = &hosts[i]
			break
		}
	}
	if target == nil {
		fatal("alias not found")
	}

	managedPath, err := sshconfig.ManagedConfigPath()
	if err != nil {
		fatal(err.Error())
	}
	if target.SourceFile != managedPath {
		fatal(fmt.Sprintf("host %q is not managed by jump (source: %s)", alias, target.SourceFile))
	}

	fmt.Printf("Delete host %q (%s)? [y/N] ", alias, sshconfig.FormatHost(*target))
	line, _ := stdin.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	if line != "y" && line != "yes" {
		fmt.Println("aborted")
		return
	}

	if err := sshconfig.DeleteHost(alias); err != nil {
		fatal(err.Error())
	}
	output.Success(fmt.Sprintf("jump: deleted host %q", alias))
}

func mustTag(alias string, tags []string) {
	hosts, err := sshconfig.ParseDefault()
	if err != nil {
		fatal(err.Error())
	}

	var target *sshconfig.Host
	for i := range hosts {
		if hosts[i].Alias == alias {
			target = &hosts[i]
			break
		}
	}
	if target == nil {
		fatal("alias not found")
	}

	managedPath, err := sshconfig.ManagedConfigPath()
	if err != nil {
		fatal(err.Error())
	}

	if target.SourceFile == managedPath {
		if err := sshconfig.UpdateHostMeta(alias, map[string]string{
			"tags": strings.Join(tags, ","),
		}); err != nil {
			fatal(err.Error())
		}
	} else {
		if err := store.SetMeta(alias, func(e *store.MetadataEntry) {
			existing := map[string]bool{}
			for _, t := range e.Tags {
				existing[t] = true
			}
			for _, t := range tags {
				t = strings.TrimSpace(t)
				if t != "" && !existing[t] {
					e.Tags = append(e.Tags, t)
					existing[t] = true
				}
			}
		}); err != nil {
			fatal(err.Error())
		}
	}
	output.Success(fmt.Sprintf("jump: tagged %q with %s", alias, strings.Join(tags, ", ")))
}

func mustDescribe(alias string, description string) {
	hosts, err := sshconfig.ParseDefault()
	if err != nil {
		fatal(err.Error())
	}

	var target *sshconfig.Host
	for i := range hosts {
		if hosts[i].Alias == alias {
			target = &hosts[i]
			break
		}
	}
	if target == nil {
		fatal("alias not found")
	}

	managedPath, err := sshconfig.ManagedConfigPath()
	if err != nil {
		fatal(err.Error())
	}

	if target.SourceFile == managedPath {
		if err := sshconfig.UpdateHostMeta(alias, map[string]string{
			"description": description,
		}); err != nil {
			fatal(err.Error())
		}
	} else {
		if err := store.SetMeta(alias, func(e *store.MetadataEntry) {
			e.Description = description
		}); err != nil {
			fatal(err.Error())
		}
	}
	output.Success(fmt.Sprintf("jump: set description for %q", alias))
}

func promptPicker(results []search.Result, limit int) search.Result {
	output.Matches(results, limit)
	fmt.Fprintln(os.Stdout)

	chosen, err := output.InteractivePicker(results, limit)
	if err == nil {
		return chosen
	}
	if errors.Is(err, output.ErrQuit) {
		fatal("aborted")
	}

	// TTY unavailable (piped/scripted) — fall back to number input
	fmt.Print("Select host number: ")
	line, _ := stdin.ReadString('\n')
	line = strings.TrimSpace(line)
	var n int
	_, serr := fmt.Sscanf(line, "%d", &n)
	maxCap := limit
	if maxCap <= 0 || maxCap > len(results) {
		maxCap = len(results)
	}
	if serr != nil || n < 1 || n > maxCap {
		fatal("invalid selection")
	}
	return results[n-1]
}

func promptString(prompt, defaultVal string) string {
	fmt.Print(prompt)
	line, _ := stdin.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal
	}
	return line
}

func promptInt(prompt string, defaultVal int) int {
	for {
		raw := promptString(prompt, "")
		if raw == "" {
			return defaultVal
		}
		var n int
		if _, err := fmt.Sscanf(raw, "%d", &n); err == nil && n > 0 {
			return n
		}
		fmt.Println("  (enter a positive number)")
	}
}

func promptEnum(prompt string, options []string) string {
	for {
		val := promptString(prompt, "")
		for _, o := range options {
			if val == o {
				return val
			}
		}
		fmt.Printf("  (choose one of: %s)\n", strings.Join(options, ", "))
	}
}

func promptRequired(prompt string) string {
	for {
		val := promptString(prompt, "")
		if val != "" {
			return val
		}
		fmt.Println("  (required)")
	}
}

func buildHostParts(clientCode, clientName, env, service string) (alias, tags, desc string, meta map[string]string) {
	if service != "" {
		alias = fmt.Sprintf("%s-%s-%s", clientCode, service, env)
		desc = fmt.Sprintf("%s connection for %s [%s]", service, strings.ToUpper(clientCode), env)
		tags = clientCode + "," + env + "," + service
	} else {
		alias = fmt.Sprintf("%s-%s", clientCode, env)
		desc = fmt.Sprintf("%s connection [%s]", strings.ToUpper(clientCode), env)
		tags = clientCode + "," + env
	}
	meta = map[string]string{
		"app":         clientCode,
		"env":         env,
		"tags":        tags,
		"description": desc,
	}
	if clientName != "" {
		meta["client"] = clientName
	}
	return
}

func normalizeTags(s string) string {
	var tags []string
	for _, t := range strings.Split(s, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			tags = append(tags, t)
		}
	}
	return strings.Join(tags, ",")
}

func metadataPathStr() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "jump", "index.json"), nil
}

func usage() {
	output.PrintUsage()
}

func fatal(message string) {
	// Use plain JSON error format when JSON output is active
	if isJSONMode() {
		enc := json.NewEncoder(os.Stderr)
		_ = enc.Encode(map[string]string{"error": message})
	} else {
		fmt.Fprintln(os.Stderr, "jump:", message)
	}
	os.Exit(1)
}

func hasFlag(name string) bool {
	for _, arg := range os.Args[1:] {
		if arg == "--"+name || arg == "-"+name {
			return true
		}
	}
	return false
}

func isJSONMode() bool { return hasFlag("json") }
