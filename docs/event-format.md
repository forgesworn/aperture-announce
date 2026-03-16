# Kind 31402 Event Format

This document describes the Nostr event structure that aperture-announce produces. The format matches [402-announce](https://github.com/TheCryptoDonkey/402-announce) so that both tools produce interchangeable events.

## Event structure

Kind 31402 is a **replaceable event** (NIP-33). The `d` tag serves as the unique identifier — publishing a new event with the same `d` tag replaces the previous one on relays.

### Tags

| Tag | Format | Description |
|-----|--------|-------------|
| `d` | `["d", "aperture-<hostname>"]` | Replaceable event identifier, derived from the public URL hostname |
| `name` | `["name", "<service-name>"]` | Short service name (e.g. `loop-rpc`) |
| `url` | `["url", "<public-url>"]` | The URL agents use to reach the service |
| `about` | `["about", "<description>"]` | Longer human-readable description of the service |
| `pmi` | `["pmi", "bitcoin-lightning-bolt11"]` | Payment method indicator. Always Lightning for Aperture |
| `t` | `["t", "<topic>"]` | Topic tags: `l402`, `api`, `aperture`, plus any custom topics |
| `price` | `["price", "<cap>", "<amount>", "<currency>"]` | Positional price tag per capability. Omitted for dynamically priced capabilities with no static fallback |
| `picture` | `["picture", "<url>"]` | Optional service icon URL |

The `endpoint` field in capabilities is now a **cleaned base path** (e.g. `/v1/loop`) derived from the Aperture `pathregexp`, not the raw regexp string.

### Price tags

Price tags use **positional elements**, not JSON:

```
["price", "quote", "500", "sats"]
["price", "swap", "500", "sats"]
```

Each capability from the Aperture config gets its own price tag. If a service has no explicit capabilities, the service name is used as the capability name.

### Content field

The `content` field contains a JSON-encoded object listing capabilities with their endpoints. Each capability may include optional fields:

```json
{
  "capabilities": [
    {
      "name": "quote",
      "description": "quote via loop-rpc",
      "endpoint": "/v1/loop",
      "auth": "none",
      "timeout": 3600
    },
    {
      "name": "swap",
      "description": "swap via loop-rpc",
      "endpoint": "/v1/loop",
      "pricing": "dynamic"
    },
    {
      "name": "pool-rpc",
      "description": "Access pool-rpc",
      "endpoint": "/v1/pool",
      "auth": "freebie 5"
    }
  ]
}
```

### Auth field

The optional `auth` field on a capability specifies its authentication requirement:

- `"none"` — no authentication required; the endpoint is freely accessible
- `"freebie N"` — the first N requests are free before payment is required (e.g. `"freebie 10"`)
- omitted — payment is required (the default L402 behaviour)

### Timeout field

The optional `timeout` field specifies the L402 token validity period in **seconds**. When omitted, tokens do not carry an expiry. Example: `"timeout": 3600` means tokens are valid for one hour.

### Dynamic pricing

When a capability's price is determined at request time rather than statically:

- The capability carries `"pricing": "dynamic"` in the content JSON
- The corresponding `price` tag is omitted from the event (unless a static fallback price is also configured)
- A `["t", "dynamic-pricing"]` topic tag is added to the event to make dynamic-pricing services discoverable

## Mapping from Aperture config

| Aperture YAML field | Event field |
|---------------------|-------------|
| `services[].name` | `name` tag (short name) and `about` tag (longer description); fallback capability name |
| `services[].pathregexp` | Cleaned to a base path → `endpoint` in content capabilities |
| `services[].price` | Amount in `price` tags (omitted when `pricing: dynamic` and no static fallback) |
| `services[].capabilities` | Comma-separated list → individual `price` tags and content capabilities |
| `services[].auth` | `auth` field in content capabilities for that service |
| `services[].timeout` | `timeout` field in content capabilities for that service |
| `services[].dynamicprice.enabled` | `pricing: "dynamic"` in content capabilities; adds `dynamic-pricing` topic tag |

## Schema

A JSON Schema for the event is available at [`schemas/kind-31402.schema.json`](../schemas/kind-31402.schema.json).

## Verifying output

Use `--dry-run` to inspect the event without publishing:

```bash
aperture-announce \
  --config aperture.yaml \
  --public-urls https://api.example.com \
  --dry-run
```
