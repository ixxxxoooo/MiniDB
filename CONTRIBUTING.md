# Contributing to MiniDB

Thanks for taking the time to improve MiniDB.

## Setup

Go 1.27+ only. MyGo's CLI is a `tool` dependency of this module:

```bash
go tool mygo dev      # run with live reload
```

## Before opening a pull request

```bash
go vet ./... && go tool mygo vet .
go tool mygo i18n check
CGO_ENABLED=0 go test ./...
go tool mygo build -skip-dmg
```

## Guidelines

- Keep the dependency direction `ui → app → domain`. Domain and app packages do not import `mygo/ui`.
- View functions run every frame: no I/O in them. Use `uiThread.bg` for blocking work.
- Keep database logic, SQL generation and data transformations in Go packages below `ui`, with tests.
- Test UI flows with `ui.Tester` (see `internal/ui/*_test.go`); set `MINIDB_SHOTS=<dir>` to write PNG screenshots.
- Add every user-visible string to both `locales/zh-Hans.json` and `locales/en.json`, then `go tool mygo generate`.
- Keep the original visual style: see `docs/FRONTEND_DEVELOPMENT_GUIDE.md` (tokens and rules) and `internal/ui/theme.go`.
