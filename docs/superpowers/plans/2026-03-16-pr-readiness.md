# PR Readiness Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the two real technical issues (misleading `CleanEndpoint` and sequential relay publishing), clean up cosmetic issues, add Claude CI workflows, and file future work as GitHub issues — preparing the codebase for scrutiny by the Aperture (Lightning Labs) team.

**Architecture:** Four independent workstreams: (1) fix `CleanEndpoint` to return empty on lossy truncation rather than a misleading partial path, (2) make `Publish` concurrent with goroutines, (3) cosmetic fixes (gofmt, badge, unused struct field), (4) add Claude Code CI workflows modelled on Aperture's own setup. Then file GitHub issues for future work.

**Tech Stack:** Go 1.24, go-nostr, sync.WaitGroup, GitHub Actions, anthropics/claude-code-action@v1

---

## Chunk 1: Fix CleanEndpoint lossy truncation

### Task 1: Make CleanEndpoint return empty on group-boundary truncation

The current `CleanEndpoint` truncates at `(` and returns whatever prefix remains. For `^/v1/(quote|swap)/.*$` this produces `/v1/` — a misleading path that will 404. The fix: when truncation happens at a group boundary `(`, return empty string if the remaining prefix is just a directory with no specific path segment after the last `/`.

More precisely: after truncation, if the result ends with `/` and the truncation was caused by `(`, `[`, or `{`, return empty — the path is ambiguous.

**Files:**
- Modify: `internal/announce/announce.go:31-67`
- Modify: `internal/announce/announce_test.go:205-230`

- [ ] **Step 1: Update the test expectations first**

In `announce_test.go`, the `TestCleanEndpoint` table currently expects `"/v1/"` for the alternation pattern. Change it to `""` since the path is ambiguous. Also add new test cases for patterns where truncation at `(` still leaves a meaningful path.

```go
// In TestCleanEndpoint, update these test cases (all produce trailing slash after truncation):
{"^/v1/loop/.*$", ""},              // was "/v1/loop/" — trailing slash is ambiguous
{"/v1/pool/.*", ""},                // was "/v1/pool/" — trailing slash is ambiguous
{"^/v1/(quote|swap)/.*$", ""},      // was "/v1/" — trailing slash after truncation is ambiguous

// Keep these UNCHANGED (truncation produces a meaningful non-trailing-slash path):
// {"/v1/data{2,5}", "/v1/data"},    — already correct, no trailing slash
// {"/v1/users[0-9]+", "/v1/users"}, — already correct, no trailing slash

// Add new test cases:
{"/api/v2/(users|admin)", ""},                      // truncation at ( leaves trailing slash → ambiguous
{"^/v1/loop/specific(opt)?$", "/v1/loop/specific"}, // truncation at ( but no trailing slash → meaningful
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/announce/ -run TestCleanEndpoint -v`
Expected: FAIL — current code returns `"/v1/"` not `""`

- [ ] **Step 3: Update CleanEndpoint implementation**

In `announce.go`, after the truncation at step 2, check if the result ends with `/` — if so, the truncation removed the only meaningful path segment and the result is ambiguous. Return empty.

```go
func CleanEndpoint(regex string) string {
	if regex == "" {
		return ""
	}

	s := regex

	// Step 1: strip anchors
	s = strings.TrimPrefix(s, "^")
	s = strings.TrimSuffix(s, "$")

	// Step 2: truncate at first regex metacharacter
	cutSet := []string{".*", ".+", "[", "(", "{"}
	minIdx := len(s)
	for _, pat := range cutSet {
		if idx := strings.Index(s, pat); idx != -1 && idx < minIdx {
			minIdx = idx
		}
	}
	// '?' makes the preceding character optional, so strip it too
	if idx := strings.Index(s, "?"); idx != -1 && idx < minIdx {
		minIdx = idx - 1
		if minIdx < 0 {
			minIdx = 0
		}
	}
	s = s[:minIdx]

	// Step 3: clean up — if the result ends with '/' the truncation
	// removed the only specific path segment, leaving an ambiguous
	// directory prefix. Return empty rather than mislead agents.
	if s == "/" || s == "" || strings.HasSuffix(s, "/") {
		return ""
	}

	return s
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/announce/ -run TestCleanEndpoint -v`
Expected: PASS

