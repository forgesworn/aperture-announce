# Contributing

Contributions are welcome! This is a small Go project — getting started is straightforward.

## Setup

```bash
git clone https://github.com/TheCryptoDonkey/aperture-announce.git
cd aperture-announce
go build ./cmd/aperture-announce
```

## Development

```bash
go test ./...   # Run all tests
go vet ./...    # Lint
```

## Project structure

```
cmd/aperture-announce/main.go      # CLI entry point
internal/
  config/config.go                 # Aperture YAML parser
  announce/announce.go             # Event building (kind 31402)
  announce/publish.go              # Relay publishing
  key/key.go                       # Key generation + persistence
  validate/validate.go             # URL validation (private host rejection)
testdata/
  sample-conf.yaml                 # Example Aperture config for tests
```

## Conventions

- **British English** — colour, initialise, behaviour, licence
- **Go standard layout** — `cmd/` for binaries, `internal/` for private packages
- **Commit messages** — `type: description` format (e.g. `feat:`, `fix:`, `docs:`)

## Testing

Tests use Go's standard `testing` package with table-driven patterns. When adding or modifying functionality:

- Add test cases covering both success and error paths
- Use table-driven tests for functions with multiple input variations
- Run `go test ./...` — all tests must pass before submitting a PR

## Key documentation

- [Event format guide](docs/event-format.md) — kind 31402 tag semantics and content structure
- [JSON Schema](schemas/kind-31402.schema.json) — machine-readable event schema

## Submitting changes

1. Fork the repository
2. Create a feature branch
3. Make your changes with tests
4. Ensure `go test ./...` and `go vet ./...` pass
5. Open a pull request against `main`
