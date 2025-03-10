package main

import (
	"container/list"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/iamramtin/binance-trader/internal/api"
	"github.com/iamramtin/binance-trader/internal/trader"
	"github.com/iamramtin/binance-trader/internal/utils"
)

type Timers struct {
	OrderBook    *time.Ticker
	ManualTrade  *time.Ticker
	ManualCancel *time.Ticker
	OrderSummary *time.Ticker
}

type Config struct {
	Symbol           string
	Quantity         float64
	SpreadPercentage float64
	Price            string
	TickSize         string
	OrderbookDepth   int
	WebSocketURL     string
	APIKey           string
	SecretKey        string
}

type TradingComponents struct {
	ManualOrderQueue  *list.List
	ManualMutex       sync.Mutex
	MarketMaker       *trader.MarketMaker
	MarketMakerActive bool
}

func main() {
	log.Println("Starting Binance WebSocket trading application...")

	config := &Config{
		WebSocketURL:     "wss://testnet.binance.vision/ws-api/v3",
		APIKey:           os.Getenv("BINANCE_API_KEY"),
		SecretKey:        os.Getenv("BINANCE_SECRET_KEY"),
		Symbol:           "BTCTUSD",
		Quantity:         0.001,
		SpreadPercentage: 0.0001,
		OrderbookDepth:   5,
		Price:            "0.01",
		TickSize:         "0.01",
	}

	choice := getUserPrompt(config)

	if err := validateConfig(config); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := api.New(config.WebSocketURL, config.APIKey, config.SecretKey, config.Symbol)
	if err := client.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer client.Close()

	// Test the signature if API keys are provided
	if err := testAuthentication(client, config); err != nil {
		log.Fatalf("Authentication failed: %v", err)
		os.Exit(1)
	}

	printAccountBalance(client)

	timers := setupTimers()
	defer stopTimers(timers)

	components := initTradingComponents(choice, client, config)

	log.Printf("Application running. Trading %s. Press Ctrl+C to exit.", config.Symbol)

	for {
		select {
		case <-timers.OrderBook.C:
			printOrderBook(client, config.OrderbookDepth)

		case <-timers.OrderSummary.C:
			client.GetOrderManager().PrintOrderSummary()

		case <-timers.ManualTrade.C:
			if choice != "1" {
				continue
			}

			orderID := placeTestOrder(client, "MARKET", config.Symbol, fmt.Sprintf("%f", config.Quantity), config.OrderbookDepth, ctx)
			if orderID != -1 {
				components.ManualMutex.Lock()
				components.ManualOrderQueue.PushBack(orderID)
				components.ManualMutex.Unlock()
			}

		case <-timers.ManualCancel.C:
			if choice != "1" {
				continue
			}

			handleManualOrderCancellation(components, client, ctx)

		case <-sigCh:
			log.Println("Shutdown signal received, exiting...")

			if choice != "1" && components.MarketMakerActive && components.MarketMaker != nil {
				log.Println("Stopping market maker strategy...")
				components.MarketMaker.Stop()
			}

			return
		}
	}
}

func promptForConfig(config *Config) *Config {
	fmt.Println("\nEnter trading parameters (press Enter to use defaults):")

	fmt.Printf("Symbol [%s]: ", config.Symbol)
	var input string
	fmt.Scanln(&input)
	if strings.TrimSpace(input) != "" {
		config.Symbol = strings.ToUpper(strings.TrimSpace(input))
	}

	fmt.Printf("Quantity [%f]: ", config.Quantity)
	fmt.Scanln(&input)
	if strings.TrimSpace(input) != "" {
		if val, err := strconv.ParseFloat(input, 64); err == nil && val > 0 {
			config.Quantity = val
		} else {
			log.Printf("Invalid quantity, using default: %f", config.Quantity)
		}
	}

	fmt.Printf("Spread Percentage [%f]: ", config.SpreadPercentage)
	fmt.Scanln(&input)
	fmt.Println()
	if strings.TrimSpace(input) != "" {
		if val, err := strconv.ParseFloat(input, 64); err == nil && val > 0 {
			config.SpreadPercentage = val
		} else {
			log.Printf("Invalid spread, using default: %f", config.SpreadPercentage)
		}
	}

	return config
}

func validateConfig(config *Config) error {
	if config.Symbol == "" {
		return fmt.Errorf("trading symbol cannot be empty")
	}

	if config.Quantity <= 0 {
		return fmt.Errorf("quantity must be greater than 0")
	}

	if config.SpreadPercentage <= 0 {
		return fmt.Errorf("spread percentage must be greater than 0")
	}

	return utils.AuthenticateAPIKeys(config.APIKey, config.SecretKey)
}

