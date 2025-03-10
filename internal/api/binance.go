package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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

	log.Printf("")

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
