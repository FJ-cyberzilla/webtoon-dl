# Contributing to Webtoon-dl

We welcome contributions to `webtoon-dl`! Please follow these guidelines to ensure consistency and high quality.

---

## 1. Fundamental Execution Rules (Strict Enforcement)

1. **Zero Hallucinated Code:** NEVER provide placeholder functions, un-exported types without definitions, dummy blocks (`// TODO: implement here`), or non-working standard library hacks.
2. **Complete & Runnable Code:** All generated Go files MUST be complete, syntactically valid Go code with all necessary package imports, type definitions, and proper error handling.
3. **No Breaking Refactors:** Before introducing new Go packages or external dependencies, verify compatibility with the existing module structure (`go.mod`, `go.sum`).
4. **Context Respect:** Do not rename core exported models, struct signatures, or interface contracts without explicitly updating all call sites across `pkg/`.

---

## 2. Go Language & Style Conventions

* **Go Version:** Go 1.21+ standard syntax and idioms.
* **Concurrency Principles:**
  * Use `context.Context` propagation for cancellation and timeouts across all scrapers, HTTP requests, and disk I/O.
  * Prefer `golang.org/x/sync/errgroup` over raw `sync.WaitGroup` when managing bounded, concurrent worker pools.
  * Always protect concurrent memory mutations using standard primitives (`sync.Mutex`, atomic counters).
* **Error Handling:**
  * Use Go wrapped errors: `fmt.Errorf("failed doing action: %w", err)`.
  * Never ignore returned errors with blank identifiers `_` unless explicitly safe and commented.
* **HTTP & I/O Safeguards:**
  * All HTTP clients MUST be decorated or created using the `pkg/httpclient` package (`httpclient.NewClient` or `httpclient.DecorateClient`) to automatically and dynamically attach the standard `User-Agent` header via an `http.RoundTripper` decorator.
  * Every HTTP request MUST configure appropriate additional headers like `Referer` where necessary (e.g., `Referer: https://www.webtoons.com/`) to prevent `403 Forbidden` responses.
  * Close all `io.ReadCloser` (`resp.Body.Close()`) immediately using `defer`.

---

## 3. Terminal & TUI Architecture (`pkg/ui` & CLI)

* **Charm Stack Usage:**
  * `bubbletea` for TUI state management (Model-View-Update pattern).
  * `lipgloss` for styling and terminal UI rendering.
  * **Lipgloss Rule:** Lipgloss styles DO NOT possess a `.Sprintf()` method. Always format with standard Go strings: `style.Render(fmt.Sprintf(...))`.
* **Cobra & Viper Integration:**
  * Command definitions belong in `cmd/webtoon-dl/root.go`.
  * `cmd/webtoon-dl/main.go` MUST remain ultra-lean, invoking only `main.Execute()`.
  * Flags bound via `viper.BindPFlag` must maintain fallback configuration support via `config.yaml`.

---

## 4. Development & Workflow
* **Makefile:** The project uses a comprehensive `Makefile` to handle building, testing, linting, and quality audits.
* **Linting:** Run `make lint` before submitting any PRs.
* **Testing:** Run `make test` and `make coverage` to ensure code quality and coverage thresholds (>80%).