func testAuthentication(client *api.BinanceClient, config *Config) error {
	log.Println("Testing API key and signature...")
	if err := client.TestSignature(); err != nil {
		return err
	}

	log.Println("Signature test passed")

	// Get the orderbook to verify connectivity
	orderbook, err := client.GetOrderbook(config.OrderbookDepth)
	if err != nil {
		return fmt.Errorf("failed to get orderbook: %v", err)
	}

	client.DisplayOrderbook(orderbook, config.OrderbookDepth)
	return nil
}

func getUserPrompt(config *Config) string {
	fmt.Println("\nEnter trading parameters (press Enter to use default values):")

	// Symbol
	fmt.Printf("Symbol [%s]: ", config.Symbol)
	var input string
	fmt.Scanln(&input)
	if strings.TrimSpace(input) != "" {
		config.Symbol = strings.ToUpper(strings.TrimSpace(input))
	}

	// Quantity
	fmt.Printf("Quantity [%f]: ", config.Quantity)
	fmt.Scanln(&input)
	if strings.TrimSpace(input) != "" {
		if val, err := strconv.ParseFloat(input, 64); err == nil && val > 0 {
			config.Quantity = val
		} else {
			fmt.Printf("Invalid quantity '%s', using default: %f\n", input, config.Quantity)
		}
	}

	fmt.Println("\nChoose operating mode:")
	fmt.Println("1. Manual mode - Place individual test market orders")
	fmt.Println("2. Basic market maker - Continuously place bid/ask orders at a fixed spread")
	fmt.Print("Enter choice (1 or 2): ")

	var choice string
	fmt.Scanln(&choice)
	fmt.Println()

	if choice == "2" || choice == "3" {
		fmt.Printf("Spread Percentage [%f]: ", config.SpreadPercentage)
		var input string
		fmt.Scanln(&input)
		if strings.TrimSpace(input) != "" {
			if val, err := strconv.ParseFloat(input, 64); err == nil && val > 0 {
				config.SpreadPercentage = val
			} else {
				fmt.Printf("Invalid spread percentage '%s', using default: %f\n", input, config.SpreadPercentage)
			}
		}
	}

	return choice
}

func setupTimers() *Timers {
	return &Timers{
		OrderBook:    time.NewTicker(10 * time.Second),
		OrderSummary: time.NewTicker(10 * time.Second),
		ManualTrade:  time.NewTicker(15 * time.Second),
		ManualCancel: time.NewTicker(30 * time.Second),
	}
}

func stopTimers(timers *Timers) {
	if timers.OrderBook != nil {
		timers.OrderBook.Stop()
	}
	if timers.ManualTrade != nil {
		timers.ManualTrade.Stop()
	}
	if timers.ManualCancel != nil {
		timers.ManualCancel.Stop()
	}
	if timers.OrderSummary != nil {
		timers.OrderSummary.Stop()
	}
}

func initTradingComponents(choice string, client *api.BinanceClient, config *Config) *TradingComponents {
	components := &TradingComponents{
		MarketMakerActive: false,
	}

	if choice == "2" {
		log.Println("\nStarting basic market maker strategy...")
		log.Printf("Using spread percentage: %f, quantity: %f", config.SpreadPercentage, config.Quantity)

		components.MarketMaker = trader.New(
			client,
			config.Symbol,
			config.SpreadPercentage,
			fmt.Sprintf("%f", config.Quantity),
			config.TickSize,
		)

		components.MarketMaker.Start()
		components.MarketMakerActive = true
	} else {
		log.Println("\nRunning in manual mode - placing test orders")
		components.ManualOrderQueue = list.New()
	}

	return components
}

func printAccountBalance(client *api.BinanceClient) {
	balance, err := client.GetAccountBalance()
	if err != nil {
		log.Printf("Failed to get account balance: %v", err)
		return
	}

	client.DisplayAccountBalance(balance)
}

func printOrderBook(client *api.BinanceClient, depth int) {
	orderbook, err := client.GetOrderbook(depth)
	if err != nil {
		log.Printf("Failed to get orderbook: %v", err)
		return
	}

	client.DisplayOrderbook(orderbook, depth)
}

func handleManualOrderCancellation(components *TradingComponents, client *api.BinanceClient, ctx context.Context) {
	components.ManualMutex.Lock()
	defer components.ManualMutex.Unlock()

	if components.ManualOrderQueue.Len() > 0 {
		oldestOrder := components.ManualOrderQueue.Front()
		orderID, ok := oldestOrder.Value.(int64)
		if !ok {
			fmt.Println("Failed to convert order ID to int64")
			return
		}

		fmt.Println("Dequeuing oldest order:", orderID)
		go cancelTestOrder(client, orderID, ctx)
		components.ManualOrderQueue.Remove(oldestOrder)
	}
}

func placeTestOrder(client *api.BinanceClient, orderType string, symbol string, quantity string, limit int, ctx context.Context) int64 {
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

		order, err := client.PlaceOrder("BUY", orderType, buyPrice, quantity)
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
