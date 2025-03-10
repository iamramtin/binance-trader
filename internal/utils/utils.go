package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"sort"
	"strconv"
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
		log.Println("Generate Generate HMAC-SHA-256 Key: https://testnet.binance.vision")
		log.Println("Set BINANCE_API_KEY and BINANCE_SECRET_KEY environment variables.")
		return fmt.Errorf("API key and Secret key are required for order operations to work")
	}
	return nil
}

// FormatPrice formats a price according to tick size
func FormatPrice(price float64, tickSize string) string {
	// Parse tick size
	tickSizeFloat, err := strconv.ParseFloat(tickSize, 64)
	if err != nil {
		log.Printf("Error parsing tick size: %v", err)
		return fmt.Sprintf("%.2f", price) // Fallback to 2 decimal places
	}

	// Round to the nearest tick size
	nearestPrice := math.Round(price/tickSizeFloat) * tickSizeFloat

	// Calculate the number of decimal places
	decimalPlaces := 0
	if tickSizeFloat < 1 {
		tickStr := fmt.Sprintf("%g", tickSizeFloat)
		parts := strings.Split(tickStr, ".")
		if len(parts) > 1 {
			decimalPlaces = len(parts[1])
		}
	}

	// Format the price with the correct number of decimal places
	return fmt.Sprintf(fmt.Sprintf("%%.%df", decimalPlaces), nearestPrice)
}
