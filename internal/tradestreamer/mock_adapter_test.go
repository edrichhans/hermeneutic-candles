package tradestreamer

import (
	"encoding/json"
	"fmt"
	"hermeneutic-candles/internal/exchange"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

// MockExchangeAdapter implements the ExchangeAdapter interface for testing
type MockExchangeAdapter struct {
	wsURL        string
	tradeChannel chan<- exchange.Trade
	conn         *websocket.Conn
}

type MockTrade struct {
	Symbol    string  `json:"symbol"`
	Price     float64 `json:"price"`
	Quantity  float64 `json:"quantity"`
	Timestamp int64   `json:"timestamp"`
	Source    string  `json:"source"`
}

func NewMockExchangeAdapter(name, wsURL string, tradeChannel chan<- exchange.Trade) *MockExchangeAdapter {
	return &MockExchangeAdapter{wsURL: wsURL, tradeChannel: tradeChannel}
}

func (m *MockExchangeAdapter) Name() string {
	return "Mock"
}

func (m *MockExchangeAdapter) ConnectAndSubscribe(symbols []exchange.SymbolPair) (*websocket.Conn, error) {
	conn, _, err := websocket.DefaultDialer.Dial(m.wsURL, nil)
	m.conn = conn
	return conn, err
}

func (m *MockExchangeAdapter) HandleMessage(message []byte) error {
	log.Printf("Received message: %s", message)
	var trade MockTrade
	if err := json.Unmarshal(message, &trade); err != nil {
		return err
	}
	// Convert MockTrade to exchange.Trade
	exchangeTrade := exchange.Trade{
		Symbol:    trade.Symbol,
		Price:     trade.Price,
		Quantity:  trade.Quantity,
		Timestamp: time.UnixMilli(trade.Timestamp),
		Source:    trade.Source,
	}
	m.tradeChannel <- exchangeTrade

	return nil
}

func (m *MockExchangeAdapter) Ping() error {
	if m.conn == nil {
		return fmt.Errorf("no connection")
	}
	return m.conn.WriteMessage(websocket.PingMessage, nil)
}

func (m *MockExchangeAdapter) GetPongChan() <-chan time.Time {
	return make(chan time.Time)
}
