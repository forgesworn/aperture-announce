# Event Format Improvements Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Improve kind 31402 event output to match 402-announce conventions and handle Aperture config fields (auth, timeout, dynamic pricing) that were previously ignored.

**Architecture:** Config layer parses new YAML fields (auth, timeout). Announce layer gains CleanEndpoint(), BuildOptions struct, and enriched capability content. CLI layer adds --topics flag. Changes are layered: config first, then announce, then CLI.

**Tech Stack:** Go 1.24, goccy/go-yaml, nbd-wtf/go-nostr

**Spec:** `docs/superpowers/specs/2026-03-16-event-format-improvements-design.md`

---

## Chunk 1: Config Layer

### Task 1: Add Auth and Timeout to config parser

**Files:**
- Modify: `internal/config/config.go:22-29` (Service struct)
- Modify: `internal/config/config.go:31-37` (rawService struct)
- Modify: `internal/config/config.go:60-93` (Parse loop)
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write failing tests for auth parsing**

Add to `internal/config/config_test.go`:

```go
func TestParseAuth(t *testing.T) {
	yml := `
services:
  - name: "api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/.*"
    price: 100
    auth: "freebie 5"
`
	cfg, err := Parse([]byte(yml))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Services[0].Auth != "freebie 5" {
		t.Errorf("auth = %q, want %q", cfg.Services[0].Auth, "freebie 5")
	}
}

func TestParseAuthOff(t *testing.T) {
	yml := `
services:
  - name: "api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/.*"
    price: 100
    auth: "off"
`
	cfg, err := Parse([]byte(yml))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Services[0].Auth != "off" {
		t.Errorf("auth = %q, want %q", cfg.Services[0].Auth, "off")
	}
}

func TestParseAuthDefault(t *testing.T) {
	yml := `
services:
  - name: "api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/.*"
    price: 100
`
	cfg, err := Parse([]byte(yml))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Services[0].Auth != "" {
		t.Errorf("auth = %q, want empty (default)", cfg.Services[0].Auth)
	}
}

func TestParseTimeout(t *testing.T) {
	yml := `
services:
  - name: "api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/.*"
    price: 100
    timeout: 3600
`
	cfg, err := Parse([]byte(yml))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Services[0].Timeout != 3600 {
		t.Errorf("timeout = %d, want 3600", cfg.Services[0].Timeout)
	}
}

func TestParseAuthUnrecognisedWarns(t *testing.T) {
	yml := `
services:
  - name: "api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/.*"
    price: 100
    auth: "maybe"
`
	cfg, err := Parse([]byte(yml))
	if err != nil {
		t.Fatal(err)
	}
	// Unrecognised auth values are accepted (treated as "on" default)
	// but the warning goes to stderr (tested via CLI integration tests).
	if cfg.Services[0].Auth != "maybe" {
		t.Errorf("auth = %q, want %q (stored as-is, warning emitted by caller)", cfg.Services[0].Auth, "maybe")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -run "TestParseAuth|TestParseTimeout" -v`
Expected: FAIL — `Service` struct has no `Auth` or `Timeout` field.

- [ ] **Step 3: Implement Auth and Timeout in config**

Update `internal/config/config.go`:

Add `Auth` and `Timeout` to the `Service` struct:
```go
type Service struct {
	Name         string
	HostRegexp   string
	PathRegexp   string
	Price        int64
	DynamicPrice bool
	Capabilities []string
	Auth         string
	Timeout      int64
}
```

Add `Auth` and `Timeout` to the `rawService` struct:
```go
type rawService struct {
	Name         string      `yaml:"name"`
	HostRegexp   string      `yaml:"hostregexp"`
	PathRegexp   string      `yaml:"pathregexp"`
	Price        int64       `yaml:"price"`
	Capabilities string      `yaml:"capabilities"`
	DynamicPrice rawDynPrice `yaml:"dynamicprice"`
	Auth         string      `yaml:"auth"`
	Timeout      int64       `yaml:"timeout"`
}
```

In the `Parse` loop, after capabilities parsing, add:
```go
		s.Auth = rs.Auth
		s.Timeout = rs.Timeout
```

