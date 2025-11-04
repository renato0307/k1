# k1 💨

A blazing-fast terminal UI for Kubernetes cluster management at Mach 1 speed.

## Features

- **⚡ Lightning Fast**: Near-instant resource viewing with real-time cluster updates
- **🔍 Fuzzy Search**: Type to filter resources with intelligent matching and negation support
- **🎨 11 Beautiful Themes**: 8 dark themes + 3 light themes for any terminal preference
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

For detailed build instructions and development setup, see [CLAUDE.md](CLAUDE.md).

## Quick Start Tutorial

Your first 5 minutes with k1:

1. **Start k1**: Run `k1` to connect to your current Kubernetes context
   ```bash
   k1
   ```

2. **View resources**: k1 starts on the Pods screen showing all pods in your current namespace

3. **Filter resources**: Type any text to start filtering
   ```
   Type: nginx    # Shows only pods matching "nginx"
   Type: !prod    # Excludes pods containing "prod"
   Press Esc      # Clear filter
   ```

4. **Navigate screens**: Press `:` to open navigation palette
   ```
   :deployments   # Switch to Deployments screen
   :services      # Switch to Services screen
   :nodes         # View cluster nodes
   ```

5. **Run commands**: Press `/` to open command palette
   ```
   /scale 3       # Scale selected deployment to 3 replicas
   /restart       # Restart selected deployment
   /yaml          # View resource YAML (or press Ctrl+Y)
   ```

6. **View details**: Navigate to a resource and press:
   ```
   Ctrl+Y         # View YAML
   Ctrl+D         # Describe with events
   Ctrl+L         # Get logs command (pods only)
   ```

7. **Change theme**: Restart with your preferred theme
   ```bash
   k1 -theme dracula
   ```

That's it! Explore the command palette (`/`) to discover more operations.

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
- **`Ctrl+C`**: Quit

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

k1 includes 8 carefully crafted themes. See the Configuration section for details on how to use them.

## Configuration

### Kubeconfig

k1 uses your existing Kubernetes configuration:

**Config location (in order of precedence)**:
1. `-kubeconfig` flag: `k1 -kubeconfig /path/to/config`
2. `KUBECONFIG` environment variable
3. Default: `~/.kube/config`

**Context switching**:
```bash
# Use specific context
k1 -context production

# List available contexts
kubectl config get-contexts

# Set default context (k1 will use it)
kubectl config use-context staging
```

### Themes

k1 includes 11 built-in themes with distinctive personalities:

**Dark Themes:**
- **charm** (default) - Purple/teal accents, balanced and modern
- **dracula** - Vibrant purple, high-energy aesthetic
- **catppuccin** - Soft pastel mauve, cozy feel
- **nord** - Cool arctic blues, minimalism
- **gruvbox** - Warm retro brown/orange
- **tokyo-night** - Deep blue urban, sleek
- **solarized** - Scientific precision, balanced
- **monokai** - Neon on black, high-contrast

**Light Themes:**
- **catppuccin-latte** - Soft pastels on cream
- **solarized-light** - Precision colors with warm background
- **gruvbox-light** - Warm retro on cream

**Usage**:
```bash
k1 -theme dracula           # Dark theme
k1 -theme gruvbox-light     # Light theme
```

### Persistent Configuration (Future)

Currently planned for `~/.config/k1/config.yaml`:
- Default theme
- Preferred namespace
- Window layout preferences
- Command aliases

## Troubleshooting

### Connection Issues

**k1 won't start or shows connection errors**
- Verify kubeconfig is valid: `kubectl cluster-info`
- Check current context: `kubectl config current-context`
- Try specifying context explicitly: `k1 -context my-cluster`
- If using custom kubeconfig: `k1 -kubeconfig /path/to/config`
- Check cluster API server is reachable from your network

**RBAC / Permission errors**
- Verify you have list permissions: `kubectl auth can-i list pods`
- Check namespace access: `kubectl auth can-i list pods -n <namespace>`
- Some resources require cluster-level permissions (nodes, namespaces)
- Contact your cluster admin if permissions are missing

### UI / Display Issues

