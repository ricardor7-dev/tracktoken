# Exchange APIs

The four public ticker endpoints the suppliers call, what they return, and how each response maps to `Quote`.
None of them needs an API key. All examples were captured on 2026-10-25 with `curl`.

## Summary

| Exchange | Request for `BTC-USD`                                              | Pair format | Bid / Ask / Last                        | Unknown pair                          |
|----------|--------------------------------------------------------------------|-------------|-----------------------------------------|---------------------------------------|
| Coinbase | `GET https://api.exchange.coinbase.com/products/BTC-USD/ticker`    | `BTC-USD`   | `bid` / `ask` / `price`                 | HTTP 404, `{"message":"NotFound"}`    |
| Kraken   | `GET https://api.kraken.com/0/public/Ticker?pair=BTCUSD`           | `BTCUSD`    | `b[0]` / `a[0]` / `c[0]`                | HTTP 200, non-empty `error` array     |
| Bitstamp | `GET https://www.bitstamp.net/api/v2/ticker/btcusd/`               | `btcusd`    | `bid` / `ask` / `last`                  | HTTP 200, array of **all** tickers    |
| Gemini   | `GET https://api.gemini.com/v1/pubticker/btcusd`                   | `btcusd`    | `bid` / `ask` / `last`                  | HTTP 404, plain-text body             |

All of them send prices as **JSON strings** (`"80647.86"`), not numbers. Decode them as `string` and parse with
`strconv.ParseFloat`, or use the `json:",string"` tag option on a `float64` field.

The rate limits below are what each exchange documents for public endpoints. They were not checked in this
project. One request per exchange per `/price` call stays far below all of them.

---

## Coinbase Exchange

- **Docs:** https://docs.cdp.coinbase.com/exchange/reference/exchangerestapi_getproductticker
- **Request:** `GET {base}/products/{BASE}-{QUOTE}/ticker`, with `base = https://api.exchange.coinbase.com`
- **Pair format:** `BTC-USD`, the same as `Pair.String()`. Lower case also works.
- **Rate limit:** about 10 requests/s per IP for public endpoints.

Response (HTTP 200):

```json
{
  "ask": "80647.87",
  "bid": "80647.86",
  "volume": "2005.25047732",
  "trade_id": 1110494454,
  "price": "80647.86",
  "size": "0.00749288",
  "time": "2026-10-25T17:37:20.412855727Z",
  "rfq_volume": "71.312494"
}
```

| `Quote` field | JSON field |
|---------------|------------|
| `Bid`         | `bid`      |
| `Ask`         | `ask`      |
| `Last`        | `price`    |

Unknown pair: HTTP 404 with `{"message":"NotFound"}`. Treat any non-2xx status as an error.

---

## Kraken

- **Docs:** https://docs.kraken.com/api/docs/rest-api/get-ticker-information
- **Request:** `GET {base}/0/public/Ticker?pair={BASE}{QUOTE}`, with `base = https://api.kraken.com`
- **Pair format:** `BTCUSD` (base and quote joined, no separator). Kraken's own name for BTC is `XBT`, but it
  accepts `BTCUSD` and `XBTUSD` the same way, so no translation is needed.
- **Rate limit:** about 1 request/s per IP for public endpoints.

Response (HTTP 200):

```json
{
  "error": [],
  "result": {
    "XXBTZUSD": {
      "a": ["80650.60000", "1", "1.000"],
      "b": ["80650.50000", "1", "1.000"],
      "c": ["80650.60000", "0.00092883"],
      "v": ["533.01322826", "1062.95394034"],
      "p": ["80541.58668", "80485.31352"],
      "t": [48546, 70955],
      "l": ["80000.00000", "80000.00000"],
      "h": ["80919.20000", "80919.20000"],
      "o": "80772.00000"
    }
  }
}
```

| `Quote` field | JSON field | Meaning                                      |
|---------------|------------|----------------------------------------------|
| `Bid`         | `b[0]`     | `b` = `[price, whole lot volume, lot volume]` |
| `Ask`         | `a[0]`     | `a` = `[price, whole lot volume, lot volume]` |
| `Last`        | `c[0]`     | `c` = `[price, lot volume]` of the last trade |

