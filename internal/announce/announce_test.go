package announce

import (
	"encoding/json"
	"fmt"
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
	ev, err := BuildEvent(sk, cfg, BuildOptions{PublicURL: "https://api.example.com"})
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
	ev, err := BuildEvent(sk, cfg, BuildOptions{PublicURL: "https://api.example.com"})
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

func TestBuildEvent_EndpointsCleaned(t *testing.T) {
	cfg := &config.ApertureConfig{
		Services: []config.Service{
			{Name: "api", PathRegexp: "^/v1/loop/.*$", Price: 100},
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
	if content.Capabilities[0].Endpoint != "/v1/loop/" {
		t.Errorf("endpoint = %q, want %q", content.Capabilities[0].Endpoint, "/v1/loop/")
	}
}

func TestBuildEvent_MultipleServices(t *testing.T) {
	cfg := &config.ApertureConfig{
		Services: []config.Service{
			{Name: "read-api", PathRegexp: "/v1/read", Price: 50},
			{Name: "write-api", PathRegexp: "/v1/write", Price: 200},
		},
	}
	sk := nostr.GeneratePrivateKey()
	ev, err := BuildEvent(sk, cfg, BuildOptions{PublicURL: "https://api.example.com"})
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
	ev, err := BuildEvent(sk, cfg, BuildOptions{PublicURL: "https://api.example.com", Picture: "https://example.com/icon.png"})
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
	ev, err := BuildEvent(sk, cfg, BuildOptions{PublicURL: "https://api.example.com"})
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
