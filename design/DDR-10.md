# Flag Parsing: Simplicity-First Approach

| Metadata | Value                      |
|----------|----------------------------|
| Date     | 2025-10-05                 |
| Author   | @renato0307                |
| Status   | `Proposed`                 |
| Tags     | cli, flags, configuration  |

| Revision | Date       | Author       | Info                              |
|----------|------------|--------------|-----------------------------------|
| 1        | 2025-10-05 | @renato0307  | Initial design                    |
| 2        | 2025-10-05 | @renato0307  | Add config file requirement       |
| 3        | 2025-10-05 | @renato0307  | Add pflag+manual as recommended   |

## Context and Problem Statement

k1 currently uses stdlib `flag` for command-line parsing. As the
application grows, we need to evaluate whether to continue with stdlib
or adopt a more feature-rich library. The choice must prioritize
**simplicity** while enabling future growth (logging flags, debug
modes, potential subcommands).

**New requirement:** Configuration file support is needed for
persistent settings (~/.config/k1/config.yaml).

Key requirements:
- Simple to use and maintain (minimal boilerplate)
- Good UX (standard flag conventions like `--flag`)
- Configuration file support (YAML preferred)
- Priority order: flags > env vars > config file > defaults
- Support for future growth without major refactoring
- Minimal dependencies

## Option Analysis

### Option 1: stdlib flag + manual config

**Pros:**
- Zero external dependencies
- Already in use, no migration needed
- Complete control over config loading
- Simple, predictable behavior

**Cons:**
- Single-dash only (`-theme`, not `--theme`)
- No built-in subcommand support
- Manual config file handling required
- Need to implement priority logic manually

**Config approach:**
Use gopkg.in/yaml.v3 (or encoding/json) to load config, then
override with flags.

**Example:**
```go
// 1. Load config
type Config struct {
    Theme      string `yaml:"theme"`
    Context    string `yaml:"context"`
}
cfg := loadConfig("~/.config/k1/config.yaml")

// 2. Define flags with config defaults
themeFlag := flag.String("theme", cfg.Theme, "Theme")
flag.Parse()

// 3. Use flag value (already includes override logic)
fmt.Println(*themeFlag)
```

### Option 2: pflag + viper

**Pros:**
- POSIX-style: supports both `-t` and `--theme`
- Used by Kubernetes ecosystem (kubectl, kubebuilder, etc.)
- viper handles config files automatically (YAML/JSON/TOML/etc.)
- Automatic priority: flags > env > config > defaults
- Can bind flags to viper keys
- Battle-tested combo (kubectl, helm, etc.)

**Cons:**
- Two dependencies (pflag + viper)
- viper is feature-heavy (may be overkill)
- More "magic" than manual approach
- Slightly more complex setup

**Config approach:**
Viper automatically merges config file and flags.

**Example:**
```go
import (
    "github.com/spf13/pflag"
    "github.com/spf13/viper"
)

// Define flags
pflag.String("theme", "charm", "Theme")
pflag.Parse()

// Bind flags to viper
viper.BindPFlags(pflag.CommandLine)

// Load config (viper handles file discovery)
viper.SetConfigName("config")
viper.AddConfigPath("~/.config/k1")
viper.ReadInConfig()

// Use (priority already handled)
fmt.Println(viper.GetString("theme"))
```

### Option 3: kong + kong-yaml

**Pros:**
- Struct-based declaration (very clean, DRY)
- Auto-generates help from struct tags
- Built-in support for subcommands
- Modern, elegant API
- Excellent validation support
- Single struct for flags AND config
- Growing in popularity

**Cons:**
- Requires refactoring to struct-based approach
- Config support through separate plugin (kong-yaml)
- Less mature ecosystem than viper
- More "magic" (reflection-based)

**Config approach:**
Use kong.Configuration resolver to load YAML/JSON config.

**Example:**
```go
type CLI struct {
    Theme      string `default:"charm" help:"Theme to use"`
    Kubeconfig string `help:"Path to kubeconfig"`
    Context    string `help:"Kubernetes context"`
    Dummy      bool   `help:"Use dummy data"`
}

var cli CLI
parser := kong.Must(&cli,
    kong.Configuration(kong.JSON, "~/.config/k1/config.json"),
)
parser.Parse(os.Args[1:])
fmt.Println(cli.Theme)
```

**Note:** YAML support requires kong-yaml plugin or custom resolver.

### Option 4: cobra + viper

**Pros:**
- Full-featured framework with subcommands
- Used by major projects (kubectl, docker, hugo)
- Excellent documentation and ecosystem
- Viper integration is seamless (same author)
- Most mature config + flags combo

**Cons:**
- **Overkill for k1's current needs**
- Heavy framework (not just a flag parser)
- More boilerplate and complexity
- Multiple dependencies (cobra + viper + pflag)

**Verdict:** Too complex for simplicity-first requirement.

**Note:** If k1 evolves into a complex CLI with many subcommands,
cobra + viper becomes the obvious choice. For now, it's too much.

## Comparison Summary

