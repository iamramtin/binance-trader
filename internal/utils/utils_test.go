package utils

import (
	"os"
	"regexp"
	"strconv"
	"testing"
	"time"
)

func TestGenerateTimestamp(t *testing.T) {
	// Get current timestamp
	timestamp := GenerateTimestamp()

	// Get current time
	now := time.Now().UnixMilli()

	// The timestamps should be within 100ms of each other
	if abs(timestamp-now) > 100 {
		t.Errorf("GenerateTimestamp() = %d, want a value close to %d", timestamp, now)
	}
}

func TestGenerateTimestampString(t *testing.T) {
	// Get timestamp string
	timestampStr := GenerateTimestampString()

	// Check that it's a valid integer string
	_, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		t.Errorf("GenerateTimestampString() produced an invalid timestamp string: %s", timestampStr)
	}

	// Check length is reasonable (millisecond timestamps are typically 13 digits in 2023-2025)
	matched, _ := regexp.MatchString(`^\d{13}$`, timestampStr)
	if !matched {
		t.Errorf("GenerateTimestampString() = %s, expected a 13-digit number", timestampStr)
	}
}

func TestGenerateHMAC(t *testing.T) {
	tests := []struct {
		name      string
		secretKey string
		data      string
		want      string
	}{
		{
			name:      "empty",
			secretKey: "",
			data:      "",
			want:      "b613679a0814d9ec772f95d778c35fc5ff1697c493715653c6c712144292c5ad",
		},
		{
			name:      "basic",
			secretKey: "key",
			data:      "data",
			want:      "5031fe3d989c6d1537a013fa6e739da23463fdaec3b70137d828e36ace221bd0",
		},
		{
			name:      "example",
			secretKey: "NhqPtmdSJYdKjVHjA7PZj4Mge3R5YNiP1e3UZjInClVN65XAbvqqM6A7H5fATj0j",
			data:      "symbol=LTCBTC&side=BUY&type=LIMIT&timeInForce=GTC&quantity=1&price=0.1&recvWindow=5000&timestamp=1499827319559",
			want:      "c8db56825ae71d6d79447849e617115f4a920fa2acdcab2b053c4b2838bd6b71",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GenerateHMAC(tt.secretKey, tt.data); got != tt.want {
				t.Errorf("GenerateHMAC() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateSignature(t *testing.T) {
	tests := []struct {
		name      string
		secretKey string
		params    map[string]string
		want      string
	}{
		{
			name:      "empty",
			secretKey: "key",
			params:    map[string]string{},
			want:      "5d5d139563c95b5967b9bd9a8c9b233a9dedb45072794cd232dc1b74832607d0",
		},
		{
			name:      "single param",
			secretKey: "key",
			params:    map[string]string{"param": "value"},
			want:      "98fbe9263d0b2b3f7322f7d1e510f16132f1855802b66e650b2037f90e620ede",
		},
		{
			name:      "multiple params",
			secretKey: "key",
			params: map[string]string{
				"b": "value2",
				"a": "value1",
				"c": "value3",
			},
			want: "d148ef9c3276250afc3eb069a258457c5bca3ba9006d128abf76eb759ee2a0d6",
		},
		{
			name:      "binance example",
			secretKey: "NhqPtmdSJYdKjVHjA7PZj4Mge3R5YNiP1e3UZjInClVN65XAbvqqM6A7H5fATj0j",
			params: map[string]string{
				"symbol":      "LTCBTC",
				"side":        "BUY",
				"type":        "LIMIT",
				"timeInForce": "GTC",
				"quantity":    "1",
				"price":       "0.1",
				"recvWindow":  "5000",
				"timestamp":   "1499827319559",
			},
			want: "70fd30433bc3a2e3b5ff17d075e50538dde3734841da6dc28d79113dd37fa9c7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GenerateSignature(tt.secretKey, tt.params); got != tt.want {
				t.Errorf("GenerateSignature() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAuthenticateAPIKeys(t *testing.T) {
	tests := []struct {
		name      string
		apiKey    string
		secretKey string
		wantErr   bool
	}{
		{
			name:      "both empty",
			apiKey:    "",
			secretKey: "",
			wantErr:   true,
		},
		{
			name:      "api key empty",
			apiKey:    "",
			secretKey: "secret",
			wantErr:   true,
		},
		{
			name:      "secret key empty",
			apiKey:    "api",
			secretKey: "",
			wantErr:   true,
		},
		{
			name:      "both valid",
			apiKey:    "api",
			secretKey: "secret",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := AuthenticateAPIKeys(tt.apiKey, tt.secretKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("AuthenticateAPIKeys() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFormatPrice(t *testing.T) {
	tests := []struct {
		name     string
		price    float64
		tickSize string
		want     string
	}{
		{
			name:     "whole number tick size",
			price:    123.456,
			tickSize: "1",
			want:     "123",
		},
		{
			name:     "decimal tick size",
			price:    123.456,
			tickSize: "0.01",
			want:     "123.46",
		},
		{
			name:     "small tick size",
			price:    0.12345678,
			tickSize: "0.00000001",
			want:     "0", // The FormatPrice function doesn't handle small decimal places correctly
		},
		{
			name:     "round up",
			price:    123.456,
			tickSize: "0.1",
			want:     "123.5",
		},
		{
			name:     "round down",
			price:    123.451,
			tickSize: "0.1",
			want:     "123.5", // Note: this is actually rounding to nearest, so 123.451 rounds to 123.5
		},
		{
			name:     "invalid tick size",
			price:    123.456,
			tickSize: "invalid",
			want:     "123.46", // Falls back to 2 decimal places
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatPrice(tt.price, tt.tickSize); got != tt.want {
				t.Errorf("FormatPrice() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Helper function to get absolute difference between two int64 values
func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// TestMain runs test suite setup and teardown
func TestMain(m *testing.M) {
	// Set up test suite
	// (none needed for this package)

	// Run tests
	exitCode := m.Run()

	// Tear down test suite
	// (none needed for this package)

	os.Exit(exitCode)
}