Auth validation happens in the caller (main.go or announce layer), not in Parse — the config parser stores the raw value. This matches the spec: "warn (not fatal) on unrecognised auth values."

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -v`
Expected: ALL PASS

- [ ] **Step 5: Run full test suite**

Run: `go test ./... && go vet ./...`
Expected: ALL PASS (no signature changes yet)

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: parse auth and timeout fields from Aperture config"
```

---

## Chunk 2: Announce Layer — Foundation

### Task 2: Add CleanEndpoint function

**Files:**
- Modify: `internal/announce/announce.go` (add function)
- Test: `internal/announce/announce_test.go`

- [ ] **Step 1: Write failing tests for CleanEndpoint**

Add to `internal/announce/announce_test.go`:

```go
func TestCleanEndpoint(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"^/v1/loop/.*$", "/v1/loop/"},
		{"/v1/pool/.*", "/v1/pool/"},
		{"^/looprpc.SwapServer/LoopOutTerms.*$", "/looprpc.SwapServer/LoopOutTerms"},
		{"^/v1/(quote|swap)/.*$", "/v1/"},
		{"^/.*$", ""},
		{".*", ""},
		{"", ""},
		{"/v1/status", "/v1/status"},
		{"^/v1/status$", "/v1/status"},
		{"/v1/items?", "/v1/item"},
		{"/v1/data{2,5}", "/v1/data"},
		{"/v1/users[0-9]+", "/v1/users"},
		{"/v1/exact", "/v1/exact"},
	}
	for _, tc := range tests {
		got := CleanEndpoint(tc.input)
		if got != tc.want {
			t.Errorf("CleanEndpoint(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/announce/ -run TestCleanEndpoint -v`
Expected: FAIL — `CleanEndpoint` undefined.

- [ ] **Step 3: Implement CleanEndpoint**

Add to `internal/announce/announce.go`:

```go
// CleanEndpoint strips regex syntax from an Aperture path pattern to produce
// a usable base path for agents. Returns empty string if no usable path remains.
func CleanEndpoint(regex string) string {
	if regex == "" {
		return ""
	}

	s := regex

	// Step 1: strip anchors
	s = strings.TrimPrefix(s, "^")
	s = strings.TrimSuffix(s, "$")

	// Step 2: truncate at first regex metacharacter
	cutSet := []string{".*", ".+", "[", "(", "?", "{"}
	minIdx := len(s)
	for _, pat := range cutSet {
		if idx := strings.Index(s, pat); idx != -1 && idx < minIdx {
			minIdx = idx
		}
	}
	s = s[:minIdx]

	// Step 3: clean up but preserve meaningful trailing slash
	if s == "/" || s == "" {
		return ""
	}

	return s
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/announce/ -run TestCleanEndpoint -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/announce/announce.go internal/announce/announce_test.go
git commit -m "feat: add CleanEndpoint to strip regex from path patterns"
```

### Task 3: Refactor BuildEvent to use BuildOptions

**Files:**
- Modify: `internal/announce/announce.go:28-29` (signature)
- Modify: `internal/announce/announce_test.go` (all callers)
- Modify: `cmd/aperture-announce/main.go:113` (caller)

- [ ] **Step 1: Add BuildOptions struct and update signature**

In `internal/announce/announce.go`, add before `BuildEvent`:

```go
// BuildOptions holds optional parameters for event construction.
type BuildOptions struct {
	PublicURL string
	Picture   string
	Topics    []string
}
```

Change `BuildEvent` signature from:
```go
func BuildEvent(secretKey string, cfg *config.ApertureConfig, publicURL string, picture string) (*nostr.Event, error) {
```
to:
```go
func BuildEvent(secretKey string, cfg *config.ApertureConfig, opts BuildOptions) (*nostr.Event, error) {
```

Inside `BuildEvent`, replace all references:
- `publicURL` → `opts.PublicURL`
- `picture` → `opts.Picture`

- [ ] **Step 2: Update all test callers**

In `internal/announce/announce_test.go`, update all 6 existing `BuildEvent` calls:

1. `TestBuildEvent_SingleService` (line 18)
2. `TestBuildEvent_WithCapabilities` (line 45)
3. `TestBuildEvent_DynamicPricing` (line 74)
4. `TestBuildEvent_MultipleServices` (line 90)
5. `TestBuildEvent_WithPicture` (line 106)
6. `TestBuildEvent_SignatureValid` (line 121)

Two patterns:

