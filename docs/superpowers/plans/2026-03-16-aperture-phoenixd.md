# aperture-phoenixd Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A standalone Go module that lets Aperture use Phoenixd as its Lightning backend — ~150 lines of core code.

**Architecture:** Two-layer design: a thin Phoenixd HTTP client (`client.go`) and a challenger wrapper (`challenger.go`) that adapts the client to Aperture's `mint.Challenger` and `auth.InvoiceChecker` interfaces. No Aperture import — plain Go types only. Ships with an echo demo server and an integration patch.

**Tech Stack:** Go 1.24, net/http, testify/require, httptest

---

## Chunk 1: Scaffold and Phoenixd HTTP client

### Task 1: Scaffold the Go module

**Files:**
- Create: `go.mod`
- Create: `doc.go`
- Create: `CLAUDE.md`
- Create: `LICENSE`
- Create: `.golangci.yml`

- [ ] **Step 1: Create the project directory**

```bash
mkdir -p /Users/darren/WebstormProjects/aperture-phoenixd
cd /Users/darren/WebstormProjects/aperture-phoenixd
```

- [ ] **Step 2: Initialise Go module**

```bash
go mod init github.com/TheCryptoDonkey/aperture-phoenixd
```

- [ ] **Step 3: Create doc.go**

```go
// Package phoenixd provides a Phoenixd-backed challenger for Aperture's
// L402 authentication system. It implements Aperture's mint.Challenger
// and auth.InvoiceChecker interfaces against Phoenixd's REST API,
// removing the requirement for a full LND node.
//
// This package has no dependency on Aperture or LND. It uses plain Go
// types ([32]byte for payment hashes) that are assignment-compatible
// with Aperture's lntypes.Hash.
package phoenixd
```

- [ ] **Step 4: Create CLAUDE.md**

```markdown
# CLAUDE.md — aperture-phoenixd

Standalone Go module: Phoenixd challenger for Aperture L402 auth.

## Commands

go build ./...         # Build
go test ./...          # Run all tests
go test -race ./...    # Run with race detector
go vet ./...           # Lint

## Structure

client.go              # Phoenixd HTTP client (createinvoice, getpayment)
challenger.go          # PhoenixdChallenger (NewChallenge, VerifyInvoiceStatus)
cmd/echo-server/       # Minimal demo API for Aperture to proxy

## Conventions

- British English
- Go standard layout — cmd/ for binaries
- Git: commit messages use type: description format
- Git: Do NOT include Co-Authored-By lines
- testify/require for all test assertions
- golangci-lint with Aperture-compatible linter set
```

- [ ] **Step 5: Create LICENSE (MIT)**

Use standard MIT licence text with `Copyright (c) 2026 TheCryptoDonkey`.

- [ ] **Step 6: Create .golangci.yml**

Copy from aperture-announce — same linter set:

```yaml
linters:
  enable-all: false
  enable:
    - bodyclose
    - copyloopvar
    - dupl
    - goconst
    - gocritic
    - gocyclo
    - gofmt
    - misspell
    - nakedret
    - prealloc
    - staticcheck
    - unconvert
    - unparam

issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - gocyclo
        - unparam

linters-settings:
  gofmt:
    simplify: true
  goconst:
    ignore-tests: true
  gocyclo:
    min-complexity: 30
```

- [ ] **Step 7: Initial commit**

```bash
git init
git add go.mod doc.go CLAUDE.md LICENSE .golangci.yml
git commit -m "chore: scaffold Go module"
```

---

### Task 2: Phoenixd HTTP client

**Files:**
- Create: `client.go`
- Create: `client_test.go`

- [ ] **Step 1: Write the failing test for CreateInvoice**

```go
package phoenixd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateInvoice_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/createinvoice", r.URL.Path)

		// Verify Basic Auth (empty username, password as password).
		user, pass, ok := r.BasicAuth()
		require.True(t, ok, "Basic Auth required")
		require.Equal(t, "", user)
		require.Equal(t, "test-password", pass)

		// Verify form fields.
		require.NoError(t, r.ParseForm())
		require.Equal(t, "100", r.FormValue("amountSat"))
		require.Equal(t, "L402", r.FormValue("description"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"amountSat":   100,
			"paymentHash": "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
			"serialized":  "lnbc1000n1test...",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-password")
	inv, err := c.CreateInvoice(context.Background(), 100, "L402")
	require.NoError(t, err)
	require.Equal(t, "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789", inv.PaymentHash)
	require.Equal(t, "lnbc1000n1test...", inv.Serialized)
}
```

