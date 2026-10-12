package sources

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
)

// RunHeader is the correlation header woven into every HTTP artifact a run produces.
const RunHeader = "X-Shoebill-Run"

// HTTP is a signal source that performs an HTTP probe and reports the status code as
// the observation Value (so an assertion can `expect value eq 401`, etc.). The query
// is "METHOD URL" (method optional, defaults to GET).
type HTTP struct {
	client *http.Client
}

// NewHTTP builds an HTTP probe source.
func NewHTTP() *HTTP {
	return &HTTP{client: &http.Client{Timeout: 10 * time.Second}}
}

// Name implements Source.
func (h *HTTP) Name() model.SignalSource { return model.SourceHTTP }

// Query performs the probe. Found is true when a response was received; Value is the
// HTTP status code. The run uuid is sent as the correlation header.
func (h *HTTP) Query(ctx context.Context, query, uuid string) (Observation, error) {
	method, target := http.MethodGet, strings.TrimSpace(query)
	if parts := strings.Fields(query); len(parts) == 2 {
		method, target = strings.ToUpper(parts[0]), parts[1]
	}
	req, err := http.NewRequestWithContext(ctx, method, target, nil)
	if err != nil {
		return Observation{}, err
	}
	if uuid != "" {
		req.Header.Set(RunHeader, uuid)
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return Observation{}, fmt.Errorf("http probe %s %s: %w", method, target, err)
	}
	defer resp.Body.Close()
	return Observation{
		Found:       true,
		Value:       float64(resp.StatusCode),
		Count:       1,
		MatchedUUID: uuid != "", // the probe itself carried the correlation header
	}, nil
}
