// Package key handles Nostr signing key generation, persistence, and resolution.
// Keys are 32-byte random values stored as 64-character hex strings. If no key
// is provided explicitly, one is auto-generated and saved to
// ~/.aperture-announce/announce.key with 0600 permissions.
package key
