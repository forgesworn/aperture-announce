# Deployment

aperture-announce can run as a one-shot command or as a long-running service that re-publishes on an interval.

## One-shot (cron)

Publish once per hour via cron:

```bash
# Install the binary
go install github.com/forgesworn/aperture-announce/cmd/aperture-announce@latest

# Add to crontab (crontab -e)
0 * * * * /home/you/go/bin/aperture-announce \
  --config /etc/aperture/aperture.yaml \
  --relays wss://relay.damus.io,wss://nos.lol \
  --public-urls https://api.example.com
```

Kind 31402 events are replaceable — publishing the same service repeatedly just updates the existing event on relays. There is no harm in frequent re-publishing.

## Long-running (systemd)

Use the built-in `--interval` flag to re-publish periodically:

```ini
# /etc/systemd/system/aperture-announce.service
[Unit]
Description=Announce Aperture L402 services on Nostr
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/aperture-announce \
  --config /etc/aperture/aperture.yaml \
  --relays wss://relay.damus.io,wss://nos.lol \
  --public-urls https://api.example.com \
  --interval 6h
Restart=on-failure
RestartSec=30

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=read-only
ReadWritePaths=/home/you/.aperture-announce

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now aperture-announce
sudo journalctl -u aperture-announce -f   # Watch logs
```

## Environment variables

All required flags have environment variable fallbacks, useful for containers or secret managers:

```bash
export APERTURE_CONFIG=/etc/aperture/aperture.yaml
export ANNOUNCE_RELAYS=wss://relay.damus.io,wss://nos.lol
export PUBLIC_URLS=https://api.example.com
export ANNOUNCE_KEY=<64-char-hex>   # Optional — auto-generated if omitted

aperture-announce
```

## Docker

```dockerfile
FROM golang:1.24-alpine AS build
RUN go install github.com/forgesworn/aperture-announce/cmd/aperture-announce@latest

FROM alpine:3.20
COPY --from=build /go/bin/aperture-announce /usr/local/bin/
ENTRYPOINT ["aperture-announce"]
```

```bash
docker run --rm \
  -v /etc/aperture/aperture.yaml:/config/aperture.yaml:ro \
  -v aperture-announce-keys:/root/.aperture-announce \
  aperture-announce \
  --config /config/aperture.yaml \
  --relays wss://relay.damus.io,wss://nos.lol \
  --public-urls https://api.example.com \
  --interval 6h
```

Mount a volume for `/root/.aperture-announce` to persist the auto-generated signing key across container restarts.

## Key management

By default, aperture-announce generates a Nostr key and saves it to `~/.aperture-announce/announce.key`. To use an existing key, pass `--announce-key <hex>` or set `ANNOUNCE_KEY`.

The key determines the pubkey that owns your service announcements on Nostr. If you lose the key, you'll need to publish with a new one — old events will remain on relays until they expire or are replaced by a new `d`-tag match from the new key.

## Verifying it works

After publishing, verify your service appears on relays:

```bash
# Dry-run first to check the event looks right
aperture-announce --config aperture.yaml --public-urls https://api.example.com --dry-run

# After publishing, search via 402-mcp (if available)
# Or query a relay directly for kind 31402 events
```
