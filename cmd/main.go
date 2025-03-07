package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/iamramtin/binance-trader/internal/models"
	"github.com/iamramtin/binance-trader/internal/websocket"
)

func parseOrderBook(data *models.OrderbookDepth) (*models.ParsedOrderBook, error) {
	result := &models.ParsedOrderBook{
		LastUpdateID: data.LastUpdateID,
		Bids:         make([]models.PriceLevel, len(data.Bids)),
		Asks:         make([]models.PriceLevel, len(data.Asks)),
	}

	for i, bid := range data.Bids {
		if len(bid) < 2 {
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
		if len(ask) < 2 {
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

func displayOrderBook(book *models.ParsedOrderBook) {
	log.Printf("Orderbook LastUpdateID: %d", book.LastUpdateID)

	log.Println("Asks (Sell Orders):")
	log.Println("Price\t\tQuantity")

	maxAsks := min(len(book.Asks), 5)
	for i := range maxAsks {
		log.Printf("%.8f\t%.8f", book.Asks[i].Price, book.Asks[i].Quantity)
	}

	log.Println()
	log.Println("Bids (Buy Orders):")
	log.Println("Price\t\tQuantity")

	maxBids := min(len(book.Bids), 5)
	for i := range maxBids {
		log.Printf("%.8f\t%.8f", book.Bids[i].Price, book.Bids[i].Quantity)
	}
}

func main() {
	log.Println("Starting Binance WebSocket trading application...")

	symbol := "BTCUSDT"
	limit := 10 // levels to request

	// Create a new WebSocket client
	// TODO: get values environment variables or config file
	client := websocket.New("wss://testnet.binance.vision/ws-api/v3", "", "")

	// Create a context that can be canceled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to the WebSocket server
	if err := client.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Set up a ticker for regular pings
	pingTicker := time.NewTicker(20 * time.Second)
	defer pingTicker.Stop()

	// Set up a ticker for orderbook requests
	orderbookTicker := time.NewTicker(5 * time.Second)
	defer orderbookTicker.Stop()

	// Set up a channel to handle OS signals for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("Application running. Trading %s. Press Ctrl+C to exit.", symbol)

	for {
		select {
		case <-pingTicker.C:
			// Send a ping to keep the connection alive
			if err := client.Ping(); err != nil {
				log.Printf("Failed to send ping: %v", err)
			}

		case <-orderbookTicker.C:
			// Request orderbook
			requestID, err := client.SendRequest("depth", map[string]any{
				"symbol": symbol,
				"limit":  limit,
			}, func(response []byte) {
				// Parse the response
				var wsResponse models.WebSocketResponse
				if err := json.Unmarshal(response, &wsResponse); err != nil {
					log.Printf("Error parsing orderbook response: %v", err)
					return
				}

				if wsResponse.Error != nil {
					log.Printf("API error: %s", wsResponse.Error.Msg)
					return
				}

				// Convert the result to OrderbookDepth
				resultJSON, err := json.Marshal(wsResponse.Result)
				if err != nil {
					log.Printf("Error marshaling result: %v", err)
					return
				}

				var orderbook models.OrderbookDepth
				if err := json.Unmarshal(resultJSON, &orderbook); err != nil {
					log.Printf("Error parsing orderbook data: %v", err)
					return
				}

				// Parse and display the orderbook
				parsedBook, err := parseOrderBook(&orderbook)
				if err != nil {
					log.Printf("Error parsing orderbook values: %v", err)
					return
				}
				parsedBook.Symbol = symbol

				displayOrderBook(parsedBook)
			})

			if err != nil {
				log.Printf("Failed to request orderbook: %v", err)
			} else {
				log.Printf("Sent orderbook request with ID: %s", requestID)
			}

		case <-sigCh:
			log.Println("Shutdown signal received, exiting...")
			return
		}
	}
}
