package models

import "time"

// WebSocketRequest represents a WebSocket API request to Binance
type WebSocketRequest struct {
	ID     string    `json:"id"`     // Arbitrary ID used to match responses to requests
	Method string `json:"method"` // Request method name
	// TODO: update param object - https://developers.binance.com/docs/binance-spot-api-docs/web-socket-api/request-format
	Params any `json:"params,omitempty"` // Request parameters. May be omitted if there are no parameters
}

// WebSocketResponse represents a WebSocket API response from Binance
type WebSocketResponse struct {
	ID         string            `json:"id"`                   // ID that matches the original request
	Status     int            `json:"status"`               // HTTP-like status code
	Result     OrderbookDepth `json:"result,omitempty"`     // Result data, omitted if empty
	RateLimits []RateLimit    `json:"rateLimits,omitempty"` // Rate limiting status
	Error      *APIError      `json:"error,omitempty"`      // Error description, omitted if empty
}

// APIError represents an error returned by the Binance API
type APIError struct {
	Code int    `json:code`
	Msg  string `json:msg`
}

// RateLimit represents API rate limit information
type RateLimit struct {
	// TODO: update RateLimitType object - https://developers.binance.com/docs/binance-spot-api-docs/web-socket-api/rate-limits
	RateLimitType string `json:"rateLimitType"` // Rate limit type: REQUEST_WEIGHT, ORDERS
	Interval      string `json:"interval"`      // Rate limit interval: SECOND, MINUTE, HOUR, DAY
	IntervalNum   int    `json:"intervalNum"`   // Number of rate limit intervals
	Limit         int    `json:"limit"`         // Request limit per interval
	Count         int    `json:"count"`         // Current usage per interval
}

// OrderbookDepth represents the depth of the orderbook
type OrderbookDepth struct {
	LastUpdateID int        `json:"lastUpdateId"` // Last update ID
	Bids         [][]string `json:"bids"`         // Bids as [price, quantity] pairs
	Asks         [][]string `json:"asks"`         // Asks as [price, quantity] pairs
}

// OrderStatus represents the status of an order
type OrderStatus string

// Constants for order status values
const (
	OrderStatusNew             OrderStatus = "NEW"
	OrderStatusPartiallyFilled OrderStatus = "PARTIALLY_FILLED"
	OrderStatusFilled          OrderStatus = "FILLED"
	OrderStatusCanceled        OrderStatus = "CANCELED"
	OrderStatusRejected        OrderStatus = "REJECTED"
	OrderStatusExpired         OrderStatus = "EXPIRED"
)

// Order represents a trade order
type Order struct {
	Symbol                  string `json:"symbol"`
	OrderID                 int64  `json:"orderId"`
	OrderListID             int64  `json:"orderListId"`
	ClientOrderID           string `json:"clientOrderId"`
	TransactTime            int64  `json:"transactTime"`
	Price                   string `json:"price"`
	OrigQty                 string `json:"origQty"`
	ExecutedQty             string `json:"executedQty"`
	CummulativeQuoteQty     string `json:"cummulativeQuoteQty"`
	Status                  string `json:"status"`
	TimeInForce             string `json:"timeInForce"`
	Type                    string `json:"type"`
	Side                    string `json:"side"`
	WorkingTime             int64  `json:"workingTime"`
	SelfTradePreventionMode string `json:"selfTradePreventionMode"`
}

// OrderParams represents parameters for placing an order
type OrderParams struct {
	Symbol           string `json:"symbol"`
	Side             string `json:"side"`                  // BUY or SELL
	Type             string `json:"type"`                  // LIMIT, MARKET, etc.
	TimeInForce      string `json:"timeInForce,omitempty"` // GTC, IOC, FOK
	Price            string `json:"price,omitempty"`
	Quantity         string `json:"quantity,omitempty"`
	NewClientOrderID string `json:"newClientOrderId,omitempty"`
	Timestamp        int64  `json:"timestamp"` // Unix timestamp in milliseconds
}

// ParsedOrderBook represents a parsed version of the orderbook with float values
type ParsedOrderBook struct {
	Symbol       string
	LastUpdateID int
	Bids         []PriceLevel
	Asks         []PriceLevel
}

// PriceLevel represents a price level in the orderbook
type PriceLevel struct {
	Price    float64
	Quantity float64
}

// FormatTime formats a Unix timestamp (milliseconds) as a human-readable string
func FormatTime(timestamp int64) string {
	// Convert milliseconds to time.Time
	t := time.Unix(0, timestamp*int64(time.Millisecond))
	return t.Format(time.RFC3339) // ISO 8601 format
}
