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
}

type eventContent struct {
	Capabilities []capability `json:"capabilities,omitempty"`
}

// BuildEvent creates and signs a kind 31402 Nostr event from an Aperture config.
func BuildEvent(secretKey string, cfg *config.ApertureConfig, publicURL string, picture string) (*nostr.Event, error) {
	u, err := url.Parse(publicURL)
	if err != nil {
		return nil, fmt.Errorf("invalid public URL: %w", err)
	}

	identifier := "aperture-" + u.Hostname()

	// Build service descriptions for the about tag
	var serviceNames []string
	for _, s := range cfg.Services {
		serviceNames = append(serviceNames, s.Name)
	}
	about := "L402-gated API via Aperture"
	if len(serviceNames) > 0 {
		about += " — " + strings.Join(serviceNames, ", ")
	}

	tags := nostr.Tags{
		{"d", identifier},
		{"name", about},
		{"url", publicURL},
		{"about", about},
		{"pmi", "bitcoin-lightning-bolt11"},
		{"t", "l402"},
		{"t", "api"},
		{"t", "aperture"},
	}

	if picture != "" {
		tags = append(tags, nostr.Tag{"picture", picture})
	}

	var caps []capability

	for _, svc := range cfg.Services {
		if len(svc.Capabilities) > 0 {
			for _, capName := range svc.Capabilities {
				tags = append(tags, nostr.Tag{
					"price", capName, strconv.FormatInt(svc.Price, 10), "sats",
				})
				caps = append(caps, capability{
					Name:        capName,
					Description: fmt.Sprintf("%s via %s", capName, svc.Name),
					Endpoint:    svc.PathRegexp,
				})
			}
		} else {
			tags = append(tags, nostr.Tag{
				"price", svc.Name, strconv.FormatInt(svc.Price, 10), "sats",
			})
			caps = append(caps, capability{
				Name:        svc.Name,
				Description: fmt.Sprintf("Access %s", svc.Name),
				Endpoint:    svc.PathRegexp,
			})
		}
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