Things to watch:

- **The key inside `result` is not the pair you sent.** `BTCUSD` comes back as `XXBTZUSD`, `ETHUSD` as
  `XETHZUSD`, but `SOLUSD` comes back as `SOLUSD`. Don't build the key yourself. Decode `result` as
  `map[string]tickerStruct` and take its one entry.
- **Errors come back with HTTP 200.** Check the `error` array: for an unknown pair the response is
  `{"error":["EQuery:Unknown asset pair"]}`, with no `result`.
- The arrays mix strings and numbers (`t` holds integers), so decode only the fields you need, e.g.
  `` A []string `json:"a"` ``.

---

## Bitstamp

- **Docs:** https://www.bitstamp.net/api/#tag/Tickers
- **Request:** `GET {base}/api/v2/ticker/{base}{quote}/`, with `base = https://www.bitstamp.net`
- **Pair format:** `btcusd` (lower case, no separator). Upper case also works. Keep the trailing slash.
- **Rate limit:** 400 requests/s, 10,000 requests per 10 minutes per IP.

Response (HTTP 200):

```json
{
  "timestamp": "1792949845",
  "open": "80771.95",
  "high": "80915.13",
  "low": "80000.00",
  "last": "80648.02",
  "volume": "799.48735688",
  "vwap": "80466.02",
  "bid": "80648.02",
  "ask": "80648.03",
  "side": "1",
  "open_24": "80039.47",
  "percent_change_24": "0.76",
  "market_type": "SPOT"
}
```

| `Quote` field | JSON field |
|---------------|------------|
| `Bid`         | `bid`      |
| `Ask`         | `ask`      |
| `Last`        | `last`     |

Things to watch:

- **An unknown pair does not return an error.** `/api/v2/ticker/foousd/` answers HTTP 200 with a JSON
  **array** of every ticker Bitstamp has (`BTC/USD`, `BTC/EUR`, ...). Decoding that into the ticker struct fails
  with a `json: cannot unmarshal array` error, which is enough to report it as a failure. Don't try to "fix" it
  by decoding the array, or you might pick the wrong market.

---

## Gemini

- **Docs:** https://docs.gemini.com/rest-api/#ticker
- **Request:** `GET {base}/v1/pubticker/{base}{quote}`, with `base = https://api.gemini.com`
- **Pair format:** `btcusd` (lower case, no separator). Upper case also works.
- **Rate limit:** 120 requests/min per IP for public endpoints (recommended at most 1 request/s).

Response (HTTP 200):

```json
{
  "bid": "80667.29000",
  "ask": "80667.30000",
  "last": "80667.30000",
  "volume": {
    "BTC": "34.79518989",
    "USD": "2806834.021413597",
    "timestamp": 1792949820000
  }
}
```

| `Quote` field | JSON field |
|---------------|------------|
| `Bid`         | `bid`      |
| `Ask`         | `ask`      |
| `Last`        | `last`     |

Things to watch:

- `volume` has keys named after the pair's currencies, and `timestamp` is a number while the others are strings.
  Leave `volume` out of the struct; it is not needed.
- Unknown pair: HTTP 404 with a **plain-text** body (`'foousd' does not have available data yet`), not JSON.
  Check the status code before decoding.

---

## Common rules for adapters

1. Build the request with `http.NewRequestWithContext(ctx, ...)` so timeouts and client disconnects cancel it.
2. Check `resp.StatusCode` first. A non-2xx status is an error; include the status and a short piece of the
   body in the message.
3. Always close `resp.Body`, including on errors.
4. Decode into an unexported struct that matches the exchange's JSON, then convert the string prices to
   `float64`. A price that fails to parse, or is zero, is an error.
5. Exchange-specific error signals: Kraken's `error` array, and Bitstamp's array response for an unknown pair.

Examples:
https://www.bitstamp.net/api/v2/ticker/BTCUSD