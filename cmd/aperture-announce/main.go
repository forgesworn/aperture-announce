package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/TheCryptoDonkey/aperture-announce/internal/announce"
	"github.com/TheCryptoDonkey/aperture-announce/internal/config"
	"github.com/TheCryptoDonkey/aperture-announce/internal/key"
	"github.com/TheCryptoDonkey/aperture-announce/internal/validate"
)

func main() {
	configPath := flag.String("config", "", "Path to Aperture YAML config (required)")
	relays := flag.String("relays", "", "Comma-separated relay URLs (required unless --dry-run)")
	publicURL := flag.String("public-url", "", "Public URL for the service (required)")
	announceKey := flag.String("announce-key", "", "Nostr secret key (64-char hex; auto-generated if omitted)")
	interval := flag.Duration("interval", 0, "Re-publish interval (e.g. 6h); default: one-shot")
	picture := flag.String("picture", "", "Optional service icon URL")
	dryRun := flag.Bool("dry-run", false, "Print event JSON without publishing")
	verbose := flag.Bool("verbose", false, "Verbose logging")
	flag.Parse()

	// Env var fallbacks
	if *configPath == "" {
		*configPath = os.Getenv("APERTURE_CONFIG")
	}
	if *relays == "" {
		*relays = os.Getenv("ANNOUNCE_RELAYS")
	}
	if *publicURL == "" {
		*publicURL = os.Getenv("PUBLIC_URL")
	}
	if *announceKey == "" {
		*announceKey = os.Getenv("ANNOUNCE_KEY")
	}

	// Validate required flags
	if *configPath == "" {
		fatal("--config is required")
	}
	if *publicURL == "" {
		fatal("--public-url is required — this is the URL agents will use to reach your service")
	}
	if !*dryRun && *relays == "" {
		fatal("--relays is required (or use --dry-run to preview)")
	}

	// Validate public URL
	if err := validate.ValidatePublicURL(*publicURL); err != nil {
		fatal("invalid --public-url: %v", err)
	}

	// Parse config
	data, err := os.ReadFile(*configPath)
	if err != nil {
		fatal("read config: %v", err)
	}
	cfg, err := config.Parse(data)
	if err != nil {
		fatal("parse config: %v", err)
	}

	if *verbose {
		fmt.Fprintf(os.Stderr, "Parsed %d service(s) from %s\n", len(cfg.Services), *configPath)
	}

	// Resolve key
	home, _ := os.UserHomeDir()
	keyDir := filepath.Join(home, ".aperture-announce")
	sk, err := key.Resolve(*announceKey, keyDir)
	if err != nil {
		fatal("key: %v", err)
	}

	// Build and publish (loop or one-shot)
	run := func() {
		ev, err := announce.BuildEvent(sk, cfg, *publicURL, *picture)
		if err != nil {
			fatal("build event: %v", err)
		}

		if *dryRun {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(ev)
			return
		}

		relayList := strings.Split(*relays, ",")
		results := announce.Publish(context.Background(), ev, relayList)

		successCount := 0
		for _, r := range results {
			if r.Success {
				successCount++
				if *verbose {
					fmt.Fprintf(os.Stderr, "Published to %s\n", r.Relay)
				}
			} else {
				fmt.Fprintf(os.Stderr, "Warning: %s — %v\n", r.Relay, r.Error)
			}
		}

		if successCount == 0 {
			fatal("failed to publish to any relay")
		}

		fmt.Fprintf(os.Stderr, "Announced %s to %d/%d relay(s) (event %s)\n",
			*publicURL, successCount, len(relayList), ev.ID[:12]+"...")
	}

	run()

	if *interval > 0 {
		if *dryRun {
			fatal("--interval and --dry-run cannot be combined")
		}
		ticker := time.NewTicker(*interval)
		defer ticker.Stop()
		for range ticker.C {
			run()
		}
	}
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "aperture-announce: "+format+"\n", args...)
	os.Exit(1)
}
