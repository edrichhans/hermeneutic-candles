package okx

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

type OkxAdapter struct{}

type okxTradeData struct {
	InstId    string  `json:"instId"`
	Price     float64 `json:"px,string"`
	Size      float64 `json:"sz,string"`
	TimeStamp int64   `json:"ts,string"`
}
type okxTrade struct {
	Data []okxTradeData `json:"data"`
}
type subscribeArgs struct {
	Channel string `json:"channel"`
	InstId  string `json:"instId"`
}

func (b *OkxAdapter) Name() string {
	return "Okx"
}

func (b *OkxAdapter) ConnectAndSubscribe(symbols []exchange.SymbolPair) (*websocket.Conn, error) {
	cfg := cmd.GetConfig()
	addr := fmt.Sprintf("%s:%d", cfg.OkxAddress, cfg.OkxPort)
	u := url.URL{Scheme: "wss", Host: addr, Path: "/ws/v5/public"}

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

func (b *OkxAdapter) HandleMessage(message []byte, out chan<- exchange.Trade) error {
	var bt okxTrade
	if err := json.Unmarshal(message, &bt); err != nil {
		return fmt.Errorf("okx failed to unmarshal message: %w", err)
	}

	for _, data := range bt.Data {
		out <- b.okxTradeDataToDomainTrade(
			data,
		)
	}
	return nil
}

func (b *OkxAdapter) symbolsToSubscribeArgs(symbols []exchange.SymbolPair) []subscribeArgs {
	var args []subscribeArgs
	for _, symbol := range symbols {
		first := strings.ToUpper(symbol.First)
		second := strings.ToUpper(symbol.Second)
		okxSymbol := fmt.Sprintf("%s-%s", first, second)
		args = append(args, subscribeArgs{
			Channel: "trades",
			InstId:  okxSymbol,
		})
	}
	return args
}

func (b *OkxAdapter) responseSymbolToDomainSymbolString(s string) string {
	return strings.ToLower(s)
}

func (b *OkxAdapter) okxTradeDataToDomainTrade(
	data okxTradeData,
) exchange.Trade {
	return exchange.Trade{
		Symbol:    b.responseSymbolToDomainSymbolString(data.InstId),
		Price:     data.Price,
		Quantity:  data.Size,
		Timestamp: time.UnixMilli(data.TimeStamp),
		Source:    b.Name(),
	}
}
