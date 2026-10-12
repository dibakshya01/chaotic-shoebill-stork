package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
)

// Alertmanager queries a Prometheus Alertmanager (`/api/v2/alerts`) for active alerts
// by name. `paged` is approximated by an active alert whose receiver indicates paging;
// production deployments can point the paging check at a dedicated paging provider.
type Alertmanager struct {
	base   string
	client *http.Client
}

// NewAlertmanager builds an Alertmanager source for the given base URL.
func NewAlertmanager(base string) *Alertmanager {
	return &Alertmanager{base: strings.TrimRight(base, "/"), client: &http.Client{Timeout: 10 * time.Second}}
}

// Name implements Source.
func (a *Alertmanager) Name() model.SignalSource { return model.SourceAlertmanager }

type amAlert struct {
	Labels map[string]string `json:"labels"`
	Status struct {
		State string `json:"state"`
	} `json:"status"`
	Receivers []struct {
		Name string `json:"name"`
	} `json:"receivers"`
}

// Query treats `query` as an alertname and reports whether a matching active alert exists.
func (a *Alertmanager) Query(ctx context.Context, query, uuid string) (Observation, error) {
	u := fmt.Sprintf("%s/api/v2/alerts?active=true&filter=%s",
		a.base, url.QueryEscape(fmt.Sprintf("alertname=%q", query)))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return Observation{}, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return Observation{}, fmt.Errorf("alertmanager query: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Observation{}, fmt.Errorf("alertmanager returned %d", resp.StatusCode)
	}
	var alerts []amAlert
	if err := json.NewDecoder(resp.Body).Decode(&alerts); err != nil {
		return Observation{}, fmt.Errorf("decode alertmanager response: %w", err)
	}
	obs := Observation{Count: len(alerts), Found: len(alerts) > 0}
	for _, al := range alerts {
		for _, v := range al.Labels {
			if uuid != "" && strings.Contains(v, uuid) {
				obs.MatchedUUID = true
			}
		}
	}
	return obs, nil
}
