# User Guide: webtoon-dl (Version 2.1.7)

Welcome to **webtoon-dl**, a high-performance, robust CLI tool for downloading webtoon series and converting them into organized PDF files. This project is proudly branded under **FJ™ Cybertronic Systems**.

---

## 🚀 Introduction

**webtoon-dl** streamlines the process of fetching high-quality webtoons from supported platforms, handling concurrent image downloads, and generating scaled PDF outputs with ease.

---

## 🛠️ Getting Started

### Installation
Ensure you have [Go 1.22+](https://golang.org/dl/) installed.

```bash
git clone https://github.com/FJ-cyberzilla/webtoon-dl.git
cd webtoon-dl
make build
# Binary is available at bin/webtoon-dl
```

---

## 📖 Usage

Run the tool simply by executing:

```bash
./webtoon-dl
```

### Integrated Dashboard & Workflow
Upon launch, you will be prompted to enter a **Comic Name** within the dashboard.

1.  **Select Platform:** Use `[P]` to toggle between **WEBTOON** and **COMICLAND**.
2.  **Search Comic:** Type the name of the comic you wish to find and press `[Enter]`.
3.  **Manage Settings:** Press `[S]` to configure API keys for extended capabilities.
4.  **Exit:** Press `[Q]` or `[Ctrl+C]` to quit.

The dashboard footer provides real-time information about your system status and active API configuration. Settings are automatically persisted to a `.env` file in the project root. **Note:** This file contains sensitive API credentials and is intentionally excluded from git tracking to keep your configuration local and secure.

---

## ⚙️ Advanced Features & API Fallback

To ensure stable operations, **webtoon-dl** implements a multi-tier fallback mechanism for web scraping.

### API Tiers
1.  **Tier 1 (Native):** The application attempts to fetch content directly.
2.  **Tier 2 (ScraperDog):** If Tier 1 fails, the application falls back to using the ScraperDog API.
3.  **Tier 3 (Secondary API):** If previous tiers fail, the application utilizes a secondary scraping API.

*Note: You can configure these keys in the dashboard.*

---

## 🛡️ Robustness & Reliability

**webtoon-dl** includes built-in safety mechanisms to ensure reliable downloads:

- **Input Validation:** All directory paths and titles are validated to prevent invalid file paths, path traversal, and empty input issues.
- **Atomic Filesystem Writes:** All image downloads are performed atomically, preventing partial/corrupt files on disk if a download is interrupted.
- **HTTP Retry Logic:** The downloader automatically retries transient HTTP errors (408, 429, 500-504) using exponential backoff, ensuring minimal impact from temporary network or server issues.

---

## 🏗️ Architecture

The project is modularized for maximum maintainability:

| Component | Description |
| :--- | :--- |
| `cmd/webtoon-dl/` | CLI entry point and command routing. |
| `pkg/config/` | Configuration handling (viper) and `.env` persistence. |
| `pkg/httpclient/` | Centralized HTTP client and User-Agent RoundTripper middleware. |
| `pkg/scraper/` | Web scraping logic. |
| `pkg/downloader/` | Concurrent image downloader engine. |
| `pkg/pdf/` | PDF generation logic. |
| `pkg/ui/` | Branded CLI UI and dashboard. |
| `my-webtoon-project/` | Multi-module project support using Go Workspaces. |

---

## 🛡️ Development & Standards

*   **Makefile:** Use `make` commands to build, test, and analyze the project code.
*   **Linting:** The project utilizes `golangci-lint` to maintain strict quality standards.


---

## 🏗️ Architectural Decisions

### File System & Case Sensitivity
- **Decision:** `webtoon-dl` preserves exact title casing provided by scraper sources and CLI arguments.
- **Rationale:** Operating system filesystems handle case sensitivity natively (Linux is case-sensitive; Windows/macOS are case-insensitive). Avoiding forced normalization preserves accurate comic titles while keeping file IO operations lightweight and portable.

---

*This application is proudly brought to you by **FJ™ Cybertronic Systems**.*
