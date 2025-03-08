package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/iamramtin/binance-trader/internal/models"
)

// Handle WebSocket responses
type ResponseHandler func(response []byte)

// WebSocket client
type Client struct {
	connection       *websocket.Conn
	url              string
	apiKey           string
	secretKey        string
	requestID        string                     // Incremental request ID
	responseHandlers map[string]ResponseHandler // Maps request IDs to response handlers
	mu               sync.RWMutex               // Mutex for thread safety
	done             chan struct{}              // Channel to signal shutdown
}

// Create a new WebSocket client
func New(url, apiKey, secretKey string) *Client {
	return &Client{
		url:              url,
		apiKey:           apiKey,
		secretKey:        secretKey,
		responseHandlers: make(map[string]ResponseHandler),
		done:             make(chan struct{}),
	}
}

// Establish a WebSocket connection to Binance API
func (client *Client) Connect(ctx context.Context) error {
	log.Printf("Connecting to Binance WebSocket API: %s", client.url)

	// Create a websocket dialer
	dialer := websocket.Dialer{}

	// Connect to the websocket
	connection, _, err := dialer.DialContext(ctx, client.url, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	client.connection = connection

	go client.readMessages()

	log.Println("Connected to Binance WebSocket API")
	return nil
}

func (client *Client) Close() {
	close(client.done) // close channel

	if client.connection != nil {
		client.connection.Close()
	}

	log.Println("WebSocket connection closed")
}

func (client *Client) SendRequest(method string, params any, handler ResponseHandler) (string, error) {
	if client.connection == nil {
		return "", fmt.Errorf("WebSocket connection is not established")
	}

	requestID := uuid.New().String()

	request := models.WebSocketRequest{
		ID:     requestID,
		Method: method,
		Params: params,
	}

	// Register the handler
	if handler != nil {
		client.mu.Lock()
		client.responseHandlers[requestID] = handler
		client.mu.Unlock()
	}

	requestJSON, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	log.Printf("Sending request: %s", string(requestJSON))

	client.mu.Lock()
	defer client.mu.Unlock()

	// Ensure connection is still valid
	if client.connection == nil {
		return "", fmt.Errorf("WebSocket connection is not established")
	}

	// Send the request
	if err := client.connection.WriteMessage(websocket.TextMessage, requestJSON); err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}

	return requestID, nil
}

func (client *Client) Ping() error {
	_, err := client.SendRequest("ping", nil, func(response []byte) {
		log.Println("Received pong response")
	})

	return err
}

// Read messages from the WebSocket connection
func (client *Client) readMessages() {
	for {
		select {
		case <-client.done:
			return

		default:
			_, message, err := client.connection.ReadMessage()
			if err != nil {
				log.Printf("Error reading message: %v", err)

				// TODO: attempt to reconnect here
				return
			}

			go client.handleMessage(message)
		}
	}
}

// Process the incoming WebSocket message
func (client *Client) handleMessage(message []byte) {
	// Parse the message
	var response models.WebSocketResponse
	if err := json.Unmarshal(message, &response); err != nil {
		log.Printf("Error parsing response: %v", err)
		return
	}

	if response.Error != nil {
		log.Printf("API Error: Code %d - %s", response.Error.Code, response.Error.Msg)
	}

	// Find the corresponding handler for ID
	if response.ID != "" {
		id := fmt.Sprintf("%v", response.ID)

		client.mu.RLock()
		handler, exists := client.responseHandlers[id]
		client.mu.RUnlock()

		if exists {
			handler(message)

			// Remove one-time handlers
			client.mu.Lock()
			delete(client.responseHandlers, id)
			client.mu.Unlock()
		}
	}

	if response.Status == 200 {
		log.Printf("Received success response for ID: %v", response.ID)
	}
}
