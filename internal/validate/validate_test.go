package validate

import "testing"

func TestIsPrivateHost(t *testing.T) {
	private := []string{
		"localhost",
		"localhost.",
		"foo.localhost",
		"127.0.0.1",
		"127.255.0.1",
		"0.0.0.0",
		"10.0.0.1",
		"10.255.255.255",
		"172.16.0.1",
		"172.31.255.255",
		"192.168.0.1",
		"192.168.255.255",
		"169.254.1.1",
		"::1",
		"[::1]",
		"::",
		"fe80::1",
		"fc00::1",
		"fd00::1",
	}
	for _, host := range private {
		if !IsPrivateHost(host) {
			t.Errorf("expected %q to be private", host)
		}
	}

	public := []string{
		"8.8.8.8",
		"1.1.1.1",
		"relay.example.com",
		"api.example.com",
		"172.32.0.1",
		"172.15.0.1",
	}
	for _, host := range public {
		if IsPrivateHost(host) {
			t.Errorf("expected %q to be public", host)
		}
	}
}

func TestValidatePublicURL(t *testing.T) {
	valid := []string{
		"https://api.example.com",
		"https://satgate.trotters.dev",
		"http://api.example.com:8080",
	}
	for _, u := range valid {
		if err := ValidatePublicURL(u); err != nil {
			t.Errorf("expected %q to be valid, got: %v", u, err)
		}
	}

	invalid := []string{
		"http://localhost:3000",
		"http://127.0.0.1:8080",
		"http://192.168.1.1/api",
	}
	for _, u := range invalid {
		if err := ValidatePublicURL(u); err == nil {
			t.Errorf("expected %q to be rejected as private", u)
		}
	}

	if err := ValidatePublicURL("ftp://example.com"); err == nil {
		t.Error("expected ftp:// to be rejected")
	}
}

func TestIsPrivateHost_ReservedRanges(t *testing.T) {
	reserved := []string{
		"100.64.0.1",    // CGNAT
		"100.127.255.1", // CGNAT upper
		"198.18.0.1",    // Benchmarking
		"198.19.0.1",    // Benchmarking
		"192.0.2.1",     // TEST-NET-1
		"198.51.100.1",  // TEST-NET-2
		"203.0.113.1",   // TEST-NET-3
		"192.0.0.1",     // IETF protocol assignments
		"192.88.99.1",   // 6to4 relay
		"240.0.0.1",     // Future use
		"255.255.255.254",
	}
	for _, host := range reserved {
		if !IsPrivateHost(host) {
			t.Errorf("expected %q to be private (reserved)", host)
		}
	}
}

func TestIsPrivateHost_IPv6MappedIPv4(t *testing.T) {
	mapped := []string{
		"::ffff:127.0.0.1",
		"::ffff:10.0.0.1",
		"::ffff:192.168.1.1",
	}
	for _, host := range mapped {
		if !IsPrivateHost(host) {
			t.Errorf("expected %q to be private (IPv6 mapped IPv4)", host)
		}
	}
}

func TestValidateRelayURL(t *testing.T) {
	valid := []string{
		"wss://relay.damus.io",
		"ws://localhost:7777",
		"wss://nos.lol",
	}
	for _, u := range valid {
		if err := ValidateRelayURL(u); err != nil {
			t.Errorf("expected %q to be valid relay URL, got: %v", u, err)
		}
	}

	invalid := []string{
		"http://relay.damus.io",
		"https://relay.damus.io",
		"ftp://relay.damus.io",
		"not-a-url",
		"wss://",
	}
	for _, u := range invalid {
		if err := ValidateRelayURL(u); err == nil {
			t.Errorf("expected %q to be rejected as invalid relay URL", u)
		}
	}
}
