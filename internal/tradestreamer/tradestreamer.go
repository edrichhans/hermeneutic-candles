package tradestreamer

import (
	"context"
	"fmt"
	"hermeneutic-candles/cmd"
	"hermeneutic-candles/internal/exchange"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

type TradeStreamer struct {
	adapter exchange.ExchangeAdapter
}

func NewTradeStreamer(adapter exchange.ExchangeAdapter) *TradeStreamer {
	return &TradeStreamer{adapter: adapter}
}

func (ts *TradeStreamer) StreamTrades(parent context.Context, out chan<- exchange.Trade, symbols []exchange.SymbolPair) error {
	cfg := cmd.GetConfig()
	connectionMaxRetries := cfg.WSConnectionMaxRetries

	// Create a context that cancels on interrupt signal
	ctx, cancel := signal.NotifyContext(parent, os.Interrupt)
	defer cancel()

	u := ts.adapter.URL(symbols)

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

		log.Printf("Connecting to %s on %s", ts.adapter.Name(), u.String())

		c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		if err != nil {
			log.Printf("Failed to connect (attempt %d/%d): %v", retries+1, connectionMaxRetries, err)

			select {
			case <-ctx.Done():
				return fmt.Errorf("%s: %w", ts.adapter.Name(), ctx.Err())
			default:
				continue
			}
		}

		// Connection successful, reset retry counter
		retries = 0

		// Handle the connection
		// This will block until the connection is closed or an error occurs
		err = ts.handleConnection(ctx, c, out)
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

func (ts *TradeStreamer) handleConnection(ctx context.Context, c *websocket.Conn, out chan<- exchange.Trade) error {
	done := make(chan error, 1)

	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				done <- err
				return
			}

			err = ts.adapter.HandleMessage(message, out)
			if err != nil {
				log.Printf("Failed to handle message: %v", err)
				continue
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			// Close the connection gracefully
			log.Printf("Closing connection due to context cancellation")
			c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			return ctx.Err()
		// Possible read error from WS connection
		case err := <-done:
			return err
		}
	}
}
