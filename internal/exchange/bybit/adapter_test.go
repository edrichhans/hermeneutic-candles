package bybit

import (
	"hermeneutic-candles/internal/exchange"
	"testing"
	"time"
)

func TestBybitAdapter_HandleMessage(t *testing.T) {
	// Create a buffered channel to capture trades
	tradeChannel := make(chan exchange.Trade, 10)

	// Create the adapter
	adapter := NewAdapter(tradeChannel)

	tests := []struct {
		name        string
		payload     string
		expected    []exchange.Trade
		shouldError bool
	}{
		{
			name: "valid BTC trade",
			payload: `{
				"topic": "publicTrade.BTCUSDT",
				"ts": 1753453611046,
				"type": "snapshot",
				"data": [
					{
						"i": "2290000000865397070",
						"T": 1753453611045,
						"p": "115570.7",
						"v": "0.000866",
						"S": "Buy",
						"s": "BTCUSDT",
						"BT": false,
						"RPI": false
					}
				]
			}`,
			expected: []exchange.Trade{
				{
					Symbol:    "btcusdt",
					Price:     115570.7,
					Quantity:  0.000866,
					Timestamp: time.UnixMilli(1753453611045),
					Source:    "Bybit",
				},
			},
			shouldError: false,
		},
		{
			name: "valid ETH trade",
			payload: `{
				"topic": "publicTrade.ETHUSDT",
				"ts": 1753453611050,
				"type": "snapshot", 
				"data": [
					{
						"i": "2290000000865397071",
						"T": 1753453611049,
						"p": "3245.12",
						"v": "1.5",
						"S": "Sell",
						"s": "ETHUSDT",
						"BT": false,
						"RPI": false
					}
				]
			}`,
			expected: []exchange.Trade{
				{
					Symbol:    "ethusdt",
					Price:     3245.12,
					Quantity:  1.5,
					Timestamp: time.UnixMilli(1753453611049),
					Source:    "Bybit",
				},
			},
			shouldError: false,
		},
		{
			name: "pong message",
			payload: `{
				"success":true,
				"ret_msg":"pong",
				"conn_id":"4ca12100-57fb-4821-860e-1b332ac7dace",
				"op":"ping"
			}`,
			expected:    []exchange.Trade{},
			shouldError: false,
		},
		{
			name: "invalid JSON with wrong price format",
			payload: `{
				"topic": "publicTrade.BTCUSDT",
				"ts": 1753453611046,
				"type": "snapshot",
				"data": [
					{
						"i": "2290000000865397070",
						"T": 1753453611045,
						"p": "invalid_price",
						"v": "0.000866",
						"S": "Buy",
						"s": "BTCUSDT",
						"BT": false,
						"RPI": false
					}
				]
			}`,
			shouldError: true,
		},
		{
			name: "malformed JSON",
			payload: `{
				"topic": "publicTrade.BTCUSDT"
				"data": [
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

			// Check number of trades sent to channel
			expectedCount := len(tt.expected)
			if len(tradeChannel) != expectedCount {
				t.Errorf("Expected %d trades in channel, got %d", expectedCount, len(tradeChannel))
				return
			}

			// Verify each trade
			for i, expectedTrade := range tt.expected {
				receivedTrade := <-tradeChannel

				if receivedTrade.Symbol != expectedTrade.Symbol {
					t.Errorf("Trade %d: Expected symbol %s, got %s", i, expectedTrade.Symbol, receivedTrade.Symbol)
				}
				if receivedTrade.Price != expectedTrade.Price {
					t.Errorf("Trade %d: Expected price %f, got %f", i, expectedTrade.Price, receivedTrade.Price)
				}
				if receivedTrade.Quantity != expectedTrade.Quantity {
					t.Errorf("Trade %d: Expected quantity %f, got %f", i, expectedTrade.Quantity, receivedTrade.Quantity)
				}
				if receivedTrade.Timestamp.UnixMilli() != expectedTrade.Timestamp.UnixMilli() {
					t.Errorf("Trade %d: Expected timestamp %d, got %d", i, expectedTrade.Timestamp.UnixMilli(), receivedTrade.Timestamp.UnixMilli())
				}
				if receivedTrade.Source != expectedTrade.Source {
					t.Errorf("Trade %d: Expected source %s, got %s", i, expectedTrade.Source, receivedTrade.Source)
				}
			}
		})
	}
}

func TestBybitAdapter_SymbolsToSubscribeArgs(t *testing.T) {
	tradeChannel := make(chan exchange.Trade, 1)
	adapter := NewAdapter(tradeChannel)

	tests := []struct {
		name     string
		symbols  []exchange.SymbolPair
		expected []string
	}{
		{
			name: "single symbol",
			symbols: []exchange.SymbolPair{
				{First: "BTC", Second: "USDT"},
			},
			expected: []string{"publicTrade.BTCUSDT"},
		},
		{
			name: "multiple symbols",
			symbols: []exchange.SymbolPair{
				{First: "BTC", Second: "USDT"},
				{First: "ETH", Second: "USDT"},
			},
			expected: []string{"publicTrade.BTCUSDT", "publicTrade.ETHUSDT"},
		},
		{
			name: "mixed case symbols",
			symbols: []exchange.SymbolPair{
				{First: "btc", Second: "usdt"},
				{First: "ETH", Second: "USDT"},
				{First: "sol", Second: "USDT"},
			},
			expected: []string{"publicTrade.BTCUSDT", "publicTrade.ETHUSDT", "publicTrade.SOLUSDT"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.symbolsToSubscribeArgs(tt.symbols)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d args, got %d", len(tt.expected), len(result))
				return
			}
			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("Expected arg[%d] '%s', got '%s'", i, expected, result[i])
				}
			}
		})
	}
}

func TestBybitAdapter_ResponseSymbolToOutputString(t *testing.T) {
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

func TestBybitAdapter_BybitTradeDataToDomainTrade(t *testing.T) {
	tradeChannel := make(chan exchange.Trade, 1)
	adapter := NewAdapter(tradeChannel)

	tests := []struct {
		name     string
		input    bybitTradeData
		expected exchange.Trade
	}{
		{
			name: "BTC trade data conversion",
			input: bybitTradeData{
				Symbol:   "BTCUSDT",
				Price:    115570.7,
				Quantity: 0.000866,
				Time:     1753453611045,
			},
			expected: exchange.Trade{
				Symbol:    "btcusdt",
				Price:     115570.7,
				Quantity:  0.000866,
				Timestamp: time.UnixMilli(1753453611045),
				Source:    "Bybit",
			},
		},
		{
			name: "zero values handling",
			input: bybitTradeData{
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
				Source:    "Bybit",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.bybitTradeDataToDomainTrade(tt.input)

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
