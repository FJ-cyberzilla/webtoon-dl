# 🌐 Webtoon Downloader CLI 
(Powered by FJ™ Cybertronic Systems)
------------------------------------------
https://fj-cyberzilla.github.io/webtoon-dl/
------------------------------------------
**Version:** 2.1.7

A high-performance, robust, and portable CLI tool written in **Go** to fetch and download webtoons from supported sites and convert them into organized PDF files.

*Brought to you by **FJ™ Cybertronic Systems**.*

---

## 🚀 Features

*   **Static Binary:** Built in Go for true portability. No complex Python dependencies required at runtime.
*   **Parallel Downloading:** Uses a worker-pool pattern to download multiple images per chapter simultaneously, significantly increasing speed.
*   **Professional CLI UI:** Branded terminal UI by **FJ™ Cybertronic Systems**, featuring color-coded logs and real-time animations.
*   **Persistent Logging:** Automatically captures all activity into daily log files (`./logs/`).
*   **Industrial-Grade Robustness (NEW):**
    *   **Atomic File Operations:** Prevents data corruption during interrupted writes.
    *   **Proactive Diagnostics:** Verifies disk space, network health, and write permissions before starting batches.
    *   **Resilient HTTP:** Centralized retry mechanism with exponential backoff and jitter to handle transient network errors.
    *   **Concurrency Safety:** Uses mutexes, thread-safe primitives, and the `singleflight` pattern to optimize duplicate network requests.
    *   **Context Propagation:** Comprehensive support for cancellation and timeouts across all I/O and network operations.
    *   **Worker Panic Recovery:** Automatically catches and logs panics in worker goroutines to prevent entire download processes from crashing.
*   **PDF Conversion:** Native PDF generation from images using pure Go libraries (no external C/C++ dependencies).
*   **Configurable API Integration:** Seamlessly configure and manage API keys for extended scraping capabilities via the integrated dashboard settings.

---

## 🛠️ Installation

### Prerequisites
*   [Go 1.22+](https://golang.org/dl/) installed.

### Build from Source
```bash
# Clone the repository
git clone https://github.com/FJ-cyberzilla/webtoon-dl.git
cd webtoon-dl

# Build the application using the branded Makefile
make build

# Move to your preferred bin directory
mv bin/webtoon-dl /usr/local/bin/ # or any path in your PATH
```

---

## 📖 Usage

Run the tool simply by executing:

```bash
./webtoon-dl
```

### Unified Dashboard
The application features an integrated TUI. Navigate and interact using:

*   **Platform Selection:** Press `[P]` to toggle between **WEBTOON** and **COMICLAND**.
*   **Comic Search:** Instead of a URL, simply type the **Comic Name** (e.g., *Tower of God*) directly into the dashboard prompt and press `[Enter]`. The application will automatically search for the comic on the selected platform.
*   **API Settings:** Press `[S]` to open the **API Settings** menu for managing your API keys.
*   **Exit:** Press `[Q]` or `[Ctrl+C]` to exit the application.
*   **Real-time Visibility:** The dashboard footer displays your system status and which API (if any) is currently active.

These settings are automatically persisted to a `.env` file in the project root. **Note:** This file contains sensitive API credentials and is intentionally excluded from git tracking to keep your configuration local and secure.

---

## 🏗️ Architecture

The project is structured for modularity and maintainability:

*   `cmd/webtoon-dl/`: Main entry point and CLI command handling.
*   `pkg/config/`: Configuration management, including `.env` and environment variable handling.
*   `pkg/scraper/`: Logic for parsing web pages and discovering chapters/images.
*   `pkg/downloader/`: Concurrent downloader engine using worker pools.
*   `pkg/diagnostics/`: System health, disk, and network probes.
*   `pkg/cache/`: Multi-tier (Memory + Disk) caching.
*   `pkg/pdf/`: Image-to-PDF conversion logic.
*   `pkg/ui/`: Branded CLI formatting, logging, and terminal UI components.
*   `my-webtoon-project/`: Multi-module project support using Go Workspaces for improved dependency management.

---

## 📝 Logging & Branding

*   **Logging:** Persistent logs are maintained in the `./logs` directory (`./logs/log_YYYY-MM-DD.txt`).
*   **Branding:** The project is proudly branded under **FJ™ Cybertronic Systems**, with distinctive purple styling applied to build outputs and the dashboard UI.

---

## 🎯 Supported Sites
*   ✅ Webtoons.com
*   ✅ Comicland.org

---

## 🛡️ Development & Standards

*   **Makefile:** The project uses a comprehensive `Makefile` to handle building, testing, linting, and quality audits, now enhanced with branded outputs.
*   **Linting:** The project utilizes a strict `golangci-lint` configuration to ensure high code quality.

---

*Built in Go by FJ™ Cybertronic Systems.*
