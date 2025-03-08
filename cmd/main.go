package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/iamramtin/binance-trader/internal/api"
	"github.com/iamramtin/binance-trader/internal/models"
)

func main() {
	log.Println("Starting Binance WebSocket trading application...")

	symbol := "BTCUSDT"
	limit := 5 // levels to request

	url := "wss://testnet.binance.vision/ws-api/v3"
	apiKey := os.Getenv("BINANCE_API_KEY")
	secretKey := os.Getenv("BINANCE_SECRET_KEY")

	if apiKey == "" || secretKey == "" {
		log.Println("Warning: API key and/or secret key not provided. Order operations will not work.")
		log.Println("Set BINANCE_API_KEY and BINANCE_SECRET_KEY environment variables.")
	}

	client := api.New(url, apiKey, secretKey, symbol)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer client.Close()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	orderManager := client.GetOrderManager()

	placeTestOrder(client, limit, symbol, sigCh)

	// Ticker for order manager status
	statusTicker := time.NewTicker(30 * time.Second)
	defer statusTicker.Stop()

	log.Printf("Application running. Trading %s. Press Ctrl+C to exit.", symbol)

	// Main application loop
	for {
		select {
		case <-statusTicker.C:
			orderManager.PrintOrderSummary()

			orderbook, err := client.GetOrderbook(limit)
			if err != nil {
				log.Printf("Failed to get orderbook: %v", err)
			} else {
				displayOrderbook(orderbook, limit)

			}

		case <-sigCh:
			log.Println("Shutdown signal received, exiting...")
			return
		}
	}

}

func displayOrderbook(book *models.ParsedOrderBook, limit int) {
	log.Printf("Orderbook LastUpdateID: %d", book.LastUpdateID)

	log.Println("Asks (Sell Orders):")
	log.Println("Price\t\tQuantity")

	maxAsks := min(len(book.Asks), limit)
	for i := range maxAsks {
		log.Printf("%.8f\t%.8f", book.Asks[i].Price, book.Asks[i].Quantity)
	}

	log.Println()
	log.Println("Bids (Buy Orders):")
	log.Println("Price\t\tQuantity")

	maxBids := min(len(book.Bids), limit)
	for i := range maxBids {
		log.Printf("%.8f\t%.8f", book.Bids[i].Price, book.Bids[i].Quantity)
	}
}

func placeTestOrder(client *api.BinanceClient, limit int, symbol string, sigCh chan os.Signal) {
	// TODO: subscribe to the orderbook WebSocket stream for real-time updates
	// Consider adding something like client.SubscribeOrderbook() to keep orderbook data live
	orderbook, err := client.GetOrderbook(limit)

	if err != nil {
		log.Printf("Failed to get orderbook: %v", err)
	} else {
		log.Println("Current Orderbook:")
		displayOrderbook(orderbook, limit)

		// TODO: Replace with (advanced) trading logic
		if len(orderbook.Asks) > 0 {
			askPrice := orderbook.Asks[0].Price
			buyPrice := formatPrice(askPrice*0.99, "0.01") // 1% below the lowest ask
			buyQty := "0.0001"                             // Small quantity for testing

			log.Printf("Placing test LIMIT BUY order: %s %s @ %s", symbol, buyQty, buyPrice)

			// TODO: Ensure qty at that price exists
			order, err := client.PlaceOrder("BUY", "LIMIT", buyPrice, buyQty)
			if err != nil {
				log.Printf("Failed to place order: %v", err)
			} else {
				log.Printf("Order placed successfully: ID=%d, Status=%s", order.OrderID, order.Status)

				go func(orderID int64) {
					time.Sleep(1 * time.Minute)

					// Check if the application is still running
					select {
					case <-sigCh:
						return
					default:
					}

					// Check if the order is still active
					order, err := client.GetOrderStatus(orderID)
					if err != nil {
						log.Printf("Failed to get order status: %v", err)
						return
					}

					if order.Status == "NEW" || order.Status == "PARTIALLY_FILLED" {
						log.Printf("Canceling test order: %d", orderID)

						// TODO: Replace with real-time listener to auto-cancel based on live events / criteria
						canceledOrder, err := client.CancelOrder(orderID)
						if err != nil {
							log.Printf("Failed to cancel order: %v", err)
							return
						}

						log.Printf("Order canceled: ID=%d, Status=%s", canceledOrder.OrderID, canceledOrder.Status)
					} else {
						log.Printf("Order %d is already in final state: %s", orderID, order.Status)
					}
				}(order.OrderID)
			}
		}
	}
}

// FormatPrice formats a price according to the tick size rules
func formatPrice(price float64, tickSize string) string {
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

// TODO: simulate checking account balance before placing the order

// TODO: Handle WebSocket Reconnection
// If Binance disconnects the WebSocket, your bot should reconnect automatically

// TODO: Improve Error Handling
// If Binance API fails, nothing happens. Add retry logic for failures.

// TODO: Concurrency improvements (manage multiple trades at once)

// TODO: Adjust strategies to account for changes in tick size
