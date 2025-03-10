package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strconv"
	"time"

	"github.com/iamramtin/binance-trader/internal/models"
	"github.com/iamramtin/binance-trader/internal/ordermanager"
	"github.com/iamramtin/binance-trader/internal/utils"
	"github.com/iamramtin/binance-trader/internal/websocket"
)

type BinanceClient struct {
	wsClient     *websocket.Client     // WebSocket client
	orderManager *ordermanager.Manager // Order manager
	apiKey       string                // API key
	secretKey    string                // Secret key
	symbol       string                // Trading symbol
}

func New(wsURL, apiKey, secretKey, symbol string) *BinanceClient {
	return &BinanceClient{
		wsClient:     websocket.New(wsURL, apiKey, secretKey),
		orderManager: ordermanager.New(),
		apiKey:       apiKey,
		secretKey:    secretKey,
		symbol:       symbol,
	}
}

func (c *BinanceClient) Connect(ctx context.Context) error {
	return c.wsClient.Connect(ctx)
}

func (c *BinanceClient) Close() {
	c.wsClient.Close()
}

func (c *BinanceClient) GetWSClient() *websocket.Client {
	return c.wsClient
}

func (c *BinanceClient) GetOrderManager() *ordermanager.Manager {
	return c.orderManager
}

func (c *BinanceClient) TestSignature() error {
	timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())

	params := map[string]string{
		"timestamp": timestamp,
		"apiKey":    c.apiKey,
	}

	params["signature"] = utils.GenerateSignature(c.secretKey, params)

	requestParams := make(map[string]any)
	for k, v := range params {
		requestParams[k] = v
	}

	resultCh := make(chan bool, 1)
	errCh := make(chan error, 1)

	logParams, _ := json.Marshal(requestParams)
	log.Printf("Sending test request: %s", string(logParams))

	_, err := c.wsClient.SendRequest("account.status", requestParams, func(response []byte) {
		var wsResponse models.WebSocketResponse
		if err := json.Unmarshal(response, &wsResponse); err != nil {
			errCh <- fmt.Errorf("error parsing test response: %w", err)
			return
		}

		if wsResponse.Error != nil {
			errCh <- fmt.Errorf("API error: %s", wsResponse.Error.Msg)
			return
		}

		resultCh <- true
	})

	if err != nil {
		return err
	}

	select {
	case <-resultCh:
		return nil
	case err := <-errCh:
		return err
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout waiting for test response")
	}
}

func (c *BinanceClient) GetAccountBalance() (*models.AccountResponse, error) {
	resultCh := make(chan *models.AccountResponse, 1)
	errCh := make(chan error, 1)

	timestamp := utils.GenerateTimestampString()

	params := map[string]string{
		"timestamp": timestamp,
		"apiKey":    c.apiKey,
	}

	params["signature"] = utils.GenerateSignature(c.secretKey, params)

	_, err := c.wsClient.SendRequest("account.status", params, func(response []byte) {
		var wsResponse models.WebSocketResponse
		if err := json.Unmarshal(response, &wsResponse); err != nil {
			errCh <- fmt.Errorf("error parsing WebSocket response: %w", err)
			return
		}

		if wsResponse.Status != 200 {
			errCh <- fmt.Errorf("API error: %d - %s", wsResponse.Error.Code, wsResponse.Error.Msg)
			return
		}

		var accountInfo models.AccountInfo
		if err := json.Unmarshal(wsResponse.Result, &accountInfo); err != nil {
			errCh <- fmt.Errorf("error parsing account data: %w", err)
			return
		}

		accountResp := models.AccountResponse{
			Status:      wsResponse.Status,
			AccountInfo: accountInfo,
		}

		resultCh <- &accountResp
	})

	if err != nil {
		return nil, err
	}

	select {
	case result := <-resultCh:
		return result, nil
	case err := <-errCh:
		return nil, err
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timeout waiting for order response")
	}
}

// Retrieve and returns balance information for a specific trading pair
func (c *BinanceClient) GetTradingPairBalance(baseAsset string, quoteAsset string) (map[string]float64, error) {
	accountResp, err := c.GetAccountBalance()
	if err != nil {
		return nil, fmt.Errorf("error getting account balance: %w", err)
	}

	symbol := fmt.Sprintf("%s%s", baseAsset, quoteAsset)
	if baseAsset == "" || quoteAsset == "" {
		return nil, fmt.Errorf("invalid trading pair format: %s", symbol)
	}

	// Extract balances for these specific assets
	balances := make(map[string]float64)
	balances[baseAsset] = 0
	balances[quoteAsset] = 0

	// Find the assets in the account balances
	for _, balance := range accountResp.AccountInfo.Balances {
		if balance.Asset == baseAsset || balance.Asset == quoteAsset {
			// Parse the free balance as a float
			freeAmount, err := strconv.ParseFloat(balance.Free, 64)
			if err != nil {
				log.Printf("Warning: Could not parse free balance for %s: %v", balance.Asset, err)
				freeAmount = 0
			}

			// Parse the locked balance as a float
			lockedAmount, err := strconv.ParseFloat(balance.Locked, 64)
			if err != nil {
				log.Printf("Warning: Could not parse locked balance for %s: %v", balance.Asset, err)
				lockedAmount = 0
			}

			// Store the total balance (free + locked)
			balances[balance.Asset] = freeAmount + lockedAmount
		}
	}

	return balances, nil
}

