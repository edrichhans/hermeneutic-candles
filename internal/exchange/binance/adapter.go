package binance

import (
	"encoding/json"
	"fmt"
	"hermeneutic-candles/cmd"
	"hermeneutic-candles/internal/exchange"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type BinanceAdapter struct{}

type binanceTradeData struct {
	Symbol   string  `json:"s"`
	Price    float64 `json:"p,string"`
	Quantity float64 `json:"q,string"`
	Time     int64   `json:"T"`
}
type binanceTrade struct {
	Data binanceTradeData `json:"data"`
}

func (b *BinanceAdapter) Name() string {
	return "Binance"
}

func (b *BinanceAdapter) ConnectAndSubscribe(symbols []exchange.SymbolPair) (*websocket.Conn, error) {
	cfg := cmd.GetConfig()
	addr := fmt.Sprintf("%s:%d", cfg.BinanceAddress, cfg.BinancePort)
	query := b.symbolsToQuery(symbols)
	u := url.URL{Scheme: "wss", Host: addr, Path: "/stream", RawQuery: query}

	log.Printf("Connecting to %s at %s", b.Name(), u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	return c, err
}

func (b *BinanceAdapter) HandleMessage(message []byte, out chan<- exchange.Trade) error {
	var bt binanceTrade
	if err := json.Unmarshal(message, &bt); err != nil {
		return fmt.Errorf("binance failed to unmarshal message: %w", err)
	}

	out <- b.binanceTradeDataToDomainTrade(
		bt.Data,
	)
	return nil
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

func (b *BinanceAdapter) responseSymbolToOutputString(s string) string {
	return strings.ToLower(s)
}

func (b *BinanceAdapter) binanceTradeDataToDomainTrade(
	data binanceTradeData,
) exchange.Trade {
	return exchange.Trade{
		Symbol:    b.responseSymbolToOutputString(data.Symbol),
		Price:     data.Price,
		Quantity:  data.Quantity,
		Timestamp: time.UnixMilli(data.Time),
		Source:    b.Name(),
	}
}