```go
// Without picture (tests 1, 2, 3, 4, 6):
// Before:
ev, err := BuildEvent(sk, cfg, "https://api.example.com", "")
// After:
ev, err := BuildEvent(sk, cfg, BuildOptions{PublicURL: "https://api.example.com"})

// With picture (test 5):
// Before:
ev, err := BuildEvent(sk, cfg, "https://api.example.com", "https://example.com/icon.png")
// After:
ev, err := BuildEvent(sk, cfg, BuildOptions{PublicURL: "https://api.example.com", Picture: "https://example.com/icon.png"})
```

- [ ] **Step 3: Update main.go caller**

In `cmd/aperture-announce/main.go`, change:
```go
ev, err := announce.BuildEvent(sk, cfg, *publicURL, *picture)
```
to:
```go
ev, err := announce.BuildEvent(sk, cfg, announce.BuildOptions{
    PublicURL: *publicURL,
    Picture:   *picture,
})
```

- [ ] **Step 4: Run full test suite**

Run: `go test ./... && go vet ./...`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/announce/announce.go internal/announce/announce_test.go cmd/aperture-announce/main.go
git commit -m "refactor: replace BuildEvent positional params with BuildOptions struct"
```

---

## Chunk 3: Announce Layer — Features

### Task 4: Separate name and about tags

**Files:**
- Modify: `internal/announce/announce.go` (BuildEvent tag building)
- Test: `internal/announce/announce_test.go`

- [ ] **Step 1: Write failing test**

Add to `internal/announce/announce_test.go`:

```go
func TestBuildEvent_NameAboutSplit(t *testing.T) {
	cfg := &config.ApertureConfig{
		Services: []config.Service{
			{Name: "loop-rpc", PathRegexp: "/v1/loop/.*", Price: 500},
			{Name: "pool-rpc", PathRegexp: "/v1/pool/.*", Price: 1000},
		},
	}
	sk := nostr.GeneratePrivateKey()
	ev, err := BuildEvent(sk, cfg, BuildOptions{PublicURL: "https://api.example.com"})
	if err != nil {
		t.Fatal(err)
	}

	assertTag(t, ev, "name", "loop-rpc, pool-rpc")
	assertTag(t, ev, "about", "L402-gated API via Aperture — loop-rpc, pool-rpc")
}

func TestBuildEvent_NameTruncation(t *testing.T) {
	services := make([]config.Service, 7)
	for i := range services {
		services[i] = config.Service{
			Name: fmt.Sprintf("svc-%d", i+1), PathRegexp: "/", Price: 1,
		}
	}
	cfg := &config.ApertureConfig{Services: services}
	sk := nostr.GeneratePrivateKey()
	ev, err := BuildEvent(sk, cfg, BuildOptions{PublicURL: "https://api.example.com"})
	if err != nil {
		t.Fatal(err)
	}

	assertTag(t, ev, "name", "svc-1, svc-2, svc-3 and 4 more")
}
```

Note: you will need to add `"fmt"` to the test file imports.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/announce/ -run "TestBuildEvent_Name" -v`
Expected: FAIL — name tag still contains full description.

- [ ] **Step 3: Implement name/about split in BuildEvent**

In `internal/announce/announce.go`, replace the name/about building section:

```go
	// Build name tag (short) and about tag (full description)
	var serviceNames []string
	for _, s := range cfg.Services {
		serviceNames = append(serviceNames, s.Name)
	}

	var name string
	if len(serviceNames) > 5 {
		name = strings.Join(serviceNames[:3], ", ") + fmt.Sprintf(" and %d more", len(serviceNames)-3)
	} else {
		name = strings.Join(serviceNames, ", ")
	}

	about := "L402-gated API via Aperture"
	if len(serviceNames) > 0 {
		about += " — " + strings.Join(serviceNames, ", ")
	}

	tags := nostr.Tags{
		{"d", identifier},
		{"name", name},
		{"url", opts.PublicURL},
		{"about", about},
		{"pmi", "bitcoin-lightning-bolt11"},
		{"t", "l402"},
		{"t", "api"},
		{"t", "aperture"},
	}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/announce/ -v`
Expected: ALL PASS (some existing tests may need the `name` assertion updated if they check the exact value)

- [ ] **Step 5: Commit**

```bash
git add internal/announce/announce.go internal/announce/announce_test.go
git commit -m "feat: separate name and about tags in kind 31402 events"
```

