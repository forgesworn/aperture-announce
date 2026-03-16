package announce

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/TheCryptoDonkey/aperture-announce/internal/config"
	"github.com/nbd-wtf/go-nostr"
)

// KindL402Announce is the Nostr event kind for L402 service announcements.
const KindL402Announce = 31402

type capability struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Endpoint    string `json:"endpoint,omitempty"`
	Pricing     string `json:"pricing,omitempty"`
	Auth        string `json:"auth,omitempty"`
	Timeout     int64  `json:"timeout,omitempty"`
}

type eventContent struct {
	Capabilities []capability `json:"capabilities,omitempty"`
}

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

	// Step 3: clean up
	if s == "/" || s == "" {
		return ""
	}

	return s
}

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

// BuildOptions holds optional parameters for event construction.
type BuildOptions struct {
	PublicURL string
	Picture   string
	Topics    []string
}

// BuildEvent creates and signs a kind 31402 Nostr event from an Aperture config.
func BuildEvent(secretKey string, cfg *config.ApertureConfig, opts BuildOptions) (*nostr.Event, error) {
	u, err := url.Parse(opts.PublicURL)
	if err != nil {
		return nil, fmt.Errorf("invalid public URL: %w", err)
	}

	identifier := "aperture-" + u.Hostname()

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
	}

	if opts.Picture != "" {
		tags = append(tags, nostr.Tag{"picture", opts.Picture})
	}

	var caps []capability
	hasDynamicPricing := false

	for _, svc := range cfg.Services {
		endpoint := CleanEndpoint(svc.PathRegexp)

		if svc.DynamicPrice {
			hasDynamicPricing = true
		}

		if len(svc.Capabilities) > 0 {
			for _, capName := range svc.Capabilities {
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
				cap.Auth = authForEvent(svc.Auth)
				cap.Timeout = svc.Timeout
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
			cap.Auth = authForEvent(svc.Auth)
			cap.Timeout = svc.Timeout
			caps = append(caps, cap)
		}
	}

	// Build topic list: defaults first, then custom, deduplicated, capped at 50
	const maxTopics = 50
	defaults := []string{"l402", "api", "aperture"}
	seen := make(map[string]bool)
	var allTopics []string
	for _, t := range defaults {
		if !seen[t] {
			seen[t] = true
			allTopics = append(allTopics, t)
		}
	}
	for _, t := range opts.Topics {
		t = strings.TrimSpace(t)
		if t != "" && !seen[t] {
			seen[t] = true
			allTopics = append(allTopics, t)
		}
	}
	if hasDynamicPricing && !seen["dynamic-pricing"] {
		allTopics = append(allTopics, "dynamic-pricing")
	}
	if len(allTopics) > maxTopics {
		allTopics = allTopics[:maxTopics]
	}
	for _, topic := range allTopics {
		tags = append(tags, nostr.Tag{"t", topic})
	}

	content := eventContent{Capabilities: caps}
	contentJSON, err := json.Marshal(content)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal content: %w", err)
	}

	ev := nostr.Event{
		CreatedAt: nostr.Timestamp(time.Now().Unix()),
		Kind:      KindL402Announce,
		Tags:      tags,
		Content:   string(contentJSON),
	}

	if err := ev.Sign(secretKey); err != nil {
		return nil, fmt.Errorf("failed to sign event: %w", err)
	}

	return &ev, nil
}