- [ ] **Step 2: Add testify dependency**

```bash
go get github.com/stretchr/testify/require
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./... -run TestCreateInvoice_Success -v`
Expected: FAIL — `NewClient` not defined

- [ ] **Step 4: Write client.go**

```go
package phoenixd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Client communicates with a Phoenixd instance via its REST API.
type Client struct {
	baseURL  string
	password string
	http     *http.Client
}

// Invoice holds the response from Phoenixd's createinvoice endpoint.
type Invoice struct {
	PaymentHash string `json:"paymentHash"`
	Serialized  string `json:"serialized"`
}

// Payment holds the response from Phoenixd's incoming payment endpoint.
type Payment struct {
	IsPaid    bool  `json:"isPaid"`
	AmountSat int64 `json:"amountSat"`
}

// NewClient creates a Phoenixd client. The password is used for HTTP
// Basic Auth with an empty username.
func NewClient(baseURL, password string) *Client {
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		password: password,
		http:     &http.Client{},
	}
}

// CreateInvoice creates a Lightning invoice via Phoenixd.
func (c *Client) CreateInvoice(ctx context.Context, amountSats int64, description string) (*Invoice, error) {
	form := url.Values{
		"amountSat":   {strconv.FormatInt(amountSats, 10)},
		"description": {description},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/createinvoice", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("phoenixd: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("", c.password)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("phoenixd: connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("phoenixd: createinvoice: HTTP %d", resp.StatusCode)
	}

	var inv Invoice
	if err := json.NewDecoder(resp.Body).Decode(&inv); err != nil {
		return nil, fmt.Errorf("phoenixd: invalid response: %w", err)
	}

	return &inv, nil
}

// GetPayment retrieves an incoming payment by hash. Reserved for
// future strictVerify=true support.
func (c *Client) GetPayment(ctx context.Context, paymentHash string) (*Payment, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/payments/incoming/"+paymentHash, nil)
	if err != nil {
		return nil, fmt.Errorf("phoenixd: build request: %w", err)
	}
	req.SetBasicAuth("", c.password)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("phoenixd: connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("phoenixd: getpayment: HTTP %d", resp.StatusCode)
	}

	var pmt Payment
	if err := json.NewDecoder(resp.Body).Decode(&pmt); err != nil {
		return nil, fmt.Errorf("phoenixd: invalid response: %w", err)
	}

	return &pmt, nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./... -run TestCreateInvoice_Success -v`
Expected: PASS

- [ ] **Step 6: Add error path tests**

Add to `client_test.go`:

```go
func TestCreateInvoice_PhoenixdDown(t *testing.T) {
	c := NewClient("http://127.0.0.1:1", "pass")
	_, err := c.CreateInvoice(context.Background(), 100, "L402")
	require.Error(t, err)
	require.Contains(t, err.Error(), "phoenixd: connect:")
}

func TestCreateInvoice_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "pass")
	_, err := c.CreateInvoice(context.Background(), 100, "L402")
	require.Error(t, err)
	require.Contains(t, err.Error(), "HTTP 500")
}

func TestCreateInvoice_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "pass")
	_, err := c.CreateInvoice(context.Background(), 100, "L402")
	require.Error(t, err)
	require.Contains(t, err.Error(), "phoenixd: invalid response:")
}
```

- [ ] **Step 7: Run all tests**

Run: `go test ./... -v -race`
Expected: all PASS

- [ ] **Step 8: Commit**

```bash
git add client.go client_test.go go.mod go.sum
git commit -m "feat: add Phoenixd HTTP client for invoice creation"
```

---

## Chunk 2: PhoenixdChallenger

### Task 3: Challenger implementation

**Files:**
- Create: `challenger.go`
- Create: `challenger_test.go`

- [ ] **Step 1: Write the failing test for NewChallenge**

```go
package phoenixd

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

const testHash = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"

func mockPhoenixd(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"amountSat":   100,
			"paymentHash": testHash,
			"serialized":  "lnbc1000n1test...",
		})
	}))
}

func TestNewChallenge_Success(t *testing.T) {
	srv := mockPhoenixd(t)
	defer srv.Close()

	ch := NewChallenger(srv.URL, "test-password")
	bolt11, hash, err := ch.NewChallenge(100)
	require.NoError(t, err)
	require.Equal(t, "lnbc1000n1test...", bolt11)

	expectedHash, _ := hex.DecodeString(testHash)
	require.Equal(t, [32]byte(expectedHash), hash)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -run TestNewChallenge_Success -v`