### Task 5: Handle dynamic pricing in event building

**Files:**
- Modify: `internal/announce/announce.go` (capability and tag building loop)
- Test: `internal/announce/announce_test.go`

- [ ] **Step 1: Update capability struct**

In `internal/announce/announce.go`, update:

```go
type capability struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Endpoint    string `json:"endpoint,omitempty"`
	Pricing     string `json:"pricing,omitempty"`
	Auth        string `json:"auth,omitempty"`
	Timeout     int64  `json:"timeout,omitempty"`
}
```

- [ ] **Step 2: Write failing tests**

Add to `internal/announce/announce_test.go`:

```go
func TestBuildEvent_DynamicPricingNoPriceTag(t *testing.T) {
	cfg := &config.ApertureConfig{
		Services: []config.Service{
			{Name: "dynamic-api", PathRegexp: "/v1/.*", DynamicPrice: true, Price: 0},
		},
	}
	sk := nostr.GeneratePrivateKey()
	ev, err := BuildEvent(sk, cfg, BuildOptions{PublicURL: "https://api.example.com"})
	if err != nil {
		t.Fatal(err)
	}

	// Should NOT have a price tag (dynamic, no fallback)
	for _, tag := range ev.Tags {
		if len(tag) >= 1 && tag[0] == "price" {
			t.Error("dynamic-priced service with no fallback should not have a price tag")
		}
	}

	// Should have dynamic-pricing topic tag
	assertTag(t, ev, "t", "dynamic-pricing")

	// Content should have pricing: "dynamic"
	var content eventContent
	if err := json.Unmarshal([]byte(ev.Content), &content); err != nil {
		t.Fatal(err)
	}
	if len(content.Capabilities) != 1 {
		t.Fatalf("expected 1 capability, got %d", len(content.Capabilities))
	}
	if content.Capabilities[0].Pricing != "dynamic" {
		t.Errorf("pricing = %q, want %q", content.Capabilities[0].Pricing, "dynamic")
	}
}

func TestBuildEvent_DynamicPricingWithFallback(t *testing.T) {
	cfg := &config.ApertureConfig{
		Services: []config.Service{
			{Name: "api", PathRegexp: "/v1/.*", DynamicPrice: true, Price: 500},
		},
	}
	sk := nostr.GeneratePrivateKey()
	ev, err := BuildEvent(sk, cfg, BuildOptions{PublicURL: "https://api.example.com"})
	if err != nil {
		t.Fatal(err)
	}

	// Should still have a price tag (static fallback)
	assertPriceTag(t, ev, "api", "500", "sats")

	// Should have dynamic-pricing topic
	assertTag(t, ev, "t", "dynamic-pricing")

	// Content should have pricing: "dynamic"
	var content eventContent
	if err := json.Unmarshal([]byte(ev.Content), &content); err != nil {
		t.Fatal(err)
	}
	if content.Capabilities[0].Pricing != "dynamic" {
		t.Errorf("pricing = %q, want %q", content.Capabilities[0].Pricing, "dynamic")
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/announce/ -run "TestBuildEvent_DynamicPricing" -v`
Expected: FAIL — old test expects price tag "0"; new tests fail on missing topic/pricing field.

- [ ] **Step 4: Implement dynamic pricing logic**

In `BuildEvent`, replace the service loop with:

```go
	var caps []capability
	hasDynamicPricing := false

	for _, svc := range cfg.Services {
		endpoint := CleanEndpoint(svc.PathRegexp)

		if svc.DynamicPrice {
			hasDynamicPricing = true
		}

		if len(svc.Capabilities) > 0 {
			for _, capName := range svc.Capabilities {
				// Skip price tag for dynamic-priced services with no fallback
				if !svc.DynamicPrice || svc.Price > 0 {
					tags = append(tags, nostr.Tag{
						"price", capName, strconv.FormatInt(svc.Price, 10), "sats",
					})
				}

				cap := capability{
					Name:        capName,
					Description: fmt.Sprintf("%s via %s", capName, svc.Name),
					Endpoint:    endpoint,
				}
				if svc.DynamicPrice {
					cap.Pricing = "dynamic"
				}
				caps = append(caps, cap)
			}
		} else {
			if !svc.DynamicPrice || svc.Price > 0 {
				tags = append(tags, nostr.Tag{
					"price", svc.Name, strconv.FormatInt(svc.Price, 10), "sats",
				})
			}

			cap := capability{
				Name:        svc.Name,
				Description: fmt.Sprintf("Access %s", svc.Name),
				Endpoint:    endpoint,
			}
			if svc.DynamicPrice {
				cap.Pricing = "dynamic"
			}
			caps = append(caps, cap)
		}
	}

	if hasDynamicPricing {
		tags = append(tags, nostr.Tag{"t", "dynamic-pricing"})
	}
```

