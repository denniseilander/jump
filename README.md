# jump

A fast SSH launcher for the terminal. Reads `~/.ssh/config`, lets you search and connect to hosts by alias, hostname, user, or metadata — with a fuzzy TUI picker and a connect-time spinner.

## Install

**One-liner (macOS/Linux):**

```bash
curl -fsSL https://raw.githubusercontent.com/denniseilander/jump/main/install.sh | sh
```

**With Go:**

```bash
go install github.com/denniseilander/jump/cmd/jump@latest
```

**From source:**

```bash
git clone https://github.com/denniseilander/jump.git
cd jump
make install
```

Make sure `$(go env GOPATH)/bin` is on your `$PATH`.

## Usage

```bash
jump                        # open TUI picker
jump myapp                  # search and connect (strong match connects directly)
jump myapp prod             # multi-word search
jump --print myapp prod     # print ssh command instead of connecting
jump -                      # reconnect to last host
```

## Commands

| Command | Description |
|---|---|
| `jump list` | List all hosts |
| `jump show <alias>` | Show host details |
| `jump explain <query>` | Show why results matched |
| `jump add` | Add a new host interactively |
| `jump bulk-add` | Add hosts for multiple environments at once |
| `jump edit <alias>` | Edit a managed host |
| `jump delete <alias>` | Delete a managed host |
| `jump rename <old> <new>` | Rename an alias |
| `jump tag <alias> <tags...>` | Tag a host |
| `jump describe <alias> <text>` | Set description |
| `jump copy <query>` | Copy ssh command to clipboard |
| `jump ping <alias>` | Check TCP reachability |
| `jump history` | Show recently used hosts |
| `jump recent` | Show recent hosts / connect to nth |
| `jump scan` | Show config files and stats |
| `jump doctor` | Check setup health |
| `jump config` | Configure defaults |
| `jump init` | Initialize managed config |

## Metadata

Add optional metadata comments before a `Host` block:

```sshconfig
# jump: app=myapp env=prod tags=web,production
Host myapp-prod
  HostName prod-01.example.com
  User deploy
  Port 22
```

Supported keys: `app`, `env`, `client`, `tags`, `description`.

Metadata is searchable: `jump myapp prod` matches both alias and metadata fields.

## Managed config

`jump init` creates `~/.ssh/config.d/jump.conf` and adds an `Include` line to `~/.ssh/config`. Hosts added via `jump add` or `jump bulk-add` are written here, keeping them separate from your existing config.

## TUI

Open the picker with `jump` or `jump <query>`. Navigate with arrow keys, press `Tab` for the action panel (connect, copy, edit, delete), `Enter` to connect, `Esc` to quit.

## Requirements

- Go 1.21+ (for building from source)
- `ssh` binary on `$PATH`
- `~/.ssh/config` readable

## License

MIT
