# Binance WebSocket Trader

A Golang application for algorithmic trading on Binance using WebSocket connections. This project implements a market making strategy with order placement and tracking using Binance's WebSocket API.

## Features

- WebSocket-based order placement and tracking
- Order management system to track active, executed, and canceled orders
- Market making strategy with configurable spread percentages
- Real-time order book monitoring
- Balance checking and management

## Prerequisites

- Go 1.18 or higher
- Docker and Docker Compose
- Binance API key and Secret key (obtain from https://testnet.binance.vision)

## Setup and Installation

### Option 1: Using Docker (Recommended)

1. Clone the repository:

   ```bash
   git clone https://github.com/iamramtin/binance-trader.git
   cd binance-trader
   ```

2. Build Docker container:

   ```bash
   docker build -t binance-trader .
   ```

3. Run Docker container:

   ```bash
   docker run -it --rm \
      -e BINANCE_API_KEY="api_key" \
      -e BINANCE_SECRET_KEY="secret_key" \
      binance-trader
   ```

### Option 2: Manual Setup

1. Clone the repository:

   ```bash
   git clone https://github.com/iamramtin/binance-trader.git
   cd binance-trader
   ```

2. Install dependencies:

   ```go
   go mod download
   ```

3. Build the application:

   ```go
   go build -o binance-trader ./cmd/main.go
   ```

4. Run the application:
   ```bash
   BINANCE_API_KEY="api_key" BINANCE_SECRET_KEY="secret_key" ./binance-trader
   ```

## Usage

When you start the application, you'll be prompted to choose an operating mode:

1. **Manual Mode**: Place individual test orders manually
2. **Market Maker Mode**: Continuously place bid/ask orders at a configurable spread

### Manual Mode

In manual mode, the application will:

- Display the current orderbook every 10 seconds
- Place a test market order every 15 seconds
- Cancel the oldest active order every 30 seconds

### Market Maker Mode

In market maker mode, the application will:

- Display the current orderbook every 10 seconds
- Place and maintain bid/ask orders around the market mid price
- Automatically cancel and replace orders to maintain the desired spread
- Print order summaries periodically

## Design Decisions

### WebSocket-Based Approach

- Lower latency for time-sensitive operations
- Reduced network overhead compared to multiple HTTP requests
- Real-time updates on order status changes
- Single persistent connection instead of multiple ephemeral connections

### Concurrency Model

- WebSocket message handling runs in separate goroutines
- Order placement and cancellation use non-blocking patterns
- Channel-based communication for synchronizing responses
- Mutex-protected shared state for thread safety

### Error Handling and Recovery

- Connection loss detection and automatic reconnection
- Request timeouts with context cancellation
- Graceful shutdown on application termination

## Performance Considerations

1. **Connection Management**:

- Automatic reconnection with exponential backoff when connections fail
- Connection health monitoring with periodic health checks

2. **Memory Optimization**:

- Efficient data structures for order tracking
- Minimized memory allocations in hot paths
- Proper cleanup of resources during shutdown

3. **Rate Limiting**:

- Respect for Binance's API rate limits
- Throttling of order placements to avoid hitting limits
- Sleep intervals between consecutive API calls

## Limitations and Future Improvements

1. **Current Limitations**:

- Limited to a single trading pair at a time
- Basic market making strategy without advanced features
- No persistent storage for order history
- Limited risk management features

2. **Room for Improvement**:

- Support for multiple trading pairs simultaneously
- Support for various (more advanced) trading algorithms
- Support for spot, margin, and futures trading
- Account balance verification before placing orders
- Enhanced error handling with retry mechanisms
- Concurrency improvements to manage multiple trades simultaneously
- Dynamic strategy adjustment based on changing tick sizes
- Automatic retrieval of exchange information for symbol parameters

## Testing

Run the test suite:

```go
go test ./internal/utils
go test ./internal/ordermanager
go test ./internal/api
go test ./internal/trader
```
