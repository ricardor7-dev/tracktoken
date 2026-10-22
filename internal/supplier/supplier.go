package supplier

import (
	"context"
	"fmt"
	"strings"
)

// Supplier fetches a ticker quote for a pair from one exchange.
// Quote must respect ctx cancellation and return an error if the
// exchange does not list the pair.
type Supplier interface {
	Name() string
	Quote(ctx context.Context, p Pair) (Quote, error)
}

// Quote is one exchange's ticker for a pair.
type Quote struct {
	Exchange  string  `json:"exchange"`
	Bid       float64 `json:"bid"`
	Ask       float64 `json:"ask"`
	Last      float64 `json:"last"`
	LatencyMS int64   `json:"latency_ms"`
}

// Pair is a trading pair such as BTC-USD. Both fields are upper case.
type Pair struct {
	Base  string // e.g. BTC
	Quote string // e.g. USD
}

// String returns the pair as "BASE-QUOTE", e.g. "BTC-USD".
func (p Pair) String() string {
	return p.Base + "-" + p.Quote
}

// ParsePair parses "BASE-QUOTE" (case-insensitive, e.g. "btc-usd") into an upper-case Pair.
func ParsePair(s string) (Pair, error) {
	base, quote, ok := strings.Cut(strings.ToUpper(strings.TrimSpace(s)), "-")
	if !ok || base == "" || quote == "" || strings.Contains(quote, "-") {
		return Pair{}, fmt.Errorf("invalid pair %q, want BASE-QUOTE", s)
	}
	return Pair{Base: base, Quote: quote}, nil
}