// Display balance information for a specific trading pair
func (c *BinanceClient) DisplayTradingPairBalance(baseAsset string, quoteAsset string) error {
	symbol := fmt.Sprintf("%s%s", baseAsset, quoteAsset)
	balances, err := c.GetTradingPairBalance(baseAsset, quoteAsset)
	if err != nil {
		return err
	}

	fmt.Printf("\n=== BALANCE FOR %s ===\n", symbol)
	fmt.Printf("Base Asset (%s): %.8f\n", baseAsset, balances[baseAsset])
	fmt.Printf("Quote Asset (%s): %.8f\n", quoteAsset, balances[quoteAsset])

	// If we have market price information, we can calculate the total value
	orderbook, err := c.GetOrderbook(1)
	if err == nil && len(orderbook.Bids) > 0 {
		midPrice := orderbook.Bids[0].Price
		baseValue := balances[baseAsset] * midPrice
		totalValue := baseValue + balances[quoteAsset]

		fmt.Printf("\nCurrent Price: %.8f %s/%s\n", midPrice, baseAsset, quoteAsset)
		fmt.Printf("Base Asset Value: %.8f %s\n", baseValue, quoteAsset)
		fmt.Printf("Total Value: %.8f %s\n", totalValue, quoteAsset)
	}

	fmt.Println("========================")
	return nil
}

// Get current order book
func (c *BinanceClient) GetOrderbook(limit int) (*models.ParsedOrderBook, error) {
	resultCh := make(chan *models.ParsedOrderBook, 1)
	errCh := make(chan error, 1)

	// Send the request
	_, err := c.wsClient.SendRequest("depth", map[string]any{
		"symbol": c.symbol,
		"limit":  limit,
	}, func(response []byte) {
		var wsResponse models.WebSocketResponse
		if err := json.Unmarshal(response, &wsResponse); err != nil {
			errCh <- fmt.Errorf("error parsing orderbook response: %w", err)
			return
		}

		if wsResponse.Error != nil {
			errCh <- fmt.Errorf("API error: %s", wsResponse.Error.Msg)
			return
		}

		resultJSON, err := json.Marshal(wsResponse.Result)
		if err != nil {
			errCh <- fmt.Errorf("error marshaling result: %w", err)
			return
		}

		var orderbook models.OrderbookDepth
		if err := json.Unmarshal(resultJSON, &orderbook); err != nil {
			errCh <- fmt.Errorf("error parsing orderbook data: %w", err)
			return
		}

		parsedBook, err := parseOrderbook(&orderbook)
		if err != nil {
			errCh <- fmt.Errorf("error parsing orderbook values: %w", err)
			return
		}

		parsedBook.Symbol = c.symbol
		resultCh <- parsedBook
	})

	if err != nil {
		return nil, err
	}

	select {
	case result := <-resultCh:
		return result, nil
	case err := <-errCh:
		return nil, err
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timeout waiting for orderbook response")
	}
}

