package bybit

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

type BybitAdapter struct{}

type bybitTradeData struct {
	Symbol   string  `json:"s"`
	Price    float64 `json:"p,string"`
	Quantity float64 `json:"v,string"`
	Time     int64   `json:"T"`
}
type bybitTrade struct {
	Data []bybitTradeData `json:"data"`
}

func (b *BybitAdapter) Name() string {
	return "Bybit"
}

func (b *BybitAdapter) ConnectAndSubscribe(symbols []exchange.SymbolPair) (*websocket.Conn, error) {
	cfg := cmd.GetConfig()
	addr := cfg.BybitAddress
	u := url.URL{Scheme: "wss", Host: addr, Path: "/v5/public/spot"}

	log.Printf("Connecting to %s at %s", b.Name(), u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return nil, err
	}

	// send a subscription message
	subscribeMessage := map[string]interface{}{
		"op":   "subscribe",
		"args": b.symbolsToSubscribeArgs(symbols),
	}
	if err := c.WriteJSON(subscribeMessage); err != nil {
		c.Close()
		return nil, fmt.Errorf("failed to subscribe to symbols: %w", err)
	}

	return c, nil
}

func (b *BybitAdapter) HandleMessage(message []byte, out chan<- exchange.Trade) error {
	var bt bybitTrade
	if err := json.Unmarshal(message, &bt); err != nil {
		return fmt.Errorf("bybit failed to unmarshal message: %w", err)
	}

	for _, data := range bt.Data {
		out <- b.bybitTradeDataToDomainTrade(
			data,
		)
	}
	return nil
}

func (b *BybitAdapter) symbolsToSubscribeArgs(symbols []exchange.SymbolPair) []string {
	var args []string
	for _, symbol := range symbols {
		first := strings.ToUpper(symbol.First)
		second := strings.ToUpper(symbol.Second)
		bybitSymbol := fmt.Sprintf("%s%s", first, second)
		args = append(args, fmt.Sprintf("publicTrade.%s", bybitSymbol))
	}
	return args
}

func (b *BybitAdapter) responseSymbolToDomainSymbolString(s string) string {
	return strings.ToLower(s)
}

func (b *BybitAdapter) bybitTradeDataToDomainTrade(
	data bybitTradeData,
) exchange.Trade {
	return exchange.Trade{
		Symbol:    b.responseSymbolToDomainSymbolString(data.Symbol),
		Price:     data.Price,
		Quantity:  data.Quantity,
		Timestamp: time.UnixMilli(data.Time),
		Source:    b.Name(),
	}
}
