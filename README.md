# aperture-announce

[![MIT licence](https://img.shields.io/badge/licence-MIT-blue.svg)](./LICENSE)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8)](https://golang.org/)

Announce your [Aperture](https://github.com/lightninglabs/aperture) L402 services on Nostr for decentralised discovery.

Reads Aperture's YAML config, extracts service definitions, and publishes [kind 31402](https://github.com/TheCryptoDonkey/402-announce) events to Nostr relays. AI agents using [402-mcp](https://github.com/TheCryptoDonkey/402-mcp) can then discover your services via `l402_search`.

## Quick start

```bash
go install github.com/TheCryptoDonkey/aperture-announce/cmd/aperture-announce@latest

aperture-announce \
  --config /path/to/aperture.yaml \
  --relays wss://relay.damus.io,wss://nos.lol \
  --public-url https://api.example.com
```

## Usage

```
aperture-announce [flags]

Required:
  --config <path>        Path to Aperture YAML config
  --relays <urls>        Comma-separated Nostr relay URLs
  --public-url <url>     Public URL agents will use to reach your service

Optional:
  --announce-key <hex>   Nostr signing key (auto-generated if omitted)
  --interval <duration>  Re-publish interval (e.g. 6h); default: one-shot
  --picture <url>        Service icon URL
  --dry-run              Print event JSON without publishing
  --verbose              Verbose logging
```

Environment variables: `APERTURE_CONFIG`, `ANNOUNCE_RELAYS`, `PUBLIC_URL`, `ANNOUNCE_KEY`.

## How it works

1. Reads your `aperture.yaml` and extracts service names, paths, and pricing
2. Builds a kind 31402 Nostr event with the service metadata
3. Signs it with your announce key (auto-generated and saved to `~/.aperture-announce/announce.key` if not provided)
4. Publishes to the configured Nostr relays

Agents discover your service via `l402_search("your service")` and can then pay and consume it.

## Preview before publishing

```bash
aperture-announce \
  --config aperture.yaml \
  --public-url https://api.example.com \
  --dry-run
```

This prints the event JSON to stdout without connecting to any relay.

## Ecosystem

| Project | Role |
|---------|------|
| [aperture](https://github.com/lightninglabs/aperture) | L402 reverse proxy (what you're announcing) |
| [402-mcp](https://github.com/TheCryptoDonkey/402-mcp) | MCP client that discovers and pays for L402 APIs |
| [402-announce](https://github.com/TheCryptoDonkey/402-announce) | TypeScript equivalent of this tool |
| [toll-booth](https://github.com/TheCryptoDonkey/toll-booth) | Alternative L402 middleware |

## Licence

[MIT](LICENSE)
