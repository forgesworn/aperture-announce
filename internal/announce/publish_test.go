package announce

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ws "github.com/coder/websocket"
	"github.com/TheCryptoDonkey/aperture-announce/internal/config"
	"github.com/nbd-wtf/go-nostr"
)

// mockRelay is a minimal NIP-01 relay that accepts EVENT messages and replies OK.
func mockRelay(t *testing.T, acceptEvent bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := ws.Accept(w, r, &ws.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			t.Logf("mock relay accept error: %v", err)
			return
		}
		defer c.CloseNow()

		for {
			_, msg, err := c.Read(r.Context())
			if err != nil {
				return
			}

			// Parse the envelope: ["EVENT", {...}]
			var envelope []json.RawMessage
			if err := json.Unmarshal(msg, &envelope); err != nil {
				continue
			}
			if len(envelope) < 2 {
				continue
			}

			var label string
			if err := json.Unmarshal(envelope[0], &label); err != nil {
				continue
			}
			if label != "EVENT" {
				continue
			}

			// Extract event ID
			var ev struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(envelope[1], &ev); err != nil {
				continue
			}

			// Reply with OK
			ok := []interface{}{"OK", ev.ID, acceptEvent, ""}
			resp, _ := json.Marshal(ok)
			_ = c.Write(r.Context(), ws.MessageText, resp)
		}
	}))
}

func buildTestEvent(t *testing.T) *nostr.Event {
	t.Helper()
	cfg := &config.ApertureConfig{
		Services: []config.Service{
			{Name: "test-api", PathRegexp: "/v1/.*", Price: 100},
		},
	}
	sk := nostr.GeneratePrivateKey()
	ev, err := BuildEvent(sk, cfg, BuildOptions{PublicURL: "https://api.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	return ev
}

func TestPublish_Success(t *testing.T) {
	srv := mockRelay(t, true)
	defer srv.Close()

	relayURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	ev := buildTestEvent(t)

	results := Publish(context.Background(), ev, []string{relayURL})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Success {
		t.Errorf("expected success, got error: %v", results[0].Error)
	}
	if results[0].Relay != relayURL {
		t.Errorf("relay = %q, want %q", results[0].Relay, relayURL)
	}
}

func TestPublish_ConnectFailure(t *testing.T) {
	ev := buildTestEvent(t)

	// Use an unreachable URL
	results := Publish(context.Background(), ev, []string{"ws://127.0.0.1:1"})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Success {
		t.Error("expected failure for unreachable relay")
	}
	if results[0].Error == nil {
		t.Error("expected error to be set")
	}
}

func TestPublish_Rejected(t *testing.T) {
	srv := mockRelay(t, false) // relay rejects the event
	defer srv.Close()

	relayURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	ev := buildTestEvent(t)

	results := Publish(context.Background(), ev, []string{relayURL})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Success {
		t.Error("expected failure when relay rejects event")
	}
}

func TestPublish_MultipleRelays(t *testing.T) {
	srv := mockRelay(t, true)
	defer srv.Close()

	relayURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	ev := buildTestEvent(t)

	results := Publish(context.Background(), ev, []string{
		relayURL,
		"ws://127.0.0.1:1", // unreachable
	})

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// First should succeed, second should fail
	if !results[0].Success {
		t.Errorf("first relay should succeed, got: %v", results[0].Error)
	}
	if results[1].Success {
		t.Error("second relay should fail (unreachable)")
	}
}

func TestPublish_ContextCancelled(t *testing.T) {
	ev := buildTestEvent(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	results := Publish(ctx, ev, []string{"ws://127.0.0.1:1"})

	// Context already cancelled — should return early without attempting any relay.
	if len(results) != 0 {
		t.Fatalf("expected 0 results (early exit), got %d", len(results))
	}
}

func TestPublish_EmptyRelayList(t *testing.T) {
	ev := buildTestEvent(t)

	results := Publish(context.Background(), ev, []string{})

	if len(results) != 0 {
		t.Fatalf("expected 0 results for empty relay list, got %d", len(results))
	}
}