- [ ] **Step 5: Verify no regressions in the full event build tests**

Run: `go test ./internal/announce/ -v`
Expected: all PASS. `TestBuildEvent_EndpointsCleaned` uses `"^/v1/loop/.*$"` which truncates to `/v1/loop/` — this now ends with `/` and will return `""`. This test must also be updated.

- [ ] **Step 6: Update TestBuildEvent_EndpointsCleaned**

The test at `announce_test.go:131-150` expects endpoint `"/v1/loop/"`. Update to expect `""` since trailing-slash results are now treated as ambiguous. Also update the test to use a non-ambiguous pattern to verify endpoints still work.

```go
func TestBuildEvent_EndpointsCleaned(t *testing.T) {
	cfg := &config.ApertureConfig{
		Services: []config.Service{
			{Name: "api", PathRegexp: "^/v1/loop/.*$", Price: 100},
			{Name: "exact", PathRegexp: "^/v1/status$", Price: 50},
		},
	}
	sk := nostr.GeneratePrivateKey()
	ev, err := BuildEvent(sk, cfg, BuildOptions{PublicUrls: []string{"https://api.example.com"}})
	if err != nil {
		t.Fatal(err)
	}

	var content eventContent
	if err := json.Unmarshal([]byte(ev.Content), &content); err != nil {
		t.Fatal(err)
	}
	// Trailing-slash result is ambiguous — omitted.
	if content.Capabilities[0].Endpoint != "" {
		t.Errorf("endpoint = %q, want empty (ambiguous trailing slash)", content.Capabilities[0].Endpoint)
	}
	// Exact path is preserved.
	if content.Capabilities[1].Endpoint != "/v1/status" {
		t.Errorf("endpoint = %q, want %q", content.Capabilities[1].Endpoint, "/v1/status")
	}
}
```

- [ ] **Step 7: Run full test suite**

Run: `go test ./... -count=1`
Expected: all PASS

- [ ] **Step 8: Update README example output**

The README example at `README.md:112` contains `"endpoint":"/v1/loop/"` and `"endpoint":"/v1/pool/"`. After the CleanEndpoint change, these patterns (`^/v1/loop/.*$`, `^/v1/pool/.*$`) produce empty strings, and `omitempty` omits the field from JSON. Update the example content JSON to remove the endpoint fields:

Change line 112 from:
```
"content": "{\"capabilities\":[{\"name\":\"quote\",\"description\":\"quote via loop-rpc\",\"endpoint\":\"/v1/loop/\"},{\"name\":\"swap\",\"description\":\"swap via loop-rpc\",\"endpoint\":\"/v1/loop/\"},{\"name\":\"pool-rpc\",\"description\":\"Access pool-rpc\",\"endpoint\":\"/v1/pool/\"}]}"
```
To:
```
"content": "{\"capabilities\":[{\"name\":\"quote\",\"description\":\"quote via loop-rpc\"},{\"name\":\"swap\",\"description\":\"swap via loop-rpc\"},{\"name\":\"pool-rpc\",\"description\":\"Access pool-rpc\"}]}"
```

- [ ] **Step 9: Update docs/event-format.md example and description**

In `docs/event-format.md:22`, update the endpoint description to note it is best-effort.

Update the content JSON example at lines 39-63 to show one capability with an endpoint (from an exact pattern) and others without:

```json
{
  "capabilities": [
    {
      "name": "quote",
      "description": "quote via loop-rpc",
      "auth": "none",
      "timeout": 3600
    },
    {
      "name": "swap",
      "description": "swap via loop-rpc",
      "pricing": "dynamic"
    },
    {
      "name": "status",
      "description": "Access status",
      "endpoint": "/v1/status",
      "auth": "freebie 5"
    }
  ]
}
```

Also update the endpoint description on line 22:

Change line 22 from:
> The `endpoint` field in capabilities is now a **cleaned base path** (e.g. `/v1/loop`) derived from the Aperture `pathregexp`, not the raw regexp string.