Expected: FAIL — `NewChallenger` not defined

- [ ] **Step 3: Write challenger.go**

```go
package phoenixd

import (
	"context"
	"encoding/hex"
	"fmt"
)

// PhoenixdChallenger implements Aperture's mint.Challenger and
// auth.InvoiceChecker interfaces using a Phoenixd Lightning node.
// Only strictVerify=false is supported (the Aperture default).
type PhoenixdChallenger struct {
	client *Client
}

// NewChallenger creates a challenger backed by a Phoenixd instance.
func NewChallenger(phoenixdURL, password string) *PhoenixdChallenger {
	return &PhoenixdChallenger{
		client: NewClient(phoenixdURL, password),
	}
}

// NewChallenge creates a Lightning invoice via Phoenixd and returns the
// BOLT11 payment request and 32-byte payment hash.
func (p *PhoenixdChallenger) NewChallenge(price int64) (string, [32]byte, error) {
	var hash [32]byte

	inv, err := p.client.CreateInvoice(context.Background(), price, "L402")
	if err != nil {
		return "", hash, err
	}

	if inv.PaymentHash == "" {
		return "", hash, fmt.Errorf("phoenixd: empty payment hash in response")
	}

	hashBytes, err := hex.DecodeString(inv.PaymentHash)
	if err != nil {
		return "", hash, fmt.Errorf("phoenixd: invalid payment hash: %w", err)
	}
	if len(hashBytes) != 32 {
		return "", hash, fmt.Errorf("phoenixd: payment hash wrong length: got %d bytes", len(hashBytes))
	}

	copy(hash[:], hashBytes)
	return inv.Serialized, hash, nil
}

// VerifyInvoiceStatus is a no-op. With strictVerify=false (Aperture's
// default), macaroon + preimage verification is the security model.
// strictVerify=true is not supported in this version.
func (p *PhoenixdChallenger) VerifyInvoiceStatus(hash [32]byte, price int64, service string) error {
	return nil
}

// Stop is a no-op. The Phoenixd client is stateless.
func (p *PhoenixdChallenger) Stop() {}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -run TestNewChallenge_Success -v`
Expected: PASS

- [ ] **Step 5: Add error and edge case tests**

Add to `challenger_test.go`:

```go
func TestNewChallenge_EmptyHash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"paymentHash": "",
			"serialized":  "lnbc...",
		})
	}))
	defer srv.Close()

	ch := NewChallenger(srv.URL, "pass")
	_, _, err := ch.NewChallenge(100)
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty payment hash")
}

func TestNewChallenge_InvalidHex(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"paymentHash": "not-hex",
			"serialized":  "lnbc...",
		})
	}))
	defer srv.Close()

	ch := NewChallenger(srv.URL, "pass")
	_, _, err := ch.NewChallenge(100)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid payment hash")
}

func TestNewChallenge_WrongHashLength(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"paymentHash": "abcdef",
			"serialized":  "lnbc...",
		})
	}))
	defer srv.Close()

	ch := NewChallenger(srv.URL, "pass")
	_, _, err := ch.NewChallenge(100)
	require.Error(t, err)
	require.Contains(t, err.Error(), "wrong length")
}

func TestVerifyInvoiceStatus_NoOp(t *testing.T) {
	ch := NewChallenger("http://unused", "pass")
	err := ch.VerifyInvoiceStatus([32]byte{}, 100, "test-service")
	require.NoError(t, err)
}

func TestStop_NoOp(t *testing.T) {
	ch := NewChallenger("http://unused", "pass")
	ch.Stop() // Should not panic.
}
```

- [ ] **Step 6: Run all tests with race detector**

Run: `go test ./... -v -race`
Expected: all PASS

- [ ] **Step 7: Commit**

```bash
git add challenger.go challenger_test.go
git commit -m "feat: add PhoenixdChallenger for Aperture L402 auth"
```

---

## Chunk 3: Echo server, docs, and integration patch

### Task 4: Echo demo server

**Files:**
- Create: `cmd/echo-server/main.go`

- [ ] **Step 1: Create the echo server**

