# Codex Project Rules

## After Code Changes

After every code change, Codex must verify the project before reporting completion.

Run the applicable checks from the repository root:

```bash
go test ./...
cd frontend && pnpm test && pnpm build
wails3 generate bindings
wails3 build -config ./build/config.yml
```

If the change affects packaging, the desktop app bundle, or release artifacts, rebuild the target instance and DMG as well:

```bash
wails3 task package:darwin ARCH=arm64
./scripts/build.sh --arch arm64
```

After the build succeeds, restart the newly built instance so the latest code is running. Stop any previous app/dev instance first, then start a fresh instance with the new build or with:

```bash
wails3 dev -config ./build/config.yml
```

If any required command cannot be run in the current environment, Codex must state exactly which command was skipped and why.
