# aperture-phoenixd Design Spec

## Goal

A standalone Go module that lets Aperture use Phoenixd as its Lightning backend instead of LND. ~150 lines of code. Strategic play — credibility with Lightning Labs, not community scale.

## Problem

Aperture requires LND for invoice creation. LND is complex, resource-heavy, and requires ongoing management. Phoenixd is a single binary with automatic liquidity. The gap between "I want to charge for my API" and "I'm actually charging" is days with LND, hours with Phoenixd. This adapter closes that gap.

## Approach

Standalone Go module (`github.com/forgesworn/aperture-phoenixd`) that implements Aperture's challenger pattern against Phoenixd's REST API. No Aperture import — define matching interfaces with plain Go types to avoid pulling in LND's enormous dependency tree. Ship with an integration guide (a 20-line diff to `aperture.go`) and offer as a PR to `lightninglabs/aperture` later.

## Interfaces

Aperture's auth system depends on two interfaces:

### mint.Challenger

```go
type Challenger interface {
    NewChallenge(price int64) (string, [32]byte, error)
    Stop()
}
```

- `NewChallenge`: creates a Lightning invoice for the given price in sats. Returns the BOLT11 string and the 32-byte payment hash.
- `Stop`: cleanup. No-op for Phoenixd (stateless HTTP client).

### auth.InvoiceChecker

```go
type InvoiceChecker interface {
    VerifyInvoiceStatus(hash [32]byte, price int64, service string) error
}
```

- With `strictVerify=false` (Aperture's default): no-op. Macaroon + preimage verification is the security model.
- With `strictVerify=true`: not supported in v1. Document as a known limitation.

## Phoenixd API mapping

| Aperture need | Phoenixd endpoint | Notes |
|---------------|-------------------|-------|
| Create invoice | `POST /createinvoice` | Params: `amountSat`, `description`. Returns: `paymentHash`, `serialized` (BOLT11). |
| Check payment (optional) | `GET /payments/incoming/{hash}` | Returns `isPaid` boolean. Only needed if `strictVerify=true` (not supported in v1). |

Auth: HTTP Basic Auth with empty username and Phoenixd's `http-password` as password.

## Module structure

```
aperture-phoenixd/
├── challenger.go          # PhoenixdChallenger: NewChallenge, VerifyInvoiceStatus, Stop
├── challenger_test.go     # Mock Phoenixd server, test invoice flow
├── client.go              # Phoenixd HTTP client (createinvoice, getpayment)
├── client_test.go         # HTTP client unit tests
├── doc.go                 # Package documentation
├── cmd/echo-server/
│   └── main.go            # Minimal demo API: GET /v1/echo returns request body, 1 sat
├── go.mod
├── go.sum
├── README.md
├── CLAUDE.md
├── LICENSE                # MIT
└── testdata/
    └── aperture-patch.diff  # 20-line diff showing how to wire into Aperture
```

## Components

### client.go — Phoenixd HTTP client

Thin wrapper around `net/http`. Two methods:

- `CreateInvoice(ctx context.Context, amountSats int64, description string) (*Invoice, error)` — POST to `/createinvoice` with Basic Auth. The HTTP form field is `amountSat` (Phoenixd's name, singular) — not `amountSats`. Parses JSON response into `Invoice{PaymentHash, Serialized}`.
- `GetPayment(ctx context.Context, paymentHash string) (*Payment, error)` — GET `/payments/incoming/{hash}`. Parses JSON into `Payment{IsPaid, AmountSat}`. Reserved for future `strictVerify=true` support.

Constructor: `NewClient(baseURL, password string) *Client`. Configurable HTTP client for testing.

### challenger.go — PhoenixdChallenger

Wraps the client to satisfy Aperture's interfaces:

- `NewChallenge(price int64) (string, [32]byte, error)` — calls `client.CreateInvoice`, decodes hex payment hash into `[32]byte`, returns `(bolt11, hash, nil)`.
- `VerifyInvoiceStatus(hash [32]byte, price int64, service string) error` — returns `nil` (no-op for `strictVerify=false`).
- `Stop()` — no-op.

Constructor: `NewChallenger(phoenixdURL, password string) *PhoenixdChallenger`.

### cmd/echo-server/main.go — Demo API

Minimal HTTP server. One endpoint: `GET /v1/echo` returns the request body (or "hello" if no body). Listens on `localhost:8080`. No dependencies beyond stdlib. This is what sits behind Aperture in the demo.

### testdata/aperture-patch.diff — Integration guide

A diff against Aperture's `aperture.go` showing how to wire in the PhoenixdChallenger. Adds a config field `PhoenixdURL` and a third case in the auth switch:

```go
case cfg.PhoenixdURL != "":
    challenger = phoenixd.NewChallenger(cfg.PhoenixdURL, cfg.PhoenixdPassword)
```

This diff IS the future PR to Aperture.

## Error handling

- Phoenixd connection failure → `fmt.Errorf("phoenixd: connect: %w", err)`
- Non-200 response → `fmt.Errorf("phoenixd: createinvoice: HTTP %d", status)`
- Invalid JSON response → `fmt.Errorf("phoenixd: invalid response: %w", err)`
- Empty payment hash → `fmt.Errorf("phoenixd: empty payment hash in response")`
- Invalid hex payment hash → `fmt.Errorf("phoenixd: invalid payment hash: %w", err)`
- Wrong hash length (not 32 bytes) → `fmt.Errorf("phoenixd: payment hash wrong length: got %d bytes", n)`
- Never include the password in error messages.

## Testing

- Mock Phoenixd HTTP server using `httptest.NewServer` (same pattern as aperture-announce).
- Test cases: successful invoice creation, Phoenixd down, non-200 response, malformed JSON, empty payment hash, invalid hex payment hash, wrong hash length, Basic Auth header correctness.
- Use `testify/require` (Lightning Labs convention).
- Run with `-race`.

## Conventions

- British English throughout.
- `go vet`, `gofmt`, `golangci-lint` with the same linter set as aperture-announce.
- Commit messages: `type: description` format. No Co-Authored-By.
- Godoc on all exported symbols, ending in periods.

## Scope — what this is NOT

- Not a fork of Aperture.
- Not a general Lightning backend abstraction.
- Not a full LND replacement (no channels, routing, or key management).
- Not supporting `strictVerify=true` in v1.

## Success criteria

1. `PhoenixdChallenger.NewChallenge(100)` creates a 100-sat invoice via Phoenixd and returns valid BOLT11 + payment hash.
2. The echo server runs, Aperture proxies it, and an agent can pay and access the endpoint.
3. The aperture-patch.diff applies cleanly to the current Aperture main branch.
4. All tests pass with `-race`.
