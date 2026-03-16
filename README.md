# aperture-announce

[![MIT licence](https://img.shields.io/badge/licence-MIT-blue.svg)](./LICENSE)
[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8)](https://golang.org/)
[![CI](https://github.com/TheCryptoDonkey/aperture-announce/actions/workflows/ci.yml/badge.svg)](https://github.com/TheCryptoDonkey/aperture-announce/actions/workflows/ci.yml)

Announce your [Aperture](https://github.com/lightninglabs/aperture) L402 services on Nostr for decentralised discovery.

Reads Aperture's YAML config, extracts service definitions, and publishes [kind 31402](https://github.com/TheCryptoDonkey/402-announce) events to Nostr relays. AI agents using [402-mcp](https://github.com/TheCryptoDonkey/402-mcp) can then discover your services via `l402_search`.

## Quick start

```bash
go install github.com/TheCryptoDonkey/aperture-announce/cmd/aperture-announce@latest

aperture-announce \
  --config /path/to/aperture.yaml \
  --relays wss://relay.damus.io,wss://nos.lol \
  --public-urls https://api.example.com
```

## Why aperture-announce?

If you already run [Aperture](https://github.com/lightninglabs/aperture), this is the simplest way to make your services discoverable by AI agents. Unlike [402-announce](https://github.com/TheCryptoDonkey/402-announce) (TypeScript), aperture-announce:

- **Reads your existing `aperture.yaml` directly** — no separate config to maintain
- **Single binary, zero runtime dependencies** — no Node.js required
- **Built for Aperture operators** — understands Aperture's service format natively

## Usage

```
aperture-announce [flags]

Required:
  --config <path>         Path to Aperture YAML config
  --relays <urls>         Comma-separated Nostr relay URLs
  --public-urls <urls>    Comma-separated public URLs agents will use to reach your service (1–10)

Optional:
  --public-url <url>      Alias for --public-urls (single URL; backwards-compatible)
  --announce-key <hex>    Nostr signing key (auto-generated if omitted)
  --interval <duration>   Re-publish interval (e.g. 6h); default: one-shot
  --picture <url>         Service icon URL
  --topics <tags>         Comma-separated custom topic tags (appended to defaults)
  --dry-run               Print event JSON without publishing
  --verbose               Verbose logging
```

Environment variables: `APERTURE_CONFIG`, `ANNOUNCE_RELAYS`, `PUBLIC_URLS`, `PUBLIC_URL` (single-URL alias), `ANNOUNCE_KEY`, `ANNOUNCE_TOPICS`.

To announce multiple endpoints (clearnet, .onion, Handshake name) for the same service:

```bash
aperture-announce \
  --config /path/to/aperture.yaml \
  --relays wss://relay.damus.io,wss://nos.lol \
  --public-urls https://api.example.com,https://api.example.onion
```

Each URL produces a separate `url` tag in the kind 31402 event; agents try them in order.

### Multiple URLs vs multiple events

**Multiple URLs in one event** — pass several `--public-urls` entries when they represent the **same service** on different transports (clearnet, Tor, Handshake). Pricing, credentials, and the macaroon signing key are shared across all URLs. This provides censorship resistance and redundancy.

**Separate kind 31402 events** — run `aperture-announce` again for a genuinely different service (different `--config`, different Aperture backend) when the pricing, capabilities, or availability differ per transport. Each Aperture service should map to its own event with a distinct `d` tag.

In short: same service + different network paths → one event with multiple URL tags. Different services → separate runs, separate events.

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
  --public-urls https://api.example.com \
  --dry-run
```

This prints the event JSON to stdout without connecting to any relay.

<details>
<summary>Example output</summary>

Using `testdata/sample-conf.yaml`:

```json
{
  "kind": 31402,
  "tags": [
    ["d", "aperture-api.example.com"],
    ["name", "loop-rpc, pool-rpc"],
    ["about", "L402-gated API via Aperture — loop-rpc, pool-rpc"],
    ["pmi", "bitcoin-lightning-bolt11"],
    ["url", "https://api.example.com"],
    ["t", "l402"],
    ["t", "api"],
    ["t", "aperture"],
    ["price", "quote", "500", "sats"],
    ["price", "swap", "500", "sats"],
    ["price", "pool-rpc", "1000", "sats"]
  ],
  "content": "{\"capabilities\":[{\"name\":\"quote\",\"description\":\"quote via loop-rpc\"},{\"name\":\"swap\",\"description\":\"swap via loop-rpc\"},{\"name\":\"pool-rpc\",\"description\":\"Access pool-rpc\"}]}"
}
```

(Fields `id`, `pubkey`, `created_at`, and `sig` omitted — they vary per run.)

</details>

## Ecosystem

| Project | Role |
|---------|------|
| [aperture](https://github.com/lightninglabs/aperture) | L402 reverse proxy (what you're announcing) |
| [402-mcp](https://github.com/TheCryptoDonkey/402-mcp) | MCP client that discovers and pays for L402 APIs |
| [402-announce](https://github.com/TheCryptoDonkey/402-announce) | TypeScript equivalent of this tool |
| [toll-booth](https://github.com/TheCryptoDonkey/toll-booth) | Alternative L402 middleware |

## Support

For issues and feature requests, see [GitHub Issues](https://github.com/TheCryptoDonkey/aperture-announce/issues).

If you find aperture-announce useful, consider sending a tip:

- **Lightning:** `thedonkey@strike.me`
- **Nostr zaps:** `npub1mgvlrnf5hm9yf0n5mf9nqmvarhvxkc6remu5ec3vf8r0txqkuk7su0e7q2`

## Licence

[MIT](LICENSE)