// Place a new order
func (c *BinanceClient) PlaceOrder(side, orderType, price, quantity string) (*models.Order, error) {
	if err := utils.AuthenticateAPIKeys(c.apiKey, c.secretKey); err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}

	resultCh := make(chan *models.Order, 1)
	errCh := make(chan error, 1)

	timestamp := utils.GenerateTimestampString()

	params := map[string]string{
		"symbol":    c.symbol,
		"side":      side,
		"type":      orderType,
		"timestamp": timestamp,
		"apiKey":    c.apiKey,
	}

	if orderType == "LIMIT" {
		params["price"] = price
		params["quantity"] = quantity
		params["timeInForce"] = "GTC"

		log.Printf("Placing %s BUY order: %s %s @ %s", orderType, c.symbol, quantity, price)

	} else if orderType == "MARKET" {
		params["quantity"] = quantity

		log.Printf("Placing %s BUY order: %s %s", orderType, c.symbol, quantity)
	}

	params["signature"] = utils.GenerateSignature(c.secretKey, params)

	_, err := c.wsClient.SendRequest("order.place", params, func(response []byte) {
		var wsResponse models.WebSocketResponse
		if err := json.Unmarshal(response, &wsResponse); err != nil {
			errCh <- fmt.Errorf("error parsing order response: %w", err)
			return
		}

		if wsResponse.Error != nil {
			errCh <- fmt.Errorf("API error: %s", wsResponse.Error.Msg)
			return
		}

		resultJSON, err := json.Marshal(wsResponse.Result)
		if err != nil {
			errCh <- fmt.Errorf("error marshaling result: %w", err)
			return
		}

		var order models.Order
		if err := json.Unmarshal(resultJSON, &order); err != nil {
			errCh <- fmt.Errorf("error parsing order data: %w", err)
			return
		}

		c.orderManager.TrackOrder(&order)

		resultCh <- &order
	})

	if err != nil {
		return nil, err
	}

	select {
	case result := <-resultCh:
		return result, nil
	case err := <-errCh:
		return nil, err
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timeout waiting for order response")
	}
}

// Cancel an active order
func (c *BinanceClient) CancelOrder(orderID int64) (*models.Order, error) {
	if err := utils.AuthenticateAPIKeys(c.apiKey, c.secretKey); err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}

	resultCh := make(chan *models.Order, 1)
	errCh := make(chan error, 1)

	timestamp := utils.GenerateTimestampString()

	params := map[string]string{
		"symbol":    c.symbol,
		"orderId":   fmt.Sprintf("%d", orderID),
		"timestamp": timestamp,
		"apiKey":    c.apiKey,
	}

	params["signature"] = utils.GenerateSignature(c.secretKey, params)

	_, err := c.wsClient.SendRequest("order.cancel", params, func(response []byte) {
		var wsResponse models.WebSocketResponse
		if err := json.Unmarshal(response, &wsResponse); err != nil {
			errCh <- fmt.Errorf("error parsing cancel response: %w", err)
			return
		}

		if wsResponse.Error != nil {
			errCh <- fmt.Errorf("API error: %s", wsResponse.Error.Msg)
			return
		}

		resultJSON, err := json.Marshal(wsResponse.Result)
		if err != nil {
			errCh <- fmt.Errorf("error marshaling result: %w", err)
			return
		}

		var order models.Order
		if err := json.Unmarshal(resultJSON, &order); err != nil {
			errCh <- fmt.Errorf("error parsing order data: %w", err)
			return
		}

		c.orderManager.UpdateOrder(&order)

		resultCh <- &order
	})

	if err != nil {
		return nil, err
	}

	select {
	case result := <-resultCh:
		return result, nil
	case err := <-errCh:
		return nil, err
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timeout waiting for cancel response")
	}
}

// Check execution status of an order
func (c *BinanceClient) GetOrderStatus(orderID int64) (*models.Order, error) {
	if err := utils.AuthenticateAPIKeys(c.apiKey, c.secretKey); err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}

	if orderID == -1 {
		return nil, fmt.Errorf("invalid orderID")
	}

	if order, err := c.orderManager.GetOrder(orderID); err == nil {
		return order, nil
	}

	resultCh := make(chan *models.Order, 1)
	errCh := make(chan error, 1)

	timestamp := utils.GenerateTimestampString()

	params := map[string]string{
		"symbol":    c.symbol,
		"orderId":   fmt.Sprintf("%d", orderID),
		"timestamp": timestamp,
		"apiKey":    c.apiKey,
	}

	params["signature"] = utils.GenerateSignature(c.secretKey, params)

	fmt.Printf("Params: %s", params)

	_, err := c.wsClient.SendRequest("order.status", params, func(response []byte) {
		var wsResponse models.WebSocketResponse
		if err := json.Unmarshal(response, &wsResponse); err != nil {
			errCh <- fmt.Errorf("error parsing status response: %w", err)
			return
		}

		if wsResponse.Error != nil {
			errCh <- fmt.Errorf("API error: %s", wsResponse.Error.Msg)
			return
		}

		resultJSON, err := json.Marshal(wsResponse.Result)
		if err != nil {
			errCh <- fmt.Errorf("error marshaling result: %w", err)
			return
		}

		var order models.Order
		if err := json.Unmarshal(resultJSON, &order); err != nil {
			errCh <- fmt.Errorf("error parsing order data: %w", err)
			return
		}

		c.orderManager.TrackOrder(&order)

		resultCh <- &order
	})

	if err != nil {
		return nil, err
	}

	select {
	case result := <-resultCh:
		return result, nil
	case err := <-errCh:
		return nil, err
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timeout waiting for status response")
	}
}