To:
> The `endpoint` field in capabilities is a **best-effort base path** (e.g. `/v1/status`) derived from the Aperture `pathregexp`. When the path pattern contains alternation or is otherwise ambiguous, the field is omitted rather than emitting a misleading partial path. Agents should treat this as a hint, not a contract.

In `schemas/kind-31402.schema.json:91`, update the endpoint description:

Change:
> "description": "Cleaned base path for this capability (e.g. /v1/loop)"

To:
> "description": "Best-effort base path hint for this capability (e.g. /v1/status). Omitted when the path pattern is ambiguous."

- [ ] **Step 10: Update schema endpoint description**

In `schemas/kind-31402.schema.json:91`, update the endpoint description.

- [ ] **Step 11: Commit**

```bash
git add internal/announce/announce.go internal/announce/announce_test.go docs/event-format.md schemas/kind-31402.schema.json README.md
git commit -m "fix: return empty endpoint on ambiguous path truncation

CleanEndpoint now returns empty string when the truncated result
ends with '/' — indicating the only specific path segment was
removed and the remaining prefix would mislead agents. Documents
the endpoint field as best-effort in the schema and event format."
```

---

## Chunk 2: Concurrent relay publishing

### Task 2: Make Publish concurrent with goroutines

Replace the sequential relay loop with concurrent publishing. Each relay already gets its own 10-second context timeout via `publishToRelay`, so concurrent execution is safe.

**Files:**
- Modify: `internal/announce/publish.go:1-57`
- Modify: `internal/announce/publish_test.go` (existing tests should pass without changes — the API contract is identical)

- [ ] **Step 1: Run existing publish tests to establish baseline**

Run: `go test ./internal/announce/ -run TestPublish -v`
Expected: all PASS

- [ ] **Step 2: Rewrite Publish to use goroutines**

Replace the sequential loop with concurrent publishing. Use a fixed-size results slice indexed by position (no channel needed, no mutex needed — each goroutine writes to its own index).

```go
package announce

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

// PublishResult records the outcome per relay.
type PublishResult struct {
	Relay   string
	Success bool
	Error   error
}

// Publish sends a signed event to the given relay URLs concurrently.
// Returns one result per relay. Respects context cancellation.
func Publish(ctx context.Context, ev *nostr.Event, relays []string) []PublishResult {
	if len(relays) == 0 {
		return nil
	}

	// Check context before spawning goroutines.
	select {
	case <-ctx.Done():
		return nil
	default:
	}

	results := make([]PublishResult, len(relays))
	var wg sync.WaitGroup

	for i, url := range relays {
		wg.Add(1)
		go func(idx int, relay string) {
			defer wg.Done()
			results[idx] = publishToRelay(ctx, ev, relay)
		}(i, url)
	}

	wg.Wait()
	return results
}

func publishToRelay(ctx context.Context, ev *nostr.Event, url string) PublishResult {
	rCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	relay, err := nostr.RelayConnect(rCtx, url)
	if err != nil {
		return PublishResult{
			Relay: url, Success: false,
			Error: fmt.Errorf("connect: %w", err),
		}
	}
	defer relay.Close()

	if err := relay.Publish(rCtx, *ev); err != nil {
		return PublishResult{
			Relay: url, Success: false,
			Error: fmt.Errorf("publish: %w", err),
		}
	}

	return PublishResult{Relay: url, Success: true}
}
```

- [ ] **Step 3: Run publish tests**

Run: `go test ./internal/announce/ -run TestPublish -v`
Expected: all PASS. Key behavioural difference: `TestPublish_MultipleRelays` — the result ordering is now by input index (guaranteed by the indexed slice) rather than by completion order. The test checks `results[0]` and `results[1]` by position, which still works.

Note: `TestPublish_ContextCancelled` cancels before calling Publish — the early `select` returns nil, matching the existing `len(results) != 0` assertion.

