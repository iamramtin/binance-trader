package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Current Unix timestamp in milliseconds
func GenerateTimestamp() int64 {
	return time.Now().UnixMilli()
}

// String representation of current Unix timestamp in milliseconds
func GenerateTimestampString() string {
	return fmt.Sprintf("%d", GenerateTimestamp())
}

// Generate HMAC using the SHA-256 hash function and a key
func GenerateHMAC(secretKey string, data string) string {
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

// Generate HMAC using the SHA-256 hash function and a key
func GenerateSignature(secretKey string, params map[string]string) string {
	// Sort keys alphabetically
	var keys []string
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build query string
	var queryParts []string
	for _, k := range keys {
		queryParts = append(queryParts, fmt.Sprintf("%s=%s", k, params[k]))
	}
	queryString := strings.Join(queryParts, "&")

	return GenerateHMAC(secretKey, queryString)
}

func AuthenticateAPIKeys(apiKey string, secretKey string) error {
	if apiKey == "" || secretKey == "" {
		return fmt.Errorf("API key and Secret key are required for canceling orders")
	}
	return nil
}
