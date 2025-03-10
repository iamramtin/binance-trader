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
func (m *Manager) TrackOrder(order *models.Order) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create the order state
	state := &OrderState{
		Order:          *order,
		LastUpdateTime: time.Now(),
		Updated:        false,
	}

	// Store by order ID
	m.orders[order.OrderID] = state

	// Also store by client order ID if available
	if order.ClientOrderID != "" {
		m.clientOrders[order.ClientOrderID] = state
	}

	log.Printf("Tracking new order: %d (%s)", order.OrderID, order.ClientOrderID)
}

// Update an existing order
func (m *Manager) UpdateOrder(order *models.Order) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.orders[order.OrderID]
	if !exists {
		state, exists = m.clientOrders[order.ClientOrderID]
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
func (m *Manager) GetOrder(orderID int64) (*models.Order, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.orders[orderID]
	if !exists {
		if !exists {
			return nil, fmt.Errorf("order not found: %d", orderID)
		}
	}

	return &state.Order, nil
}

// Retrieve an order by client ID
func (m *Manager) GetClientOrders(clientOrderID string) (*models.Order, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.clientOrders[clientOrderID]
	if !exists {
		if !exists {
			return nil, fmt.Errorf("order not found: %s", clientOrderID)
		}
	}

	return &state.Order, nil
}

// Retrieve all orders
func (m *Manager) GetAllOrders() []models.Order {
	m.mu.RLock()
	defer m.mu.RUnlock()

	orders := make([]models.Order, 0, len(m.orders))
	for _, state := range m.orders {
		orders = append(orders, state.Order)
	}

	return orders
}

// Return all orders with the specified status
func (m *Manager) GetOrdersByStatus(status models.OrderStatus) []models.Order {
	m.mu.RLock()
	defer m.mu.RUnlock()

	orders := make([]models.Order, 0, len(m.orders))
	for _, state := range m.orders {
		if models.OrderStatus(state.Order.Status) == status {
			orders = append(orders, state.Order)
		}
	}

	return orders
}

// Return all orders with any of the specified statuses
func (m *Manager) GetOrdersByStatuses(statuses []models.OrderStatus) []models.Order {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statusSet := make(map[models.OrderStatus]struct{}, len(statuses))
	for _, status := range statuses {
		statusSet[status] = struct{}{}
	}

	orders := make([]models.Order, 0, len(m.orders))
	for _, state := range m.orders {
		if _, exists := statusSet[models.OrderStatus(state.Order.Status)]; exists {
			orders = append(orders, state.Order)
		}
	}

	return orders
}

// Return all active orders (not filled, canceled, or rejected)
func (m *Manager) GetActiveOrders() []models.Order {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.GetOrdersByStatuses([]models.OrderStatus{models.OrderStatusNew, models.OrderStatusPartiallyFilled})
}

// Return all executed (filled) orders
func (m *Manager) GetExecutedOrders() []models.Order {
	return m.GetOrdersByStatus(models.OrderStatusFilled)
}

// Return all canceled orders
func (m *Manager) GetCanceledOrders() []models.Order {
	return m.GetOrdersByStatus(models.OrderStatusCanceled)
}

// Remove an order from tracking
func (m *Manager) RemoveOrder(orderID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	order, err := m.GetOrder(orderID)
	if err != nil {
		return err
	}

	delete(m.orders, orderID)
	if order.ClientOrderID != "" {
		delete(m.clientOrders, order.ClientOrderID)
	}

	log.Printf("Removed order from tracking: %d", orderID)
	return nil
}

// Print a summary of the current orders
func (m *Manager) PrintOrderSummary() {
	if m == nil {
		log.Println("Warning: Order manager is nil")
		return
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	log.Println("===== ORDER SUMMARY =====")
	log.Printf("Total Orders: %d", len(m.orders))

	// Count by status
	statusCounts := make(map[string]int)
	for _, state := range m.orders {
		if state == nil {
			continue
		}
		statusCounts[state.Order.Status]++
	}

	for status, count := range statusCounts {
		log.Printf("Status %s: %d orders", status, count)
	}

	log.Println("=========================")

	filledOrders := m.GetOrdersByStatus("FILLED")
	if len(filledOrders) > 0 {
		log.Printf("Found %d filled orders", len(filledOrders))

		// For now, just log them
		for _, order := range filledOrders {
			log.Printf("Filled order: %d, Side: %s, Qty: %s, Price: %s",
				order.OrderID, order.Side, order.ExecutedQty, order.Price)
		}
	}
}
