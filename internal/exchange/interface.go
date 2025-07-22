package exchange

import (
	"context"
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
	StreamTrades(ctx context.Context, out chan<- Trade, symbols []SymbolPair) error
}
