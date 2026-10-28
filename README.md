# small-concurrent-service-go

A small Go HTTP service that fetches crypto prices from several exchanges at the same time and merges them
into one answer. It is a practice project for Go concurrency: worker pools, `context`, `sync.WaitGroup` /
`errgroup`, and mutexes.

> **Status:** work in progress. See [PLAN.md](PLAN.md) for the build steps and what is planned next.

## What it does

One request fans out to several exchanges. Each exchange returns a different JSON format, and the service
converts each one into a common shape.

```
GET /price?pair=BTC-USD
```

For each exchange that answered, the response includes the bid, ask, last price and latency. It also includes
the best bid, best ask and spread across exchanges, plus an `errors` list for exchanges that failed or timed
out. A partial result is still HTTP 200.

Example response (shape only, values are made up):

```json
{
  "pair": "BTC-USD",
  "quotes": [
    { "exchange": "coinbase", "bid": 67010.12, "ask": 67011.40, "last": 67010.90, "latency_ms": 142 },
    { "exchange": "kraken",   "bid": 67008.50, "ask": 67012.00, "last": 67009.10, "latency_ms": 188 }
  ],
  "best_bid": { "exchange": "coinbase", "price": 67010.12 },
  "best_ask": { "exchange": "coinbase", "price": 67011.40 },
  "spread": 1.28,
  "errors": [
    { "exchange": "gemini", "error": "context deadline exceeded" }
  ]
}
```

## Exchanges

v1 uses four exchanges with public ticker endpoints that need no API key. All quote in USD.

| Exchange | Endpoint |
|----------|----------|
| Coinbase | `https://api.exchange.coinbase.com/products/BTC-USD/ticker` |
| Kraken   | `https://api.kraken.com/0/public/Ticker?pair=XBTUSD` |
| Bitstamp | `https://www.bitstamp.net/api/v2/ticker/btcusd/` |
| Gemini   | `https://api.gemini.com/v1/pubticker/btcusd` |

These endpoints can change or be blocked in some countries. See [docs/exchanges.md](docs/exchanges.md) for
each exchange's response format, field mapping and error behavior.

## How the concurrency works

- **Bounded worker pool:** a fixed number of workers (for example 3) handle the suppliers, so concurrency
  stays capped even when there are more suppliers than workers.
- **Timeouts with `context`:** each supplier has its own timeout (for example 2s), and the whole request has
  an overall timeout (for example 3s).
- **Cancellation:** if the client disconnects, all in-flight exchange calls are cancelled.
- **Partial results:** a slow or failing exchange does not fail the request. It goes into `errors` and the
  rest are still returned.
- **Graceful shutdown:** on SIGINT or SIGTERM, the server finishes in-flight requests and then exits.

Total latency should be about the slowest supplier, not the sum of all of them.

## Project layout

| File | Purpose |
|------|---------|
| `main.go` | Config, server start, signal handling, graceful shutdown |
| `handler.go` | `/price` handler: validates `pair`, builds the request context, writes JSON |
| `supplier.go` | `Supplier` interface and the common `Quote` struct |
| `suppliers_*.go` | One small adapter per exchange |
| `aggregator.go` | Worker pool, fan-out/fan-in, timeouts, best bid/ask and spread |
| `*_test.go` | Tests using `httptest.Server` fakes |

Every exchange implements the same interface:

```go
type Supplier interface {
    Name() string
    Quote(ctx context.Context, pair string) (Quote, error)
}
```

Adding a new exchange means adding one `suppliers_<name>.go` file.

## Running

Requires Go 1.26 or newer.

```sh
go run .
curl 'localhost:8080/price?pair=BTC-USD'
```

## Testing

Tests use a fake supplier server with controllable delay and failure, so they never call the internet.

```sh
go vet ./...
go test -race ./...
```

Covered scenarios: all suppliers succeed, one is slow (timeout), one fails (partial result), the client
cancels, and all fail.

## Roadmap

Not in v1, planned as later exercises:

- More exchanges behind config flags: Bitfinex, Binance.US, OKX, KuCoin, Bybit, Crypto.com
- In-memory cache with `sync.RWMutex` and a TTL, then `singleflight` to merge duplicate requests
- Background refresher with `time.Ticker`
- Rate limiting (`golang.org/x/time/rate`) and retries with backoff
- "Fastest N" mode: return once N suppliers answer and cancel the rest
- Goroutine-leak checks with `goleak` and profiling with `pprof`
