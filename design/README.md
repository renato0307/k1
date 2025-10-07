# Design Documents for k1

DDR stands for Design Decision Record. This is a collection of design documents for k1.

| Index | Status | Tags | Description |
|-------|--------|------|-------------|
| [DDR-01](DDR-01.md) | ✅ Implemented | architecture, bubble-tea | Bubble Tea architecture patterns: single root model, state management, navigation, modals, async operations, repository pattern |
| [DDR-02](DDR-02.md) | ✅ Implemented | ui, theming, lipgloss | Theming system: component-based themes, adaptive colors, runtime switching, modal styling |
| [DDR-03](DDR-03.md) | ✅ Implemented | kubernetes, informers, repository, caching | Kubernetes informer-based repository: real-time data caching, query performance, lifecycle management, Bubble Tea integration |
| [DDR-04](DDR-04.md) | ✅ Implemented | testing, kubernetes, informers, envtest, integration-tests | Testing strategy for Kubernetes informer-based repository: envtest with shared TestMain pattern, real API server integration tests, CI/CD setup |
| [DDR-05](DDR-05.md) | 🚧 Partial | ui, ux, commands, llm | Command-enhanced list browser: expandable command bar, predefined and LLM commands, inline confirmations, full-screen views, command history and caching |
| [DDR-06](DDR-06.md) | ✅ Implemented | commands, describe, yaml, kubectl, informer | Resource detail commands: fast describe and YAML generation using informer cache and kubectl/pkg/describe library |
| [DDR-07](DDR-07.md) | ✅ Implemented | architecture, repository, screens, DRY, generics | Scalable multi-resource architecture: config-driven screens and dynamic informers to support 17+ resources with 67% less code |
| [DDR-08](DDR-08.md) | 🚧 Partial | commands, kubectl, performance, pragmatism, strategy | Pragmatic command implementation: kubectl subprocess first for quick delivery, migrate to pure Go client-go for performance over time |
| [DDR-09](DDR-09.md) | 📋 Proposed | logging, troubleshooting, diagnostics, slog | File-based structured logging system using slog: rotation, configurable verbosity, performance metrics, non-intrusive to TUI |
| [DDR-10](DDR-10.md) | 🚧 Partial | cli, flags, configuration | Flag parsing simplicity-first approach: pflag as drop-in stdlib replacement with better UX, Kubernetes ecosystem alignment, minimal migration |
| [DDR-11](DDR-11.md) | 📋 Proposed | metrics, observability, telemetry | Prometheus metrics collection: historical time-series retention with on-demand remote push to Push Gateway, Grafana Cloud, and self-hosted Grafana, privacy-first opt-in model |
| [DDR-12](DDR-12.md) | 📋 Proposed | llm, ai, kubectl, ollama, quantization, privacy | Local LLM architecture for kubectl command generation: Ollama-based inference with quantized SLMs (Phi-3/CodeLlama), Q4_K_M quantization strategy, domain-specific benchmarking, optional QLoRA fine-tuning |
| [DDR-13](DDR-13.md) | 📋 Proposed | navigation, ux | Contextual navigation with Enter key: drill-down from parent to child resources (Deployment→Pods, Service→Pods, Pod→Containers), detail views for ConfigMaps/Secrets, breadcrumb navigation |
| [DDR-14](DDR-14.md) | ✅ Implemented | refactoring, code-quality, architecture, solid, testing | Code quality analysis and refactoring implementation: reduced code duplication by 500 lines, split 1257-line CommandBar into 4 components, improved test coverage (commands 0%→65.9%), standardized error handling, extracted constants. Implemented via PLAN-08 |
| [DDR-14B](DDR-14B.md) | ✅ Implemented | refactoring, architecture, technical-debt, appcontext | Post-PLAN-08 architectural improvements: introduced AppContext pattern for dependency injection, audited and resolved false positive issues (ConfigScreen SRP, goroutine context), documented remaining items for future DDRs. Key architectural improvements complete |
