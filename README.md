# MiniDB

MiniDB is an AI-assisted desktop database client written in Go with [MyGo](https://mygo.run/)'s native UI: no webview, no frontend toolchain, one small binary. The interface keeps the compact, TablePlus-like macOS look.

**Language:** [简体中文](README.zh-CN.md) | English

## Features

- **Connections**: MySQL, PostgreSQL, SQLite, TiDB, StarRocks; SSH tunnels, read-only mode, environment tags and colors (the accent follows the current connection), encrypted backup and import.
- **Browse and edit**: database/table sidebar, paged data grid with server-side sort and filter (column conditions or raw SQL), block selection copied as TSV / CSV / JSON / SQL INSERT / Markdown; double-click to edit, add and delete rows, preview the exact SQL before it runs, commit in one transaction.
- **SQL editor**: syntax highlighting, table/column/alias completion, run current / selection / all, cancellable, multiple result sets, formatting, history and favorites.
- **Structure**: edit columns and indexes with a DDL preview; DDL view; Markdown table docs (AI can draft them).
- **AI assistant**: OpenAI-compatible APIs; streaming answers, a tool timeline (schema, sample rows, read-only SQL, explain plans…), structured results shown as tables; guardrails allow only a single read-only statement; chats are saved.
- **Import / export**: streaming CSV / JSON / SQL INSERT export, CSV / TSV / JSON import with column mapping, progress and cancel.
- **More**: command palette (⌘P), database switcher (⌘K), process list, Chinese and English, light/dark, session restore (tabs and query drafts), auto update.

Requires macOS 12 or later (Apple Silicon and Intel).

## Development

Go 1.27+ is all you need: no Node, pnpm or cgo.

```bash
go tool mygo dev            # development mode, rebuilds on Go changes
go test ./...               # tests (the UI runs windowless through ui.Tester)
go tool mygo build          # build/darwin-<arch>/MiniDB.app and .dmg
```

After editing `locales/*.json`, run `go tool mygo generate` to regenerate the message functions and `go tool mygo i18n check` to compare the two languages.

### Layout

```text
main.go            startup: logging, single instance, store, wiring
mygo.json          app config (name, identifier, version, i18n, macOS)
locales/           zh-Hans.json, en.json and the generated catalog_gen.go
internal/
  database/ agent/ ai/ schemaindex/ sshtunnel/ dataimport/ export/   domain layer, no UI
  sqltext/         SQL splitting, formatting, completion
  storage/         SQLite store, encryption, import of the old BoltDB data
  keychain/        the local encryption key from the system keychain
  app/             application layer: connections, queries, import/export, AI, history… (no UI)
  ui/              interface layer: reads and writes view state, calls the app layer
```

Dependencies point one way: `ui → app → domain`. View functions run every frame, so they do **no I/O**: blocking work goes through `uiThread.bg` and returns to the UI thread.

## Data and security

| Item | Location |
| --- | --- |
| App data | `~/Library/Application Support/MiniDB/minidb.sqlite` |
| Logs | `~/Library/Application Support/MiniDB/logs/` |
| Local encryption key | login keychain (falls back to `secret.key` in the data folder without one) |

- Database passwords, AI keys and SSH secrets are encrypted with AES-GCM before they are stored.
- On first launch, data of the old (Wails) version in `~/.minidb` is imported automatically; the old folder is left untouched.
- AI auto-execution allows only a single read-only statement and rejects writes, multiple statements and risky forms like `EXPLAIN ANALYZE`; writes happen only when you confirm the previewed SQL.
- The AI provider receives schema context and what you send; do not send sensitive data to a provider you do not trust.

`MINIDB_NO_KEYCHAIN=1` keeps the key in a file, for tests and headless runs.

## Shortcuts

| Shortcut | Action |
| --- | --- |
| `⌘N` / `⌘T` / `⌘W` | New connection / new query / close tab |
| `⌘P` / `⌘K` / `⌘I` | Command palette / switch database / AI assistant |
| `⌘↵` / `⇧⌘↵` | Run current statement or selection / run all |
| `⇧⌘F` | Format SQL |
| `⌘Y` | Query history |
| `⌘R` | Refresh |
| `⌘,` | Settings |

## Releasing

Pushing a `vX.Y.Z` tag runs `.github/workflows/release.yml`: tests, a check that the tag matches the version in `mygo.json`, a universal macOS build, and an upload to a draft GitHub release. Without a Developer ID the app is signed ad hoc; users open it once with right-click → Open.

Auto-update needs a key pair first: `go tool mygo keygen`, the public key into `updates` in `mygo.json`, the private key into the repository secret `MYGO_UPDATER_PRIVATE_KEY`.

## License

[MIT](LICENSE)
