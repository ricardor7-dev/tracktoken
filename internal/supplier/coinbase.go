// information on exchanges.md file regarding the end-point calls
package supplier

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const coinbaseURL = "https://api.exchange.coinbase.com"

type coinBase struct {
	baseUrl string
	client  *http.Client
}

func NewCoinBase(baseUrl string, client *http.Client) Supplier {
	if baseUrl == "" {
		baseUrl = coinbaseURL
	}
	return &coinBase{baseUrl: baseUrl, client: client}
}

func (s *coinBase) Name() string {
	return "coinbase"
}

func (s *coinBase) Quote(ctx context.Context, p Pair) (Quote, error) {
	url := fmt.Sprintf("%s/products/%s/ticker", s.baseUrl, p)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Quote{}, err
	}

	start := time.Now()
	res, err := s.client.Do(req)
	if err != nil {
		return Quote{}, nil
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return Quote{}, err
	}
	//validate the status code of the response
	t := coinBaseTicker{}
	if err := json.NewDecoder(res.Body).Decode(&t); err != nil {
		return Quote{}, fmt.Errorf("coinbase decode ticker: %w", err)
	}
	return Quote{
		Exchange:  s.Name(),
		Bid:       t.Bid,
		Ask:       t.Ask,
		Last:      t.Price,
		LatencyMS: time.Since(start).Milliseconds(),
	}, nil
}

type coinBaseTicker struct {
	Bid   float64 `json:"bid,string"`
	Ask   float64 `json:"ask,string"`
	Price float64 `json:"price,string"`
}
