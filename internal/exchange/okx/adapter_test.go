package okx

import (
	"hermeneutic-candles/internal/exchange"
	"testing"
	"time"
)

func TestOkxAdapter_HandleMessage(t *testing.T) {
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
				"arg": {
					"channel": "trades",
					"instId": "BTC-USDT"
				},
				"data": [
					{
						"instId": "BTC-USDT",
						"tradeId": "778663706",
						"px": "115168",
						"sz": "0.00193696",
						"side": "sell",
						"ts": "1753454650864",
						"count": "3",
						"seqId": 58700214713
					}
				]
			}`,
			expected: []exchange.Trade{
				{
					Symbol:    "btcusdt",
					Price:     115168,
					Quantity:  0.00193696,
					Timestamp: time.UnixMilli(1753454650864),
					Source:    "Okx",
				},
			},
			shouldError: false,
		},
		{
			name: "valid ETH trade",
			payload: `{
				"arg": {
					"channel": "trades",
					"instId": "ETH-USDT"
				},
				"data": [
					{
						"instId": "ETH-USDT",
						"tradeId": "445123789",
						"px": "3245.67",
						"sz": "1.5",
						"side": "buy",
						"ts": "1753454650870",
						"count": "1",
						"seqId": 58700214714
					}
				]
			}`,
			expected: []exchange.Trade{
				{
					Symbol:    "ethusdt",
					Price:     3245.67,
					Quantity:  1.5,
					Timestamp: time.UnixMilli(1753454650870),
					Source:    "Okx",
				},
			},
			shouldError: false,
		},
		{
			name:        "pong message",
			payload:     `pong`,
			expected:    []exchange.Trade{},
			shouldError: false,
		},
		{
			name: "valid trade with different symbol format",
			payload: `{
				"arg": {
					"channel": "trades",
					"instId": "ADA-USDT"
				},
				"data": [
					{
						"instId": "ADA-USDT",
						"tradeId": "998877665",
						"px": "0.4567",
						"sz": "1000.123456",
						"side": "buy",
						"ts": "1753454650880",
						"count": "1",
						"seqId": 58700214717
					}
				]
			}`,
			expected: []exchange.Trade{
				{
					Symbol:    "adausdt",
					Price:     0.4567,
					Quantity:  1000.123456,
					Timestamp: time.UnixMilli(1753454650880),
					Source:    "Okx",
				},
			},
			shouldError: false,
		},
		{
			name: "invalid JSON with wrong price format",
			payload: `{
				"arg": {
					"channel": "trades",
					"instId": "BTC-USDT"
				},
				"data": [
					{
						"instId": "BTC-USDT",
						"tradeId": "778663706",
						"px": "invalid_price",
						"sz": "0.00193696",
						"side": "sell",
						"ts": "1753454650864",
						"count": "3",
						"seqId": 58700214713
					}
				]
			}`,
			shouldError: true,
		},
		{
			name: "malformed JSON",
			payload: `{
				"arg": {
					"channel": "trades"
					"instId": "BTC-USDT"
				}
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

func TestOkxAdapter_SymbolsToSubscribeArgs(t *testing.T) {
	tradeChannel := make(chan exchange.Trade, 1)
	adapter := NewAdapter(tradeChannel)

	tests := []struct {
		name     string
		symbols  []exchange.SymbolPair
		expected []subscribeArgs
	}{
		{
			name: "single symbol",
			symbols: []exchange.SymbolPair{
				{First: "BTC", Second: "USDT"},
			},
			expected: []subscribeArgs{
				{Channel: "trades", InstId: "BTC-USDT"},
			},
		},
		{
			name: "multiple symbols",
			symbols: []exchange.SymbolPair{
				{First: "BTC", Second: "USDT"},
				{First: "ETH", Second: "USDT"},
			},
			expected: []subscribeArgs{
				{Channel: "trades", InstId: "BTC-USDT"},
				{Channel: "trades", InstId: "ETH-USDT"},
			},
		},
		{
			name: "mixed case symbols",
			symbols: []exchange.SymbolPair{
				{First: "btc", Second: "usdt"},
				{First: "ETH", Second: "USDT"},
				{First: "sol", Second: "USDT"},
			},
			expected: []subscribeArgs{
				{Channel: "trades", InstId: "BTC-USDT"},
				{Channel: "trades", InstId: "ETH-USDT"},
				{Channel: "trades", InstId: "SOL-USDT"},
			},
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
				if result[i].Channel != expected.Channel {
					t.Errorf("Expected arg[%d].Channel '%s', got '%s'", i, expected.Channel, result[i].Channel)
				}
				if result[i].InstId != expected.InstId {
					t.Errorf("Expected arg[%d].InstId '%s', got '%s'", i, expected.InstId, result[i].InstId)
				}
			}
		})
	}
}

func TestOkxAdapter_ResponseSymbolToOutputString(t *testing.T) {
	tradeChannel := make(chan exchange.Trade, 1)
	adapter := NewAdapter(tradeChannel)

	tests := []struct {
		input    string
		expected string
	}{
		{"BTC-USDT", "btcusdt"},
		{"ETH-USDT", "ethusdt"},
		{"SOL-USDT", "solusdt"},
		{"ADA-USDT", "adausdt"},
		{"btc-usdt", "btcusdt"}, // Should handle lowercase
		{"INVALID", ""},         // Should handle invalid format
		{"BTC", ""},             // Should handle missing separator
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

func TestOkxAdapter_OkxTradeDataToDomainTrade(t *testing.T) {
	tradeChannel := make(chan exchange.Trade, 1)
	adapter := NewAdapter(tradeChannel)

	testData := okxTradeData{
		InstId:    "BTC-USDT",
		Price:     115168,
		Size:      0.00193696,
		TimeStamp: 1753454650864,
	}

	expected := exchange.Trade{
		Symbol:    "btcusdt",
		Price:     115168,
		Quantity:  0.00193696,
		Timestamp: time.UnixMilli(1753454650864),
		Source:    "Okx",
	}

	result := adapter.okxTradeDataToDomainTrade(testData)

	if result.Symbol != expected.Symbol {
		t.Errorf("Expected symbol %s, got %s", expected.Symbol, result.Symbol)
	}
	if result.Price != expected.Price {
		t.Errorf("Expected price %f, got %f", expected.Price, result.Price)
	}
	if result.Quantity != expected.Quantity {
		t.Errorf("Expected quantity %f, got %f", expected.Quantity, result.Quantity)
	}
	if result.Timestamp.UnixMilli() != expected.Timestamp.UnixMilli() {
		t.Errorf("Expected timestamp %d, got %d", expected.Timestamp.UnixMilli(), result.Timestamp.UnixMilli())
	}
	if result.Source != expected.Source {
		t.Errorf("Expected source %s, got %s", expected.Source, result.Source)
	}
}
