# Burnmail

🔥 A Go CLI and terminal UI for generating and managing disposable email addresses through the [mail.tm](https://mail.tm) API.

## Development Setup

```bash
# Installation
go mod download        # or make deps (download + tidy + verify)

# Development
go run main.go         # or make run; the TUI needs a real terminal

# Build
make build             # optimized: -ldflags "-s -w", -trimpath, version from git tag
make build-dev         # with debug symbols
make build-all         # cross-compile 5 platforms (linux/darwin amd64+arm64, windows amd64)

# Tests
make test              # go test -v -race -cover ./...
go test ./cmd          # single package
make bench             # go test -bench=. -benchmem ./...

# Lint (CI gates on all three)
go vet ./...
gofmt -s -w .
golangci-lint run ./...   # default config, no .golangci.yml
```

## Tech Layers

- **Framework**: spf13/cobra (command tree) + Bubble Tea v2 (charm.land/bubbletea, bubbles, lipgloss) for the TUI; manifoldco/promptui for the classic view — both interactive frontends live behind `internal/ui`
- **Language**: Go 1.26 (go.mod: 1.26.0; CI matrix pins 1.26 with GOTOOLCHAIN=local; release.yml uses `stable`; Dockerfile builds on `golang:1.26-alpine`; local dev via mise `go = "latest"`)
- **Styling**: lipgloss styles in the TUI; fatih/color print helpers (`cyan()/green()/yellow()/red()` closures) in command output
- **Storage**: `~/.burnmail.json` (0600) encrypted with AES-256-GCM (PBKDF2-HMAC-SHA256, 100k iterations); key lives in the OS keyring via zalando/go-keyring, with a warned plaintext fallback. Message cache in `~/.burnmail-cache.json` (5 min expiry)
- **API**: mail.tm REST (`https://api.mail.tm`), plain-array or `hydra:member` response envelopes
- **Testing**: standard library `testing` only (no testify), table-driven, colocated

## Project Structure

```
burnmail/
├── main.go                # minimal entry: calls cmd.Execute()
├── cmd/                   # cobra wiring only — no rendering
│   ├── root.go            # command tree, Execute(), error reporting
│   ├── account.go         # g/generate, d/delete, me (plain stdout commands)
│   ├── messages.go        # m (TUI) and list (classic): loadAccount -> internal/ui
│   ├── export.go          # export/exp (JSON dump of messages)
│   ├── helpers.go         # loadAccount, fetchMessages, generateRandomString
│   └── *_test.go          # tests for the cmd package
├── internal/
│   ├── api/
│   │   ├── client.go      # mail.tm client: singleton via GetClient(), rate limiter, models
│   │   └── retry.go       # RetryWithBackoff (429 exponential backoff), RequestTimeout
│   ├── config/
│   │   └── config.go      # Version (injected via -X, git tag is the source of truth)
│   ├── storage/
│   │   ├── storage.go     # Save/Load/Delete/Exists for the account file
│   │   └── encryption.go  # Encrypt/Decrypt (AES-GCM + PBKDF2)
│   └── ui/                # interactive frontends — leaf package
│       ├── ui.go          # RunInbox: Bubble Tea entry point
│       ├── model.go       # model struct, Init, tick, cache, state mutators
│       ├── update.go      # Update, tea.Cmd factories, confirm flow
│       ├── view.go        # View, render helpers, truncate
│       ├── styles.go      # lipgloss styles
│       ├── classic.go     # RunClassic: promptui list view (plain printing)
│       ├── html.go        # email HTML → formatted plain text
│       ├── browser.go     # openInBrowser (temp HTML file + cleanup)
│       ├── attachments.go # downloads dir resolution, safeFilename, download
│       └── *_test.go      # tests moved with their code
├── completions/           # pre-generated shell completion scripts
└── .github/workflows/     # CI.yml (lint+test+build), release.yml (tag-driven)
```

## Code Standards

### General Rules
- `gofmt -s` on every file; imports grouped: stdlib, third-party, local (`burnmail/internal/api`, …)
- Cobra commands always use `RunE`, never `Run`: handlers `return fmt.Errorf("failed to x: %w", err)`; the only printing error site is `Execute()` (red ✗ to stderr, exit 1)
- Error strings lowercase, no trailing punctuation; wrap with `%w`; check every error (errcheck in CI — its default exclusions cover `fmt.Printf` and writes to `os.Stdout`/`os.Stderr`, **not** `fmt.Fprintf` to arbitrary `io.Writer`)
- Command output goes through `cmd.OutOrStdout()`/`cmd.Printf`; declare `Args:` validators (`cobra.NoArgs`, `MatchAll(ExactArgs(1), OnlyValidArgs)`) instead of len() checks
- Use `errors.Is`/`errors.As` for sentinel inspection, never `==`
- Keep `main.go` minimal; version is ldflags-injected into `internal/config.Version` (`-X burnmail/internal/config.Version=`), source of truth is the git tag — the exact same string lives in `Makefile`, `Dockerfile`, `build.sh`, `build.ps1`, `release.yml` and the Homebrew formula, so any change must update all of them atomically

### Naming Conventions
- Exported: PascalCase (`GetDomains`, `MessageDetail`); unexported: camelCase (`retryMaxAttempts`, `loadAccount`)
- Cobra command vars: `<name>Cmd` (`generateCmd`, `messagesListCmd`)
- Constants: MixedCaps (`retryMaxAttempts`, `htmlFileCleanupDelay`), no SCREAMING_SNAKE
- Receiver names short and consistent; pointer receivers for methods on structs
- Types for API responses carry JSON tags; `time.Time` for dates

### File Organization
- One command family per file in `cmd/`; tests colocated and in the same package (they exercise unexported code directly)
- `internal/api` owns wire types + HTTP; `internal/storage` owns persistence + crypto; `internal/ui` owns *all* interactive rendering (TUI, promptui view, HTML conversion, attachment downloads); `cmd/` owns cobra wiring and plain command output — no cross-layer leakage (e.g. credentials must not ride along into UI structs; see `exportAccount`)
- Unexport aggressively; exporting is a breaking commitment

## Important Patterns

### Dependency direction (law)
Import flow is strictly one-way: `cmd → ui → {api, storage}`. The frontends are leaf packages; the domain must never know they exist:
- Only `cmd/` may import `internal/ui`; `internal/ui` never imports `cmd`
- `charm.land/*` (bubbletea, bubbles, lipgloss) and `promptui` may appear ONLY inside `internal/ui`
- `internal/api`, `internal/storage`, `internal/config` take plain args and return results — testable without a terminal
- Shared HTTP plumbing (backoff, timeouts) lives in `internal/api` so both `cmd` and `ui` can use it without either importing the other's layer
- Self-check: `grep -rlE 'charm.land|promptui' --include='*.go' cmd internal` lists only `internal/ui/` files

This convention is shared with mkvtea (same rules, package names aside); keep both AGENTS.md files aligned when either layout changes.

### API Calls
`api.GetClient()` returns a process-wide singleton with a token-bucket rate limiter (5 tokens, 200 ms min spacing). Every endpoint method calls `waitForRateLimit()` first — including mutations, so bulk operations stay throttled. Wrap calls in the generic helper for retry on 429:
```go
client := api.GetClient()
client.SetToken(accountData.Token)

ctx, cancel := context.WithTimeout(context.Background(), api.RequestTimeout)
defer cancel()
messages, err := api.RetryWithBackoff(ctx, func() ([]api.Message, error) {
	return client.GetMessages()
})
if err != nil {
	return fmt.Errorf("failed to get messages: %w", err)
}
```
Responses may be a plain JSON array or a `hydra:member` envelope; both shapes must parse.

### State Management
- TUI state lives entirely in `model` (internal/ui/model.go); mutate only inside `Update`, never from goroutines
- All async work is a `tea.Cmd` returning a typed message (`messagesLoadedMsg`, `attachmentDownloadedMsg`, …) — no fire-and-forget goroutines; failures must surface via `statusMessage`, not silence
- Exactly one self-perpetuating tick chain (started in `Init`, re-armed only by `tickMsg`); no other handler may call `tickCmd()` — that bug once doubled timers per refresh
- Remote input is hostile by default: attachment filenames pass through `safeFilename()` before joining paths; HTML is converted, never rendered raw
- Random identifiers/passwords use `crypto/rand` with rejection sampling (no modulo bias)

## Testing Guidelines

- Write tests alongside implementation; standard library only, table-driven
- Test behavior with pure seams: `newTestModel()` + feeding `m.Update(msg)` directly; assert on resulting state and returned cmds, not internals
- Use `t.Setenv`/`t.TempDir` for filesystem-touching code; guard platform-specific assertions with `runtime.GOOS`
- Regression tests get a comment naming the bug they pin (see `TestMessagesLoadedDoesNotScheduleExtraTick`)
- Run `go test -race ./...` before pushing; CI adds golangci-lint (default linters) and the race suite — a lint failure fails the job before tests run
- No coverage threshold is enforced; critical paths (crypto round-trip, converter output) are covered with golden-style cases

## Common Pitfalls to Avoid

- DON'T: report errors by printing inside handlers and returning nil — exit code must reflect failure
- DON'T: render outside `internal/ui` — anything with a cursor, a spinner or a keymap belongs to the UI package; `cmd/` handlers print plain results or call `ui.RunInbox`/`ui.RunClassic`
- DON'T: use `interface{}` + bare type assertions where a generic works (`api.RetryWithBackoff[T]`)
- DON'T: trust remote strings for paths, filenames, or HTML display
- DON'T: let credentials leak into exports, logs, or cache; `storage.AccountData` fields are secrets
- DON'T: add `fmt.Fprintf(w, …)` unchecked to satisfy errcheck — use cobra's `cmd.Printf` or check the error
- DON'T: sleep while holding a mutex; reserve the deadline under the lock, sleep outside
- DON'T: write tests that only `t.Log` — a test that cannot fail is dead weight
- DO: run the exact CI linter version locally (`golangci-lint run ./...`) before pushing
- DO: follow the existing command pattern (RunE + wrapped errors + single print site) for every new subcommand
- DO: update this file when commands, storage layout, or release flow change

## Performance Considerations

- One shared `http.Client` (30 s timeout, keep-alive, 10 idle conns) — never per-call clients
- Rate limiting serializes API bursts by design; bulk operations are sequential, not goroutine-per-item
- Messages cache to disk (5 min TTL) and re-render the table only on state change
- `strings.Builder` for all multi-write string assembly; truncate/pad in runes to keep the table aligned without invalid UTF-8
- Keep binaries small: `-s -w -trimpath`; zero runtime dependencies, single static binary

## Deployment

- Tag-driven release: `git tag vX.Y.Z && git push origin vX.Y.Z` triggers `release.yml` — tests, cross-builds 5 platforms, GitHub Release (changelog + `checksums.txt`), multi-arch image to `ghcr.io/fraluc06/burnmail` (versioned + `latest`)
- The tag is the single source of truth for the version; `internal/config` keeps `Version = "dev"` and every build site injects it via `-X burnmail/internal/config.Version`
- Users install via Homebrew (`brew install fraluc06/burnmail/burnmail`), `go install github.com/fraluc06/burnmail@latest`, or Docker (`docker pull ghcr.io/fraluc06/burnmail:latest`)
- CI on push/PR to `main` and `dev`: golangci-lint → `go test -race -coverprofile` → build; CodeQL runs on push + weekly schedule
- No secrets in CI beyond the default `GITHUB_TOKEN`; no runtime env vars required (public mail.tm API)

## Additional Resources

- Feature/usage docs: README.md
- API endpoint walkthrough: API_EXAMPLES.md
- Release pipeline: .github/workflows/release.yml; CI: .github/workflows/CI.yml
- Container build: Dockerfile (non-root, multi-arch) + docker-compose.yml
- Upstream API: https://mail.tm (REST, JWT bearer auth)
- Sister project sharing the layout conventions: mkvtea (AGENTS.md there mirrors the dependency-direction law above)
- MIT License © 2025 Francesco Lucarelli
