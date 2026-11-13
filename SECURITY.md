# Security Policy

## Supported Versions

MiniDB is pre-1.0. Security fixes are provided for the latest released version only.

## Reporting a Vulnerability

Please do not report security vulnerabilities in public GitHub issues.

Email the maintainer at `security@example.com` with:

- A clear description of the issue
- Steps to reproduce
- Affected version or commit
- Any relevant logs, screenshots, or proof of concept

Replace the placeholder email before the first public release.

## Secret Handling

MiniDB stores database passwords, AI API keys, and custom AI headers locally. These values are encrypted before being written to the local SQLite store. The encryption key lives in the system keychain (or in a key file next to the store when no keychain is available).
