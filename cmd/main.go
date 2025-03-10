package main

import (
	"container/list"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/iamramtin/binance-trader/internal/api"
	"github.com/iamramtin/binance-trader/internal/trader"
	"github.com/iamramtin/binance-trader/internal/utils"
)

func main() {
	log.Println("Starting Binance WebSocket trading application...")

	url := "wss://testnet.binance.vision/ws-api/v3"
	apiKey := os.Getenv("BINANCE_API_KEY")
	secretKey := os.Getenv("BINANCE_SECRET_KEY")

	if err := utils.AuthenticateAPIKeys(apiKey, secretKey); err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}

	// TODO: add balance check
	symbol := "BTCTUSD"
	limit := 5 // levels to request

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := api.New(url, apiKey, secretKey, symbol)
	if err := client.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer client.Close()

	// Test the signature if API keys are provided
	log.Println("Testing API key and signature...")
	if err := client.TestSignature(); err != nil {
		log.Printf("Signature test failed: %v", err)
		os.Exit(1)
	}
	log.Println("Signature test passed")

	// Get the orderbook to verify connectivity
	orderbook, err := client.GetOrderbook(limit)
	if err != nil {
		log.Printf("Failed to get orderbook: %v", err)
		os.Exit(1)
	} else {
		client.DisplayOrderbook(orderbook, limit)
	}

	fmt.Println("\nChoose operating mode:")
	fmt.Println("1. Manual mode - Place individual test orders")
	fmt.Println("2. Market maker mode - Continuously place bid/ask orders at a spread")
	fmt.Print("Enter choice (1 or 2): ")

	var choice string
	fmt.Scanln(&choice)

	orderBookTicker := time.NewTicker(10 * time.Second)
	manualTradeTicker := time.NewTicker(15 * time.Second)
	manualCancelTicker := time.NewTicker(30 * time.Second)
	marketMakerTicker := time.NewTicker(10 * time.Second)

	defer func() {
		if orderBookTicker != nil {
			orderBookTicker.Stop()
		}
		if manualTradeTicker != nil {
			manualTradeTicker.Stop()
		}
		if manualCancelTicker != nil {
			manualCancelTicker.Stop()
		}
		if marketMakerTicker != nil {
			marketMakerTicker.Stop()
		}
	}()

	// Manual
	var manualOrderQueue *list.List
	var manualMutex sync.Mutex

	// Market Maker
	var marketMaker *trader.MarketMaker
	marketMakerActive := false

	if choice == "1" {
		log.Println("Running in manual mode - placing test orders")
		manualOrderQueue = list.New()
	} else {
		log.Println("Starting market maker strategy...")

		// TODO: determine tick & quantity size - for now use 0.01 and 0.0001 for BTCTUSD
		marketMaker = trader.New(client, symbol, 1.0, "0.0001", "0.01")
		marketMaker.Start()
		marketMakerActive = true
	}

	log.Printf("Application running. Trading %s. Press Ctrl+C to exit.", symbol)

	for {
		select {
		case <-orderBookTicker.C:
			orderbook, err := client.GetOrderbook(limit)
			if err != nil {
				log.Printf("Failed to get orderbook: %v", err)
				continue
			}

			client.DisplayOrderbook(orderbook, limit)

		case <-manualTradeTicker.C:
			if choice != "1" {
				continue
			}

			orderID := placeTestOrder(client, "MARKET", symbol, limit, ctx)
			if orderID != -1 {
				manualMutex.Lock()
				manualOrderQueue.PushBack(orderID)
				manualMutex.Unlock()
			}

		case <-manualCancelTicker.C:
			if choice != "1" {
				continue
			}

			manualMutex.Lock()
			if manualOrderQueue.Len() > 0 {
				oldestOrder := manualOrderQueue.Front()
				orderID, ok := oldestOrder.Value.(int64)
				if !ok {
					fmt.Println("Failed to convert order ID to int64")
					manualMutex.Unlock()
					continue
				}

				fmt.Println("Dequeuing oldest order:", orderID)
				go cancelTestOrder(client, orderID, ctx)
				manualOrderQueue.Remove(oldestOrder)
			}
			manualMutex.Unlock()

		case <-marketMakerTicker.C:
			if choice == "1" || !marketMakerActive || marketMaker == nil {
				continue
			}

			client.GetOrderManager().PrintOrderSummary()

		case <-sigCh:
			log.Println("Shutdown signal received, exiting...")

			// Stop the market maker if active
			if choice != "1" && marketMakerActive && marketMaker != nil {
				log.Println("Stopping market maker strategy...")
				marketMaker.Stop()
			}

			return
		}
	}
}

func placeTestOrder(client *api.BinanceClient, orderType string, symbol string, limit int, ctx context.Context) int64 {
	select {
	case <-ctx.Done():
		return -1
	default:
	}

	orderbook, err := client.GetOrderbook(limit)
	if err != nil {
		log.Printf("Failed to get orderbook: %v", err)
		return -1
	}

	if len(orderbook.Asks) > 0 {
		askPrice := orderbook.Asks[0].Price
		buyPrice := utils.FormatPrice(askPrice*0.99, "0.01") // 1% below the lowest ask
		buyQty := "0.001"                                    // Small quantity for testing

		order, err := client.PlaceOrder("BUY", orderType, buyPrice, buyQty)
		if err != nil {
			log.Printf("Failed to place order: %v", err)
			return -1
		}

		log.Printf("%s order placed successfully: ID=%d, Status=%s", orderType, order.OrderID, order.Status)

		client.GetOrderManager().PrintOrderSummary()

		return order.OrderID
	}

	return -1
}

func cancelTestOrder(client *api.BinanceClient, orderID int64, ctx context.Context) {
	select {
	case <-ctx.Done():
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

		canceledOrder, err := client.CancelOrder(orderID)
		if err != nil {
			log.Printf("Failed to cancel order: %v", err)
			return
		}

		log.Printf("Order canceled: ID=%d, Status=%s", canceledOrder.OrderID, canceledOrder.Status)
	} else {
		log.Printf("Order %d is already in final state: %s", orderID, order.Status)
	}
}

// TODO: simulate checking account balance before placing the order

// TODO: Handle WebSocket Reconnection
// If Binance disconnects the WebSocket, your bot should reconnect automatically

// TODO: Improve Error Handling
// If Binance API fails, nothing happens. Add retry logic for failures.

// TODO: Concurrency improvements (manage multiple trades at once)

// TODO: Adjust strategies to account for changes in tick size

// TODO: Use GET /api/v3/exchangeInfo for the latest tick size
