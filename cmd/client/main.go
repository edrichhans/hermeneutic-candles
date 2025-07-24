package main

import (
	"context"
	"flag"
	candlesv1 "hermeneutic-candles/gen/proto/candles/v1"
	"hermeneutic-candles/gen/proto/candles/v1/candlesv1connect"
	"log"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
)

func main() {
	ctx := context.Background()
	// Initialize flags
	serverAddrFlag := flag.String("server", "http://localhost:8080", "Server address")
	symbolsFlag := flag.String("symbols", "btc-usdt", "Comma-separated list of symbols to subscribe to")
	flag.Parse()

	client := candlesv1connect.NewCandlesServiceClient(
		http.DefaultClient,
		*serverAddrFlag,
		connect.WithGRPC(),
	)

	symbols := strings.Split(*symbolsFlag, ",")
	stream, err := client.StreamCandles(
		ctx,
		connect.NewRequest(&candlesv1.StreamCandlesRequest{Symbols: symbols}),
	)
	if err != nil {
		log.Println(err)
		return
	}

	log.Println("Connected to Candles Service")
	for stream.Receive() {
		candle := stream.Msg()
		log.Println("Timestamp:", time.Unix(candle.Timestamp, 0).String())
		log.Println("Symbol:", candle.Symbol)
		log.Println("Open:", candle.Open)
		log.Println("Close:", candle.Close)
		log.Println("High:", candle.High)
		log.Println("Low:", candle.Low)
		log.Println("Volume:", candle.Volume)
		log.Println("")
	}
	log.Println("Stream closed")
}
