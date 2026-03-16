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

// publishToRelay connects to a single relay and publishes the event.
//
// Note: go-nostr v0.52.3 has a known data race between the background
// goroutine spawned by RelayConnect (which writes to relay fields during
// init at relay.go:175) and Close (which reads those fields at relay.go:576).
// This is an upstream issue — our code is correct but `go test -race` will
// report a false positive for tests that exercise this path.
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
