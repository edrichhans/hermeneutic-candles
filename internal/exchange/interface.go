package exchange

import (
	"time"

	"github.com/gorilla/websocket"
)

type Trade struct {
	Symbol    string
	Price     float64
	Quantity  float64
	Timestamp time.Time
}

type SymbolPair struct {
	First  string
	Second string
}

type ExchangeAdapter interface {
	Name() string
	ConnectAndSubscribe(symbols []SymbolPair) (*websocket.Conn, error)
	HandleMessage(message []byte, out chan<- Trade) error
}
