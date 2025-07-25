package binance

import (
	"hermeneutic-candles/internal/exchange"
	"testing"
	"time"
)

func TestBinanceAdapter_HandleMessage(t *testing.T) {
	tradeChannel := make(chan exchange.Trade, 10)
	adapter := NewAdapter(tradeChannel)

	tests := []struct {
		name        string
		payload     string
		expected    exchange.Trade
		shouldError bool
	}{
		{
			name: "valid BTC trade",
			payload: `{
				"stream": "btcusdt@trade",
				"data": {
					"e": "trade",
					"E": 1753445787020,
					"s": "BTCUSDT",
					"t": 5112263122,
					"p": "116489.45000000",
					"q": "0.00032000",
					"T": 1753445787020,
					"m": false,
					"M": true
				}
			}`,
			expected: exchange.Trade{
				Symbol:    "btcusdt",
				Price:     116489.45,
				Quantity:  0.00032,
				Timestamp: time.UnixMilli(1753445787020),
				Source:    "Binance",
			},
			shouldError: false,
		},
		{
			name: "valid ETH trade",
			payload: `{
				"stream": "ethusdt@trade",
				"data": {
					"e": "trade",
					"E": 1753445787025,
					"s": "ETHUSDT",
					"t": 2845123456,
					"p": "3256.78000000",
					"q": "1.50000000",
					"T": 1753445787025,
					"m": true,
					"M": false
				}
			}`,
			expected: exchange.Trade{
				Symbol:    "ethusdt",
				Price:     3256.78,
				Quantity:  1.5,
				Timestamp: time.UnixMilli(1753445787025),
				Source:    "Binance",
			},
			shouldError: false,
		},
		{
			name: "valid SOL trade with decimal precision",
			payload: `{
				"stream": "solusdt@trade",
				"data": {
					"e": "trade",
					"E": 1753445787030,
					"s": "SOLUSDT",
					"t": 789123456,
					"p": "245.67890000",
					"q": "10.12345678",
					"T": 1753445787030,
					"m": false,
					"M": true
				}
			}`,
			expected: exchange.Trade{
				Symbol:    "solusdt",
				Price:     245.6789,
				Quantity:  10.12345678,
				Timestamp: time.UnixMilli(1753445787030),
				Source:    "Binance",
			},
			shouldError: false,
		},
		{
			name: "invalid JSON",
			payload: `{
				"stream": "btcusdt@trade",
				"data": {
					"s": "BTCUSDT",
					"p": "invalid_price",
					"q": "0.00032000",
					"T": 1753445787020
				}
			}`,
			shouldError: true,
		},
		{
			name: "malformed JSON",
			payload: `{
				"stream": "btcusdt@trade"
				"data": {
			}`,
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear the channel before each test
			for len(tradeChannel) > 0 {
				<-tradeChannel
			}

			// Handle the message
			err := adapter.HandleMessage([]byte(tt.payload))

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Check if trade was sent to channel
			if len(tradeChannel) != 1 {
				t.Errorf("Expected 1 trade in channel, got %d", len(tradeChannel))
				return
			}

			// Get the trade from channel
			receivedTrade := <-tradeChannel

			// Verify trade data
			if receivedTrade.Symbol != tt.expected.Symbol {
				t.Errorf("Expected symbol %s, got %s", tt.expected.Symbol, receivedTrade.Symbol)
			}
			if receivedTrade.Price != tt.expected.Price {
				t.Errorf("Expected price %f, got %f", tt.expected.Price, receivedTrade.Price)
			}
			if receivedTrade.Quantity != tt.expected.Quantity {
				t.Errorf("Expected quantity %f, got %f", tt.expected.Quantity, receivedTrade.Quantity)
			}
			if receivedTrade.Timestamp.UnixMilli() != tt.expected.Timestamp.UnixMilli() {
				t.Errorf("Expected timestamp %d, got %d", tt.expected.Timestamp.UnixMilli(), receivedTrade.Timestamp.UnixMilli())
			}
			if receivedTrade.Source != tt.expected.Source {
				t.Errorf("Expected source %s, got %s", tt.expected.Source, receivedTrade.Source)
			}
		})
	}
}

