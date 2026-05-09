# Contributing to MiniDB

Thanks for taking the time to improve MiniDB.

## Development Setup

Requirements:

- Go 1.25+
- Node.js 18+
- pnpm 10+
- Wails CLI v3 alpha

Install Wails CLI:

```bash
set -a && . ./project.env && set +a
go install github.com/wailsapp/wails/v3/cmd/wails3@${WAILS_VERSION}
```

Install dependencies:

```bash
cd frontend && pnpm install && cd ..
go mod download
```

Run the app in development mode:

```bash
wails3 dev -config ./build/config.yml
```

## Verification

Run these checks before opening a pull request:

```bash
go test ./...
cd frontend && pnpm test && pnpm build
wails3 generate bindings -clean=true -ts
wails3 build
```

If you change exported Go service methods, regenerate Wails bindings and include the generated `frontend/bindings/` changes.

## Pull Requests

- Keep changes focused and explain user-visible behavior.
- Use pnpm for frontend dependencies; do not add npm or Yarn lockfiles.
- Add or update tests for behavior changes.
- Keep database logic, SQL generation, and data transformations in Go services rather than React components.
