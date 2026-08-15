# Security Policy

## Supported Versions

We active maintain and release security patches for `webtoon-dl`. Please ensure you are running the latest version before submitting a vulnerability report.

| Version | Supported          |
| ------- | ------------------ |
| 2.1.x   | :white_check_mark: |
| < 2.1.0 | :x:                |

## Reporting a Vulnerability

**Please do NOT report security vulnerabilities through public GitHub issues, discussions, or pull requests.**

If you discover a security vulnerability within `webtoon-dl`, please report it privately through one of the following methods:

1. **Direct Email (Preferred):** Send an email to **[cyberzilla.systems@gmail.com](mailto:cyberzilla.systems@gmail.com)** with the subject line `[SECURITY] webtoon-dl Vulnerability Report`.
2. **GitHub Private Advisory:** Navigate to the [Security Tab](https://github.com/FJ-cyberzilla/webtoon-dl/security/advisories/new) of this repository and click **"Report a vulnerability"**.

### What to Include in Your Report

To help us triage and resolve the issue quickly, please include:

* A clear description of the vulnerability and its potential impact.
* Detailed steps to reproduce (CLI commands, sample configuration, or proof-of-concept code).
* Affected components (e.g., `pkg/downloader`, `pkg/pdf`, `pkg/scraper`, CLI flags).
* Any suggested mitigations or fixes.

## Security Response Timeline

* **Acknowledgement:** We will acknowledge receipt of your security report within **48 hours**.
* **Assessment & Patching:** We will work to verify, patch, and test the fix as quickly as possible.
* **Public Disclosure:** Once a fix is released (e.g., v2.1.8+), we will publish a release note acknowledging your contribution (unless you request to remain anonymous).

Thank you for helping keep `webtoon-dl` secure!
