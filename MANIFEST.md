# 📜 MANIFEST.md — Webtoon-dl

> **Application Name:** Webtoon-dl  
> **Author / Maintainer:** [FJ-cyberzilla](https://github.com/FJ-cyberzilla)  
> **License:** MIT  

---

## 🎯 Supported Sites

Webtoon-dl supports downloading comics and chapters from the following platforms:

* ✅ **Webtoons.com**
* ✅ **Comicland.org**

---

## 🚀 Quick Start & Installation

To clone and set up the repository locally:

```bash
git clone [https://github.com/FJ-cyberzilla/webtoon-dl.git](https://github.com/FJ-cyberzilla/webtoon-dl.git)
cd webtoon-dl
go mod download

Running the Application
​You can start the interactive Terminal UI (TUI) directly using Go:

go run ./cmd/webtoon-dl


Or build the local binary using the Makefile:



make build
./bin/webtoon-dl



🛡️ Development & Standards
​Makefile Automation: The project utilizes a comprehensive Makefile to streamline building, testing, linting, and diagnostics with custom branded output.
​Strict Code Quality: Integrated with golangci-lint to enforce idiomatic Go practices, static analysis, and zero dead code.
​Optimized PDF Generation: Uses sync.Pool buffer pooling and multi-worker goroutines (golang.org/x/sync/errgroup) for high-throughput parallel image processing.
​🏛️ System Architecture


[ CLI Layer (cmd/webtoon-dl) ]
             │
             ▼
[ Scraper Layer (pkg/scraper) ] ──► Webtoons.com / Comicland.org
             │
             ▼
[ Downloader Engine (pkg/downloader) ] ──► Concurrent Worker Pools & Retries
             │
             ▼
[ PDF Processor (pkg/pdf) ] ──► Parallel Compression & Output
             │
             ▼
[ Interactive TUI (pkg/ui) ] ──► Bubble Tea / Lipgloss Interface




📁 Repository Manifest (pkg/ Structure)



Directory / FileDescription
cmd/webtoon-dl/Application entry point (main.go), Cobra CLI commands (root.go), and site specific sub-commands (doctor_toon.go).
pkg/downloader/Concurrent HTTP client, batch downloading engine, retry mechanics, and dual-client routines.
pkg/httpclient/Centralized HTTP client builder and User-Agent RoundTripper middleware.
pkg/pdf/PDF compilation engine with multi-core worker pools, image scaling, JPEG compression, and sync.Pool memory optimization.
pkg/scraper/HTML parsing and metadata extraction routines for target webtoon platforms.
pkg/ui/Interactive Terminal UI (TUI) state machines, dashboards, selectors, and Lipgloss styles.
pkg/config/Global application configuration loader and options management.
pkg/cache/Local metadata and session caching layer.
pkg/diagnostics/System health checks, runtime diagnostics, and log utilities.
pkg/models/Core domain models (Comic, Chapter, Page).
docs/Architectural diagrams (STRUCTURE.mmd) and user manuals (USERGUIDE.md).




🛠️ Common Makefile Commands


CommandAction
make buildCompiles the webtoon-dl binary to bin/.
make testRuns unit tests across all packages (pkg/...).
make lintRuns golangci-lint code checks.
make benchmarkRuns PDF processing and downloader performance benchmarks.
make cleanRemoves compiled binaries and temporary log files.



