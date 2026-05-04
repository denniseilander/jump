# ⚡ jump

**jump** is a fast SSH launcher for the terminal. It reads your `~/.ssh/config`, lets you search and connect to hosts by alias, hostname, user, or metadata — with a fuzzy TUI picker, a real-time connect spinner, and a managed config layer that keeps your hosts organised.

---

## Features

- **Fuzzy search** across alias, hostname, user, tags, app, environment, and description
- **TUI picker** with keyboard navigation, action panel, and inline management
- **Smart direct connect** — jumps straight to a host when the match is unambiguous
- **Managed config** — add, edit, and delete hosts in `~/.ssh/config.d/jump.conf` without touching your existing config
- **Metadata** — attach app codes, environments, tags, and descriptions as inline SSH config comments
- **History & recency scoring** — frequently and recently used hosts rank higher
- **Clipboard support** — copy any SSH command to clipboard
- **TCP ping** — check if a host is reachable before connecting
- **JSON output** — all list/scan/history commands support `--json` for scripting
- **Cross-platform** — macOS, Linux, Windows (amd64 + arm64)

---

## Installation

**One-liner (macOS / Linux):**

```bash
curl -fsSL https://raw.githubusercontent.com/denniseilander/jump/main/install.sh | sh
```

The script auto-detects your OS and architecture, downloads the correct binary from the latest GitHub release, and installs it to `/usr/local/bin`.

**With Go:**

```bash
go install github.com/denniseilander/jump/cmd/jump@latest
```

Make sure `$(go env GOPATH)/bin` is on your `$PATH`.

**From source:**

```bash
git clone https://github.com/denniseilander/jump.git
cd jump
make install
```

**Manual download:**

Download the binary for your platform from the [Releases](https://github.com/denniseilander/jump/releases) page and place it on your `$PATH`.

---

## Quick start

```bash
# First-time setup — creates ~/.ssh/config.d/jump.conf and adds the Include line
jump init

# Add your first host
jump add

# Open the TUI picker
jump

# Search and connect
jump myapp prod
```

---

## Usage

```
jump [--print] [--json] [query...]
```

Searches all SSH hosts and connects to the best match. If multiple hosts match at similar scores, the TUI picker opens automatically. A single strong match connects immediately without prompting.

```bash
jump                          # open TUI picker (all hosts)
jump myapp                    # search and connect
jump myapp production         # multi-term search
jump gateway acc              # connect to acceptance gateway
jump -                        # reconnect to last used host
jump --print myapp prod       # print ssh command, do not connect
jump --json myapp             # output match as JSON
```

---

## Commands

### Search & inspect

| Command | Description |
|---|---|
| `jump list [--json]` | List all known hosts, grouped by client |
| `jump show <alias>` | Show full details for a host |
| `jump scan [--json]` | Scan SSH config files and show stats |
| `jump explain <query>` | Show search score breakdown for a query |
| `jump aliases [--env e] [--tag t]` | Print host aliases (for scripting) |
| `jump ping <query>` | Check TCP reachability on SSH port |

### Connect

| Command | Description |
|---|---|
| `jump -` | Reconnect to the last used host |
| `jump recent [n]` | List recent hosts; connect to the nth entry |
| `jump history [--limit n]` | Show full connection history |
| `jump copy <query>` | Copy SSH command to clipboard |

### Manage hosts

| Command | Description |
|---|---|
| `jump init` | Initialise managed SSH config |
| `jump add` | Add a new managed host interactively |
| `jump bulk-add` | Add multiple hosts from a template (multi-environment) |
| `jump edit <alias>` | Edit a managed host |
| `jump rename <old> <new>` | Rename a host alias |
| `jump delete <alias>` | Delete a managed host |
| `jump tag <alias> <tag...>` | Add tags to a host |
| `jump describe <alias> <text>` | Set a host description |
| `jump set-client <code> <name>` | Set client name on all hosts with matching app code |

### Config & tools

| Command | Description |
|---|---|
| `jump config` | View and edit jump preferences |
| `jump doctor` | Validate jump and SSH setup |
| `jump open-config [--managed\|--ssh\|--metadata\|--history\|--config]` | Open a config file in your editor |

---

## Flags

| Flag | Description |
|---|---|
| `--print` | Print SSH command without executing |
| `--json` | Machine-readable JSON output |
| `--plain` | Disable colours and styling |
| `--no-color` | Disable colours |
| `--limit <n>` | Limit number of results shown (default: 20) |
| `--pick` | Always open the TUI picker, even on a strong match |
| `--no-tui` | Disable TUI; use classic numbered CLI picker |

---

## TUI

Open the picker with `jump` or `jump <query>`. Type to filter in real time.

| Key | Action |
|---|---|
| `↑` / `↓` | Move cursor |
| `Enter` | Connect to selected host |
| `Tab` | Open action panel (connect, copy, edit, delete) |
| `Esc` | Clear search / quit |
| `q` | Quit (when search is empty) |
| `Ctrl+C` | Quit |

---

## Metadata

Attach optional metadata to any host by adding a `# jump:` comment on the line immediately before the `Host` block:

```sshconfig
# jump: app=myapp client="My Project" env=prod tags=web,production description="Production web server"
Host myapp-web-prod
  HostName prod-01.example.com
  User deploy
  Port 22
```

Supported keys:

| Key | Description |
|---|---|
| `app` | Application or project code |
| `client` | Human-readable client or project name |
| `env` | Environment (`prod`, `acc`, `dev`, `test`) |
| `tags` | Comma-separated list of tags |
| `description` | Free-text description |

All metadata fields are searchable. `jump myapp prod` matches on both the alias and metadata fields. Environment synonyms are resolved automatically — searching `production` also matches `prod`, `prd`, and `productie`.

---

## Managed config

`jump init` sets up the following structure:

```
~/.ssh/config              ← your existing config (Include line added at top)
~/.ssh/config.d/
  jump.conf                ← managed by jump
```

Hosts added via `jump add` or `jump bulk-add` are written to `jump.conf`, leaving your existing `~/.ssh/config` untouched. Backups of `jump.conf` are created automatically before every write to `~/.config/jump/backups/`.

---

## Configuration

Run `jump config` to set defaults used when adding new hosts:

| Setting | Description |
|---|---|
| Default IdentityFile | Default SSH key path |
| Default User | Default SSH username |
| Default Port | Default port (omitted if 22) |
| Connect timeout | Seconds before connection attempt times out |

Config is stored at `~/.config/jump/config.json`.

---

## Requirements

- `ssh` binary available on `$PATH`
- `~/.ssh/config` readable
- Go 1.21+ (only for building from source)

---

## License

[MIT](LICENSE)