```go
package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

func main() {
	addr := ":8080"
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}

	http.HandleFunc("/v1/echo", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil || len(body) == 0 {
			fmt.Fprintln(w, "hello")
			return
		}
		w.Write(body)
	})

	fmt.Fprintf(os.Stderr, "echo-server listening on %s\n", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Fprintf(os.Stderr, "echo-server: %v\n", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Verify it builds**

Run: `go build ./cmd/echo-server`
Expected: builds without error

- [ ] **Step 3: Commit**

```bash
git add cmd/echo-server/main.go
git commit -m "feat: add minimal echo server for Aperture demo"
```

### Task 5: README and integration patch

**Files:**
- Create: `README.md`
- Create: `testdata/aperture-patch.diff`

- [ ] **Step 1: Create README.md**

```markdown
# aperture-phoenixd

[![MIT licence](https://img.shields.io/badge/licence-MIT-blue.svg)](./LICENSE)
[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8)](https://golang.org/)

Use [Phoenixd](https://phoenix.acinq.co/server) as the Lightning backend for [Aperture](https://github.com/lightninglabs/aperture) — no LND required.

Implements Aperture's `mint.Challenger` and `auth.InvoiceChecker` interfaces against Phoenixd's REST API. Drop-in replacement for LND with `strictVerify=false` (Aperture's default).

## Quick start

```go
import "github.com/TheCryptoDonkey/aperture-phoenixd"

challenger := phoenixd.NewChallenger("http://localhost:9740", "your-phoenixd-password")

// Use challenger.NewChallenge(priceSats) to create invoices
// Use challenger.VerifyInvoiceStatus(...) — no-op for strictVerify=false
// Use challenger.Stop() — no-op (stateless HTTP client)
```

## Integrating with Aperture

See `testdata/aperture-patch.diff` for the ~20-line diff to wire this into Aperture's `aperture.go`. Adds a `PhoenixdURL` config field alongside the existing `LndHost` and `Passphrase` options.

## Demo

The included echo server provides a minimal API to proxy through Aperture:

```bash
go run ./cmd/echo-server
# Listens on :8080, returns request body at /v1/echo
```

## Limitations

- `strictVerify=true` is not supported. Phoenixd's WebSocket does not emit invoice cancellation events required for full invoice status tracking. With `strictVerify=false` (the default), macaroon + preimage verification is the security model.

## Licence

[MIT](LICENSE)
```

- [ ] **Step 2: Create the Aperture integration patch**

Fetch the current Aperture `aperture.go` and `config.go` to understand the exact wiring points, then create a minimal diff showing how to add Phoenixd support. The diff should:
- Add `PhoenixdURL` and `PhoenixdPassword` fields to the config struct
- Add a third case in the auth switch after the LND and LNC cases
- Import the phoenixd package

Save to `testdata/aperture-patch.diff`.

- [ ] **Step 3: Commit**

```bash
git add README.md testdata/aperture-patch.diff
git commit -m "docs: add README and Aperture integration patch"
```

### Task 6: CI and final verification

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Create CI workflow**

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

permissions:
  contents: read

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'

      - name: Install golangci-lint
        run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.5

      - run: go build ./...
      - run: go test -race ./...
      - run: go vet ./...
      - run: golangci-lint run ./...
```

- [ ] **Step 2: Run full verification locally**

```bash
go test -race ./...
go vet ./...
gofmt -l .
golangci-lint run ./...
```

Expected: all clean

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add test, race detector, and golangci-lint pipeline"
```

---

## Summary

| Task | Files | Commit |
|------|-------|--------|
| 1. Scaffold | go.mod, doc.go, CLAUDE.md, LICENSE, .golangci.yml | `chore: scaffold Go module` |
| 2. HTTP client | client.go, client_test.go | `feat: add Phoenixd HTTP client for invoice creation` |
| 3. Challenger | challenger.go, challenger_test.go | `feat: add PhoenixdChallenger for Aperture L402 auth` |
| 4. Echo server | cmd/echo-server/main.go | `feat: add minimal echo server for Aperture demo` |
| 5. Docs + patch | README.md, testdata/aperture-patch.diff | `docs: add README and Aperture integration patch` |
| 6. CI | .github/workflows/ci.yml | `ci: add test, race detector, and golangci-lint pipeline` |

Total new code: ~150 lines (client.go + challenger.go), ~20 lines (echo server), plus tests and docs.
