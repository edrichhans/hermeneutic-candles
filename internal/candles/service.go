package candles

import (
	"context"
	candlesv1 "hermeneutic-candles/gen/proto/candles/v1"
	"hermeneutic-candles/internal/exchange"
	"hermeneutic-candles/internal/exchange/binance"
	"log"
	"time"

	"connectrpc.com/connect"
)

type CandlesService struct{}

func (s *CandlesService) StreamCandles(
	ctx context.Context,
	req *connect.Request[candlesv1.StreamCandlesRequest],
	serverstream *connect.ServerStream[candlesv1.StreamCandlesResponse],
) error {
	ticker := time.NewTicker(time.Duration(req.Msg.Interval) * time.Second)
	defer ticker.Stop()

	// TODO: add more exchanges
	tradeChannel := make(chan exchange.Trade)
	exchangeAdapters := []exchange.ExchangeAdapter{
		&binance.BinanceAdapter{},
	}
	exchangeAggregator := exchange.NewAggregator(exchangeAdapters)

	go exchangeAggregator.StreamTrades(ctx, tradeChannel, []exchange.SymbolPair{
		{First: "btc", Second: "usdt"},
	})

	go func() {
		trades := []exchange.Trade{}
		for {
			select {
			case <-ctx.Done():
				return
			case trade := <-tradeChannel:
				trades = append(trades, trade)
			case <-ticker.C:
				if len(trades) == 0 {
					continue
				}
				candle := s.tradesToCandle(trades)
				if err := serverstream.Send(candle); err != nil {
					log.Printf("failed to send candle: %v", err)
					return
				}
				// Reset trades for the next interval
				trades = trades[:0]
			}
		}

	}()

	<-ctx.Done()

	return nil
}

func (s *CandlesService) tradesToCandle(trades []exchange.Trade) *candlesv1.StreamCandlesResponse {
	if len(trades) == 0 {
		return nil
	}

	open := trades[0].Price
	high := open
	low := open
	close := trades[len(trades)-1].Price
	volume := 0.0

	for _, trade := range trades {
		if trade.Price > high {
			high = trade.Price
		}
		if trade.Price < low {
			low = trade.Price
		}
		volume += trade.Quantity
	}

	return &candlesv1.StreamCandlesResponse{
		Timestamp: trades[0].Timestamp.Unix(),
		Open:      open,
		High:      high,
		Low:       low,
		Close:     close,
		Volume:    volume,
	}
}
