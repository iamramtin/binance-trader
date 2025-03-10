package api

import (
	"context"
	"testing"

	"github.com/iamramtin/binance-trader/internal/models"
)

// Mock version of the websocket client for testing
type MockWebSocketClient struct {
	connected      bool
	mockResponses  map[string][]byte
	requestCounter int
}

func NewMockWebSocketClient() *MockWebSocketClient {
	return &MockWebSocketClient{
		mockResponses: make(map[string][]byte),
	}
}

func (m *MockWebSocketClient) Connect(ctx context.Context) error {
	m.connected = true
	return nil
}

func (m *MockWebSocketClient) Close() {
	m.connected = false
}

func (m *MockWebSocketClient) IsConnected() bool {
	return m.connected
}

func (m *MockWebSocketClient) SendRequest(method string, params interface{}, handler func([]byte)) (int, error) {
	m.requestCounter++

	if response, ok := m.mockResponses[method]; ok {
		go handler(response)
		return m.requestCounter, nil
	}

	// Default empty response if no mock response is set
	emptyResponse := []byte(`{"id": 1, "status": 200, "result": {}}`)
	go handler(emptyResponse)
	return m.requestCounter, nil
}

func (m *MockWebSocketClient) SetMockResponse(method string, response []byte) {
	m.mockResponses[method] = response
}

// TestNew tests the creation of a new BinanceClient
func TestNew(t *testing.T) {
	client := New("wss://testnet.binance.vision/ws", "apiKey", "secretKey", "BTCUSDT")

	if client == nil {
		t.Fatal("New() returned nil")
	}

	if client.wsClient == nil {
		t.Error("websocket client was not initialized")
	}

	if client.orderManager == nil {
		t.Error("order manager was not initialized")
	}

	if client.apiKey != "apiKey" {
		t.Errorf("expected apiKey to be 'apiKey', got %s", client.apiKey)
	}

	if client.secretKey != "secretKey" {
		t.Errorf("expected secretKey to be 'secretKey', got %s", client.secretKey)
	}

	if client.symbol != "BTCUSDT" {
		t.Errorf("expected symbol to be 'BTCUSDT', got %s", client.symbol)
	}
}

// TestParseOrderbook tests the parseOrderbook function
func TestParseOrderbook(t *testing.T) {
	input := &models.OrderbookDepth{
		LastUpdateID: 12345,
		Bids: [][]string{
			{"40000.00", "1.5"},
			{"39900.00", "2.0"},
		},
		Asks: [][]string{
			{"40100.00", "1.0"},
			{"40200.00", "3.0"},
		},
	}

	result, err := parseOrderbook(input)
	if err != nil {
		t.Fatalf("parseOrderbook() returned error: %v", err)
	}

	if result.LastUpdateID != 12345 {
		t.Errorf("expected LastUpdateID to be 12345, got %d", result.LastUpdateID)
	}

	if len(result.Bids) != 2 {
		t.Errorf("expected 2 bids, got %d", len(result.Bids))
	}

	if len(result.Asks) != 2 {
		t.Errorf("expected 2 asks, got %d", len(result.Asks))
	}

	if result.Bids[0].Price != 40000.00 {
		t.Errorf("expected first bid price to be 40000.00, got %f", result.Bids[0].Price)
	}

	if result.Bids[0].Quantity != 1.5 {
		t.Errorf("expected first bid quantity to be 1.5, got %f", result.Bids[0].Quantity)
	}

	if result.Asks[0].Price != 40100.00 {
		t.Errorf("expected first ask price to be 40100.00, got %f", result.Asks[0].Price)
	}

	if result.Asks[0].Quantity != 1.0 {
		t.Errorf("expected first ask quantity to be 1.0, got %f", result.Asks[0].Quantity)
	}
}
