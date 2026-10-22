# Plan: Crypto price comparison service (concurrency practice)

## Context
Practice project for learning Go concurrency: worker pool, `context`, `WaitGroup`/`errgroup`, mutexes.
Real-world shape: a price/quote comparison service. One request fans out to many suppliers (crypto exchanges),
each with a different JSON format, and the results are merged into one answer. Goal: a working v1 in a few hours,
with clear places to add more topics later.

Starting point: fresh directory. Only `go.mod` (module `snsgo`, go 1.26.9) and `GO_COMMANDS.md`. No git repo.

## What it does
`GET /price?pair=BTC-USD` returns, for each exchange that answered: bid, ask, last price and latency.
It also returns the best bid, best ask, spread across exchanges, and an `errors` list for exchanges that
failed or timed out. A partial result is still HTTP 200.

## v1 scope (a few hours)
- 4 real exchanges with public, no-key ticker endpoints: **Coinbase, Kraken, Bitstamp, Gemini** (all quote in USD).
- Bounded worker pool (limit defaults to `runtime.GOMAXPROCS(0)`, overridable via `WORKERS`) so concurrency is capped even with more suppliers than workers.
- Per-supplier timeout (e.g. 2s) and overall request timeout (e.g. 3s), both via `context`.
- Client disconnect cancels all in-flight calls.
- Graceful shutdown on SIGINT/SIGTERM.
- Fake supplier server for tests (controllable delay and failure), so tests never use the internet.

Endpoint details to verify when implementing (they can change or be geo-restricted):
- Coinbase `https://api.exchange.coinbase.com/products/BTC-USD/ticker`
- Kraken `https://api.kraken.com/0/public/Ticker?pair=XBTUSD`
- Bitstamp `https://www.bitstamp.net/api/v2/ticker/btcusd/`
- Gemini `https://api.gemini.com/v1/pubticker/btcusd`

Later suppliers (extension exercise, one file each): Bitfinex, Binance (geo-blocked in some countries; use
Binance.US), OKX, KuCoin, Bybit, Crypto.com. Each needs an on/off flag in config.

## Structure
```
cmd/server/main.go
internal/supplier/    supplier.go, coinbase.go, kraken.go, bitstamp.go, gemini.go, supplier_test.go
internal/aggregator/  aggregator.go, aggregator_test.go
internal/handler/     handler.go
```
- `cmd/server/main.go`: config, wiring of concrete types, server start, signal handling, graceful shutdown
- `internal/supplier/supplier.go`: `Supplier` interface (`Name() string`, `Quote(ctx, pair) (Quote, error)`) and the common `Quote` struct
- `internal/supplier/<exchange>.go`: one small adapter per exchange (convert its JSON and pair format to `Quote`)
- `internal/aggregator/aggregator.go`: worker pool, fan-out/fan-in, per-supplier timeout, collects results and errors, computes best bid/ask/spread
- `internal/handler/handler.go`: `/price` handler, validates `pair`, builds the request context, writes JSON
- `*_test.go`: tests using `httptest.Server` fakes

Dependencies point one way: `handler` → `aggregator` → `supplier`. `internal/` keeps the packages private to the module.

## Design
- **Adapter**: each exchange file converts its own JSON and pair format into the common `Quote`.
- **Strategy via interface**: the aggregator depends on `Supplier`, not on concrete exchanges, so tests swap in
  fakes and adding the 5th to 10th supplier is a one-file change.
- **Fan-out / fan-in with a worker pool**: the concurrency core in the aggregator.
- **Dependency injection via constructors**: `aggregator.New(...)`, `handler.New(...)`; only `main.go` wires
  concrete types. No globals or singletons.
- **Decorator (later)**: cache, retries and rate limiting wrap a `Supplier` and still satisfy the interface,
  e.g. `NewCached(NewRetrying(coinbase))`.
- Skipped on purpose: factories/registries (a `[]Supplier` built in `main.go` is enough), repository/service
  layers, clean/hexagonal architecture.

### Data types (two layers, no separate API DTO)
1. **Exchange response structs** (unexported, one per adapter): match each exchange's JSON exactly (Kraken's
   dynamic nested key, Coinbase's string prices). Required to decode; they never leave the `supplier` package.
2. **`Quote` with JSON tags**: used both internally and in the API output. The handler wraps it in a small
   response struct (quotes, best bid, best ask, spread, errors) and encodes it directly.

A separate API DTO layer is not worth it for this project. Add one only if the internal model and the public
response need to diverge.

Prices: parse exchange strings into `float64` for simplicity (a real system would use a decimal type or integer
minor units).

### Endpoint URLs
Each adapter owns its URL, split in two parts:
- **Base URL** (e.g. `https://api.exchange.coinbase.com`): a constant in the adapter file used as the default,
  stored in a struct field and overridable through the constructor (`NewCoinbase(baseURL, client)`; empty string
  means the default). Tests pass an `httptest.Server` URL so they never use the internet.
- **Path and pair format** (e.g. `/products/{pair}/ticker`, Kraken's `XBTUSD`, Bitstamp's `btcusd`): hardcoded
  in the adapter, no override. This is the exchange's API contract; if it changes, the JSON usually changes too,
  so the adapter code changes anyway.

`main.go` builds the `[]Supplier` with empty base URLs (production defaults) and a shared `*http.Client`.
No config file or URL registry.

Later, only if needed (sandbox, proxy): env vars read in `main.go` (e.g. `COINBASE_URL`), falling back to the
adapter default when unset. The adapters don't change. The on/off flags for extra suppliers go in the same place.

## Steps
1. `Quote`, `Supplier` interface, and one adapter (Coinbase). Call it synchronously from the handler (baseline).
2. Add Kraken, Bitstamp and Gemini adapters. Unit-test each adapter's JSON mapping against a canned response.
3. Run suppliers concurrently with `WaitGroup`, each goroutine writing to its own slot of a results slice.
4. Add `context.WithTimeout` for the request and per supplier; use `http.NewRequestWithContext`.
5. Replace unbounded fan-out with a worker pool (jobs channel, N workers, results channel). Think through who closes which channel.
6. Partial results: failed suppliers go into `errors`, the rest are merged. Compute best bid/ask and spread.
7. Graceful shutdown in `main.go`.
8. Tests with fake suppliers: all succeed, one slow (timeout), one fails (partial result), client cancels, all fail.
9. Optional comparison: rewrite step 3 with `errgroup` and note the differences.

## Later extensions (not in v1)
- Remaining 6 suppliers behind config flags
- In-memory cache with `sync.RWMutex` and TTL, then `singleflight` to merge duplicate concurrent requests
- Background refresher with `time.Ticker`
- Rate limiting (`golang.org/x/time/rate`) and retries with backoff
- "Fastest N" mode: return as soon as N suppliers have answered and cancel the rest
- `goleak` for goroutine-leak checks, `pprof` for profiling

## Verification
- `go vet ./...` and `go build ./...`
- `go test -race ./...` (race detector on)
- `go run ./cmd/server` then `curl 'localhost:8080/price?pair=BTC-USD'`: JSON with all 4 exchanges and the spread
- Latency check: total time is about the slowest supplier, not the sum
- Block or slow one supplier (bad URL, or fake with delay above the timeout): still HTTP 200 with partial data and an `errors` entry
- Ctrl+C during a slow request: the server finishes the request, then exits cleanly
