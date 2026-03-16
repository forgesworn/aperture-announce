package announce

import (
	"encoding/json"
	"testing"

	"github.com/TheCryptoDonkey/aperture-announce/internal/config"
	"github.com/nbd-wtf/go-nostr"
)

func TestBuildEvent_SingleService(t *testing.T) {
	cfg := &config.ApertureConfig{
		Services: []config.Service{
			{Name: "my-api", PathRegexp: "/v1/.*", Price: 100},
		},
	}
	sk := nostr.GeneratePrivateKey()
	ev, err := BuildEvent(sk, cfg, "https://api.example.com", "")
	if err != nil {
		t.Fatal(err)
	}

	if ev.Kind != 31402 {
		t.Errorf("kind = %d, want 31402", ev.Kind)
	}

	assertTag(t, ev, "d", "aperture-api.example.com")
	assertTag(t, ev, "url", "https://api.example.com")
	assertTag(t, ev, "pmi", "bitcoin-lightning-bolt11")
	assertPriceTag(t, ev, "my-api", "100", "sats")
}

func TestBuildEvent_WithCapabilities(t *testing.T) {
	cfg := &config.ApertureConfig{
		Services: []config.Service{
			{
				Name:         "my-api",
				PathRegexp:   "/v1/.*",
				Price:        100,
				Capabilities: []string{"read", "write"},
			},
		},
	}
	sk := nostr.GeneratePrivateKey()
	ev, err := BuildEvent(sk, cfg, "https://api.example.com", "")
	if err != nil {
		t.Fatal(err)
	}

	assertPriceTag(t, ev, "read", "100", "sats")
	assertPriceTag(t, ev, "write", "100", "sats")

	var content struct {
		Capabilities []struct {
			Name     string `json:"name"`
			Endpoint string `json:"endpoint"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal([]byte(ev.Content), &content); err != nil {
		t.Fatalf("content JSON: %v", err)
	}
	if len(content.Capabilities) != 2 {
		t.Fatalf("expected 2 capabilities in content, got %d", len(content.Capabilities))
	}
}

func TestBuildEvent_DynamicPricing(t *testing.T) {
	cfg := &config.ApertureConfig{
		Services: []config.Service{
			{Name: "dynamic-api", PathRegexp: "/v1/.*", DynamicPrice: true},
		},
	}
	sk := nostr.GeneratePrivateKey()
	ev, err := BuildEvent(sk, cfg, "https://api.example.com", "")
	if err != nil {
		t.Fatal(err)
	}

	assertPriceTag(t, ev, "dynamic-api", "0", "sats")
}

func TestBuildEvent_MultipleServices(t *testing.T) {
	cfg := &config.ApertureConfig{
		Services: []config.Service{
			{Name: "read-api", PathRegexp: "/v1/read", Price: 50},
			{Name: "write-api", PathRegexp: "/v1/write", Price: 200},
		},
	}
	sk := nostr.GeneratePrivateKey()
	ev, err := BuildEvent(sk, cfg, "https://api.example.com", "")
	if err != nil {
		t.Fatal(err)
	}

	assertPriceTag(t, ev, "read-api", "50", "sats")
	assertPriceTag(t, ev, "write-api", "200", "sats")
}

func TestBuildEvent_WithPicture(t *testing.T) {
	cfg := &config.ApertureConfig{
		Services: []config.Service{
			{Name: "api", PathRegexp: "/v1/.*", Price: 10},
		},
	}
	sk := nostr.GeneratePrivateKey()
	ev, err := BuildEvent(sk, cfg, "https://api.example.com", "https://example.com/icon.png")
	if err != nil {
		t.Fatal(err)
	}

	assertTag(t, ev, "picture", "https://example.com/icon.png")
}

func TestBuildEvent_SignatureValid(t *testing.T) {
	cfg := &config.ApertureConfig{
		Services: []config.Service{
			{Name: "api", PathRegexp: "/", Price: 1},
		},
	}
	sk := nostr.GeneratePrivateKey()
	ev, err := BuildEvent(sk, cfg, "https://api.example.com", "")
	if err != nil {
		t.Fatal(err)
	}

	ok, err := ev.CheckSignature()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("signature verification failed")
	}
}

func assertTag(t *testing.T, ev *nostr.Event, key, value string) {
	t.Helper()
	for _, tag := range ev.Tags {
		if len(tag) >= 2 && tag[0] == key && tag[1] == value {
			return
		}
	}
	t.Errorf("missing tag [%q, %q]", key, value)
}

func assertPriceTag(t *testing.T, ev *nostr.Event, capability, amount, unit string) {
	t.Helper()
	for _, tag := range ev.Tags {
		if len(tag) >= 4 && tag[0] == "price" && tag[1] == capability && tag[2] == amount && tag[3] == unit {
			return
		}
	}
	t.Errorf("missing price tag [price, %q, %q, %q]", capability, amount, unit)
}
