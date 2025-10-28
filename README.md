# k1 💨

The supersonic Kubernetes TUI. Built with Go and Bubble Tea for blazing-fast cluster management at Mach 1 speed.

## Features

- **⚡ Blazing Fast**: Powered by Kubernetes informers with protobuf encoding for near-instant resource viewing
- **🔍 Fuzzy Search**: Type to filter resources with intelligent fuzzy matching and negation support
- **🎨 8 Beautiful Themes**: charm, dracula, catppuccin, nord, gruvbox, tokyo-night, solarized, monokai
- **📊 11 Resource Types**: Pods, Deployments, StatefulSets, DaemonSets, Jobs, CronJobs, Services, ConfigMaps, Secrets, Nodes, Namespaces
- **⌨️ Vim-style Navigation**: Intuitive keybindings for power users
- **🎯 Command Palette**: Quick access to operations like scale, restart, drain, cordon
- **📋 Clipboard Integration**: Generate kubectl commands and copy to clipboard
- **🤖 AI-Powered Commands**: Natural language interface with `/ai` (experimental)

## Installation

### Prerequisites

- Go 1.25.1 or later
- Access to a Kubernetes cluster with valid kubeconfig

### Build from Source

```bash
git clone https://github.com/yourusername/k1.git
cd k1
make build
sudo mv k1 /usr/local/bin/
```

Or quick build:
```bash
go build -o k1 cmd/k1/main.go
```

## Usage

### Basic Usage

```bash
# Run with default kubeconfig and context
k1

# Run with specific context
k1 -context my-cluster

# Run with custom kubeconfig
k1 -kubeconfig /path/to/kubeconfig

# Run with specific theme
k1 -theme dracula

# Run with dummy data (no cluster connection)
k1 -dummy
```

### Keybindings

#### Global
- **Type any character**: Enter filter mode with fuzzy search
- **`:`**: Open navigation palette (switch screens, namespaces)
- **`/`**: Open command palette (resource operations)
- **`↑`/`↓`**: Navigate lists or palette items
- **`Enter`**: Apply filter or execute command
- **`Esc`**: Clear filter or dismiss palette
- **`Tab`**: Auto-complete selected command in palette
- **`q`** or **`Ctrl+C`**: Quit

#### Resource Operations
- **`Ctrl+Y`**: View YAML for selected resource
- **`Ctrl+D`**: Describe selected resource (with on-demand events)
- **`Ctrl+L`**: View logs (pods only, copies kubectl command to clipboard)
- **`Ctrl+X`**: Shell into container (pods only, copies kubectl command to clipboard)

### Filter Mode

Start typing to filter the current resource list:
- **Fuzzy matching**: `depngx` matches `deployment-nginx`
- **Negation**: `!prod` excludes resources containing "prod"
- **Paste support**: Paste text directly to filter

### Command Palette

Press `/` to open the command palette and access resource operations:

#### Common Commands
- `/scale [replicas]` - Scale deployment/statefulset (default: prompt for replicas)
- `/restart` - Restart deployment (kubectl rollout restart)
- `/delete` - Delete selected resource (with confirmation)
- `/yaml` - View resource YAML
- `/describe` - Describe resource with events

#### Node Commands
- `/cordon` - Mark node as unschedulable
- `/drain [grace] [force] [ignore-daemonsets]` - Drain node (defaults: 30s, false, true)

#### Service Commands
- `/endpoints` - View service endpoints

#### Pod Commands
- `/logs [container] [tail] [follow]` - Generate logs command (copies to clipboard)
- `/shell [container] [shell]` - Generate shell command (copies to clipboard)
- `/port-forward <ports>` - Generate port-forward command (e.g., `8080:80`)

#### AI Commands (Experimental)
- `/ai <prompt>` - Natural language command generation with local LLM

### Navigation Palette

Press `:` to open the navigation palette:
- Switch between resource screens (`:pods`, `:deployments`, `:services`, etc.)
- Switch namespaces
- View help

## Available Themes

k1 includes 8 carefully crafted themes:

1. **charm** (default) - Charming pink and purple pastels
2. **dracula** - Dark theme with vibrant colors
3. **catppuccin** - Soothing pastel theme
4. **nord** - Arctic, north-bluish color palette
5. **gruvbox** - Retro groove colors
6. **tokyo-night** - Clean, elegant dark theme
7. **solarized** - Precision colors for machines and people
8. **monokai** - Sublime Text's iconic color scheme

Try them with: `k1 -theme <name>`

## Configuration

k1 uses your existing kubeconfig:
- Default location: `~/.kube/config`
- Respects `KUBECONFIG` environment variable
- Switch contexts with `-context` flag

## Architecture

k1 uses Kubernetes informers with client-side caching for blazing-fast performance:
- **Initial sync**: 1-2 seconds (one-time per session)
- **Subsequent queries**: Microsecond-fast from local cache
- **Real-time updates**: Automatic via watch connections
- **Protobuf encoding**: Reduced network transfer vs JSON

### Quick Start

```bash
# Run tests
make test

# Run with live cluster
make run

# Run with dummy data
make run-dummy

# Build binary
make build
```

## Roadmap

- [ ] Configuration persistence (~/.config/k1/)
- [ ] Resource editing
- [ ] Log streaming in TUI
- [ ] AI based commands
- [ ] Search on yaml and describe
- [ ] Copy screen to clipboard
- [ ] Shell into pod

## Inspiration

k1 is inspired by:
- [k9s](https://k9scli.io/) - The OG Kubernetes TUI
- [lazygit](https://github.com/jesseduffield/lazygit) - Elegant terminal UI
- [fzf](https://github.com/junegunn/fzf) - Blazing fast fuzzy finder

## License

GNU General Public License v3.0 - See LICENSE file for details

