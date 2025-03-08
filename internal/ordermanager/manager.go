package ordermanager

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/iamramtin/binance-trader/internal/models"
)

// Current state of an order
type OrderState struct {
	Order models.Order // The order details
	// TODO: add CreatedTime and infer Updated from UpdatedTime
	LastUpdateTime time.Time // Time of last update
	Updated        bool      // Whether the order has been updated
}

// Track and manage orders
type Manager struct {
	orders       map[int64]*OrderState  // Map of orderID to OrderState
	clientOrders map[string]*OrderState // Map of clientOrderID to OrderState
	mu           sync.RWMutex           // Mutex for thread safety
}

func New() *Manager {
	return &Manager{
		orders:       make(map[int64]*OrderState),
		clientOrders: make(map[string]*OrderState),
	}
}

// Add a new order to be tracked
func (manager *Manager) TrackOrder(order *models.Order) {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	// Create the order state
	state := &OrderState{
		Order:          *order,
		LastUpdateTime: time.Now(),
		Updated:        false,
	}

	// Store by order ID
	manager.orders[order.OrderID] = state

	// Also store by client order ID if available
	if order.ClientOrderID != "" {
		manager.clientOrders[order.ClientOrderID] = state
	}

	log.Printf("Tracking new order: %d (%s)", order.OrderID, order.ClientOrderID)
}

// Update an existing order
func (manager *Manager) UpdateOrder(order *models.Order) error {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	state, exists := manager.orders[order.OrderID]
	if !exists {
		state, exists = manager.clientOrders[order.ClientOrderID]
		if !exists {
			return fmt.Errorf("order not found: %d (%s)", order.OrderID, order.ClientOrderID)
		}
	}

	state.Order = *order
	state.LastUpdateTime = time.Now()
	state.Updated = true

	log.Printf("Updated order %d (%s) status: %s", order.OrderID, order.ClientOrderID, order.Status)
	return nil
}

// Retrieve an order
func (manager *Manager) GetOrder(orderID int64) (*models.Order, error) {
	manager.mu.RLock()
	defer manager.mu.RUnlock()

	state, exists := manager.orders[orderID]
	if !exists {
		if !exists {
			return nil, fmt.Errorf("order not found: %d", orderID)
		}
	}

	return &state.Order, nil
}

// Retrieve an order by client ID
func (manager *Manager) GetClientOrders(clientOrderID string) (*models.Order, error) {
	manager.mu.RLock()
	defer manager.mu.RUnlock()

	state, exists := manager.clientOrders[clientOrderID]
	if !exists {
		if !exists {
			return nil, fmt.Errorf("order not found: %s", clientOrderID)
		}
	}

	return &state.Order, nil
}

// Retrieve all orders
func (manager *Manager) GetAllOrders() []models.Order {
	manager.mu.RLock()
	defer manager.mu.RUnlock()

	orders := make([]models.Order, 0, len(manager.orders))
	for _, state := range manager.orders {
		orders = append(orders, state.Order)
	}

	return orders
}

// Return all orders with the specified status
func (manager *Manager) GetOrdersByStatus(status models.OrderStatus) []models.Order {
	manager.mu.RLock()
	defer manager.mu.RUnlock()

	orders := make([]models.Order, 0, len(manager.orders))
	for _, state := range manager.orders {
		if models.OrderStatus(state.Order.Status) == status {
			orders = append(orders, state.Order)
		}
	}

	return orders
}

// Return all orders with any of the specified statuses
func (manager *Manager) GetOrdersByStatuses(statuses []models.OrderStatus) []models.Order {
	manager.mu.RLock()
	defer manager.mu.RUnlock()

	statusSet := make(map[models.OrderStatus]struct{}, len(statuses))
	for _, status := range statuses {
		statusSet[status] = struct{}{}
	}

	orders := make([]models.Order, 0, len(manager.orders))
	for _, state := range manager.orders {
		if _, exists := statusSet[models.OrderStatus(state.Order.Status)]; exists {
			orders = append(orders, state.Order)
		}
	}

	return orders
}

// Return all active orders (not filled, canceled, or rejected)
func (manager *Manager) GetActiveOrders() []models.Order {
	manager.mu.RLock()
	defer manager.mu.RUnlock()

	return manager.GetOrdersByStatuses([]models.OrderStatus{models.OrderStatusNew, models.OrderStatusPartiallyFilled})
}

// Return all executed (filled) orders
func (manager *Manager) GetExecutedOrders() []models.Order {
	return manager.GetOrdersByStatus(models.OrderStatusFilled)
}

// Return all canceled orders
func (manager *Manager) GetCanceledOrders() []models.Order {
	return manager.GetOrdersByStatus(models.OrderStatusCanceled)
}

// Remove an order from tracking
func (manager *Manager) RemoveOrder(orderID int64) error {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	order, err := manager.GetOrder(orderID)
	if err != nil {
		return err
	}

	delete(manager.orders, orderID)
	if order.ClientOrderID != "" {
		delete(manager.clientOrders, order.ClientOrderID)
	}

	log.Printf("Removed order from tracking: %d", orderID)
	return nil
}

// Print a summary of the current orders
func (manager *Manager) PrintOrderSummary() {
	manager.mu.RLock()
	defer manager.mu.RUnlock()

	log.Println("===== ORDER SUMMARY =====")
	log.Printf("Total Orders: %d", len(manager.orders))

	// Count by status
	statusCounts := make(map[string]int)
	for _, state := range manager.orders {
		statusCounts[state.Order.Status]++
	}

	for status, count := range statusCounts {
		log.Printf("Status %s: %d orders", status, count)
	}

	log.Println("=========================")
}
