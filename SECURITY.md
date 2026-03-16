# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability, please report it privately rather than opening a public issue.

Email: **security@thecryptodonkey.com**

You should receive a response within 72 hours. Please include:

- Description of the vulnerability
- Steps to reproduce
- Potential impact

## Scope

This project handles Nostr signing keys and publishes events to relays. Security-relevant areas include:

- Key generation and storage (`internal/key/`)
- URL validation to prevent SSRF (`internal/validate/`)
- Relay connections and event signing (`internal/announce/`)
