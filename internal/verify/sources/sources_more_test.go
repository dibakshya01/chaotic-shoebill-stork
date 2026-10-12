package sources

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPrometheusSource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[
			{"metric":{"__name__":"sidecar_unhealthy_total","run":"uuid-42"},"value":[1700000000,"3"]}]}}`))
	}))
	defer srv.Close()

	src := NewPrometheus(srv.URL)
	if src.Name() != "prometheus" {
		t.Fatalf("name = %s", src.Name())
	}
	obs, err := src.Query(context.Background(), "sidecar_unhealthy_total", "uuid-42")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if !obs.Found || obs.Count != 1 || obs.Value != 3 {
		t.Fatalf("unexpected obs: %+v", obs)
	}
	if !obs.MatchedUUID {
		t.Fatal("uuid in a label must set MatchedUUID")
	}
	// A different uuid must NOT correlate.
	obs2, _ := src.Query(context.Background(), "x", "other")
	if obs2.MatchedUUID {
		t.Fatal("uncorrelated uuid must not match")
	}
}

func TestPrometheusErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	if _, err := NewPrometheus(srv.URL).Query(context.Background(), "x", ""); err == nil {
		t.Fatal("a 500 from prometheus must surface an error (so the engine records INCONCLUSIVE)")
	}
}

func TestPrometheusMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{not json`))
	}))
	defer srv.Close()
	if _, err := NewPrometheus(srv.URL).Query(context.Background(), "x", ""); err == nil {
		t.Fatal("malformed prometheus JSON must error")
	}
}

func TestAlertmanagerSource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"labels":{"alertname":"SidecarUnhealthy","run":"uuid-7"},"status":{"state":"active"},"receivers":[{"name":"pagerduty"}]}]`))
	}))
	defer srv.Close()

	src := NewAlertmanager(srv.URL)
	if src.Name() != "alertmanager" {
		t.Fatalf("name = %s", src.Name())
	}
	obs, err := src.Query(context.Background(), "SidecarUnhealthy", "uuid-7")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if !obs.Found || obs.Count != 1 {
		t.Fatalf("expected one active alert, got %+v", obs)
	}
	if !obs.MatchedUUID {
		t.Fatal("uuid in an alert label must set MatchedUUID")
	}
}

func TestAlertmanagerNoAlerts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	obs, err := NewAlertmanager(srv.URL).Query(context.Background(), "Nope", "")
	if err != nil {
		t.Fatal(err)
	}
	if obs.Found {
		t.Fatal("no alerts must mean Found=false")
	}
}
