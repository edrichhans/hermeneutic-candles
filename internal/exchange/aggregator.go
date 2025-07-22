package exchange

import (
	"context"
	"log"
	"sync"
)

type Aggregator struct {
	Adapters []ExchangeAdapter
}

func NewAggregator(adapters []ExchangeAdapter) *Aggregator {
	return &Aggregator{Adapters: adapters}
}

func (a *Aggregator) StreamTrades(ctx context.Context, out chan Trade, symbols []SymbolPair) {
	errCh := make(chan error, len(a.Adapters))
	var wg sync.WaitGroup

	for _, adapter := range a.Adapters {
		wg.Add(1)
		go func(adapter ExchangeAdapter) {
			defer wg.Done()
			err := adapter.StreamTrades(ctx, out, symbols)
			if err != nil {
				errCh <- err
			}
		}(adapter)
	}

	go func() {
		wg.Wait()
		close(errCh)
	}()

	// Handle errors from all adapters
	for err := range errCh {
		if err != nil {
			// Log the error or handle it as needed
			log.Printf("Adapter disconnected: %v", err)
		}
	}
}
