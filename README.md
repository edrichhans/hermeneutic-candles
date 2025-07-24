# Hermeneutic Candles

GRPC Candles service for Hermeneutic Investments. Aggregates information from 3 CEXs (Binance, ByBit, OKX)

## Server

The server runs on `localhost:8080`

### Flags

| Name     | Flag         | Mandatory | Description                                       |
| -------- | ------------ | --------- | ------------------------------------------------- |
| interval | `--interval` | NO        | interval between candle responses in milliseconds |


### Running Locally

```sh
go run ./cmd/server/main.go --interval=5000
```

### Running on docker-compose

```sh
docker compose up -d
```

## API

### Endpoints

#### proto.candles.v1.CandlesService/StreamCandles

Streams candles given a list of symbols. Aggregates data from 3 CEXs (Binance, ByBit, OKX)

##### Request fields:

| Name    | Type     | Mandatory | Description               |
| ------- | -------- | --------- | ------------------------- |
| symbols | string[] | YES       | List of symbols to stream |


```json
{
    "symbols": ["btc-usdt", "eth-usdt"]
}
```

### cURL example

```sh
grpcurl \
    -protoset <(buf build -o -) -plaintext \
    -d '{"symbols": ["btc-usdt", "eth-usdt"]}' \
    localhost:8080 proto.candles.v1.CandlesService/StreamCandles
```

## Technical

- The service uses `connect-go` for its GRPC server
- The `adapter` pattern is implemented to easily add more exchanges
- The websocket connection automatically retries for 5 times (linear backoff) in case the connection to the server is unintentionally terminated
- There is a primitive backpressure handling by way of limiting the channel buffer size and setting the maximum number of trades per interval. This can be optimized in the future.
- Each connection to the exchange runs in a goroutine, and forwards trade data into a channel. Another goroutine converts and forwards trade data from the channel to be written to the GRPC connection


## Caveats

- The input accepts the format `"%s-%s"`, but outputs `"%s%s"`. This is intentional for now for speed
    - The reason is because converting from `"%s-%s"` to `"%s%s"` is a destructive operation (no delimiter), and it is hard to convert the format back
    - Different exchanges accepts different formats, and also outputs different format

## TODO
- [x] Query data from 3 CEXs
- [x] Accept command line input for interval
- [x] Resiliency:
    - [x] network interruption from CEX
    - [x] Recover if client disconnects
- [x] docker compose
- [x] accept symbols from GRPC request
- [x] Handle backpressure (too many messages coming in)
- [ ] Client
- [ ] Recover using candlestick data from REST endpoint
- [ ] Tests
- [ ] Handle differences in symbol encoding between request and response (btc-usdt vs btcusdt)
