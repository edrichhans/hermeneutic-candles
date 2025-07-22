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

type ExchangeAdapter interface {
	StreamTrades(ctx context.Context, symbol []string, out chan<- Trade) error
}
