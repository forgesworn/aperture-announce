package config

import (
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
)

const (
	// DefaultServicePrice matches Aperture's default — when no price is
	// set and dynamic pricing is off, Aperture charges 1 sat.
	DefaultServicePrice int64 = 1
)

// ApertureConfig holds only the fields we need from Aperture's YAML.
type ApertureConfig struct {
	Services []Service
}

// Service represents a single Aperture service definition.
type Service struct {
	Name         string
	HostRegexp   string
	PathRegexp   string
	Price        int64
	DynamicPrice bool
	Capabilities []string
	Auth         string
	Timeout      int64
}

type rawService struct {
	Name         string      `yaml:"name"`
	HostRegexp   string      `yaml:"hostregexp"`
	PathRegexp   string      `yaml:"pathregexp"`
	Price        int64       `yaml:"price"`
	Capabilities string      `yaml:"capabilities"`
	DynamicPrice rawDynPrice `yaml:"dynamicprice"`
	Auth         string      `yaml:"auth"`
	Timeout      int64       `yaml:"timeout"`
}

type rawDynPrice struct {
	Enabled     bool   `yaml:"enabled"`
	GRPCAddress string `yaml:"grpcaddress"`
}

type rawConfig struct {
	Services []rawService `yaml:"services"`
}

// Parse reads Aperture YAML bytes and extracts service definitions.
func Parse(data []byte) (*ApertureConfig, error) {
	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}

	if len(raw.Services) == 0 {
		return nil, fmt.Errorf("no services found in aperture config")
	}
	const maxServices = 1000
	if len(raw.Services) > maxServices {
		return nil, fmt.Errorf("too many services (%d, max %d)", len(raw.Services), maxServices)
	}

	services := make([]Service, 0, len(raw.Services))
	for i, rs := range raw.Services {
		if rs.Name == "" {
			return nil, fmt.Errorf("service %d has no name", i)
		}
		if rs.Price < 0 {
			return nil, fmt.Errorf("service %q has negative price: %d", rs.Name, rs.Price)
		}
		if rs.Timeout < 0 {
			return nil, fmt.Errorf("service %q has negative timeout: %d", rs.Name, rs.Timeout)
		}

		price := rs.Price
		if price == 0 && !rs.DynamicPrice.Enabled {
			// Match Aperture's behaviour: default to 1 sat when no
			// price is set and dynamic pricing is off.
			price = DefaultServicePrice
		}

		s := Service{
			Name:         rs.Name,
			HostRegexp:   rs.HostRegexp,
			PathRegexp:   rs.PathRegexp,
			Price:        price,
			DynamicPrice: rs.DynamicPrice.Enabled,
		}

		if rs.Capabilities != "" {
			for _, cap := range strings.Split(rs.Capabilities, ",") {
				cap = strings.TrimSpace(cap)
				if cap != "" {
					s.Capabilities = append(s.Capabilities, cap)
				}
			}
		}

		s.Auth = rs.Auth
		s.Timeout = rs.Timeout

		services = append(services, s)
	}

	return &ApertureConfig{Services: services}, nil
}
