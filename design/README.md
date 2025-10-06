# Design Documents for k1

DDR stands for Design Decision Record. This is a collection of design documents for k1.

| Index | Tags | Description |
|-------|------|-------------|
| [DDR-01](DDR-01.md) | architecture, bubble-tea | Bubble Tea architecture patterns: single root model, state management, navigation, modals, async operations, repository pattern |
| [DDR-02](DDR-02.md) | ui, theming, lipgloss | Theming system: component-based themes, adaptive colors, runtime switching, modal styling |
| [DDR-03](DDR-03.md) | kubernetes, informers, repository, caching | Kubernetes informer-based repository: real-time data caching, query performance, lifecycle management, Bubble Tea integration |
| [DDR-04](DDR-04.md) | testing, kubernetes, informers, envtest, integration-tests | Testing strategy for Kubernetes informer-based repository: envtest with shared TestMain pattern, real API server integration tests, CI/CD setup |
| [DDR-05](DDR-05.md) | ui, ux, commands, llm | Command-enhanced list browser: expandable command bar, predefined and LLM commands, inline confirmations, full-screen views, command history and caching |
| [DDR-06](DDR-06.md) | commands, describe, yaml, kubectl, informer | Resource detail commands: fast describe and YAML generation using informer cache and kubectl/pkg/describe library |
| [DDR-07](DDR-07.md) | architecture, repository, screens, DRY, generics | Scalable multi-resource architecture: config-driven screens and dynamic informers to support 17+ resources with 67% less code |
| [DDR-08](DDR-08.md) | commands, kubectl, performance, pragmatism, strategy | Pragmatic command implementation: kubectl subprocess first for quick delivery, migrate to pure Go client-go for performance over time |
| [DDR-09](DDR-09.md) | logging, troubleshooting, diagnostics, slog | File-based structured logging system using slog: rotation, configurable verbosity, performance metrics, non-intrusive to TUI |
| [DDR-10](DDR-10.md) | cli, flags, configuration | Flag parsing simplicity-first approach: pflag as drop-in stdlib replacement with better UX, Kubernetes ecosystem alignment, minimal migration |
| [DDR-11](DDR-11.md) | metrics, observability, telemetry | Prometheus metrics collection: historical time-series retention with on-demand remote push to Push Gateway, Grafana Cloud, and self-hosted Grafana, privacy-first opt-in model |
