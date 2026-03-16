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
		select {
		case <-ctx.Done():
			return results
		default:
		}
		r := publishToRelay(ctx, ev, url)
		results = append(results, r)
	}

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
