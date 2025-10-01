# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Timoneiro is an ultra-fast TUI client for Kubernetes, built with Go and Bubble Tea.

## Development Setup

Go version: 1.24.0

## Key Dependencies

- Bubble Tea (github.com/charmbracelet/bubbletea): TUI framework
- Kubernetes client-go: Kubernetes API client
  - k8s.io/client-go/metadata: Metadata-only informers (70-90% faster)
  - k8s.io/client-go/tools/cache: Informer cache implementation

## Running Prototypes

```bash
# Kubernetes informers with metadata-only mode (blazing fast)
go run cmd/proto-k8s-informers/main.go [--context CONTEXT]

# Bubble Tea TUI exploration
go run cmd/proto-bubbletea/main.go

# Full-featured pod list viewer (main prototype)
go run cmd/proto-pods-tui/main.go [--context CONTEXT]
```

## Current Focus: Prototypes / Proofs of Concept

1. **Fast Kubernetes Resource Fetching**: Explore using informers and build caching for the most common resources and search types to achieve very fast and efficient resource fetching.

2. **Bubble Tea TUI Exploration**: Build a fast and efficient TUI client using an immediate mode UI approach with Bubble Tea.

## Architecture Notes

This is an early-stage project focused on prototyping. The codebase structure will be established as development progresses.

## Prototype Learnings (cmd/proto-pods-tui)

### What Works Well

1. **Full Pod Informers with Protobuf** (Not Metadata-Only)
   - Originally planned metadata-only, but needed full pod status (Ready, Status, Restarts, Node, IP)
   - **Trade-off accepted**: Full informers with protobuf encoding still fast enough
   - Protobuf reduces payload size vs JSON: `config.ContentType = "application/vnd.kubernetes.protobuf"`
   - Real-world sync time: ~1-2 seconds for hundreds of pods

2. **Fuzzy Search is Superior**
   - Library: `github.com/sahilm/fuzzy`
   - Much better UX than exact substring matching
   - Search speed: 1-5ms for 100s of pods, 10-50ms for 1000s
   - Automatic ranking by match score (best matches first)
   - Negation still works: `!pattern` excludes fuzzy matches

3. **Bubble Tea Immediate Mode UI**
   - Full-screen mode with `tea.WithAltScreen()`
   - `bubbles/table` component handles most complexity
   - Dynamic column widths based on terminal size
   - Real-time updates from informer cache (1-second refresh)

4. **Smart Cursor Tracking**
   - Track selected pod by `namespace/name` key across filter/sort changes
   - Maintains selection when data updates (avoids jumping cursor)
   - Falls back gracefully when pod disappears

5. **Filter UX Patterns**
   - `/` to enter filter mode (vim-style)
   - Type to filter with live preview
   - Paste support (bracketed paste from terminal)
   - `!` prefix for negation (exclude matches)
   - ESC to clear/cancel
   - Show search timing in header for transparency

### Architecture Decisions

1. **Column Layout** (fixed widths except Name)
   - Namespace: 36 chars (fits UUIDs)
   - Name: Dynamic (fills remaining space, truncates with `...`)
   - Ready: 8 chars
   - Status: 12 chars
   - Restarts: 10 chars
   - Age: 10 chars
   - Node: 40 chars (fits long node names)
   - IP: 15 chars

2. **Sorting Strategy**
   - Primary: Age (newest first) - most common use case
   - Secondary: Name (alphabetical) - stable sort
   - Fuzzy search overrides with match score ranking

3. **Search Scope**
   - Fields: Namespace, Name, Status, Node, IP
   - Combine into single lowercase string for fuzzy matching
   - Pre-lowercase at search time (not at pod creation) to save memory

### Performance Strategy

1. **Kubernetes Informer Caching**
   - **Key Finding**: Informer caches load all-at-once (not progressive)
   - Once synced, cache queries are microsecond-fast (~15-25μs)
   - Real-time updates via watch connections
   - Accept brief initial sync time for instant subsequent queries

2. **Protobuf Encoding**
   - Use `application/vnd.kubernetes.protobuf` content type
   - Reduces network transfer and parsing time vs JSON
   - Transparent to application code (client-go handles it)

3. **Search Performance**
   - Fuzzy search on 100s of pods: 1-5ms (fast enough to run on every keystroke)
   - No debouncing needed for typical cluster sizes
   - Display timing in UI for transparency

4. **UI Rendering**
   - Bubble Tea handles efficient terminal updates
   - Only re-render on state changes (filter, data refresh, resize)
   - Table component optimized for scrolling large lists

### What to Avoid

1. **Metadata-Only Informers for Pod Lists**
   - Too limiting - need Status, Node, IP for useful display
   - Full informers with protobuf are fast enough

2. **Progressive Loading**
   - Informers don't support it (all-or-nothing cache sync)
   - Better to show loading indicator + fast sync than fake progress

3. **Complex String Operations**
   - Custom `toLower`/`contains` slower than stdlib
   - Use `strings.ToLower` and fuzzy library

4. **Table Height/Width Management**
   - Centralize calculation logic (avoid manual adjustments scattered in code)
   - Recalculate on window resize only

### Next Steps for Production

1. **Multi-Resource Support**
   - Pod list works well, replicate pattern for Deployments, Services, etc.
   - Lazy-load informers (only start when user views resource type)

2. **Drill-Down Views**
   - List view with metadata/summary
   - Detail view on selection (fetch full YAML/JSON if needed)
   - Log streaming for pods

3. **Namespace Filtering**
   - Allow watching specific namespaces only (reduces memory)
   - UI to switch namespaces or watch all

4. **Configuration**
   - Save column preferences
   - Default namespace/context
   - Keybindings

5. **Error Handling**
   - Better kubeconfig error messages
   - Handle disconnections gracefully
   - Show informer sync errors in UI
- Compile the code using "go build" but delete the binary after testing
- Execute go mod tidy to fix dependencies
- if you need to download repositories, save them into .tmp