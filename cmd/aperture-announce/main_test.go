package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildBinary compiles the binary once for the test suite.
func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "aperture-announce")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return bin
}

func TestCLI_DryRun(t *testing.T) {
	bin := buildBinary(t)

	// Use the sample config from testdata
	configPath, err := filepath.Abs("../../testdata/sample-conf.yaml")
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin,
		"--config", configPath,
		"--public-url", "https://api.example.com",
		"--dry-run",
	)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			t.Fatalf("exit %d: %s", ee.ExitCode(), ee.Stderr)
		}
		t.Fatal(err)
	}

	// Parse the JSON output
	var ev map[string]interface{}
	if err := json.Unmarshal(out, &ev); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}

	// Verify event structure
	kind, ok := ev["kind"].(float64)
	if !ok || int(kind) != 31402 {
		t.Errorf("kind = %v, want 31402", ev["kind"])
	}

	tags, ok := ev["tags"].([]interface{})
	if !ok {
		t.Fatal("tags is not an array")
	}
	if len(tags) == 0 {
		t.Fatal("tags is empty")
	}

	// Check for required tags
	foundD := false
	foundURL := false
	foundPMI := false
	foundPrice := false
	for _, tag := range tags {
		arr, ok := tag.([]interface{})
		if !ok || len(arr) < 2 {
			continue
		}
		key, _ := arr[0].(string)
		switch key {
		case "d":
			foundD = true
			val, _ := arr[1].(string)
			if val != "aperture-api.example.com" {
				t.Errorf("d tag = %q, want %q", val, "aperture-api.example.com")
			}
		case "url":
			foundURL = true
		case "pmi":
			foundPMI = true
		case "price":
			foundPrice = true
		}
	}
	if !foundD {
		t.Error("missing d tag")
	}
	if !foundURL {
		t.Error("missing url tag")
	}
	if !foundPMI {
		t.Error("missing pmi tag")
	}
	if !foundPrice {
		t.Error("missing price tag")
	}

	// Check name tag has short form (not full about string)
	for _, tag := range tags {
		arr, ok := tag.([]interface{})
		if !ok || len(arr) < 2 {
			continue
		}
		key, _ := arr[0].(string)
		if key == "name" {
			val, _ := arr[1].(string)
			if val != "loop-rpc, pool-rpc" {
				t.Errorf("name tag = %q, want %q", val, "loop-rpc, pool-rpc")
			}
		}
	}

	// Verify content is valid JSON with capabilities
	content, ok := ev["content"].(string)
	if !ok {
		t.Fatal("content is not a string")
	}
	var contentObj struct {
		Capabilities []struct {
			Name string `json:"name"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal([]byte(content), &contentObj); err != nil {
		t.Fatalf("content is not valid JSON: %v", err)
	}
	if len(contentObj.Capabilities) == 0 {
		t.Error("no capabilities in content")
	}

	// Verify signature fields exist
	if _, ok := ev["sig"]; !ok {
		t.Error("missing sig field")
	}
	if _, ok := ev["pubkey"]; !ok {
		t.Error("missing pubkey field")
	}
}

func TestCLI_MissingConfig(t *testing.T) {
	bin := buildBinary(t)

	cmd := exec.Command(bin, "--public-url", "https://example.com", "--dry-run")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for missing --config")
	}
	if !strings.Contains(string(out), "--config is required") {
		t.Errorf("expected '--config is required' in output, got: %s", out)
	}
}

func TestCLI_MissingPublicURL(t *testing.T) {
	bin := buildBinary(t)

	configPath, _ := filepath.Abs("../../testdata/sample-conf.yaml")
	cmd := exec.Command(bin, "--config", configPath, "--dry-run")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for missing --public-url")
	}
	if !strings.Contains(string(out), "--public-url is required") {
		t.Errorf("expected '--public-url is required' in output, got: %s", out)
	}
}

func TestCLI_MissingRelays(t *testing.T) {
	bin := buildBinary(t)

	configPath, _ := filepath.Abs("../../testdata/sample-conf.yaml")
	cmd := exec.Command(bin, "--config", configPath, "--public-url", "https://example.com")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for missing --relays without --dry-run")
	}
	if !strings.Contains(string(out), "--relays is required") {
		t.Errorf("expected '--relays is required' in output, got: %s", out)
	}
}

func TestCLI_PrivateURL(t *testing.T) {
	bin := buildBinary(t)

	configPath, _ := filepath.Abs("../../testdata/sample-conf.yaml")
	cmd := exec.Command(bin, "--config", configPath, "--public-url", "http://localhost:3000", "--dry-run")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for private URL")
	}
	if !strings.Contains(string(out), "private/loopback") {
		t.Errorf("expected private/loopback error, got: %s", out)
	}
}

func TestCLI_EnvVarFallback(t *testing.T) {
	bin := buildBinary(t)

	configPath, _ := filepath.Abs("../../testdata/sample-conf.yaml")

	cmd := exec.Command(bin, "--dry-run")
	cmd.Env = append(os.Environ(),
		"APERTURE_CONFIG="+configPath,
		"PUBLIC_URL=https://api.example.com",
	)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			t.Fatalf("exit %d: %s", ee.ExitCode(), ee.Stderr)
		}
		t.Fatal(err)
	}

	var ev map[string]interface{}
	if err := json.Unmarshal(out, &ev); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	kind, ok := ev["kind"].(float64)
	if !ok || int(kind) != 31402 {
		t.Errorf("kind = %v, want 31402", ev["kind"])
	}
}

func TestCLI_TopicsFlag(t *testing.T) {
	bin := buildBinary(t)
	configPath, _ := filepath.Abs("../../testdata/sample-conf.yaml")

	cmd := exec.Command(bin,
		"--config", configPath,
		"--public-url", "https://api.example.com",
		"--topics", "ai,inference",
		"--dry-run",
	)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			t.Fatalf("exit %d: %s", ee.ExitCode(), ee.Stderr)
		}
		t.Fatal(err)
	}

	outStr := string(out)
	if !strings.Contains(outStr, `"ai"`) {
		t.Error("missing custom topic 'ai' in output")
	}
	if !strings.Contains(outStr, `"inference"`) {
		t.Error("missing custom topic 'inference' in output")
	}
}

func TestCLI_TopicsEnvVar(t *testing.T) {
	bin := buildBinary(t)
	configPath, _ := filepath.Abs("../../testdata/sample-conf.yaml")

	cmd := exec.Command(bin, "--dry-run")
	cmd.Env = append(os.Environ(),
		"APERTURE_CONFIG="+configPath,
		"PUBLIC_URL=https://api.example.com",
		"ANNOUNCE_TOPICS=custom1,custom2",
	)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			t.Fatalf("exit %d: %s", ee.ExitCode(), ee.Stderr)
		}
		t.Fatal(err)
	}

	outStr := string(out)
	if !strings.Contains(outStr, `"custom1"`) {
		t.Error("missing env var topic 'custom1'")
	}
}

func TestCLI_AuthWarning(t *testing.T) {
	bin := buildBinary(t)

	configYAML := `
services:
  - name: "api"
    hostregexp: "api.example.com"
    pathregexp: "/v1/.*"
    price: 100
    auth: "maybe"
`
	configPath := filepath.Join(t.TempDir(), "aperture.yaml")
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin,
		"--config", configPath,
		"--public-url", "https://api.example.com",
		"--dry-run",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Should still succeed (warning, not fatal)
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() != 0 {
			t.Fatalf("exit %d: %s", ee.ExitCode(), out)
		}
	}
	if !strings.Contains(string(out), "unrecognised auth") {
		t.Errorf("expected auth warning in output, got: %s", out)
	}
}