**Semantic change:** The old sequential code returned partial results if context was cancelled mid-way (e.g. 2 of 5 relays processed). The concurrent code spawns all goroutines immediately, so all relays are attempted — some may fail with context cancellation errors from `publishToRelay`. This is actually better behaviour (all relays get a chance). The early `select` only catches already-cancelled contexts. No existing test exercises mid-publish cancellation, so no test changes are needed.

- [ ] **Step 4: Run full test suite**

Run: `go test ./... -count=1`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/announce/publish.go
git commit -m "fix: publish to relays concurrently

Replaces sequential relay loop with goroutines. Each relay gets
its own 10-second context timeout. Results are collected in a
fixed-size slice indexed by position — no mutex needed. Worst-case
publish time is now ~10s regardless of relay count, down from
10s * N relays."
```

---

## Chunk 3: Cosmetic fixes

### Task 3: Fix gofmt import ordering

**Files:**
- Modify: `internal/announce/publish_test.go:11-13`

- [ ] **Step 1: Fix import order**

Swap the `ws` import and `forgesworn` import to alphabetical order within the third-party group:

```go
	"github.com/forgesworn/aperture-announce/internal/config"
	ws "github.com/coder/websocket"
	"github.com/nbd-wtf/go-nostr"
```

- [ ] **Step 2: Verify gofmt is clean**

Run: `gofmt -l .`
Expected: no output (all files formatted)

- [ ] **Step 3: Commit**

```bash
git add internal/announce/publish_test.go
git commit -m "style: correct import ordering in publish_test.go"
```

### Task 4: Fix README badge version

**Files:**
- Modify: `README.md:4`

- [ ] **Step 1: Update badge**

Change line 4 from:
```
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8)](https://golang.org/)
```
To:
```
[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8)](https://golang.org/)
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: fix Go version badge to match go.mod (1.24+)"
```

### Task 5: Remove unused GRPCAddress field

The `rawDynPrice.GRPCAddress` field is parsed from YAML but never read. Remove it to avoid reviewer questions about dead code.

**Files:**
- Modify: `internal/config/config.go:44-47`

- [ ] **Step 1: Remove GRPCAddress from rawDynPrice**

Change the struct from:
```go
type rawDynPrice struct {
	Enabled     bool   `yaml:"enabled"`
	GRPCAddress string `yaml:"grpcaddress"`
}
```
To:
```go
type rawDynPrice struct {
	Enabled bool `yaml:"enabled"`
}
```

- [ ] **Step 2: Run tests**

Run: `go test ./internal/config/ -v`
Expected: all PASS — no test references GRPCAddress

- [ ] **Step 3: Commit**

```bash
git add internal/config/config.go
git commit -m "refactor: remove unused GRPCAddress from rawDynPrice struct"
```

---

## Chunk 4: Add Claude CI workflows

Model these on Aperture's own setup at `lightninglabs/aperture/.github/workflows/`. Key difference: we have a CLAUDE.md, they don't — our code-review agents will have real guidelines to check against.

### Task 6: Add automated code review workflow

**Files:**
- Create: `.github/workflows/claude-code-review.yml`

- [ ] **Step 1: Create the workflow file**

```yaml
name: Claude Code Review

on:
  pull_request:
    types: [opened, synchronize, ready_for_review, reopened]

jobs:
  claude-review:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: read
      issues: read
      id-token: write

    steps:
      - name: Checkout repository
        uses: actions/checkout@v4
        with:
          fetch-depth: 1

      - name: Run Claude Code Review
        id: claude-review
        uses: anthropics/claude-code-action@v1
        with:
          claude_code_oauth_token: ${{ secrets.CLAUDE_CODE_OAUTH_TOKEN }}
          plugin_marketplaces: 'https://github.com/anthropics/claude-code.git'
          plugins: 'code-review@claude-code-plugins'
          prompt:
            '/code-review:code-review ${{ github.repository }}/pull/${{
            github.event.pull_request.number }} --comment'
          claude_args: '--model claude-sonnet-4-6'
```

Note: uses `claude-sonnet-4-6` (not opus) to keep CI costs reasonable for automated reviews.

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/claude-code-review.yml
git commit -m "ci: add automated Claude code review on PRs

Uses the official code-review plugin from Anthropic's marketplace.
Launches 4 parallel review agents with confidence-based scoring.
Our CLAUDE.md provides project-specific guidelines for compliance
checking."
```

