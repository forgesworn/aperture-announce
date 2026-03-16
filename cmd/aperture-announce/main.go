package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/TheCryptoDonkey/aperture-announce/internal/announce"
	"github.com/TheCryptoDonkey/aperture-announce/internal/config"
	"github.com/TheCryptoDonkey/aperture-announce/internal/key"
	"github.com/TheCryptoDonkey/aperture-announce/internal/validate"
)

// flags holds all CLI flag values after parsing and env var fallback resolution.
type flags struct {
	configPath  string
	relays      string
	publicURLs  string
	announceKey string
	interval    time.Duration
	picture     string
	dryRun      bool
	verbose     bool
	topics      string
}

// resolveFlags parses CLI flags, applies env var fallbacks, and validates
// required combinations. It calls fatal() and exits on any validation error.
func resolveFlags() flags {
	configPath := flag.String("config", "", "Path to Aperture YAML config (required)")
	relays := flag.String("relays", "", "Comma-separated relay URLs (required unless --dry-run)")
	publicURLs := flag.String("public-urls", "", "Comma-separated public URLs for the service (required; up to 10)")
	publicURL := flag.String("public-url", "", "Public URL for the service (alias for --public-urls; backwards-compatible)")
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
	if *publicURLs == "" {
		*publicURLs = os.Getenv("PUBLIC_URLS")
	}
	// --public-url / PUBLIC_URL are backwards-compatible aliases for a single URL.
	if *publicURL == "" {
		*publicURL = os.Getenv("PUBLIC_URL")
	}
	if *publicURLs == "" && *publicURL != "" {
		*publicURLs = *publicURL
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
	if *publicURLs == "" {
		fatal("--public-urls is required — this is the URL agents will use to reach your service")
	}
	if !*dryRun && *relays == "" {
		fatal("--relays is required (or use --dry-run to preview)")
	}
	if *dryRun && *interval > 0 {
		fatal("--interval and --dry-run cannot be combined")
	}

	return flags{
		configPath:  *configPath,
		relays:      *relays,
		publicURLs:  *publicURLs,
		announceKey: *announceKey,
		interval:    *interval,
		picture:     *picture,
		dryRun:      *dryRun,
		verbose:     *verbose,
		topics:      *topics,
	}
}

// parsePublicURLs splits a comma-separated list of public URLs, validates each
// one, and returns the resulting slice. Returns an error if any URL is invalid
// or if the count exceeds the permitted maximum of 10.
func parsePublicURLs(raw string) ([]string, error) {
	urlList := make([]string, 0, strings.Count(raw, ",")+1)
	for _, u := range strings.Split(raw, ",") {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if err := validate.ValidatePublicURL(u); err != nil {
			return nil, fmt.Errorf("invalid public URL %q: %w", u, err)
		}
		urlList = append(urlList, u)
	}
	if len(urlList) == 0 {
		return nil, fmt.Errorf("--public-urls is required — this is the URL agents will use to reach your service")
	}
	if len(urlList) > 10 {
		return nil, fmt.Errorf("--public-urls: at most 10 URLs are permitted, got %d", len(urlList))
	}
	return urlList, nil
}

// parseRelayURLs splits a comma-separated list of relay URLs, validates each
// one, and returns the valid entries. Invalid URLs are skipped with a warning
// printed to stderr. Private/loopback relay addresses also produce a warning
// but are still included.
func parseRelayURLs(raw string) []string {
	relayList := make([]string, 0, strings.Count(raw, ",")+1)
	for _, r := range strings.Split(raw, ",") {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if err := validate.ValidateRelayURL(r); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipping invalid relay %q: %v\n", r, err)
			continue
		}
		if u, parseErr := url.Parse(r); parseErr == nil && validate.IsPrivateHost(u.Hostname()) {
			fmt.Fprintf(os.Stderr, "Warning: relay %q points to a private/loopback address\n", r)
		}
		relayList = append(relayList, r)
	}
	return relayList
}

