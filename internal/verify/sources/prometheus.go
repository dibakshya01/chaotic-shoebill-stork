package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
)

// Prometheus queries a Prometheus-compatible HTTP API (`/api/v1/query`). It is the
// real adapter used against a live cluster; the offline demo uses the mock source.
type Prometheus struct {
	base   string
	client *http.Client
}

// NewPrometheus builds a Prometheus source for the given base URL (e.g.
// http://prometheus:9090).
func NewPrometheus(base string) *Prometheus {
	return &Prometheus{base: strings.TrimRight(base, "/"), client: &http.Client{Timeout: 10 * time.Second}}
}

// Name implements Source.
func (p *Prometheus) Name() model.SignalSource { return model.SourcePrometheus }

type promResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []any             `json:"value"`
		} `json:"result"`
	} `json:"data"`
	Error string `json:"error"`
}

// Query runs an instant PromQL query and summarizes the vector result.
func (p *Prometheus) Query(ctx context.Context, query, uuid string) (Observation, error) {
	u := fmt.Sprintf("%s/api/v1/query?query=%s", p.base, url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return Observation{}, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return Observation{}, fmt.Errorf("prometheus query: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Observation{}, fmt.Errorf("prometheus returned %d", resp.StatusCode)
	}
	var pr promResponse
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return Observation{}, fmt.Errorf("decode prometheus response: %w", err)
	}
	if pr.Status != "success" {
		return Observation{}, fmt.Errorf("prometheus error: %s", pr.Error)
	}

	obs := Observation{Count: len(pr.Data.Result), Found: len(pr.Data.Result) > 0}
	if len(pr.Data.Result) > 0 {
		// First sample's scalar value, if parseable.
		if len(pr.Data.Result[0].Value) == 2 {
			if s, ok := pr.Data.Result[0].Value[1].(string); ok {
				if f, perr := strconv.ParseFloat(s, 64); perr == nil {
					obs.Value = f
				}
			}
		}
		// UUID correlation: the detonator labels its artifacts with the run uuid, so a
		// correlated series carries it in a label value.
		for _, r := range pr.Data.Result {
			for _, v := range r.Metric {
				if uuid != "" && strings.Contains(v, uuid) {
					obs.MatchedUUID = true
				}
			}
		}
	}
	return obs, nil
}