### Task 7: Add interactive Claude agent workflow

**Files:**
- Create: `.github/workflows/claude.yml`

- [ ] **Step 1: Create the workflow file**

```yaml
name: Claude Code

on:
  issue_comment:
    types: [created]
  pull_request_review_comment:
    types: [created]
  issues:
    types: [opened, assigned]
  pull_request_review:
    types: [submitted]

jobs:
  claude:
    if: |
      (github.event_name == 'issue_comment' && contains(github.event.comment.body, '@claude')) ||
      (github.event_name == 'pull_request_review_comment' && contains(github.event.comment.body, '@claude')) ||
      (github.event_name == 'pull_request_review' && contains(github.event.review.body, '@claude')) ||
      (github.event_name == 'issues' && (contains(github.event.issue.body, '@claude') || contains(github.event.issue.title, '@claude')))
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: read
      issues: read
      id-token: write
      actions: read
    steps:
      - name: Checkout repository
        uses: actions/checkout@v4
        with:
          fetch-depth: 1

      - name: Run Claude Code
        id: claude
        uses: anthropics/claude-code-action@v1
        with:
          claude_code_oauth_token: ${{ secrets.CLAUDE_CODE_OAUTH_TOKEN }}
          additional_permissions: |
            actions: read
          claude_args: '--model claude-sonnet-4-6'
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/claude.yml
git commit -m "ci: add interactive Claude agent for issues and PRs

Responds to @claude mentions in issues, PR comments, and reviews.
Provides contextual answers about the codebase using our CLAUDE.md
for project-specific guidance."
```

---

## Chunk 5: File future work as GitHub issues

### Task 8: Create GitHub issues for future work

These issues show roadmap thinking without gold-plating v0.1. Create each via `gh issue create`.

- [ ] **Step 1: Create issues**

```bash
gh issue create --title "feat: announce token constraints from Aperture config" \
  --body "Aperture services can define \`constraints\` (e.g. token expiration, IP binding). These are security-relevant metadata that agents may want to know about before paying. Add an optional \`constraints\` field to capabilities in the event content."

gh issue create --title "feat: announce rate limits from Aperture config" \
  --body "Aperture services can define \`ratelimits\` per endpoint. Agents may want to know rate limit thresholds before committing to a service. Add an optional \`rateLimit\` field to capabilities in the event content."

gh issue create --title "ci: add golangci-lint to CI pipeline" \
  --body "Aperture uses golangci-lint with these linters: bodyclose, copyloopvar, dupl, goconst, gocritic, gocyclo, misspell, nakedret, prealloc, staticcheck, unconvert, unparam. Add a similar configuration for consistency with the Aperture ecosystem."

gh issue create --title "feat: watch config file for changes in interval mode" \
  --body "When running with \`--interval\`, the config is read once at startup. If the Aperture config changes, the announcements go stale. Add \`fsnotify\`-based file watching to re-read config on change and rebuild the event."

gh issue create --title "feat: filter which services to announce" \
  --body "Large Aperture deployments may want to announce only a subset of services. Add \`--include\` and \`--exclude\` flags to filter by service name pattern."
```

- [ ] **Step 2: Verify issues were created**

Run: `gh issue list --limit 10`
Expected: 5 new issues visible

---

## Summary of changes

| Change | Type | Risk |
|--------|------|------|
| CleanEndpoint returns empty on trailing-slash truncation | Behaviour change | Low — agents get no endpoint (safe) instead of wrong endpoint (unsafe) |
| Concurrent relay publishing | Performance | Low — API contract unchanged, results indexed by position |
| gofmt import ordering | Cosmetic | None |
| README badge version | Docs | None |
| Remove unused GRPCAddress field | Cleanup | None — field was parsed but never read |
| Claude code review workflow | CI | None — read-only review comments |
| Claude interactive agent workflow | CI | None — read-only responses |
| GitHub issues for future work | Project management | None |
