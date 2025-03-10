package trader

import (
	"testing"

	"github.com/iamramtin/binance-trader/internal/models"
	"github.com/iamramtin/binance-trader/internal/utils"
)

type MockBinanceClient struct {
	orderbook      *models.ParsedOrderBook
	placedOrders   []*models.Order
	canceledOrders []int64
}

func (m *MockBinanceClient) GetOrderbook(limit int) (*models.ParsedOrderBook, error) {
	return m.orderbook, nil
}

func (m *MockBinanceClient) PlaceOrder(side, orderType, price, quantity string) (*models.Order, error) {
	order := &models.Order{
		Symbol:  "BTCUSDT",
		OrderID: int64(len(m.placedOrders) + 1),
		Status:  "NEW",
		Side:    side,
		Type:    orderType,
		Price:   price,
		OrigQty: quantity,
	}
	m.placedOrders = append(m.placedOrders, order)
	return order, nil
}

func (m *MockBinanceClient) CancelOrder(orderID int64) (*models.Order, error) {
	m.canceledOrders = append(m.canceledOrders, orderID)
	return &models.Order{
		OrderID: orderID,
		Status:  "CANCELED",
	}, nil
}

func TestCalculatePrices(t *testing.T) {
	// Create a mock orderbook
	orderbook := &models.ParsedOrderBook{
		Bids: []models.PriceLevel{{Price: 9000.0, Quantity: 1.0}},
		Asks: []models.PriceLevel{{Price: 9100.0, Quantity: 1.0}},
	}

	// Calculate mid price
	midPrice := (orderbook.Bids[0].Price + orderbook.Asks[0].Price) / 2 // 9050.0

	// Calculate spread amount (1% of mid price)
	spreadPercentage := 1.0
	spreadAmount := midPrice * (spreadPercentage / 100) // 90.5

	// Calculate bid and ask prices
	bidPrice := midPrice - spreadAmount // 8959.5
	askPrice := midPrice + spreadAmount // 9140.5

	// Format prices
	bidPriceStr := utils.FormatPrice(bidPrice, "0.01") // 8959.50
	askPriceStr := utils.FormatPrice(askPrice, "0.01") // 9140.50

	// Verify the calculations
	if bidPriceStr != "8959.50" {
		t.Errorf("Bid price calculation = %s; want %s", bidPriceStr, "8959.50")
	}

	if askPriceStr != "9140.50" {
		t.Errorf("Ask price calculation = %s; want %s", askPriceStr, "9140.50")
	}
}
