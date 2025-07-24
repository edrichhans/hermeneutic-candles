# Hermeneutic Candles

Candles service for Hermeneutic Investments. Aggregates information from 3 CEXs

## TODO
- [x] Query data from 3 CEXs
- [x] Accept command line input for interval
- [x] Resiliency:
    - [x] network interruption from CEX
    - [x] Recover if client disconnects
- [x] docker compose
- [x] accept symbols from GRPC request
- [ ] Handle backpressure (too many messages coming in)
- [ ] Recover using candlestick data from REST endpoint
- [ ] Tests
- [ ] Client
- [ ] Handle differences in symbol encoding (btc-usdt vs btcusdt)