| Feature                | stdlib+manual | pflag+manual | pflag+viper | kong      | cobra+viper |
|------------------------|---------------|--------------|-------------|-----------|-------------|
| Simplicity             | ⭐⭐⭐⭐⭐        | ⭐⭐⭐⭐         | ⭐⭐⭐          | ⭐⭐⭐        | ⭐⭐            |
| Dependencies           | 1 tiny        | 2 tiny       | 2 medium    | 1 medium  | 3 heavy     |
| Config auto-merge      | Manual        | Manual       | Auto        | Auto      | Auto        |
| Learning curve         | Minimal       | Minimal      | Low         | Medium    | Steep       |
| Boilerplate            | ~20 lines     | ~20 lines    | ~10 lines   | ~5 lines  | ~30 lines   |
| Predictability         | ⭐⭐⭐⭐⭐        | ⭐⭐⭐⭐⭐       | ⭐⭐⭐          | ⭐⭐⭐        | ⭐⭐            |
| `--flag` syntax        | ❌            | ✅           | ✅          | ✅        | ✅          |
| Shorthand flags        | ❌            | ✅           | ✅          | ✅        | ✅          |
| Env var support        | Manual        | Manual       | Auto        | Auto      | Auto        |
| Subcommand support     | Manual        | Manual       | Manual      | Built-in  | Built-in    |
| K8s ecosystem usage    | kubectl       | kubectl      | kubectl     | Rare      | kubectl     |

**Legend:** ⭐ = rating, ✅ = yes, ❌ = no

## Decision

**With config files in mind, three strong options emerge:**

### Option A: pflag + manual config (Recommended - best balance)

**Why:**
1. **Best of both worlds** - Better UX + full control
2. **Minimal dependencies** - pflag + gopkg.in/yaml.v3 (both tiny)
3. **Predictable** - Same manual config pattern, no magic
4. **Better CLI UX** - `--flag`, `-f` shorthand, kubectl-style
5. **Simple migration** - One line change from stdlib