func TestBinanceAdapter_SymbolsToQuery(t *testing.T) {
	tradeChannel := make(chan exchange.Trade, 1)
	adapter := NewAdapter(tradeChannel)

	tests := []struct {
		name     string
		symbols  []exchange.SymbolPair
		expected string
	}{
		{
			name: "single symbol",
			symbols: []exchange.SymbolPair{
				{First: "BTC", Second: "USDT"},
			},
			expected: "streams=btcusdt@trade",
		},
		{
			name: "multiple symbols",
			symbols: []exchange.SymbolPair{
				{First: "BTC", Second: "USDT"},
				{First: "ETH", Second: "USDT"},
			},
			expected: "streams=btcusdt@trade/ethusdt@trade",
		},
		{
			name: "mixed case symbols",
			symbols: []exchange.SymbolPair{
				{First: "btc", Second: "USDT"},
				{First: "ETH", Second: "usdt"},
				{First: "SOL", Second: "USDT"},
			},
			expected: "streams=btcusdt@trade/ethusdt@trade/solusdt@trade",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.symbolsToQuery(tt.symbols)
			if result != tt.expected {
				t.Errorf("Expected query '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestBinanceAdapter_ResponseSymbolToOutputString(t *testing.T) {
	tradeChannel := make(chan exchange.Trade, 1)
	adapter := NewAdapter(tradeChannel)

	tests := []struct {
		input    string
		expected string
	}{
		{"BTCUSDT", "btcusdt"},
		{"ETHUSDT", "ethusdt"},
		{"btcusdt", "btcusdt"},
		{"SolUSDT", "solusdt"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := adapter.responseSymbolToOutputString(tt.input)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestBinanceAdapter_BinanceTradeDataToDomainTrade(t *testing.T) {
	tradeChannel := make(chan exchange.Trade, 1)
	adapter := NewAdapter(tradeChannel)

	tests := []struct {
		name     string
		input    binanceTradeData
		expected exchange.Trade
	}{
		{
			name: "BTC trade data conversion",
			input: binanceTradeData{
				Symbol:   "BTCUSDT",
				Price:    116489.45,
				Quantity: 0.00032,
				Time:     1753445787020,
			},
			expected: exchange.Trade{
				Symbol:    "btcusdt",
				Price:     116489.45,
				Quantity:  0.00032,
				Timestamp: time.UnixMilli(1753445787020),
				Source:    "Binance",
			},
		},
		{
			name: "zero values handling",
			input: binanceTradeData{
				Symbol:   "TESTUSDT",
				Price:    0.0,
				Quantity: 0.0,
				Time:     0,
			},
			expected: exchange.Trade{
				Symbol:    "testusdt",
				Price:     0.0,
				Quantity:  0.0,
				Timestamp: time.UnixMilli(0),
				Source:    "Binance",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.binanceTradeDataToDomainTrade(tt.input)

			if result.Symbol != tt.expected.Symbol {
				t.Errorf("Expected symbol %s, got %s", tt.expected.Symbol, result.Symbol)
			}
			if result.Price != tt.expected.Price {
				t.Errorf("Expected price %f, got %f", tt.expected.Price, result.Price)
			}
			if result.Quantity != tt.expected.Quantity {
				t.Errorf("Expected quantity %f, got %f", tt.expected.Quantity, result.Quantity)
			}
			if result.Timestamp.UnixMilli() != tt.expected.Timestamp.UnixMilli() {
				t.Errorf("Expected timestamp %d, got %d", tt.expected.Timestamp.UnixMilli(), result.Timestamp.UnixMilli())
			}
			if result.Source != tt.expected.Source {
				t.Errorf("Expected source %s, got %s", tt.expected.Source, result.Source)
			}
		})
	}
}
