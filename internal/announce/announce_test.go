package announce

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/forgesworn/aperture-announce/internal/config"
	"github.com/nbd-wtf/go-nostr"
	"github.com/stretchr/testify/require"
)

func TestBuildEvent_SingleService(t *testing.T) {
	cfg := &config.ApertureConfig{
		Services: []config.Service{
			{Name: "my-api", PathRegexp: "/v1/.*", Price: 100},
		},
	}
	sk := nostr.GeneratePrivateKey()
	ev, err := BuildEvent(sk, cfg, BuildOptions{PublicUrls: []string{"https://api.example.com"}})
	require.NoError(t, err)

	require.Equal(t, 31402, ev.Kind)

	assertTag(t, ev, "d", "aperture-api.example.com")
	assertTag(t, ev, "url", "https://api.example.com")
	assertTagSlice(t, ev, "pmi", []string{"pmi", "l402", "lightning"})
	assertPriceTag(t, ev, "my-api", "100")
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
	ev, err := BuildEvent(sk, cfg, BuildOptions{PublicUrls: []string{"https://api.example.com"}})
	require.NoError(t, err)

	assertPriceTag(t, ev, "read", "100")
	assertPriceTag(t, ev, "write", "100")

	var content struct {
		Capabilities []struct {
			Name     string `json:"name"`
			Endpoint string `json:"endpoint"`
		} `json:"capabilities"`
	}
	err = json.Unmarshal([]byte(ev.Content), &content)
	require.NoError(t, err)
	require.Len(t, content.Capabilities, 2)
}

func TestBuildEvent_DynamicPricingNoPriceTag(t *testing.T) {
	cfg := &config.ApertureConfig{
		Services: []config.Service{
			{Name: "dynamic-api", PathRegexp: "/v1/.*", DynamicPrice: true, Price: 0},
		},
	}
	sk := nostr.GeneratePrivateKey()
	ev, err := BuildEvent(sk, cfg, BuildOptions{PublicUrls: []string{"https://api.example.com"}})
	require.NoError(t, err)

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
	err = json.Unmarshal([]byte(ev.Content), &content)
	require.NoError(t, err)
	require.Len(t, content.Capabilities, 1)
	require.Equal(t, "dynamic", content.Capabilities[0].Pricing)
}

func TestBuildEvent_DynamicPricingWithFallback(t *testing.T) {
	cfg := &config.ApertureConfig{
		Services: []config.Service{
			{Name: "api", PathRegexp: "/v1/.*", DynamicPrice: true, Price: 500},
		},
	}
	sk := nostr.GeneratePrivateKey()
	ev, err := BuildEvent(sk, cfg, BuildOptions{PublicUrls: []string{"https://api.example.com"}})
	require.NoError(t, err)

	// Should still have a price tag (static fallback)
	assertPriceTag(t, ev, "api", "500")

	// Should have dynamic-pricing topic
	assertTag(t, ev, "t", "dynamic-pricing")

	// Content should have pricing: "dynamic"
	var content eventContent
	err = json.Unmarshal([]byte(ev.Content), &content)
	require.NoError(t, err)
	require.Equal(t, "dynamic", content.Capabilities[0].Pricing)
}

func TestBuildEvent_EndpointsCleaned(t *testing.T) {
	cfg := &config.ApertureConfig{
		Services: []config.Service{
			{Name: "api", PathRegexp: "^/v1/loop/.*$", Price: 100},
			{Name: "exact", PathRegexp: "^/v1/status$", Price: 50},
		},
	}
	sk := nostr.GeneratePrivateKey()
	ev, err := BuildEvent(sk, cfg, BuildOptions{PublicUrls: []string{"https://api.example.com"}})
	require.NoError(t, err)

	var content eventContent
	err = json.Unmarshal([]byte(ev.Content), &content)
	require.NoError(t, err)
	// Trailing-slash result is ambiguous — omitted.
	require.Equal(t, "", content.Capabilities[0].Endpoint)
	// Exact path is preserved.
	require.Equal(t, "/v1/status", content.Capabilities[1].Endpoint)
}

