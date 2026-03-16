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
