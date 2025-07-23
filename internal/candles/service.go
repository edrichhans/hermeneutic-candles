package candles

import (
	"context"
	"hermeneutic-candles/cmd"
	candlesv1 "hermeneutic-candles/gen/proto/candles/v1"
	"hermeneutic-candles/internal/exchange"
	"hermeneutic-candles/internal/exchange/binance"
	"hermeneutic-candles/internal/tradestreamer"
	"log"
	"time"

	"connectrpc.com/connect"
)

type CandlesService struct {
	intervalMillis int
}

func NewCandlesService(intervalMillis int) *CandlesService {
	return &CandlesService{
		intervalMillis: intervalMillis,
	}
}

func (s *CandlesService) StreamCandles(
	ctx context.Context,
	req *connect.Request[candlesv1.StreamCandlesRequest],
	serverstream *connect.ServerStream[candlesv1.StreamCandlesResponse],
) error {
	cfg := cmd.GetConfig()
	if cfg.TradeStreamBufferSize <= 0 {
		cfg.TradeStreamBufferSize = 1000 // Default buffer size if not set
	}
	if cfg.MaxTradesPerInterval <= 0 {
		cfg.MaxTradesPerInterval = 10000 // Default max trades per interval if not set
	}

	ticker := time.NewTicker(time.Duration(int32(s.intervalMillis)) * time.Millisecond)
	defer ticker.Stop()

	// TODO: add more exchanges
	tradeChannel := make(chan exchange.Trade, cfg.TradeStreamBufferSize)
	binanceTradeStreamer := tradestreamer.NewTradeStreamer(&binance.BinanceAdapter{})

	tradeStreamers := tradestreamer.NewAggregator([]*tradestreamer.TradeStreamer{
		binanceTradeStreamer,
	})

	go tradeStreamers.StreamTrades(ctx, tradeChannel, []exchange.SymbolPair{
		{First: "btc", Second: "usdt"},
	})

	go func() {
		trades := []exchange.Trade{}
		for {
			select {
			case <-ctx.Done():
				return
			case trade := <-tradeChannel:
				if len(trades) >= cfg.MaxTradesPerInterval {
					// TODO: Send an alert to increase buffer size
					log.Printf("Max trades per interval reached (%d), dropping trades", cfg.MaxTradesPerInterval)
					continue
				} else {
					trades = append(trades, trade)
				}
			case <-ticker.C:
				if len(trades) == 0 {
					continue
				}

				// copy trades
				tradesCopy := make([]exchange.Trade, len(trades))
				copy(tradesCopy, trades)
				// reset trades
				trades = trades[:0]

				go s.processCandle(serverstream, tradesCopy)
			}
		}

	}()

	<-ctx.Done()

	return nil
}

func (s *CandlesService) processCandle(serverstream *connect.ServerStream[candlesv1.StreamCandlesResponse], trades []exchange.Trade) {
	candle := s.tradesToCandle(trades)
	if err := serverstream.Send(candle); err != nil {
		log.Printf("failed to send candle: %v", err)
		return
	}
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