- [ ] **Step 5: Update the old TestBuildEvent_DynamicPricing test**

The old test asserts `assertPriceTag(t, ev, "dynamic-api", "0", "sats")`. This must be removed — dynamic-priced services with no fallback no longer get price tags. Replace the entire test:

```go
func TestBuildEvent_DynamicPricing(t *testing.T) {
	// Covered by TestBuildEvent_DynamicPricingNoPriceTag
	// and TestBuildEvent_DynamicPricingWithFallback
}
```

Or just delete it entirely.

- [ ] **Step 6: Run all tests**

Run: `go test ./... && go vet ./...`
Expected: ALL PASS

- [ ] **Step 7: Commit**

```bash
git add internal/announce/announce.go internal/announce/announce_test.go
git commit -m "feat: handle dynamic pricing — skip price tags, add pricing field"
```

### Task 6: Add auth and timeout to capabilities

**Files:**
- Modify: `internal/announce/announce.go` (capability building in loop)
- Test: `internal/announce/announce_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/announce/announce_test.go`:

```go
func TestBuildEvent_AuthInContent(t *testing.T) {
	cfg := &config.ApertureConfig{
		Services: []config.Service{
			{Name: "free-api", PathRegexp: "/v1/free", Price: 1, Auth: "off"},
			{Name: "paid-api", PathRegexp: "/v1/paid", Price: 100, Auth: "on"},
			{Name: "freebie-api", PathRegexp: "/v1/freebie", Price: 50, Auth: "freebie 3"},
		},
	}
	sk := nostr.GeneratePrivateKey()
	ev, err := BuildEvent(sk, cfg, BuildOptions{PublicURL: "https://api.example.com"})
	if err != nil {
		t.Fatal(err)
	}

	var content eventContent
	if err := json.Unmarshal([]byte(ev.Content), &content); err != nil {
		t.Fatal(err)
	}
	if len(content.Capabilities) != 3 {
		t.Fatalf("expected 3 capabilities, got %d", len(content.Capabilities))
	}

	// "off" → "none"
	if content.Capabilities[0].Auth != "none" {
		t.Errorf("free-api auth = %q, want %q", content.Capabilities[0].Auth, "none")
	}
	// "on" → omitted (empty)
	if content.Capabilities[1].Auth != "" {
		t.Errorf("paid-api auth = %q, want empty (default)", content.Capabilities[1].Auth)
	}
	// "freebie 3" → "freebie 3"
	if content.Capabilities[2].Auth != "freebie 3" {
		t.Errorf("freebie-api auth = %q, want %q", content.Capabilities[2].Auth, "freebie 3")
	}
}

func TestBuildEvent_TimeoutInContent(t *testing.T) {
	cfg := &config.ApertureConfig{
		Services: []config.Service{
			{Name: "timed-api", PathRegexp: "/v1/timed", Price: 100, Timeout: 3600},
			{Name: "untimed-api", PathRegexp: "/v1/untimed", Price: 50},
		},
	}
	sk := nostr.GeneratePrivateKey()
	ev, err := BuildEvent(sk, cfg, BuildOptions{PublicURL: "https://api.example.com"})
	if err != nil {
		t.Fatal(err)
	}

	var content eventContent
	if err := json.Unmarshal([]byte(ev.Content), &content); err != nil {
		t.Fatal(err)
	}

	if content.Capabilities[0].Timeout != 3600 {
		t.Errorf("timed-api timeout = %d, want 3600", content.Capabilities[0].Timeout)
	}
	if content.Capabilities[1].Timeout != 0 {
		t.Errorf("untimed-api timeout = %d, want 0 (omitted)", content.Capabilities[1].Timeout)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/announce/ -run "TestBuildEvent_Auth|TestBuildEvent_Timeout" -v`
Expected: FAIL — auth and timeout not set on capabilities.