func TestBuildEvent_MultipleServices(t *testing.T) {
	cfg := &config.ApertureConfig{
		Services: []config.Service{
			{Name: "read-api", PathRegexp: "/v1/read", Price: 50},
			{Name: "write-api", PathRegexp: "/v1/write", Price: 200},
		},
	}
	sk := nostr.GeneratePrivateKey()
	ev, err := BuildEvent(sk, cfg, BuildOptions{PublicUrls: []string{"https://api.example.com"}})
	require.NoError(t, err)

	assertPriceTag(t, ev, "read-api", "50")
	assertPriceTag(t, ev, "write-api", "200")
}

func TestBuildEvent_WithPicture(t *testing.T) {
	cfg := &config.ApertureConfig{
		Services: []config.Service{
			{Name: "api", PathRegexp: "/v1/.*", Price: 10},
		},
	}
	sk := nostr.GeneratePrivateKey()
	ev, err := BuildEvent(sk, cfg, BuildOptions{PublicUrls: []string{"https://api.example.com"}, Picture: "https://example.com/icon.png"})
	require.NoError(t, err)

	assertTag(t, ev, "picture", "https://example.com/icon.png")
}

func TestBuildEvent_SignatureValid(t *testing.T) {
	cfg := &config.ApertureConfig{
		Services: []config.Service{
			{Name: "api", PathRegexp: "/", Price: 1},
		},
	}
	sk := nostr.GeneratePrivateKey()
	ev, err := BuildEvent(sk, cfg, BuildOptions{PublicUrls: []string{"https://api.example.com"}})
	require.NoError(t, err)

	ok, err := ev.CheckSignature()
	require.NoError(t, err)
	require.True(t, ok)
}

func TestCleanEndpoint(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"^/v1/loop/.*$", ""},
		{"/v1/pool/.*", ""},
		{"^/looprpc.SwapServer/LoopOutTerms.*$", "/looprpc.SwapServer/LoopOutTerms"},
		{"^/v1/(quote|swap)/.*$", ""},
		{"^/.*$", ""},
		{".*", ""},
		{"", ""},
		{"/v1/status", "/v1/status"},
		{"^/v1/status$", "/v1/status"},
		{"/v1/items?", "/v1/item"},
		{"/v1/data{2,5}", "/v1/data"},
		{"/v1/users[0-9]+", "/v1/users"},
		{"/v1/exact", "/v1/exact"},
		{"/api/v2/(users|admin)", ""},
		{"^/v1/loop/specific(opt)?$", "/v1/loop/specific"},
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
	ev, err := BuildEvent(sk, cfg, BuildOptions{PublicUrls: []string{"https://api.example.com"}})
	require.NoError(t, err)

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
	ev, err := BuildEvent(sk, cfg, BuildOptions{PublicUrls: []string{"https://api.example.com"}})
	require.NoError(t, err)

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
	ev, err := BuildEvent(sk, cfg, BuildOptions{PublicUrls: []string{"https://api.example.com"}})
	require.NoError(t, err)

	var content eventContent
	err = json.Unmarshal([]byte(ev.Content), &content)
	require.NoError(t, err)
	require.Len(t, content.Capabilities, 3)

	// "off" → "none"
	require.Equal(t, "none", content.Capabilities[0].Auth)
	// "on" → omitted (empty)
	require.Equal(t, "", content.Capabilities[1].Auth)
	// "freebie 3" → "freebie 3"
	require.Equal(t, "freebie 3", content.Capabilities[2].Auth)
}

func TestBuildEvent_TimeoutInContent(t *testing.T) {
	cfg := &config.ApertureConfig{
		Services: []config.Service{
			{Name: "timed-api", PathRegexp: "/v1/timed", Price: 100, Timeout: 3600},
			{Name: "untimed-api", PathRegexp: "/v1/untimed", Price: 50},
		},
	}
	sk := nostr.GeneratePrivateKey()
	ev, err := BuildEvent(sk, cfg, BuildOptions{PublicUrls: []string{"https://api.example.com"}})
	require.NoError(t, err)

	var content eventContent
	err = json.Unmarshal([]byte(ev.Content), &content)
	require.NoError(t, err)

	require.Equal(t, int64(3600), content.Capabilities[0].Timeout)
	require.Equal(t, int64(0), content.Capabilities[1].Timeout)
}

