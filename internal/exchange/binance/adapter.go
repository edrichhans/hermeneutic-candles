package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"hermeneutic-candles/cmd"
	"hermeneutic-candles/internal/exchange"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const MAX_RETRIES = 5

type BinanceAdapter struct{}
type BinanceTradeData struct {
	Symbol   string  `json:"s"`
	Price    float64 `json:"p,string"`
	Quantity float64 `json:"q,string"`
	Time     int64   `json:"T"`
}
type BinanceTrade struct {
	Data BinanceTradeData `json:"data"`
}

func (b *BinanceAdapter) StreamTrades(parent context.Context, out chan<- exchange.Trade, symbols []exchange.SymbolPair) error {
	cfg := cmd.GetConfig()

	// Create a context that cancels on interrupt signal
	ctx, cancel := signal.NotifyContext(parent, os.Interrupt)
	defer cancel()

	addr := fmt.Sprintf("%s:%d", cfg.BinanceAddress, cfg.BinancePort)
	query := b.symbolsToQuery(symbols)
	u := url.URL{Scheme: "wss", Host: addr, Path: "/stream", RawQuery: query}

	retries := 0
	for retries = range MAX_RETRIES {
		if retries > 0 {
			delay := time.Duration(retries) * time.Second
			log.Printf("Reconnecting in %v... (attempt %d/%d)", delay, retries+1, MAX_RETRIES)

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		log.Printf("Connecting to %s", u.String())

		c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		if err != nil {
			log.Printf("Failed to connect (attempt %d/%d): %v", retries+1, MAX_RETRIES, err)

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				continue
			}
		}

		// Connection successful, reset retry counter
		retries = 0

		// Handle the connection
		// This will block until the connection is closed or an error occurs
		err = b.handleConnection(ctx, c, out)
		c.Close()

		// If context was canceled, don't retry and don't return an error
		if err != nil {
			return fmt.Errorf("binance: %w", err)
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("binance context canceled: %w", ctx.Err())
		default:
			log.Printf("Connection lost: %v", err)
		}
	}

	// Send EOF to grpc
	return fmt.Errorf("binance failed to establish stable connection after %d attempts", MAX_RETRIES)
}

func (b *BinanceAdapter) handleConnection(ctx context.Context, c *websocket.Conn, out chan<- exchange.Trade) error {
	done := make(chan error, 1)

	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				done <- err
				return
			}

			var binanceTrade BinanceTrade
			if err := json.Unmarshal(message, &binanceTrade); err != nil {
				log.Printf("Failed to unmarshal message: %v", err)
				continue
			}

			out <- b.binanceTradeDataToDomainTrade(
				binanceTrade.Data,
			)
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

func (b *BinanceAdapter) symbolsToQuery(symbols []exchange.SymbolPair) string {
	query := ""
	for _, symbol := range symbols {
		if query != "" {
			query += "/"
		}
		query += fmt.Sprintf("%s%s@trade", strings.ToLower(symbol.First), strings.ToLower(symbol.Second))
	}
	return fmt.Sprintf("streams=%s", query)
}

func (b *BinanceAdapter) binanceTradeDataToDomainTrade(
	data BinanceTradeData,
) exchange.Trade {
	return exchange.Trade{
		Symbol:    data.Symbol,
		Price:     data.Price,
		Quantity:  data.Quantity,
		Timestamp: time.UnixMilli(data.Time),
	}
}
