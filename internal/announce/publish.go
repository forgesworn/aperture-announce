package announce

import (
	"context"
	"fmt"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

// PublishResult records the outcome per relay.
type PublishResult struct {
	Relay   string
	Success bool
	Error   error
}

// Publish sends a signed event to the given relay URLs.
// Returns results per relay. Continues on individual failures.
func Publish(ctx context.Context, ev *nostr.Event, relays []string) []PublishResult {
	results := make([]PublishResult, 0, len(relays))

	for _, url := range relays {
		rCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		relay, err := nostr.RelayConnect(rCtx, url)
		if err != nil {
			cancel()
			results = append(results, PublishResult{
				Relay: url, Success: false,
				Error: fmt.Errorf("connect: %w", err),
			})
			continue
		}

		if err := relay.Publish(rCtx, *ev); err != nil {
			cancel()
			relay.Close()
			results = append(results, PublishResult{
				Relay: url, Success: false,
				Error: fmt.Errorf("publish: %w", err),
			})
			continue
		}

		cancel()
		relay.Close()
		results = append(results, PublishResult{Relay: url, Success: true})
	}

	return results
}
