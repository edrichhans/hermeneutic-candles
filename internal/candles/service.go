package candles

import (
	"context"
	"fmt"
	"hermeneutic-candles/cmd"
	candlesv1 "hermeneutic-candles/gen/proto/candles/v1"
	"hermeneutic-candles/internal/exchange"
	"hermeneutic-candles/internal/exchange/binance"
	"hermeneutic-candles/internal/exchange/bybit"
	"hermeneutic-candles/internal/exchange/okx"
	"hermeneutic-candles/internal/tradestreamer"
	"log"
	"strings"
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

	// Parse incoming symbols
	symbolPairs, err := s.parseSymbols(req.Msg.Symbols)
	if err != nil {
		return fmt.Errorf("failed to parse symbol: %w", err)
	}

	// Initialize channels
	// This channel will receive trades from the trade streamers
	tradeChannel := make(chan exchange.Trade, cfg.TradeStreamBufferSize)
	// This channel buffers candles to be sent to the client
	candleChannel := make(chan *candlesv1.StreamCandlesResponse)

	// Initialize trade streamers for each exchange
	binanceTradeStreamer := tradestreamer.NewTradeStreamer(&binance.BinanceAdapter{})
	bybitTradeStreamer := tradestreamer.NewTradeStreamer(&bybit.BybitAdapter{})
	okxTradeStreamer := tradestreamer.NewTradeStreamer(&okx.OkxAdapter{})

	tradeStreamers := tradestreamer.NewAggregator([]*tradestreamer.TradeStreamer{
		binanceTradeStreamer,
		bybitTradeStreamer,
		okxTradeStreamer,
	})

	go tradeStreamers.StreamTrades(ctx, tradeChannel, symbolPairs)

	go s.forwardTradesToCandles(ctx, cfg, tradeChannel, candleChannel)

	go s.processCandleChannel(ctx, candleChannel, serverstream)

	<-ctx.Done()

	return nil
}

// Parses incoming symbols to exchange.SymbolPair format
//
// Ex: "btc-usdt" -> {First: "btc", Second: "usdt"}
func (s *CandlesService) parseSymbols(reqSymbols []string) ([]exchange.SymbolPair, error) {
	var symbolPairs []exchange.SymbolPair
	for _, reqSymbol := range reqSymbols {
		parts := strings.Split(reqSymbol, "-")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid symbol format: %s", reqSymbol)
		}
		symbolPairs = append(symbolPairs, exchange.SymbolPair{First: parts[0], Second: parts[1]})
	}
	return symbolPairs, nil
}

// Forwards trades to the candles channel at a specified interval
func (s *CandlesService) forwardTradesToCandles(ctx context.Context, cfg *cmd.Config, tradeChannel <-chan exchange.Trade, candleChannel chan<- *candlesv1.StreamCandlesResponse) {
	ticker := time.NewTicker(time.Duration(int32(s.intervalMillis)) * time.Millisecond)
	defer ticker.Stop()

	trades := map[string][]exchange.Trade{}
	for {
		select {
		case <-ctx.Done():
			return
		case trade := <-tradeChannel:
			if len(trades) >= cfg.MaxTradesPerInterval {
				// TODO: Send an alert to increase buffer size
				log.Printf("Max trades per interval reached (%d), dropping trades", cfg.MaxTradesPerInterval)
				continue
			}

			// Initialize the slice for the symbol if it doesn't exist
			if trades[trade.Symbol] == nil {
				trades[trade.Symbol] = []exchange.Trade{}
			}
			// Append the trade to the slice for the symbol
			trades[trade.Symbol] = append(trades[trade.Symbol], trade)

		case <-ticker.C:
			for symbol, t := range trades {
				if len(t) == 0 {
					continue
				}
				// copy trades
				tc := make([]exchange.Trade, len(t))
				copy(tc, t)
				// reset trades
				trades[symbol] = t[:0]

				candle := s.tradesToCandle(tc, symbol)
				candleChannel <- candle
			}
		}
	}
}

// Sends candles to the server stream
// This is separate from the trade processing to avoid blocking, and write methods are not concurrent-safe
func (s *CandlesService) processCandleChannel(ctx context.Context, candleChannel <-chan *candlesv1.StreamCandlesResponse, serverstream *connect.ServerStream[candlesv1.StreamCandlesResponse]) {
	for {
		select {
		case <-ctx.Done():
			return
		case candle := <-candleChannel:
			if candle == nil {
				continue
			}
			if err := serverstream.Send(candle); err != nil {
				log.Printf("failed to send candle: %v", err)
				return
			}
		}
	}
}

// Converts a slice of trades into a candle
func (s *CandlesService) tradesToCandle(trades []exchange.Trade, symbol string) *candlesv1.StreamCandlesResponse {
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
		Symbol:    symbol,
		Timestamp: time.Now().Unix(),
		Open:      open,
		High:      high,
		Low:       low,
		Close:     close,
		Volume:    volume,
	}
}
