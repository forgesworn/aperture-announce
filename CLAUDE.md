# CLAUDE.md — aperture-announce

Standalone Go binary that reads Aperture's YAML config and publishes kind 31402 Nostr events.

## Commands

```bash
go build ./cmd/aperture-announce   # Build
go test ./...                      # Run all tests
go vet ./...                       # Lint
```

## Structure

```
cmd/aperture-announce/main.go      # CLI entry point
internal/
  config/config.go                 # Aperture YAML parser
  announce/announce.go             # Event building (kind 31402)
  announce/publish.go              # Relay publishing
  key/key.go                       # Key generation + persistence
  validate/validate.go             # URL validation (private host rejection)
```

## Conventions

- **British English** — colour, initialise, behaviour, licence
- **Go standard layout** — cmd/ for binaries, internal/ for private packages
- **Git:** commit messages use `type: description` format
- **Git:** Do NOT include `Co-Authored-By` lines
- **Event format:** Must match 402-announce output — `pmi` tags (not `payment`), `price` tags with positional elements (not JSON), capabilities in `content` field as JSON
