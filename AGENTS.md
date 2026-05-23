# AGENTS.md

## Commands
- Use Go from `go.mod` (`go 1.25.8`); CI installs Go from this file.
- Portable verification is `go build ./...` then `go test ./...`; CI runs exactly those commands.
- Focused tests: `go test ./internal/tui/components/prssection/...` or `go test ./internal/tui/components/prssection -run TestName`.
- `task test` is not the CI source of truth: it runs `prism test {{.CLI_ARGS}} ./...` and requires `prism` locally.
- `task test:one` and `task test:rerun` require `gotip`; do not assume they exist in a clean environment.
- Formatting is `task fmt`, which runs `gofumpt -w $(git ls-files '*.go')`; do not substitute plain `gofmt` when matching repo formatting.
- Lint is `task lint`, which runs `golangci-lint run --path-mode=abs --config=.golangci.yml --timeout=5m` with `GOEXPERIMENT` unset.
- To install the local extension, run `go build . && gh ext remove dash && gh ext install . && gh dash --version`.
- Debug TUI runs are `task debug -- <args>` or `DEBUG=true go run . --debug <args>`; logs go to `./debug.log`, and `LOG_LEVEL=warn` is supported.

## Architecture
- Entrypoint is `gh-dash.go` -> `cmd.Execute()`; Cobra command setup and config/debug/profile wiring live in `cmd/root.go`.
- TUI state starts in `internal/tui/ui.go` (`tui.Model`); Bubble Tea section models live under `internal/tui/components/*section`.
- Shared section behavior is in `internal/tui/components/section`; PR rows/views, issue views, notifications, repo view, and sidebars are separate component packages.
- GitHub data fetching lives in `internal/data` and mostly uses GitHub GraphQL via `github.com/cli/go-gh/v2`; command/task side effects are under `internal/tui/components/tasks`.
- Local git repository inspection is isolated in `internal/git`; repo-path expansion helpers are in `internal/tui/common`.
- Docs are a separate Astro app under `docs/` with `pnpm@10.14.0`; use `task docs`, `task docs-build`, or `cd docs && pnpm build` for docs-only work.

## Config And Feature Flags
- Executable config precedence in `internal/config/parser.go`: `--config`, then `GH_DASH_CONFIG`, then repo-local `.gh-dash.yml`/`.gh-dash.yaml`, merged over global `$XDG_CONFIG_HOME/gh-dash/config.yml` or `~/.config/gh-dash/config.yml`.
- If the global config file is missing, the app creates it before loading config; tests that should avoid this use `Location.SkipGlobalConfig`.
- Config merge is not a plain deep merge: keybindings are merged by key, while `prSections`, `issuesSections`, and `notificationsSections` from the provided config replace the global sections.
- `FF_REPO_VIEW` gates repo view; if config default view is `repo` without this env var, it is forced back to `prs`.
- `FF_MOCK_DATA` makes PR fetching use a localhost GraphQL mock at `https://localhost:3000` with a fake token and disabled TLS verification.

## CI And Release
- PR CI ignores `docs/**` for Go build/test but has a separate docs build workflow for docs changes.
- Go release runs GoReleaser on tags only, with `CGO_ENABLED=0` and build tag `nodbus`; it runs `go mod tidy` as a before hook.
