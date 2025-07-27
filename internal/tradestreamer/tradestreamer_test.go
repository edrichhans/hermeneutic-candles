package tradestreamer

import (
	"context"
	"encoding/json"
	"errors"
	"hermeneutic-candles/cmd"
	"hermeneutic-candles/internal/exchange"
	tests "hermeneutic-candles/tests/mock_servers"
	"sync"
	"testing"
	"time"
)

// TODO:
// The test is not perfect, it is a primitive way to test the TradeStreamer.
// Ideally we have testing helpers to poll / wait for messages to be sent or received
// So we don't have to set time.Sleep()s when we want to make assertions

// Also only happy paths are tested. We can also add tests for connection errors, etc.

func setupWSServer(t *testing.T) *tests.MockWebSocketServer {
	// Get config (it will be initialized automatically)
	cfg := cmd.GetConfig()
	// Override for testing
	cfg.WSConnectionMaxRetries = 3
	cfg.WSConnectionTimeout = 5000 // 5 seconds

	// Start mock WebSocket server
	mockServer := tests.NewMockWebSocketServer(":18080")
	go func() {
		if err := mockServer.Start(); err != nil {
			t.Logf("Mock server error: %v", err)
		}
	}()
	return mockServer
}

func TestTradeStreamer_StreamTrades(t *testing.T) {
	mockServer := setupWSServer(t)
	defer mockServer.Stop()

	// Primitive way to wait for server to start. Can improve next time
	time.Sleep(100 * time.Millisecond)

	// Set up entities
	tradeChannel := make(chan exchange.Trade, 100)
	adapter := NewMockExchangeAdapter("MockExchange", "ws://localhost:18080/ws", tradeChannel)
	streamer := NewTradeStreamer(adapter)

	symbols := []exchange.SymbolPair{
		{First: "btc", Second: "usdt"},
		{First: "eth", Second: "usdt"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	var streamErr error

	wg.Add(1)
	go func() {
		defer wg.Done()
		streamErr = streamer.StreamTrades(ctx, symbols)
	}()

	// Primitive way to wait for connection to establish
	time.Sleep(500 * time.Millisecond)

	// Check that client connected to mock server
	if mockServer.GetConnectedClients() == 0 {
		t.Fatal("No clients connected to mock server")
	}

	// Send mock trade data
	tradeData1 := map[string]interface{}{
		"symbol":    "BTCUSDT",
		"price":     50000.00,
		"quantity":  0.1,
		"timestamp": 1753453611045,
		"source":    "mock",
	}

	tradeData2 := map[string]interface{}{
		"symbol":    "ETHUSDT",
		"price":     3000.00,
		"quantity":  1.0,
		"timestamp": 1753453611045,
		"source":    "mock",
	}

	trade1Bytes, _ := json.Marshal(tradeData1)
	trade2Bytes, _ := json.Marshal(tradeData2)

	mockServer.SendMessage(trade1Bytes)
	mockServer.SendMessage(trade2Bytes)

	// Primitive way to wait for trades to be processed
	time.Sleep(200 * time.Millisecond)

	// Collect received trades
	var receivedTrades []exchange.Trade
	timeout := time.After(2 * time.Second)

collectTrades:
	for len(receivedTrades) < 2 {
		select {
		case trade := <-tradeChannel:
			receivedTrades = append(receivedTrades, trade)
		case <-timeout:
			break collectTrades
		}
	}

	// Verify trades were received
	if len(receivedTrades) < 2 {
		t.Fatalf("Expected at least 2 trades, got %d", len(receivedTrades))
	}

	// Verify trade data
	btcTrade := receivedTrades[0]
	if btcTrade.Symbol != "BTCUSDT" {
		t.Errorf("Expected symbol BTCUSDT, got %s", btcTrade.Symbol)
	}
	if btcTrade.Price != 50000.00 {
		t.Errorf("Expected price 50000.00, got %f", btcTrade.Price)
	}
	if btcTrade.Quantity != 0.1 {
		t.Errorf("Expected quantity 0.1, got %f", btcTrade.Quantity)
	}

	ethTrade := receivedTrades[1]
	if ethTrade.Symbol != "ETHUSDT" {
		t.Errorf("Expected symbol ETHUSDT, got %s", ethTrade.Symbol)
	}
	if ethTrade.Price != 3000.00 {
		t.Errorf("Expected price 3000.00, got %f", ethTrade.Price)
	}
	if ethTrade.Quantity != 1.0 {
		t.Errorf("Expected quantity 1.0, got %f", ethTrade.Quantity)
	}

	cancel()
	wg.Wait()

	// Verify that streaming stopped gracefully (context canceled)
	if streamErr != nil && !errors.Is(streamErr, context.Canceled) {
		t.Errorf("Unexpected streaming error: %v", streamErr)
	}

	t.Logf("Test completed successfully. Received %d trades", len(receivedTrades))
}
