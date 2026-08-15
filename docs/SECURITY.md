# 🛡️ Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 1.0.x   | :white_check_mark: |
| < 1.0   | :x:                |

---

## Reporting a Vulnerability

If you discover a security vulnerability within **Webtoon-dl**, please **do not** report it publicly on GitHub Issues.

Instead, please send a private disclosure to the maintainer via GitHub ([@FJ-cyberzilla](https://github.com/FJ-cyberzilla)).

### Please include:
1. Description of the issue (e.g., Arbitrary File Write, SSRF, Memory Leak).
2. Step-by-step proof of concept (PoC) or command invocation.
3. Impact assessment.


---

## Input Validation & Secure Path Handling

To prevent security vulnerabilities such as path traversal and buffer overflow attacks, Webtoon-dl implements strict input validation:

- **Path Sanitization:** All file paths are cleaned using `filepath.Clean()` to prevent directory traversal attacks (`../`).
- **Path Length Limits:** Input paths are restricted to 4096 characters to mitigate potential buffer-related issues.
- **Empty Input Rejection:** Validation functions (`ValidatePath`, `ValidateTitle`) explicitly reject empty or nil inputs to ensure valid operation.
- **Unicode Integrity:** Titles are validated for valid UTF-8 encoding to prevent injection or malformed data handling.

