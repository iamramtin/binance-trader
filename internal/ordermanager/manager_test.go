package ordermanager

import (
	"testing"

	"github.com/iamramtin/binance-trader/internal/models"
)

func TestTrackOrder(t *testing.T) {
	manager := New()

	// Create a test order
	order := &models.Order{
		Symbol:        "BTCUSDT",
		OrderID:       12345,
		ClientOrderID: "test123",
		Status:        "NEW",
		Side:          "BUY",
		Price:         "10000.00",
		OrigQty:       "1.0",
	}

	// Track the order
	manager.TrackOrder(order)

	// Verify the order was tracked
	trackedOrder, err := manager.GetOrder(12345)
	if err != nil {
		t.Errorf("GetOrder() returned error: %v", err)
	}

	if trackedOrder.OrderID != order.OrderID {
		t.Errorf("GetOrder() OrderID = %d; want %d", trackedOrder.OrderID, order.OrderID)
	}

	if trackedOrder.Status != order.Status {
		t.Errorf("GetOrder() Status = %s; want %s", trackedOrder.Status, order.Status)
	}
}

func TestUpdateOrder(t *testing.T) {
	manager := New()

	// Create and track an initial order
	initialOrder := &models.Order{
		Symbol:        "BTCUSDT",
		OrderID:       12345,
		ClientOrderID: "test123",
		Status:        "NEW",
		Side:          "BUY",
		Price:         "10000.00",
		OrigQty:       "1.0",
	}
	manager.TrackOrder(initialOrder)

	// Create an updated order
	updatedOrder := &models.Order{
		Symbol:        "BTCUSDT",
		OrderID:       12345,
		ClientOrderID: "test123",
		Status:        "PARTIALLY_FILLED",
		Side:          "BUY",
		Price:         "10000.00",
		OrigQty:       "1.0",
		ExecutedQty:   "0.5",
	}

	// Update the order
	err := manager.UpdateOrder(updatedOrder)
	if err != nil {
		t.Errorf("UpdateOrder() returned error: %v", err)
	}

	// Verify the order was updated
	trackedOrder, err := manager.GetOrder(12345)
	if err != nil {
		t.Errorf("GetOrder() returned error: %v", err)
	}

	if trackedOrder.Status != "PARTIALLY_FILLED" {
		t.Errorf("GetOrder() Status = %s; want %s", trackedOrder.Status, "PARTIALLY_FILLED")
	}

	if trackedOrder.ExecutedQty != "0.5" {
		t.Errorf("GetOrder() ExecutedQty = %s; want %s", trackedOrder.ExecutedQty, "0.5")
	}
}

func TestGetOrdersByStatus(t *testing.T) {
	manager := New()

	// Create and track several orders with different statuses
	orders := []*models.Order{
		{OrderID: 1, Status: "NEW", Symbol: "BTCUSDT"},
		{OrderID: 2, Status: "FILLED", Symbol: "BTCUSDT"},
		{OrderID: 3, Status: "CANCELED", Symbol: "BTCUSDT"},
		{OrderID: 4, Status: "NEW", Symbol: "BTCUSDT"},
		{OrderID: 5, Status: "FILLED", Symbol: "ETHUSDT"},
	}

	for _, order := range orders {
		manager.TrackOrder(order)
	}

	// Test getting orders by status
	newOrders := manager.GetOrdersByStatus("NEW")
	if len(newOrders) != 2 {
		t.Errorf("GetOrdersByStatus(\"NEW\") returned %d orders; want 2", len(newOrders))
	}

	filledOrders := manager.GetOrdersByStatus("FILLED")
	if len(filledOrders) != 2 {
		t.Errorf("GetOrdersByStatus(\"FILLED\") returned %d orders; want 2", len(filledOrders))
	}

	canceledOrders := manager.GetOrdersByStatus("CANCELED")
	if len(canceledOrders) != 1 {
		t.Errorf("GetOrdersByStatus(\"CANCELED\") returned %d orders; want 1", len(canceledOrders))
	}
}