**Filter not working or typing doesn't filter**
- Make sure you're not in command palette mode (`:` or `/`)
- Press `Esc` to exit any active mode and try again
- Filter is fuzzy - try partial matches (e.g., "ngx" matches "nginx")

**Commands don't show up in palette**
- Commands are context-sensitive (e.g., `/scale` only for deployments/statefulsets)
- Make sure you have a resource selected (highlighted row)
- Press `/` (not `:`) for command palette

**Theme looks weird or colors are wrong**
- Check your terminal supports 256 colors
- Try a different theme: `k1 -theme nord`
- Some terminals may not display all themes correctly

**Resources not showing up**
- Check namespace: k1 shows resources from current context's namespace
- Switch namespaces via navigation palette (`:`)
- Verify resources exist: `kubectl get pods -n <namespace>`
- Check RBAC permissions (see above)

### Performance Issues

**Initial startup is slow (>5 seconds)**
- First sync loads all resources into cache (one-time cost)
- On very large clusters (5000+ resources), this can take longer
- Subsequent starts in same session are instant (cached)

**High memory usage**
- k1 caches all resources locally for speed
- Large clusters (10000+ resources) may use 500MB-1GB RAM
- This is normal - informer caching trades memory for speed

**Screen updates are slow**
- Check network connection to API server
- Very large resource lists (1000+ rows) may render slower
- Try filtering to reduce visible rows

## FAQ

### How is k1 different from k9s?

k1 is inspired by k9s but focuses on simplicity and speed:
- **Simpler UI**: Fewer modes, clearer command palette
- **Faster startup**: Optimized informer usage, protobuf encoding
- **Modern stack**: Built with Bubble Tea (Go's modern TUI framework)
- **AI integration**: Experimental `/ai` commands for natural language operations

Both are excellent tools - try both and use what feels better!

### Do I need kubectl installed?

No! k1 uses the Kubernetes Go client directly. However, some generated commands (logs, shell) are designed to work with kubectl for convenience.

### Does k1 modify my cluster?

k1 only modifies your cluster when you explicitly run a command:
- Read operations (view, describe, yaml) are completely safe
- Write operations (scale, restart, delete, drain) require confirmation
- k1 never modifies resources without your explicit action

### Where does k1 store configuration?

Currently, k1 is stateless - it doesn't persist any configuration between runs. Settings like theme and context must be specified via flags each time.

Future versions will support persistent configuration in `~/.config/k1/`.

### Can I use k1 with multiple clusters?

Yes! You can switch between clusters by specifying different contexts. Each k1 instance connects to one context at a time:
```bash
# Terminal 1: Connect to production
k1 -context production

# Terminal 2: Connect to staging
k1 -context staging
```

k1 respects your kubeconfig contexts just like kubectl.

### How do I report bugs or request features?

- **Bugs**: Open an issue at https://github.com/yourusername/k1/issues
- **Features**: Open a discussion or issue describing your use case
- **Security issues**: Email security@yourdomain.com (do not open public issues)

### Can I contribute to k1?

Absolutely! See [CLAUDE.md](CLAUDE.md) for development setup and contribution guidelines.

### What's the license?

k1 is licensed under GNU General Public License v3.0 - you're free to use, modify, and distribute it under the GPL terms.

## Contributing

See [CLAUDE.md](CLAUDE.md) for development setup and architecture documentation.

## Roadmap

- [ ] **Save preferences**: Theme, default namespace, window layout
- [ ] **Edit resources**: Modify YAML directly in the TUI
- [ ] **Live log streaming**: View pod logs without leaving k1
- [ ] **Enhanced AI commands**: More powerful natural language operations
- [ ] **Advanced search**: Find text in YAML and describe output
- [ ] **Copy to clipboard**: Export entire screen or selected resources
- [ ] **Interactive shell**: Execute commands in pods without copying

## Inspiration

k1 is inspired by:
- [k9s](https://k9scli.io/) - The OG Kubernetes TUI
- [lazygit](https://github.com/jesseduffield/lazygit) - Elegant terminal UI
- [fzf](https://github.com/junegunn/fzf) - Blazing fast fuzzy finder

## License

GNU General Public License v3.0 - See LICENSE file for details