func TestBuildEvent_CustomTopics(t *testing.T) {
	cfg := &config.ApertureConfig{
		Services: []config.Service{
			{Name: "api", PathRegexp: "/v1/.*", Price: 100},
		},
	}
	sk := nostr.GeneratePrivateKey()
	ev, err := BuildEvent(sk, cfg, BuildOptions{
		PublicUrls: []string{"https://api.example.com"},
		Topics:     []string{"ai", "inference", "l402"},
	})
	require.NoError(t, err)

	assertTag(t, ev, "t", "l402")
	assertTag(t, ev, "t", "api")
	assertTag(t, ev, "t", "aperture")
	assertTag(t, ev, "t", "ai")
	assertTag(t, ev, "t", "inference")

	// l402 deduped — count total t tags
	tCount := 0
	for _, tag := range ev.Tags {
		if len(tag) >= 1 && tag[0] == "t" {
			tCount++
		}
	}
	require.Equal(t, 5, tCount)
}

func TestBuildEvent_TopicCap(t *testing.T) {
	cfg := &config.ApertureConfig{
		Services: []config.Service{
			{Name: "api", PathRegexp: "/v1/.*", Price: 100},
		},
	}
	topics := make([]string, 60)
	for i := range topics {
		topics[i] = fmt.Sprintf("topic-%d", i)
	}
	sk := nostr.GeneratePrivateKey()
	ev, err := BuildEvent(sk, cfg, BuildOptions{
		PublicUrls: []string{"https://api.example.com"},
		Topics:     topics,
	})
	require.NoError(t, err)

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

func TestBuildEvent_MultipleURLs(t *testing.T) {
	cfg := &config.ApertureConfig{
		Services: []config.Service{
			{Name: "my-api", PathRegexp: "/v1/.*", Price: 100},
		},
	}
	sk := nostr.GeneratePrivateKey()
	urls := []string{
		"https://api.example.com",
		"https://onion.example.onion",
		"https://hns.example",
	}
	ev, err := BuildEvent(sk, cfg, BuildOptions{PublicUrls: urls})
	require.NoError(t, err)

	// All three urls must appear as separate url tags.
	for _, u := range urls {
		assertTag(t, ev, "url", u)
	}

	// Identifier is derived from the first URL.
	assertTag(t, ev, "d", "aperture-api.example.com")

	// Count url tags — must be exactly 3.
	urlCount := 0
	for _, tag := range ev.Tags {
		if len(tag) >= 1 && tag[0] == "url" {
			urlCount++
		}
	}
	require.Equal(t, 3, urlCount)
}

func TestBuildEvent_TooManyURLs(t *testing.T) {
	cfg := &config.ApertureConfig{
		Services: []config.Service{
			{Name: "my-api", PathRegexp: "/v1/.*", Price: 100},
		},
	}
	sk := nostr.GeneratePrivateKey()
	urls := make([]string, 11)
	for i := range urls {
		urls[i] = fmt.Sprintf("https://api%d.example.com", i)
	}
	_, err := BuildEvent(sk, cfg, BuildOptions{PublicUrls: urls})
	require.Error(t, err)
}

func TestBuildEvent_NoURLs(t *testing.T) {
	cfg := &config.ApertureConfig{
		Services: []config.Service{
			{Name: "my-api", PathRegexp: "/v1/.*", Price: 100},
		},
	}
	sk := nostr.GeneratePrivateKey()
	_, err := BuildEvent(sk, cfg, BuildOptions{PublicUrls: nil})
	require.Error(t, err)
}

func assertTag(t *testing.T, ev *nostr.Event, key, value string) {
	t.Helper()
	for _, tag := range ev.Tags {
		if len(tag) >= 2 && tag[0] == key && tag[1] == value {
			return
		}
	}
	require.Failf(t, "missing tag", "[%q, %q]", key, value)
}

func assertTagSlice(t *testing.T, ev *nostr.Event, key string, want []string) {
	t.Helper()
	for _, tag := range ev.Tags {
		if len(tag) >= 1 && tag[0] == key {
			if len(tag) == len(want) {
				match := true
				for i, v := range want {
					if tag[i] != v {
						match = false
						break
					}
				}
				if match {
					return
				}
			}
		}
	}
	require.Failf(t, "missing tag", "%v", want)
}

func assertPriceTag(t *testing.T, ev *nostr.Event, capability, amount string) {
	t.Helper()
	for _, tag := range ev.Tags {
		if len(tag) >= 4 && tag[0] == "price" && tag[1] == capability && tag[2] == amount && tag[3] == "sats" {
			return
		}
	}
	require.Failf(t, "missing price tag", "[price, %q, %q, sats]", capability, amount)
}
