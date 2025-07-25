package tradestreamer

import (
	"context"
	"fmt"
	"hermeneutic-candles/cmd"
	"hermeneutic-candles/internal/exchange"
	"log"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type TradeStreamer struct {
	adapter exchange.ExchangeAdapter
}

func NewTradeStreamer(adapter exchange.ExchangeAdapter) *TradeStreamer {
	return &TradeStreamer{adapter: adapter}
}

func (ts *TradeStreamer) StreamTrades(parent context.Context, symbols []exchange.SymbolPair) error {
	cfg := cmd.GetConfig()
	connectionMaxRetries := cfg.WSConnectionMaxRetries

	// Create a context that cancels on interrupt signal
	ctx, cancel := signal.NotifyContext(parent, os.Interrupt)
	defer cancel()

	retries := 0
	for retries = range connectionMaxRetries {
		if retries > 0 {
			delay := time.Duration(retries) * time.Second
			log.Printf("Reconnecting in %v... (attempt %d/%d)", delay, retries+1, connectionMaxRetries)

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		c, err := ts.adapter.ConnectAndSubscribe(symbols)
		if err != nil {
			log.Printf("Failed to connect (attempt %d/%d): %v", retries+1, connectionMaxRetries, err)
			continue
		}

		// Connection successful, reset retry counter
		retries = 0

		// Handle the connection
		// This will block until the connection is closed or an error occurs
		err = ts.handleConnection(ctx, cfg, c)
		c.Close()

		// If context was canceled, don't retry and don't return an error
		if err != nil {
			return fmt.Errorf("%s: %w", ts.adapter.Name(), err)
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("%s: %w", ts.adapter.Name(), ctx.Err())
		default:
			log.Printf("Connection to %s lost: %v", ts.adapter.Name(), err)
		}
	}

	return fmt.Errorf("failed to establish stable connection to %s after %d attempts", ts.adapter.Name(), connectionMaxRetries)
}

func (ts *TradeStreamer) handleConnection(ctx context.Context, cfg *cmd.Config, c *websocket.Conn) error {
	var wg sync.WaitGroup
	// Channel to signal when connection should be terminated or retried
	done := make(chan error, 1)

	var lastTs atomic.Value
	lastTs.Store(time.Now())

	wg.Add(2)
	go ts.handleMessages(c, &lastTs, done, &wg)
	go ts.checkLiveness(cfg, &lastTs, done, &wg)

	// Wait for all goroutines to finish to close the channel
	go func() {
		wg.Wait()
		close(done)
	}()

	for {
		select {
		case <-ctx.Done():
			// Close the connection gracefully
			log.Printf("Closing connection due to context cancellation")
			c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			return ctx.Err()
		case err := <-done:
			return err
		}
	}
}

// Goroutine to handle the incoming messages from the WebSocket connection
func (ts *TradeStreamer) handleMessages(c *websocket.Conn, lastTs *atomic.Value, done chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			select {
			case done <- err:
			default:
				// Channel is full or closed, ignore
			}
			return
		}

		lastTs.Store(time.Now())
		err = ts.adapter.HandleMessage(message)
		if err != nil {
			log.Printf("Failed to handle message: %v", err)
			continue
		}
	}
}

// Goroutine to periodically check the liveness of the connection
// If no message is received within the configured timeout,
// it sends a ping and waits for a pong response
func (ts *TradeStreamer) checkLiveness(cfg *cmd.Config, lastTs *atomic.Value, done chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		tsAtomic := lastTs.Load()
		timestamp, ok := tsAtomic.(time.Time)
		if !ok {
			continue
		}

		if time.Since(timestamp) > time.Duration(cfg.WSConnectionTimeout)*time.Millisecond {
			log.Printf("No message in %dms, sending ping", cfg.WSConnectionTimeout)
			if err := ts.adapter.Ping(); err != nil {
				select {
				case done <- err:
				default:
					// Channel is full or closed, ignore
				}
				return
			}
		} else {
			continue
		}

		select {
		case <-ts.adapter.GetPongChan():
			continue
		case <-time.After(10 * time.Second):
			log.Printf("Pong not received after 10 seconds of sending a ping. Reconnect.")
			select {
			case done <- nil:
			default:
				// Channel is full or closed, ignore
			}
			return
		}
	}
}
