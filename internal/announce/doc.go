// Package announce builds and publishes kind 31402 Nostr events that advertise
// L402-gated services. It reads parsed Aperture service definitions and produces
// signed events with pricing tags, payment method indicators, and capability
// metadata in the format expected by 402-mcp's l402_search.
package announce
