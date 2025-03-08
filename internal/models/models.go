package models

// WebSocket API request to Binance
type WebSocketRequest struct {
	ID     string `json:"id"`     // Arbitrary ID used to match responses to requests
	Method string `json:"method"` // Request method name
	// TODO: update param object - https://developers.binance.com/docs/binance-spot-api-docs/web-socket-api/request-format
	Params any `json:"params,omitempty"` // Request parameters. May be omitted if there are no parameters
}

// WebSocket API response from Binance
type WebSocketResponse struct {
	ID         string      `json:"id"`                   // ID that matches the original request
	Status     int         `json:"status"`               // HTTP-like status code
	Result     any         `json:"result,omitempty"`     // Result data, omitted if empty
	RateLimits []RateLimit `json:"rateLimits,omitempty"` // Rate limiting status
	Error      *APIError   `json:"error,omitempty"`      // Error description, omitted if empty
}

// Error returned from Binance
type APIError struct {
	Code int    `json:code`
	Msg  string `json:msg`
}

// Rate limit information
type RateLimit struct {
	// TODO: update RateLimitType object - https://developers.binance.com/docs/binance-spot-api-docs/web-socket-api/rate-limits
	RateLimitType string `json:"rateLimitType"` // Rate limit type: REQUEST_WEIGHT, ORDERS
	Interval      string `json:"interval"`      // Rate limit interval: SECOND, MINUTE, HOUR, DAY
	IntervalNum   int    `json:"intervalNum"`   // Number of rate limit intervals
	Limit         int    `json:"limit"`         // Request limit per interval
	Count         int    `json:"count"`         // Current usage per interval
}

// Depth of the orderbook
type OrderbookDepth struct {
	LastUpdateID int        `json:"lastUpdateId"` // Last update ID
	Bids         [][]string `json:"bids"`         // Bids as [price, quantity] pairs
	Asks         [][]string `json:"asks"`         // Asks as [price, quantity] pairs
}

// Status of an order
type OrderStatus string

// Order status values
const (
	OrderStatusNew             OrderStatus = "NEW"
	OrderStatusPartiallyFilled OrderStatus = "PARTIALLY_FILLED"
	OrderStatusFilled          OrderStatus = "FILLED"
	OrderStatusCanceled        OrderStatus = "CANCELED"
	OrderStatusRejected        OrderStatus = "REJECTED"
	OrderStatusExpired         OrderStatus = "EXPIRED"
)

// Trade order
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

// Parameters for placing an order
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

// Parsed version of the orderbook with float values
type ParsedOrderBook struct {
	Symbol       string
	LastUpdateID int
	Bids         []PriceLevel
	Asks         []PriceLevel
}

// Price level in the orderbook
type PriceLevel struct {
	Price    float64
	Quantity float64
}
