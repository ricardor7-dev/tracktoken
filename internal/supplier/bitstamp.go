// information on exchanges.md file regarding the end-point calls
package supplier

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const bitstampURL = "https://www.bitstamp.net"

type bitstamp struct {
	baseUrl string
	client  *http.Client
}

func NewBitstamp(baseUrl string, client *http.Client) Supplier {
	if baseUrl == "" {
		baseUrl = bitstampURL
	}
	return &bitstamp{baseUrl: baseUrl, client: client}
}

func (s *bitstamp) Name() string {
	return "bitstamp"
}

func (s *bitstamp) Quote(ctx context.Context, p Pair) (Quote, error) {
	url := fmt.Sprintf("%s/api/v2/ticker/%s/", s.baseUrl, strings.ToLower(p.Base+p.Quote))

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
	t := bitstampTicker{}
	if err := json.NewDecoder(res.Body).Decode(&t); err != nil {
		return Quote{}, fmt.Errorf("bitstamp decode ticker: %w", err)
	}
	return Quote{
		Exchange:  s.Name(),
		Bid:       t.Bid,
		Ask:       t.Ask,
		Last:      t.Last,
		LatencyMS: time.Since(start).Milliseconds(),
	}, nil
}

type bitstampTicker struct {
	Bid  float64 `json:"bid,string"`
	Ask  float64 `json:"ask,string"`
	Last float64 `json:"last,string"`
}
