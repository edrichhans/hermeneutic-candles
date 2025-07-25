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

type BybitAdapter struct {
	connection   *websocket.Conn
	tradeChannel chan<- exchange.Trade
	pongChannel  chan time.Time
}

func NewAdapter(tradeChannel chan<- exchange.Trade) *BybitAdapter {
	return &BybitAdapter{
		tradeChannel: tradeChannel,
		pongChannel:  make(chan time.Time, 1),
	}
}

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

func (b *BybitAdapter) GetPongChan() <-chan time.Time {
	return b.pongChannel
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
	b.connection = c

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

func (b *BybitAdapter) HandleMessage(message []byte) error {
	var m map[string]interface{}
	if err := json.Unmarshal(message, &m); err != nil {
		return fmt.Errorf("bybit failed to unmarshal message: %w", err)
	}

	if m["op"] == "ping" {
		b.pongChannel <- time.Now()
		return nil
	} else {
		var bt bybitTrade
		if err := json.Unmarshal(message, &bt); err != nil {
			return fmt.Errorf("bybit failed to unmarshal message: %w", err)
		}

		for _, data := range bt.Data {
			b.tradeChannel <- b.bybitTradeDataToDomainTrade(
				data,
			)
		}
		return nil
	}

}

func (b *BybitAdapter) Ping() error {
	if b.connection != nil {
		pingMessage := map[string]interface{}{
			"op": "ping",
		}
		b.connection.WriteJSON(pingMessage)
		return nil
	} else {
		return fmt.Errorf("tried to ping without a valid connection")
	}
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

func (b *BybitAdapter) responseSymbolToOutputString(s string) string {
	return strings.ToLower(s)
}

func (b *BybitAdapter) bybitTradeDataToDomainTrade(
	data bybitTradeData,
) exchange.Trade {
	return exchange.Trade{
		Symbol:    b.responseSymbolToOutputString(data.Symbol),
		Price:     data.Price,
		Quantity:  data.Quantity,
		Timestamp: time.UnixMilli(data.Time),
		Source:    b.Name(),
	}
}