func (c *BinanceClient) DisplayOrderbook(book *models.ParsedOrderBook, limit int) {
	log.Printf("Orderbook LastUpdateID: %d", book.LastUpdateID)
	log.Println("Bids (Buy Orders):")
	log.Println("Price\t\tQuantity")

	maxBids := min(len(book.Bids), limit)
	for i := range maxBids {
		log.Printf("%.8f\t%.8f", book.Bids[i].Price, book.Bids[i].Quantity)
	}

	log.Println()
	log.Println("Asks (Sell Orders):")
	log.Println("Price\t\tQuantity")

	maxAsks := min(len(book.Asks), limit)
	for i := range maxAsks {
		log.Printf("%.8f\t%.8f", book.Asks[i].Price, book.Asks[i].Quantity)
	}
}

func (c *BinanceClient) DisplayAccountBalance(data *models.AccountResponse) {
	fmt.Println("\n=== ACCOUNT INFORMATION ===")
	fmt.Printf("Account Type: %s\n", data.AccountInfo.AccountType)
	fmt.Printf("Can Trade: %v\n", data.AccountInfo.CanTrade)
	fmt.Printf("Can Withdraw: %v\n", data.AccountInfo.CanWithdraw)
	fmt.Printf("Can Deposit: %v\n", data.AccountInfo.CanDeposit)

	fmt.Println("\n=== BALANCES ===")
	for _, balance := range data.AccountInfo.Balances {
		fmt.Printf("Asset: %-10s\tFree: %-20s\tLocked: %s\n",
			balance.Asset, balance.Free, balance.Locked)
	}

	fmt.Println("\n=== TRADING PERMISSIONS ===")
	for _, permission := range data.AccountInfo.Permissions {
		fmt.Println(permission)
	}
}

func (c *BinanceClient) HasSufficientBalance(baseAsset string, quoteAsset string, side string, quantity float64, price float64) (bool, error) {
	balances, err := c.GetTradingPairBalance(baseAsset, quoteAsset)
	if err != nil {
		return false, err
	}

	if side == "BUY" {
		// For a buy order, check if we have enough quote asset (e.g., USDT)
		requiredAmount := quantity * price
		return balances[quoteAsset] >= requiredAmount, nil
	} else if side == "SELL" {
		// For a sell order, check if we have enough base asset (e.g., BTC)
		return balances[baseAsset] >= quantity, nil
	}

	return false, fmt.Errorf("invalid side: %s", side)
}

// Calculate the maximum order size based on available balance
func (c *BinanceClient) GetMaxOrderSize(baseAsset string, quoteAsset string, side string, price float64) (float64, error) {
	balances, err := c.GetTradingPairBalance(baseAsset, quoteAsset)
	if err != nil {
		return 0, err
	}

	if side == "BUY" {
		// For a buy order, the max quantity is limited by quote asset (e.g., USDT)
		maxQuantity := balances[quoteAsset] / price
		// Round down to 6 decimal places or whatever precision is appropriate for the asset
		maxQuantity = math.Floor(maxQuantity*1000000) / 1000000
		return maxQuantity, nil
	} else if side == "SELL" {
		// For a sell order, the max quantity is the base asset amount (e.g., BTC)
		// Round down to 6 decimal places or whatever precision is appropriate for the asset
		maxQuantity := math.Floor(balances[baseAsset]*1000000) / 1000000
		return maxQuantity, nil
	}

	return 0, fmt.Errorf("invalid side: %s", side)
}

func parseOrderbook(data *models.OrderbookDepth) (*models.ParsedOrderBook, error) {
	result := &models.ParsedOrderBook{
		LastUpdateID: data.LastUpdateID,
		Bids:         make([]models.PriceLevel, len(data.Bids)),
		Asks:         make([]models.PriceLevel, len(data.Asks)),
	}

	for i, bid := range data.Bids {
		if len(bid) <= 1 {
			continue
		}

		price, err := strconv.ParseFloat(bid[0], 64)
		if err != nil {
			return nil, err
		}

		qty, err := strconv.ParseFloat(bid[1], 64)
		if err != nil {
			return nil, err
		}

		result.Bids[i] = models.PriceLevel{Price: price, Quantity: qty}
	}

	for i, ask := range data.Asks {
		if len(ask) <= 1 {
			continue
		}

		price, err := strconv.ParseFloat(ask[0], 64)
		if err != nil {
			return nil, err
		}

		qty, err := strconv.ParseFloat(ask[1], 64)
		if err != nil {
			return nil, err
		}

		result.Asks[i] = models.PriceLevel{Price: price, Quantity: qty}
	}

	return result, nil
}
