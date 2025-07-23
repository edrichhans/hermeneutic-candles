package exchange

import (
	"net/url"
	"time"
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
	URL(symbols []SymbolPair) url.URL
	HandleMessage(message []byte, out chan<- Trade) error
}
