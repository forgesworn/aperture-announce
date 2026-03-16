# Kind 31402 Event Format

This document describes the Nostr event structure that aperture-announce produces. The format matches [402-announce](https://github.com/TheCryptoDonkey/402-announce) so that both tools produce interchangeable events.

## Event structure

Kind 31402 is a **replaceable event** (NIP-33). The `d` tag serves as the unique identifier — publishing a new event with the same `d` tag replaces the previous one on relays.

### Tags

| Tag | Format | Description |
|-----|--------|-------------|
| `d` | `["d", "aperture-<hostname>"]` | Replaceable event identifier, derived from the public URL hostname |
| `name` | `["name", "<description>"]` | Human-readable service description with service names |
| `url` | `["url", "<public-url>"]` | The URL agents use to reach the service |
| `about` | `["about", "<description>"]` | Same as `name` — service summary |
| `pmi` | `["pmi", "bitcoin-lightning-bolt11"]` | Payment method indicator. Always Lightning for Aperture |
| `t` | `["t", "<topic>"]` | Topic tags: `l402`, `api`, `aperture` |
| `price` | `["price", "<cap>", "<amount>", "<currency>"]` | Positional price tag per capability |
| `picture` | `["picture", "<url>"]` | Optional service icon URL |

### Price tags

Price tags use **positional elements**, not JSON:

```
["price", "quote", "500", "sats"]
["price", "swap", "500", "sats"]
```

Each capability from the Aperture config gets its own price tag. If a service has no explicit capabilities, the service name is used as the capability name.

### Content field

The `content` field contains a JSON-encoded object listing capabilities with their endpoints:

```json
{
  "capabilities": [
    {
      "name": "quote",
      "description": "quote via loop-rpc",
      "endpoint": "/v1/loop/.*"
    },
    {
      "name": "swap",
      "description": "swap via loop-rpc",
      "endpoint": "/v1/loop/.*"
    }
  ]
}
```

## Mapping from Aperture config

| Aperture YAML field | Event field |
|---------------------|-------------|
| `services[].name` | Used in `name`/`about` tags and as fallback capability name |
| `services[].pathregexp` | `endpoint` in content capabilities |
| `services[].price` | Amount in `price` tags |
| `services[].capabilities` | Comma-separated list → individual `price` tags and content capabilities |

## Schema

A JSON Schema for the event is available at [`schemas/kind-31402.schema.json`](../schemas/kind-31402.schema.json).

## Verifying output

Use `--dry-run` to inspect the event without publishing:

```bash
aperture-announce \
  --config aperture.yaml \
  --public-url https://api.example.com \
  --dry-run
```
