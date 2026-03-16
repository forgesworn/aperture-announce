package config

import (
	"fmt"
	"testing"
)

func TestParseSingleService(t *testing.T) {
	yml := `
services:
  - name: "my-api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/.*"
    price: 100
`
	cfg, err := Parse([]byte(yml))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(cfg.Services))
	}
	s := cfg.Services[0]
	if s.Name != "my-api" {
		t.Errorf("name = %q, want %q", s.Name, "my-api")
	}
	if s.Price != 100 {
		t.Errorf("price = %d, want 100", s.Price)
	}
	if s.PathRegexp != "/v1/.*" {
		t.Errorf("pathregexp = %q, want %q", s.PathRegexp, "/v1/.*")
	}
}

func TestParseMultipleServices(t *testing.T) {
	yml := `
services:
  - name: "read-api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/read"
    price: 50
  - name: "write-api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/write"
    price: 200
`
	cfg, err := Parse([]byte(yml))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(cfg.Services))
	}
}

func TestParseCapabilities(t *testing.T) {
	yml := `
services:
  - name: "my-api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/.*"
    price: 100
    capabilities: "read,write,admin"
`
	cfg, err := Parse([]byte(yml))
	if err != nil {
		t.Fatal(err)
	}
	caps := cfg.Services[0].Capabilities
	if len(caps) != 3 {
		t.Fatalf("expected 3 capabilities, got %d", len(caps))
	}
	if caps[0] != "read" || caps[1] != "write" || caps[2] != "admin" {
		t.Errorf("capabilities = %v", caps)
	}
}

func TestParseDynamicPricing(t *testing.T) {
	yml := `
services:
  - name: "dynamic-api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/.*"
    dynamicprice:
      enabled: true
      grpcaddress: "localhost:10010"
`
	cfg, err := Parse([]byte(yml))
	if err != nil {
		t.Fatal(err)
	}
	s := cfg.Services[0]
	if !s.DynamicPrice {
		t.Error("expected DynamicPrice to be true")
	}
	if s.Price != 0 {
		t.Errorf("expected Price 0 for dynamic, got %d", s.Price)
	}
}

func TestParseNoServices(t *testing.T) {
	yml := `
listenaddr: "localhost:8081"
`
	_, err := Parse([]byte(yml))
	if err == nil {
		t.Error("expected error for config with no services")
	}
}

func TestParseEmptyServices(t *testing.T) {
	yml := `
services: []
`
	_, err := Parse([]byte(yml))
	if err == nil {
		t.Error("expected error for empty services")
	}
}

func TestParseEmptyServiceName(t *testing.T) {
	yml := `
services:
  - name: ""
    hostregexp: "api.example.com"
    pathregexp: "/v1/.*"
    price: 100
`
	_, err := Parse([]byte(yml))
	if err == nil {
		t.Error("expected error for empty service name")
	}
}

func TestParseNegativePrice(t *testing.T) {
	yml := `
services:
  - name: "my-api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/.*"
    price: -100
`
	_, err := Parse([]byte(yml))
	if err == nil {
		t.Error("expected error for negative price")
	}
}

func TestParseZeroPriceDefaultsToOne(t *testing.T) {
	yml := `
services:
  - name: "my-api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/.*"
`
	cfg, err := Parse([]byte(yml))
	if err != nil {
		t.Fatal(err)
	}
	s := cfg.Services[0]
	if s.Price != DefaultServicePrice {
		t.Errorf("price = %d, want %d (Aperture default)", s.Price, DefaultServicePrice)
	}
}

func TestParseZeroPriceWithDynamicPricingStaysZero(t *testing.T) {
	yml := `
services:
  - name: "dynamic-api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/.*"
    dynamicprice:
      enabled: true
      grpcaddress: "localhost:10010"
`
	cfg, err := Parse([]byte(yml))
	if err != nil {
		t.Fatal(err)
	}
	s := cfg.Services[0]
	if s.Price != 0 {
		t.Errorf("price = %d, want 0 for dynamic pricing service", s.Price)
	}
}

func TestParseAuth(t *testing.T) {
	yml := `
services:
  - name: "api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/.*"
    price: 100
    auth: "freebie 5"
`
	cfg, err := Parse([]byte(yml))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Services[0].Auth != "freebie 5" {
		t.Errorf("auth = %q, want %q", cfg.Services[0].Auth, "freebie 5")
	}
}

func TestParseAuthOff(t *testing.T) {
	yml := `
services:
  - name: "api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/.*"
    price: 100
    auth: "off"
`
	cfg, err := Parse([]byte(yml))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Services[0].Auth != "off" {
		t.Errorf("auth = %q, want %q", cfg.Services[0].Auth, "off")
	}
}

func TestParseAuthDefault(t *testing.T) {
	yml := `
services:
  - name: "api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/.*"
    price: 100
`
	cfg, err := Parse([]byte(yml))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Services[0].Auth != "" {
		t.Errorf("auth = %q, want empty (default)", cfg.Services[0].Auth)
	}
}

func TestParseTimeout(t *testing.T) {
	yml := `
services:
  - name: "api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/.*"
    price: 100
    timeout: 3600
`
	cfg, err := Parse([]byte(yml))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Services[0].Timeout != 3600 {
		t.Errorf("timeout = %d, want 3600", cfg.Services[0].Timeout)
	}
}

func TestParseNegativeTimeout(t *testing.T) {
	yml := `
services:
  - name: "api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/.*"
    price: 100
    timeout: -1
`
	_, err := Parse([]byte(yml))
	if err == nil {
		t.Error("expected error for negative timeout")
	}
}

func TestParseTooManyServices(t *testing.T) {
	// Build YAML with 1001 services
	yml := "services:\n"
	for i := 0; i < 1001; i++ {
		yml += "  - name: \"svc-" + fmt.Sprintf("%d", i) + "\"\n"
		yml += "    hostregexp: \"api.example.com\"\n"
		yml += "    pathregexp: \"/v1/.*\"\n"
		yml += "    price: 1\n"
	}
	_, err := Parse([]byte(yml))
	if err == nil {
		t.Error("expected error for too many services")
	}
}

func TestParseAuthUnrecognisedWarns(t *testing.T) {
	yml := `
services:
  - name: "api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/.*"
    price: 100
    auth: "maybe"
`
	cfg, err := Parse([]byte(yml))
	if err != nil {
		t.Fatal(err)
	}
	// Unrecognised auth values are stored as-is. Warning emitted by caller.
	if cfg.Services[0].Auth != "maybe" {
		t.Errorf("auth = %q, want %q", cfg.Services[0].Auth, "maybe")
	}
}