- [ ] **Step 3: Implement auth and timeout in capability building**

Add a helper function in `internal/announce/announce.go`:

```go
// authForEvent converts Aperture's auth level to the event content value.
// Returns empty string for default ("on") — omitted via omitempty.
func authForEvent(auth string) string {
	switch strings.ToLower(strings.TrimSpace(auth)) {
	case "off", "false":
		return "none"
	case "", "on", "true":
		return "" // default, omitted
	default:
		// "freebie N" or unrecognised — pass through
		return auth
	}
}
```

In the capability building loop, after setting `cap.Pricing`, add:
```go
				cap.Auth = authForEvent(svc.Auth)
				cap.Timeout = svc.Timeout
```

Apply this to both branches (with capabilities and without).

- [ ] **Step 4: Run all tests**

Run: `go test ./... && go vet ./...`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/announce/announce.go internal/announce/announce_test.go
git commit -m "feat: add auth and timeout to capability content in events"
```

---

## Chunk 4: CLI + Integration

### Task 7: Add --topics flag

**Files:**
- Modify: `cmd/aperture-announce/main.go` (flag, env var, pass to BuildOptions)
- Modify: `internal/announce/announce.go` (topic handling in BuildEvent)
- Test: `internal/announce/announce_test.go`
- Test: `cmd/aperture-announce/main_test.go`

- [ ] **Step 1: Write failing test for custom topics in BuildEvent**

Add to `internal/announce/announce_test.go`:

```go
func TestBuildEvent_CustomTopics(t *testing.T) {
	cfg := &config.ApertureConfig{
		Services: []config.Service{
			{Name: "api", PathRegexp: "/v1/.*", Price: 100},
		},
	}
	sk := nostr.GeneratePrivateKey()
	ev, err := BuildEvent(sk, cfg, BuildOptions{
		PublicURL: "https://api.example.com",
		Topics:    []string{"ai", "inference", "l402"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Defaults always present
	assertTag(t, ev, "t", "l402")
	assertTag(t, ev, "t", "api")
	assertTag(t, ev, "t", "aperture")
	// Custom topics appended
	assertTag(t, ev, "t", "ai")
	assertTag(t, ev, "t", "inference")

	// Count total "t" tags — l402 should NOT be duplicated
	tCount := 0
	for _, tag := range ev.Tags {
		if len(tag) >= 1 && tag[0] == "t" {
			tCount++
		}
	}
	if tCount != 5 { // l402, api, aperture, ai, inference (l402 deduped)
		t.Errorf("expected 5 topic tags, got %d", tCount)
	}
}

func TestBuildEvent_TopicCap(t *testing.T) {
	cfg := &config.ApertureConfig{
		Services: []config.Service{
			{Name: "api", PathRegexp: "/v1/.*", Price: 100},
		},
	}
	// Generate 60 custom topics
	topics := make([]string, 60)
	for i := range topics {
		topics[i] = fmt.Sprintf("topic-%d", i)
	}
	sk := nostr.GeneratePrivateKey()
	ev, err := BuildEvent(sk, cfg, BuildOptions{
		PublicURL: "https://api.example.com",
		Topics:    topics,
	})
	if err != nil {
		t.Fatal(err)
	}

	tCount := 0
	for _, tag := range ev.Tags {
		if len(tag) >= 1 && tag[0] == "t" {
			tCount++
		}
	}
	if tCount > 50 {
		t.Errorf("topic tags should be capped at 50, got %d", tCount)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/announce/ -run TestBuildEvent_CustomTopics -v`
Expected: FAIL — custom topics not in event.

- [ ] **Step 3: Implement topic handling in BuildEvent**

In `BuildEvent`, replace the hardcoded topic tags with:

```go
	// Build topic list: defaults first, then custom, deduplicated
	defaults := []string{"l402", "api", "aperture"}
	seen := make(map[string]bool)
	var topics []string
	for _, t := range defaults {
		if !seen[t] {
			seen[t] = true
			topics = append(topics, t)
		}
	}
	for _, t := range opts.Topics {
		t = strings.TrimSpace(t)
		if t != "" && !seen[t] {
			seen[t] = true
			topics = append(topics, t)
		}
	}

	// Cap at 50 topics (matching 402-announce limit)
	const maxTopics = 50
	if len(topics) > maxTopics {
		topics = topics[:maxTopics]
	}

	// ... build tags, replacing hardcoded "t" tags with:
	for _, topic := range topics {
		tags = append(tags, nostr.Tag{"t", topic})
	}
```

Also move the `dynamic-pricing` topic insertion to use the same `seen` map and cap:
```go
	if hasDynamicPricing && !seen["dynamic-pricing"] && len(topics) < maxTopics {
		tags = append(tags, nostr.Tag{"t", "dynamic-pricing"})
	}
```

Remove the hardcoded `{"t", "l402"}, {"t", "api"}, {"t", "aperture"}` from the initial tags slice.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/announce/ -v`
Expected: ALL PASS

- [ ] **Step 5: Add --topics flag to CLI**

In `cmd/aperture-announce/main.go`, add after the `verbose` flag:

```go
	topics := flag.String("topics", "", "Comma-separated custom topic tags (appended to defaults)")
```

Add env var fallback after existing env vars:
```go
	if *topics == "" {
		*topics = os.Getenv("ANNOUNCE_TOPICS")
	}
```

Parse topics into a slice and pass to BuildOptions:
```go
	var topicList []string
	if *topics != "" {
		for _, t := range strings.Split(*topics, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				topicList = append(topicList, t)
			}
		}
	}
```

Update the BuildEvent call:
```go
	ev, err := announce.BuildEvent(sk, cfg, announce.BuildOptions{
		PublicURL: *publicURL,
		Picture:   *picture,
		Topics:    topicList,
	})
```

- [ ] **Step 6: Update dynamic pricing warning and add auth validation**

In `main.go`, update the dynamic pricing warning to reflect the new behaviour, and add auth validation warnings. Add a helper function:

```go
// isRecognisedAuth returns true if the auth value is one Aperture accepts.
func isRecognisedAuth(auth string) bool {
	switch strings.ToLower(strings.TrimSpace(auth)) {
	case "", "on", "true", "off", "false":
		return true
	}
	// "freebie N" where N is a positive integer
	lower := strings.ToLower(strings.TrimSpace(auth))
	if strings.HasPrefix(lower, "freebie ") {
		rest := strings.TrimSpace(lower[len("freebie "):])
		for _, c := range rest {
			if c < '0' || c > '9' {
				return false
			}
		}
		return rest != ""
	}
	return false
}
```

Update the warning loop:

```go
	for _, svc := range cfg.Services {
		if svc.DynamicPrice {
			if svc.Price > 0 {
				fmt.Fprintf(os.Stderr, "Warning: service %q uses dynamic pricing — announced price of %d sats is the static fallback; actual price is determined at request time\n", svc.Name, svc.Price)
			} else {
				fmt.Fprintf(os.Stderr, "Warning: service %q uses dynamic pricing with no static fallback — price tag omitted from announcement\n", svc.Name)
			}
		}
		if svc.Auth != "" && !isRecognisedAuth(svc.Auth) {
			fmt.Fprintf(os.Stderr, "Warning: service %q has unrecognised auth value %q — treating as \"on\" (payment required)\n", svc.Name, svc.Auth)
		}
	}
```

- [ ] **Step 7: Run full test suite**

Run: `go test ./... && go vet ./...`
Expected: ALL PASS

- [ ] **Step 8: Commit**

```bash
git add internal/announce/announce.go internal/announce/announce_test.go cmd/aperture-announce/main.go
git commit -m "feat: add --topics flag for custom topic tags"
```

### Task 8: Update testdata and CLI integration tests

**Files:**
- Modify: `testdata/sample-conf.yaml`
- Modify: `cmd/aperture-announce/main_test.go`

- [ ] **Step 1: Update sample config**

Replace `testdata/sample-conf.yaml` with:

```yaml
listenaddr: "localhost:8081"

services:
  - name: "loop-rpc"
    hostregexp: "loop.example.com"
    pathregexp: "^/v1/loop/.*$"
    price: 500
    capabilities: "quote,swap"
    auth: "freebie 3"
    timeout: 3600

  - name: "pool-rpc"
    hostregexp: "pool.example.com"
    pathregexp: "^/v1/pool/.*$"
    price: 1000
```

- [ ] **Step 2: Update CLI dry-run test to check new format**

In `cmd/aperture-announce/main_test.go`, update `TestCLI_DryRun` to verify:
- `name` tag is `"loop-rpc, pool-rpc"` (not the full about string)
- `about` tag contains `"L402-gated API via Aperture"`
- Capabilities in content have cleaned endpoints (no regex)

Also add:

```go
func TestCLI_TopicsFlag(t *testing.T) {
	bin := buildBinary(t)
	configPath, _ := filepath.Abs("../../testdata/sample-conf.yaml")

	cmd := exec.Command(bin,
		"--config", configPath,
		"--public-url", "https://api.example.com",
		"--topics", "ai,inference",
		"--dry-run",
	)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			t.Fatalf("exit %d: %s", ee.ExitCode(), ee.Stderr)
		}
		t.Fatal(err)
	}

	outStr := string(out)
	if !strings.Contains(outStr, `"ai"`) {
		t.Error("missing custom topic 'ai' in output")
	}
	if !strings.Contains(outStr, `"inference"`) {
		t.Error("missing custom topic 'inference' in output")
	}
}

func TestCLI_TopicsEnvVar(t *testing.T) {
	bin := buildBinary(t)
	configPath, _ := filepath.Abs("../../testdata/sample-conf.yaml")

	cmd := exec.Command(bin, "--dry-run")
	cmd.Env = append(os.Environ(),
		"APERTURE_CONFIG="+configPath,
		"PUBLIC_URL=https://api.example.com",
		"ANNOUNCE_TOPICS=custom1,custom2",
	)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			t.Fatalf("exit %d: %s", ee.ExitCode(), ee.Stderr)
		}
		t.Fatal(err)
	}

	outStr := string(out)
	if !strings.Contains(outStr, `"custom1"`) {
		t.Error("missing env var topic 'custom1'")
	}
}
```

Also add a test for the auth warning. Create a temp config with `auth: "maybe"`:

```go
func TestCLI_AuthWarning(t *testing.T) {
	bin := buildBinary(t)

	configYAML := `
services:
  - name: "api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/.*"
    price: 100
    auth: "maybe"
`
	configPath := filepath.Join(t.TempDir(), "aperture.yaml")
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin,
		"--config", configPath,
		"--public-url", "https://api.example.com",
		"--dry-run",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Should still succeed (warning, not fatal)
		if ee, ok := err.(*exec.ExitError); ok {
			t.Fatalf("exit %d: %s", ee.ExitCode(), out)
		}
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "unrecognised auth") {
		t.Errorf("expected auth warning in output, got: %s", out)
	}
}
```

- [ ] **Step 3: Run full test suite**

Run: `go test ./... && go vet ./...`
Expected: ALL PASS

- [ ] **Step 4: Commit**

```bash
git add testdata/sample-conf.yaml cmd/aperture-announce/main_test.go
git commit -m "test: update sample config and CLI tests for new event format"
```

---

## Chunk 5: Docs + Cleanup

### Task 9: Update documentation

**Files:**
- Modify: `docs/event-format.md`
- Modify: `schemas/kind-31402.schema.json`
- Modify: `README.md`
- Modify: `llms.txt`
- Modify: `llms-full.txt`

- [ ] **Step 1: Update event-format.md**

Add new sections for:
- `auth` field in content capabilities (values: `"none"`, `"freebie N"`, omitted for default)
- `timeout` field in content capabilities (seconds, omitted when 0)
- `pricing` field in content capabilities (value: `"dynamic"`, omitted for static)
- `dynamic-pricing` topic tag
- Updated endpoint description (cleaned base paths, not regex)
- `--topics` flag

- [ ] **Step 2: Update JSON schema**

Add `pricing`, `auth`, and `timeout` to the `content_object.capabilities.items.properties` in `schemas/kind-31402.schema.json`.

- [ ] **Step 3: Update README.md**

Add `--topics` to the usage section. Update the "How it works" section to mention auth, timeout, and dynamic pricing handling.

- [ ] **Step 4: Update llms.txt and llms-full.txt**

Update key concepts to mention auth, timeout, dynamic pricing, and custom topics. Update the API surface section with the new `--topics` flag.

- [ ] **Step 5: Run full test suite one final time**

Run: `go test ./... && go vet ./...`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add docs/event-format.md schemas/kind-31402.schema.json README.md llms.txt llms-full.txt
git commit -m "docs: update event format, schema, and README for new features"
```
