package sources

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPSourceReportsStatusAndHeader(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get(RunHeader)
		w.WriteHeader(http.StatusUnauthorized) // 401
	}))
	defer srv.Close()

	src := NewHTTP()
	obs, err := src.Query(context.Background(), "GET "+srv.URL, "run-123")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if !obs.Found {
		t.Fatal("expected Found=true for a reachable endpoint")
	}
	if obs.Value != 401 {
		t.Fatalf("expected status 401 as value, got %v", obs.Value)
	}
	if gotHeader != "run-123" {
		t.Fatalf("expected correlation header to be sent, got %q", gotHeader)
	}
}
