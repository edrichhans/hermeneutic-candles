package tradestreamer

import (
	"context"
	"hermeneutic-candles/internal/exchange"
	"log"
	"sync"
)

type Aggregator struct {
	TradeStreamers []*TradeStreamer
}

func NewAggregator(tradeStreamers []*TradeStreamer) *Aggregator {
	return &Aggregator{TradeStreamers: tradeStreamers}
}

func (a *Aggregator) StreamTrades(ctx context.Context, symbols []exchange.SymbolPair) {
	errCh := make(chan error, len(a.TradeStreamers))
	var wg sync.WaitGroup

	for _, tradeStreamer := range a.TradeStreamers {
		wg.Add(1)
		go func(tradeStreamer *TradeStreamer) {
			defer wg.Done()
			err := tradeStreamer.StreamTrades(ctx, symbols)
			if err != nil {
				errCh <- err
			}
		}(tradeStreamer)
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
