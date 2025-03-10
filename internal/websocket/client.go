package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/iamramtin/binance-trader/internal/models"
)

// Handle WebSocket responses
type ResponseHandler func(response []byte)

// WebSocket client
type Client struct {
	connection       *websocket.Conn
	reconnecting     bool
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
func (c *Client) Connect(ctx context.Context) error {
	log.Printf("Connecting to Binance WebSocket API: %s", c.url)

	// Create a websocket dialer
	dialer := websocket.Dialer{}

	// Connect to the websocket
	connection, _, err := dialer.DialContext(ctx, c.url, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	c.connection = connection

	go c.readMessages()

	log.Println("Connected to Binance WebSocket API")
	return nil
}

func (c *Client) Close() {
	close(c.done) // close channel

	if c.connection != nil {
		c.connection.Close()
	}

	log.Println("WebSocket connection closed")
}

func (c *Client) SendRequest(method string, params any, handler ResponseHandler) (string, error) {
	c.mu.RLock()

	if c.connection == nil {
		c.mu.RUnlock()
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
		c.mu.RUnlock()
		c.mu.Lock()
		c.responseHandlers[requestID] = handler
		c.mu.Unlock()
		c.mu.RLock()
	}

	requestJSON, err := json.Marshal(request)
	if err != nil {
		c.mu.RUnlock()
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	log.Printf("Sending request: %s", string(requestJSON))

	c.mu.RUnlock()
	c.mu.Lock()

	// Ensure connection is still valid
	if c.connection == nil {
		c.mu.Unlock()
		return "", fmt.Errorf("WebSocket connection is not established")
	}

	// Send the request
	err = c.connection.WriteMessage(websocket.TextMessage, requestJSON)
	c.mu.Unlock()

	if err != nil {
		// If we failed to write, attempt to reconnect
		log.Printf("Error sending request: %v, attempting reconnect", err)
		c.attemptReconnect()
		return "", fmt.Errorf("failed to send request: %w", err)
	}

	return requestID, nil
}

func (c *Client) Ping() error {
	_, err := c.SendRequest("ping", nil, func(response []byte) {
		log.Println("Received pong response")
	})

	return err
}

// Read messages from the WebSocket connection
func (c *Client) readMessages() {
	for {
		select {
		case <-c.done:
			return

		default:
			_, message, err := c.connection.ReadMessage()
			if err != nil {
				log.Printf("Error reading message: %v", err)

				c.attemptReconnect()
				return
			}

			go c.handleMessage(message)
		}
	}
}

// Process the incoming WebSocket message
func (c *Client) handleMessage(message []byte) {
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

		c.mu.RLock()
		handler, exists := c.responseHandlers[id]
		c.mu.RUnlock()

		if exists {
			handler(message)

			// Remove one-time handlers
			c.mu.Lock()
			delete(c.responseHandlers, id)
			c.mu.Unlock()
		}
	}

	if response.Status == 200 {
		log.Printf("Received success response for ID: %v", response.ID)
	}
}

func (c *Client) attemptReconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if already reconnecting
	if c.reconnecting {
		return
	}

	c.reconnecting = true

	// Close the existing connection if any
	if c.connection != nil {
		c.connection.Close()
		c.connection = nil
	}

	// Start reconnection attempts in a goroutine
	go func() {
		attempts := 0
		maxAttempts := 5
		delay := 1 * time.Second

		for attempts < maxAttempts {
			log.Printf("Attempting to reconnect (attempt %d/%d)", attempts+1, maxAttempts)

			// Create a new dialer
			dialer := websocket.Dialer{
				HandshakeTimeout: 10 * time.Second,
			}

			// Try to connect
			conn, _, err := dialer.Dial(c.url, nil)
			if err == nil {
				// Successful reconnection
				c.mu.Lock()
				c.connection = conn
				c.reconnecting = false
				c.mu.Unlock()

				log.Println("Successfully reconnected")

				// Restart the message reader
				go c.readMessages()

				// Notify subscribers that we've reconnected
				// Implementation depends on your design

				return
			}

			log.Printf("Reconnection failed: %v", err)
			attempts++
			time.Sleep(delay)
			delay *= 2 // Exponential backoff
		}

		log.Println("Failed to reconnect after maximum attempts")
		c.mu.Lock()
		c.reconnecting = false
		c.mu.Unlock()
	}()
}
