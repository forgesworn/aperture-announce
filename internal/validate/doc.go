// Package validate checks that public URLs are safe to announce. It rejects
// private, loopback, link-local, and reserved IP ranges to prevent accidentally
// advertising internal services on Nostr relays.
package validate
