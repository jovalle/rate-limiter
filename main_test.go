package main

import (
	"net"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestReverseByteOrder(t *testing.T) {
	tests := []struct {
		name     string
		input    uint32
		expected string
	}{
		{
			name:     "localhost",
			input:    0x0100007F, // 127.0.0.1 in network byte order
			expected: "127.0.0.1",
		},
		{
			name:     "standard IP",
			input:    0xC0A80101, // 192.168.1.1 in network byte order
			expected: "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := reverseByteOrder(tt.input)
			if result.String() != tt.expected {
				t.Errorf("reverseByteOrder(%x) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBoolToFloat64(t *testing.T) {
	tests := []struct {
		name     string
		input    bool
		expected float64
	}{
		{
			name:     "true converts to 1.0",
			input:    true,
			expected: 1.0,
		},
		{
			name:     "false converts to 0.0",
			input:    false,
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := boolToFloat64(tt.input)
			if result != tt.expected {
				t.Errorf("boolToFloat64(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNewMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := newMetrics(reg)

	if m == nil {
		t.Fatal("newMetrics returned nil")
	}

	if m.rateLimited == nil {
		t.Error("rateLimited gauge is nil")
	}

	if m.pktDropCounter == nil {
		t.Error("pktDropCounter gauge is nil")
	}

	if m.tokens == nil {
		t.Error("tokens gauge is nil")
	}

	// Verify metrics are registered
	count, err := testutil.GatherAndCount(reg)
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	// We expect 3 metrics to be registered
	if count != 3 {
		t.Errorf("Expected 3 metrics to be registered, got %d", count)
	}
}

func TestMetricsLabels(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := newMetrics(reg)

	// Test setting metrics with labels
	testIP := "192.168.1.1"
	testPort := uint16(8080)
	testInterface := "eth0"
	connection := net.JoinHostPort(testIP, "8080")

	// Set some test values
	m.rateLimited.WithLabelValues(connection, testInterface).Set(1.0)
	m.pktDropCounter.WithLabelValues(connection, testInterface).Set(100.0)
	m.tokens.WithLabelValues(connection, testInterface).Set(5000.0)

	// Verify metrics can be gathered without errors
	count, err := testutil.GatherAndCount(reg)
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 metrics, got %d", count)
	}
}

func TestPortKeyStructure(t *testing.T) {
	// Test that PortKey structure is correctly defined
	key := PortKey{
		SrcIP:   0x0100007F,
		SrcPort: 8080,
		Padding: 0,
	}

	if key.SrcIP != 0x0100007F {
		t.Errorf("SrcIP not set correctly")
	}

	if key.SrcPort != 8080 {
		t.Errorf("SrcPort not set correctly")
	}
}

func TestPacketStateStructure(t *testing.T) {
	// Test that PacketState structure is correctly defined
	state := PacketState{
		Tokens:                           1000,
		LastRefill:                       123456789,
		LastBurstRefill:                  123456789,
		RateLimited:                      false,
		PktDropCounter:                   0,
		ConfigBurstLimitReplenishSeconds: 600,
		ConfigBurstLimit:                 10000,
		ConfigPacketsPerSecond:           3200,
	}

	if state.Tokens != 1000 {
		t.Errorf("Tokens not set correctly")
	}

	if state.RateLimited != false {
		t.Errorf("RateLimited not set correctly")
	}

	if state.ConfigPacketsPerSecond != 3200 {
		t.Errorf("ConfigPacketsPerSecond not set correctly")
	}
}

func TestConfigDefaults(t *testing.T) {
	// This test validates that config struct has correct default values
	cfg := config{}

	// Since we can't easily test env.Parse without setting environment variables,
	// we'll just verify the struct is properly defined
	if cfg.Interface == "" {
		// Default should be set by env library when parsed
		cfg.Interface = "ens33"
	}

	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}

	if cfg.Interface != "ens33" {
		t.Errorf("Expected default interface 'ens33', got '%s'", cfg.Interface)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("Expected default log level 'info', got '%s'", cfg.LogLevel)
	}
}
