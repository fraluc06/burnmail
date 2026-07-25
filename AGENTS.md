# Agent Guidelines for Burnmail

## Build & Test Commands
- Build: `make build` or `go build -o burnmail -ldflags="-s -w" .`
- Test all: `make test` or `go test -v ./...`
- Test single package: `go test -v ./cmd` or `go test -v ./api` or `go test -v ./storage`
- Run: `make run` or `go run main.go`
- Format: `gofmt -s -w .`
- Lint: `go vet ./...`

## Release Process
- Tag-driven: `git tag vX.Y.Z && git push origin vX.Y.Z` triggers `.github/workflows/release.yml`
- The tag is the single source of truth for the version; it is injected via `-ldflags -X main.Version=...` (main.go stays at "dev")
- The workflow tests, cross-builds 5 platforms, creates the GitHub Release with changelog + checksums, and pushes the multi-arch image to GHCR

## Code Style
- **Imports**: Group standard library, then third-party, then local packages (e.g., `burnmail/api`, `burnmail/cmd`, `burnmail/storage`)
- **Formatting**: Use `gofmt` for all Go files; tabs for indentation
- **Types**: Define types for API responses with JSON tags; use `time.Time` for dates
- **Naming**: Use camelCase for private, PascalCase for exported; descriptive names (e.g., `generateRandomString`, `GetDomains`)
- **Error handling**: Always check errors; use `fmt.Errorf` for context; return errors up the stack
- **Structs**: Use pointer receivers for methods; embed structs where appropriate (e.g., `MessageDetail` embeds `Message`)
- **CLI**: Use cobra for commands with short aliases (e.g., `g` for `generate`); use `fatih/color` for colored output
- **HTTP**: Set timeouts on HTTP clients (30s); always defer `resp.Body.Close()`; handle multiple API response formats
- **Storage**: Store config in `~/.burnmail.json` with 0600 permissions; handle missing files gracefully
