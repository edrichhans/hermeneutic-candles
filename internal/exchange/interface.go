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
	Source    string
}

type SymbolPair struct {
	First  string
	Second string
}

type ExchangeAdapter interface {
	Name() string
	GetPongChan() <-chan time.Time
	ConnectAndSubscribe(symbols []SymbolPair) (*websocket.Conn, error)
	HandleMessage(message []byte) error
	Ping() error
}