**Trade-off:** Still manual priority handling (but it's trivial)

```go
import flag "github.com/spf13/pflag"  // Drop-in replacement

// 1. Load config with defaults
cfg := Config{Theme: "charm"}
if data, err := os.ReadFile(cfgPath); err == nil {
    yaml.Unmarshal(data, &cfg)
}

// 2. Flags override config (with shorthand support!)
flag.StringVarP(&cfg.Theme, "theme", "t", cfg.Theme, "Theme")
flag.StringVarP(&cfg.Context, "context", "c", cfg.Context, "Context")
flag.Parse()

// 3. Use cfg.* (priority: flag > file > default)
```

**Usage:** `k1 -t dracula -c prod` or `k1 --theme dracula --context prod`

**Perfect balance of simplicity, control, and UX.**

### Option B: stdlib flag + manual config (Maximum simplicity)

**Why:**
1. **Maximum simplicity** - You control exactly what happens
2. **Minimal dependencies** - Only gopkg.in/yaml.v3 (or stdlib json)
3. **Predictable** - No magic, easy to debug
4. **Flexible** - Config structure matches your exact needs
5. **Zero migration** - Already using stdlib flag

**Trade-off:** Manual priority handling (but it's ~10 lines of code)

```go
// 1. Load config with defaults
cfg := Config{Theme: "charm"}  // defaults
if data, err := os.ReadFile(cfgPath); err == nil {
    yaml.Unmarshal(data, &cfg)  // override from file
}

// 2. Flags override config
flag.StringVar(&cfg.Theme, "theme", cfg.Theme, "Theme")
flag.Parse()

// 3. Use cfg.Theme (priority: flag > file > default)
```

**Simple, explicit, predictable.**

### Option C: pflag + viper (For automation needs)

**Why:**
1. **Automatic handling** - Priority, env vars, file discovery
2. **Battle-tested** - kubectl, helm, etc. use this combo
3. **Flexible** - Supports many config formats
4. **Better CLI UX** - `--flag` syntax

**Trade-off:** Two dependencies, more "magic", potential overkill

```go
pflag.String("theme", "charm", "Theme")
viper.BindPFlags(pflag.CommandLine)
viper.SetConfigName("config")
viper.AddConfigPath("~/.config/k1")
viper.ReadInConfig()
theme := viper.GetString("theme")  // Priority handled
```

**Powerful, automatic, but more complex.**

## Recommendation

**Start with Option A (pflag + manual config).**

Rationale:
- **Best balance** - Better UX + simplicity + predictability
- Minimal migration: change one import line
- Two tiny dependencies (pflag + yaml.v3)
- Same manual config pattern (no viper magic)
- Industry-standard CLI UX (`--flag`, `-f` shorthand)
- Used by kubectl, so aligned with K8s ecosystem
- Easy to migrate to viper later if automation is needed

**Choose Option B (stdlib + manual) if:**
- You absolutely must avoid any external flag library
- Zero migration cost is critical
- Single-dash flags (`-theme`) are acceptable

**Choose Option C (pflag + viper) if:**
- Need complex env var precedence
- Need multiple config formats (TOML, INI, etc.)
- Config becomes nested and complex
- Prefer "batteries included" over explicit control

## Implementation (Option A: pflag + manual config)

### Step 0: Add Dependencies
```bash
go get github.com/spf13/pflag
go get gopkg.in/yaml.v3
```

### Step 1: Define Config Struct
```go
// internal/config/config.go
package config

type Config struct {
    Theme      string `yaml:"theme"`
    Kubeconfig string `yaml:"kubeconfig"`
    Context    string `yaml:"context"`
    LogLevel   string `yaml:"log_level"`
    LogFile    string `yaml:"log_file"`
}

func Default() Config {
    return Config{
        Theme:    "charm",
        LogLevel: "info",
        LogFile:  "~/.config/k1/logs/k1.log",
    }
}
```

### Step 2: Load Config with Priority
```go
// cmd/k1/main.go
import (
    "os"
    "path/filepath"

    flag "github.com/spf13/pflag"
    "gopkg.in/yaml.v3"
    "github.com/renato0307/k1/internal/config"
)

func loadConfig() config.Config {
    // 1. Start with defaults
    cfg := config.Default()

    // 2. Load from file (if exists)
    home, _ := os.UserHomeDir()
    cfgPath := filepath.Join(home, ".config/k1/config.yaml")
    if data, err := os.ReadFile(cfgPath); err == nil {
        yaml.Unmarshal(data, &cfg)
    }

    return cfg
}

func main() {
    cfg := loadConfig()

    // 3. Flags override config (with shorthand support!)
    flag.StringVarP(&cfg.Theme, "theme", "t", cfg.Theme, "Theme")
    flag.StringVarP(&cfg.Context, "context", "c", cfg.Context, "Context")
    flag.StringVarP(&cfg.Kubeconfig, "kubeconfig", "k", cfg.Kubeconfig, "Path to kubeconfig")
    flag.BoolVarP(&cfg.Dummy, "dummy", "d", cfg.Dummy, "Dummy mode")
    flag.Parse()

    // 4. Use cfg.* throughout app
    theme := ui.GetTheme(cfg.Theme)
    // ...
}
```

### Step 3: Add Config Save (Optional)
```go
func (c Config) Save(path string) error {
    data, err := yaml.Marshal(c)
    if err != nil {
        return err
    }
    return os.WriteFile(path, data, 0644)
}
```

### Future Growth Path
- **More settings:** Just add fields to Config struct
- **Automation:** Migrate to viper (already using pflag, easy!)
- **Subcommands:** Add simple routing or migrate to cobra/kong
- **Validation:** Add custom validators in loadConfig()

**Migration to viper (if needed later):**
```go
// Replace manual load with viper
viper.BindPFlags(flag.CommandLine)
viper.SetConfigName("config")
viper.AddConfigPath("~/.config/k1")
viper.ReadInConfig()
cfg.Theme = viper.GetString("theme")  // Auto-priority
```

## Consequences

**Positive:**
- Best balance of simplicity, control, and UX
- Industry-standard CLI conventions (`--flag`, `-f`)
- Full control over config loading logic (predictable, debuggable)
- Two small dependencies (pflag ~100KB, yaml.v3 ~50KB)
- Config file structure matches app needs exactly
- Minimal migration cost (one import line change)
- Easy to add new config fields
- Clear priority: flags > file > defaults
- Kubernetes ecosystem alignment (same flags as kubectl)
- Easy path to viper later (pflag is compatible)

**Neutral:**
- Manual priority handling (~10 lines of code, but explicit)
- No automatic env var support (can add manually if needed)

**Negative:**
- Need to write config loading boilerplate (~20 lines)
- Less "batteries included" than viper (but more predictable)
- No multi-format support (YAML only, but sufficient)

## Trade-offs Analysis

**Why not stdlib + manual?**
- Single-dash flags only (`-theme` not `--theme`)
- Less conventional CLI UX (kubectl uses `--flag`)
- No shorthand support (`-t` for `--theme`)
- Migration cost is minimal anyway (one line change)

**Why not pflag + viper?**
- Viper is 5MB+ with many transitive dependencies
- Adds complexity with auto-discovery, watchers, etc.
- Current needs are simple: load YAML, merge with flags
- Can migrate later if automation becomes valuable

**Why not kong?**
- Requires refactoring to struct-based tags
- Config support less mature than viper
- Reflection-based "magic" goes against simplicity goal

**Why not cobra?**
- Complete overkill for current needs
- Heavy framework with steep learning curve

## References

**Flag Parsing:**
- stdlib flag: https://pkg.go.dev/flag
- pflag: https://github.com/spf13/pflag
- kong: https://github.com/alecthomas/kong
- cobra: https://github.com/spf13/cobra

**Config Management:**
- viper: https://github.com/spf13/viper
- gopkg.in/yaml.v3: https://github.com/go-yaml/yaml
- kong config: https://github.com/alecthomas/kong#configuration

**Examples in the Wild:**
- kubectl: cobra + viper + pflag
- helm: cobra + viper
- k9s: simple custom config (similar to Option A)
- lazydocker: simple custom config
