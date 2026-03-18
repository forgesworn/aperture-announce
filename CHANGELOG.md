# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/), and this project adheres to [Semantic Versioning](https://semver.org/).

## [0.1.0] — 2026-03-16

### Added

- Parse Aperture YAML config and extract service definitions
- Build and publish kind 31402 Nostr events with pricing, capabilities, and payment method tags
- Auto-generate and persist Nostr signing keys to `~/.aperture-announce/announce.key`
- Dry-run mode for previewing events without publishing
- Interval-based re-publishing with `--interval`
- URL validation to reject private/internal hosts
- Concurrent relay publishing with per-relay error reporting
- Environment variable fallbacks for all required flags

[0.1.0]: https://github.com/forgesworn/aperture-announce/releases/tag/v0.1.0