// normaliseServiceWarnings prints warnings for dynamic pricing configurations
// and unrecognised auth values, mutating cfg to normalise any unknown auth strings.
func normaliseServiceWarnings(cfg *config.ApertureConfig) {
	for i, svc := range cfg.Services {
		if svc.DynamicPrice {
			if svc.Price > 0 {
				fmt.Fprintf(os.Stderr, "Warning: service %q uses dynamic pricing — announced price of %d sats is the static fallback; actual price is determined at request time\n", svc.Name, svc.Price)
			} else {
				fmt.Fprintf(os.Stderr, "Warning: service %q uses dynamic pricing with no static fallback — price tag omitted from announcement\n", svc.Name)
			}
		}
		if svc.Auth != "" && !isRecognisedAuth(svc.Auth) {
			fmt.Fprintf(os.Stderr, "Warning: service %q has unrecognised auth value %q — treating as \"on\" (payment required)\n", svc.Name, svc.Auth)
			cfg.Services[i].Auth = ""
		}
	}
}

// publishOnce builds a kind 31402 event, publishes it to all relays, and
// prints a summary to stderr. It calls fatal() and exits if publishing fails
// on every relay.
func publishOnce(ctx context.Context, sk string, cfg *config.ApertureConfig, opts announce.BuildOptions, relayList []string, verbose bool) {
	ev, err := announce.BuildEvent(sk, cfg, opts)
	if err != nil {
		fatal("build event: %v", err)
	}

	results := announce.Publish(ctx, ev, relayList)

	successCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
			if verbose {
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
		strings.Join(opts.PublicUrls, ", "), successCount, len(relayList), ev.ID[:12]+"...")
}

func main() {
	f := resolveFlags()

	urlList, err := parsePublicURLs(f.publicURLs)
	if err != nil {
		fatal("%v", err)
	}

	// Validate picture URL if provided.
	if f.picture != "" {
		if err := validate.ValidatePublicURL(f.picture); err != nil {
			fatal("invalid --picture: %v", err)
		}
	}

	// Parse config (limit to 1 MB to prevent memory exhaustion from huge files).
	const maxConfigSize = 1 << 20 // 1 MB
	fi, err := os.Stat(f.configPath)
	if err != nil {
		fatal("stat config: %v", err)
	}
	if fi.Size() > maxConfigSize {
		fatal("config file too large (%d bytes, max %d)", fi.Size(), maxConfigSize)
	}
	data, err := os.ReadFile(f.configPath)
	if err != nil {
		fatal("read config: %v", err)
	}
	if len(data) > maxConfigSize { // belt-and-braces guard
		fatal("config file too large (%d bytes, max %d)", len(data), maxConfigSize)
	}
	cfg, err := config.Parse(data)
	if err != nil {
		fatal("parse config: %v", err)
	}

	if f.verbose {
		fmt.Fprintf(os.Stderr, "Parsed %d service(s) from %s\n", len(cfg.Services), f.configPath)
	}

	normaliseServiceWarnings(cfg)

	// Resolve key.
	home, err := os.UserHomeDir()
	if err != nil && f.announceKey == "" {
		fatal("cannot determine home directory for key storage (set --announce-key or HOME): %v", err)
	}
	keyDir := filepath.Join(home, ".aperture-announce")
	sk, err := key.Resolve(f.announceKey, keyDir)
	if err != nil {
		fatal("key: %v", err)
	}

	var topicList []string
	if f.topics != "" {
		topicList = make([]string, 0, strings.Count(f.topics, ",")+1)
		for _, t := range strings.Split(f.topics, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				topicList = append(topicList, t)
			}
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	opts := announce.BuildOptions{
		PublicUrls: urlList,
		Picture:    f.picture,
		Topics:     topicList,
	}

	// Build and publish (loop or one-shot).
	run := func() {
		if f.dryRun {
			ev, err := announce.BuildEvent(sk, cfg, opts)
			if err != nil {
				fatal("build event: %v", err)
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(ev); err != nil {
				fatal("encode event: %v", err)
			}
			return
		}

		relayList := parseRelayURLs(f.relays)
		if len(relayList) == 0 {
			fatal("no valid relay URLs provided")
		}
		publishOnce(ctx, sk, cfg, opts, relayList, f.verbose)
	}

	run()

	if f.interval > 0 {
		ticker := time.NewTicker(f.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run()
			}
		}
	}
}

func isRecognisedAuth(auth string) bool {
	lower := strings.ToLower(strings.TrimSpace(auth))
	switch lower {
	case "", "on", "true", "off", "false":
		return true
	}
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
