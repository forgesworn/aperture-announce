package announce

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/forgesworn/aperture-announce/internal/config"
	ws "github.com/coder/websocket"
	"github.com/nbd-wtf/go-nostr"
	"github.com/stretchr/testify/require"
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
		defer func() { _ = c.CloseNow() }()

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
	ev, err := BuildEvent(sk, cfg, BuildOptions{PublicUrls: []string{"https://api.example.com"}})
	require.NoError(t, err)
	return ev
}

func TestPublish_Success(t *testing.T) {
	srv := mockRelay(t, true)
	defer srv.Close()

	relayURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	ev := buildTestEvent(t)

	results := Publish(context.Background(), ev, []string{relayURL})

	require.Len(t, results, 1)
	require.True(t, results[0].Success, "expected success, got error: %v", results[0].Error)
	require.Equal(t, relayURL, results[0].Relay)
}

func TestPublish_ConnectFailure(t *testing.T) {
	ev := buildTestEvent(t)

	// Use an unreachable URL
	results := Publish(context.Background(), ev, []string{"ws://127.0.0.1:1"})

	require.Len(t, results, 1)
	require.False(t, results[0].Success, "expected failure for unreachable relay")
	require.NotNil(t, results[0].Error)
}

func TestPublish_Rejected(t *testing.T) {
	srv := mockRelay(t, false) // relay rejects the event
	defer srv.Close()

	relayURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	ev := buildTestEvent(t)

	results := Publish(context.Background(), ev, []string{relayURL})

	require.Len(t, results, 1)
	require.False(t, results[0].Success, "expected failure when relay rejects event")
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

	require.Len(t, results, 2)

	// First should succeed, second should fail
	require.True(t, results[0].Success, "first relay should succeed, got: %v", results[0].Error)
	require.False(t, results[1].Success, "second relay should fail (unreachable)")
}

func TestPublish_ContextCancelled(t *testing.T) {
	ev := buildTestEvent(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	results := Publish(ctx, ev, []string{"ws://127.0.0.1:1"})

	// Context already cancelled — should return early without attempting any relay.
	require.Len(t, results, 0)
}

func TestPublish_EmptyRelayList(t *testing.T) {
	ev := buildTestEvent(t)

	results := Publish(context.Background(), ev, []string{})

	require.Len(t, results, 0)
}
