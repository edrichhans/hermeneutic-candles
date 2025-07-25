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

type BinanceAdapter struct {
	tradeChannel chan<- exchange.Trade
	pongChannel  chan time.Time
	connection   *websocket.Conn
}

func NewAdapter(tradeChannel chan<- exchange.Trade) *BinanceAdapter {
	return &BinanceAdapter{
		tradeChannel: tradeChannel,
		pongChannel:  make(chan time.Time, 1),
	}
}

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

func (b *BinanceAdapter) GetPongChan() <-chan time.Time {
	return b.pongChannel
}

func (b *BinanceAdapter) ConnectAndSubscribe(symbols []exchange.SymbolPair) (*websocket.Conn, error) {
	cfg := cmd.GetConfig()
	addr := fmt.Sprintf("%s:%d", cfg.BinanceAddress, cfg.BinancePort)
	query := b.symbolsToQuery(symbols)
	u := url.URL{Scheme: "wss", Host: addr, Path: "/stream", RawQuery: query}

	log.Printf("Connecting to %s at %s", b.Name(), u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	b.connection = c

	if c != nil {
		// Set the pong handler
		c.SetPongHandler(func(string) error {
			b.pongChannel <- time.Now()
			return nil
		})
		c.SetPingHandler(func(s string) error {
			// Echo the ping back to the server
			return c.WriteMessage(websocket.PongMessage, []byte(s))
		})
	}
	return c, err
}

func (b *BinanceAdapter) HandleMessage(message []byte) error {
	var bt binanceTrade
	if err := json.Unmarshal(message, &bt); err != nil {
		return fmt.Errorf("binance failed to unmarshal message: %w", err)
	}

	b.tradeChannel <- b.binanceTradeDataToDomainTrade(
		bt.Data,
	)
	return nil
}

func (b *BinanceAdapter) Ping() error {
	if b.connection != nil {
		b.connection.WriteMessage(websocket.PingMessage, nil)
		return nil
	} else {
		return fmt.Errorf("tried to ping without a valid connection")
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
