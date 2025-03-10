package trader

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"maps"

	"github.com/iamramtin/binance-trader/internal/api"
	"github.com/iamramtin/binance-trader/internal/utils"
)

// Implement simple market making strategy
type MarketMaker struct {
	client           *api.BinanceClient // WebSocket API client
	symbol           string             // Trading symbol
	spreadPercentage float64            // Spread percentage from mid price (e.g., 0.5 for 0.5%)
	orderQty         string             // Quantity of each order
	tickSize         string             // Price tick size for the symbol
	active           bool               // Whether the trader is currently active
	activeOrders     map[int64]string   // Map of active order IDs to side (BUY/SELL)
	mu               sync.RWMutex       // Mutex for thread safety
	ctx              context.Context    // Context for cancellation
	cancel           context.CancelFunc // Cancel function for the context
}

func New(client *api.BinanceClient, symbol string, spreadPercentage float64, orderQty string, tickSize string) *MarketMaker {
	ctx, cancel := context.WithCancel(context.Background())

	return &MarketMaker{
		client:           client,
		symbol:           symbol,
		spreadPercentage: spreadPercentage,
		orderQty:         orderQty,
		tickSize:         tickSize,
		active:           false,
		activeOrders:     make(map[int64]string),
		ctx:              ctx,
		cancel:           cancel,
	}
}

func (m *MarketMaker) IsActive() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.active
}

func (m *MarketMaker) Start() {
	m.mu.Lock()
	if m.active {
		m.mu.Unlock()
		log.Println("Market maker is already running")
		return
	}

	m.active = true
	m.mu.Unlock()

	go m.tradingLoop()
}

func (m *MarketMaker) Stop() {
	m.mu.Lock()
	if !m.active {
		m.mu.Unlock()
		log.Println("Market maker is not running")
		return
	}

	m.active = false
	m.cancel()

	activeOrdersRead := make(map[int64]string)
	maps.Copy(activeOrdersRead, m.activeOrders)

	// clear active orders
	m.activeOrders = make(map[int64]string)
	m.mu.Unlock()

	log.Println("Stopping market maker and canceling all orders")

	for orderID, order := range activeOrdersRead {
		log.Printf("Canceling %s order %d", order, orderID)

		_, err := m.client.CancelOrder(orderID)
		if err != nil {
			log.Printf("Failed to cancel order %d: %v", orderID, err)
		}
	}

	m.mu.Lock()
}

func (m *MarketMaker) tradingLoop() {
	log.Printf("Starting market maker for %s with %.2f%% spread", m.symbol, m.spreadPercentage)

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// Initial market state update
	if err := m.updateMarketState(); err != nil {
		log.Printf("Initial market state update failed: %v", err)
	}

	for {
		select {
		case <-ticker.C:
			if !m.IsActive() {
				return
			}

			// Update market state and place orders
			if err := m.updateMarketState(); err != nil {
				log.Printf("Failed to update market state: %v", err)
				continue
			}

		case <-m.ctx.Done():
			log.Println("Trading loop stopped due to context cancellation")
			return
		}
	}
}

func (m *MarketMaker) updateMarketState() error {
	orderbook, err := m.client.GetOrderbook(10)
	if err != nil {
		return fmt.Errorf("failed to get orderbook: %w", err)
	}

	if len(orderbook.Asks) == 0 || len(orderbook.Bids) == 0 {
		return fmt.Errorf("empty orderbook")
	}

	highestBidPrice := orderbook.Bids[0].Price
	lowestAskPrice := orderbook.Asks[0].Price

	midPrice := (lowestAskPrice + highestBidPrice) / 2
	spreadAmount := midPrice * (m.spreadPercentage / 100)

	bidPrice := midPrice - spreadAmount
	askPrice := midPrice + spreadAmount

	log.Printf("Market: Bid=%.8f, Ask=%.8f, Mid=%.8f", highestBidPrice, lowestAskPrice, midPrice)

	askPriceStr := utils.FormatPrice(askPrice, m.tickSize)
	bidPriceStr := utils.FormatPrice(bidPrice, m.tickSize)

	log.Printf("Our prices: Bid=%s, Ask=%s", bidPriceStr, askPriceStr)

	if err := m.refreshOrders(askPriceStr, bidPriceStr); err != nil {
		return fmt.Errorf("failed to refresh orders: %w", err)
	}

	return nil
}

func (m *MarketMaker) refreshOrders(askPrice string, bidPrice string) error {
	m.mu.RLock()
	activeOrdersRead := make(map[int64]string)
	maps.Copy(activeOrdersRead, m.activeOrders)
	m.mu.RUnlock()

	for orderID, order := range activeOrdersRead {
		log.Printf("Canceling %s order %d", order, orderID)

		_, err := m.client.CancelOrder(orderID)
		if err != nil {
			log.Printf("Failed to cancel order %d: %v", orderID, err)
			// TODO: Continue with other orders even if one fails
		}

		m.mu.Lock()
		delete(m.activeOrders, orderID)
		m.mu.Unlock()
	}

	if err := m.placeNewOrder("BUY", "LIMIT", bidPrice, m.orderQty); err != nil {
		return fmt.Errorf("failed to place new bid orders: %w", err)
	}

	// Wait to avoid rate limits
	time.Sleep(200 * time.Millisecond)

	if err := m.placeNewOrder("SELL", "LIMIT", askPrice, m.orderQty); err != nil {
		return fmt.Errorf("failed to place new ask orders: %w", err)
	}

	m.client.GetOrderManager().PrintOrderSummary()

	return nil
}

func (m *MarketMaker) placeNewOrder(side string, orderType string, price string, qty string) error {
	if !m.IsActive() {
		return fmt.Errorf("market maker stopped while refreshing orders")
	}

	order, err := m.client.PlaceOrder(side, orderType, price, qty)
	if err != nil {
		return fmt.Errorf("failed to place %s order: %w", side, err)
	}

	log.Printf("Placed %s order: %d (%s @ %s)", side, order.OrderID, qty, price)

	m.mu.Lock()
	m.activeOrders[order.OrderID] = side
	m.mu.Unlock()

	return nil
}
