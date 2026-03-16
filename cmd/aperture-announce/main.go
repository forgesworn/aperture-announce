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
	topics := flag.String("topics", "", "Comma-separated custom topic tags (appended to defaults)")
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
	if *topics == "" {
		*topics = os.Getenv("ANNOUNCE_TOPICS")
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

	// Validate picture URL if provided
	if *picture != "" {
		if err := validate.ValidatePublicURL(*picture); err != nil {
			fatal("invalid --picture: %v", err)
		}
	}

	// Parse config (limit to 1 MB to prevent memory exhaustion from huge files)
	info, err := os.Stat(*configPath)
	if err != nil {
		fatal("read config: %v", err)
	}
	const maxConfigSize = 1 << 20 // 1 MB
	if info.Size() > maxConfigSize {
		fatal("config file too large (%d bytes, max %d)", info.Size(), maxConfigSize)
	}
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

	// Warn about dynamic pricing and unrecognised auth values.
	for _, svc := range cfg.Services {
		if svc.DynamicPrice {
			if svc.Price > 0 {
				fmt.Fprintf(os.Stderr, "Warning: service %q uses dynamic pricing — announced price of %d sats is the static fallback; actual price is determined at request time\n", svc.Name, svc.Price)
			} else {
				fmt.Fprintf(os.Stderr, "Warning: service %q uses dynamic pricing with no static fallback — price tag omitted from announcement\n", svc.Name)
			}
		}
		if svc.Auth != "" && !isRecognisedAuth(svc.Auth) {
			fmt.Fprintf(os.Stderr, "Warning: service %q has unrecognised auth value %q — treating as \"on\" (payment required)\n", svc.Name, svc.Auth)
		}
	}

	// Resolve key
	home, err := os.UserHomeDir()
	if err != nil && *announceKey == "" {
		fatal("cannot determine home directory for key storage (set --announce-key or HOME): %v", err)
	}
	keyDir := filepath.Join(home, ".aperture-announce")
	sk, err := key.Resolve(*announceKey, keyDir)
	if err != nil {
		fatal("key: %v", err)
	}

	var topicList []string
	if *topics != "" {
		for _, t := range strings.Split(*topics, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				topicList = append(topicList, t)
			}
		}
	}

	// Build and publish (loop or one-shot)
	run := func() {
		ev, err := announce.BuildEvent(sk, cfg, announce.BuildOptions{
			PublicURL: *publicURL,
			Picture:   *picture,
			Topics:    topicList,
		})
		if err != nil {
			fatal("build event: %v", err)
		}

		if *dryRun {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(ev)
			return
		}

		var relayList []string
		for _, r := range strings.Split(*relays, ",") {
			r = strings.TrimSpace(r)
			if r == "" {
				continue
			}
			if err := validate.ValidateRelayURL(r); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: skipping invalid relay %q: %v\n", r, err)
				continue
			}
			relayList = append(relayList, r)
		}
		if len(relayList) == 0 {
			fatal("no valid relay URLs provided")
		}
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

func isRecognisedAuth(auth string) bool {
	switch strings.ToLower(strings.TrimSpace(auth)) {
	case "", "on", "true", "off", "false":
		return true
	}
	lower := strings.ToLower(strings.TrimSpace(auth))
	if strings.HasPrefix(lower, "freebie ") {
		rest := strings.TrimSpace(lower[len("freebie "):])
		for _, c := range rest {
			if c < '0' || c > '9' {
				return false
			}
		}
		return rest != ""
	}
	return false
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "aperture-announce: "+format+"\n", args...)
	os.Exit(1)
}
