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

func (client *BinanceClient) Connect(ctx context.Context) error {
	return client.wsClient.Connect(ctx)
}

func (client *BinanceClient) Close() {
	client.wsClient.Close()
}

func (client *BinanceClient) GetWSClient() *websocket.Client {
	return client.wsClient
}

func (client *BinanceClient) GetOrderManager() *ordermanager.Manager {
	return client.orderManager
}

// Get current order book
func (client *BinanceClient) GetOrderbook(limit int) (*models.ParsedOrderBook, error) {
	resultCh := make(chan *models.ParsedOrderBook, 1)
	errCh := make(chan error, 1)

	// Send the request
	_, err := client.wsClient.SendRequest("depth", map[string]any{
		"symbol": client.symbol,
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

		parsedBook.Symbol = client.symbol
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
func (client *BinanceClient) PlaceOrder(side, orderType, price, quantity string) (*models.Order, error) {
	if err := utils.AuthenticateAPIKeys(client.apiKey, client.secretKey); err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}

	resultCh := make(chan *models.Order, 1)
	errCh := make(chan error, 1)

	timestamp := utils.GenerateTimestampString()

	params := map[string]string{
		"symbol":    client.symbol,
		"side":      side,
		"type":      orderType,
		"timestamp": timestamp,
		"apiKey":    client.apiKey,
	}

	if orderType == "LIMIT" {
		params["price"] = price
		params["quantity"] = quantity
		params["timeInForce"] = "GTC"
	} else if orderType == "MARKET" {
		params["quantity"] = quantity
	}

	params["signature"] = utils.GenerateSignature(client.secretKey, params)

	_, err := client.wsClient.SendRequest("order.place", params, func(response []byte) {
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

		client.orderManager.TrackOrder(&order)

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
func (client *BinanceClient) CancelOrder(orderID int64) (*models.Order, error) {
	if err := utils.AuthenticateAPIKeys(client.apiKey, client.secretKey); err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}

	resultCh := make(chan *models.Order, 1)
	errCh := make(chan error, 1)

	timestamp := utils.GenerateTimestampString()

	params := map[string]string{
		"symbol":    client.symbol,
		"orderId":   fmt.Sprintf("%d", orderID),
		"timestamp": timestamp,
		"apiKey":    client.apiKey,
	}

	params["signature"] = utils.GenerateSignature(client.secretKey, params)

	_, err := client.wsClient.SendRequest("order.cancel", params, func(response []byte) {
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

		client.orderManager.UpdateOrder(&order)

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
func (client *BinanceClient) GetOrderStatus(orderID int64) (*models.Order, error) {
	if err := utils.AuthenticateAPIKeys(client.apiKey, client.secretKey); err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}

	if order, err := client.orderManager.GetOrder(orderID); err == nil {
		return order, nil
	}

	resultCh := make(chan *models.Order, 1)
	errCh := make(chan error, 1)

	timestamp := utils.GenerateTimestampString()

	params := map[string]string{
		"symbol":    client.symbol,
		"orderId":   fmt.Sprintf("%d", orderID),
		"timestamp": timestamp,
		"apiKey":    client.apiKey,
	}

	params["signature"] = utils.GenerateSignature(client.secretKey, params)

	_, err := client.wsClient.SendRequest("order.status", params, func(response []byte) {
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

		client.orderManager.TrackOrder(&order)

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
