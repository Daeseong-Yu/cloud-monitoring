package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cloud-monitor/internal/store"
)

type fakeStore struct {
	resourceEnabled bool
}

func (s *fakeStore) ListAdminResources(context.Context, string) ([]store.AdminResource, error) {
	return []store.AdminResource{{ID: 1, ServiceName: "lambda", Namespace: "AWS/Lambda", ResourceID: "orders-api", Region: "us-east-1", DisplayName: "Orders API"}}, nil
}

func (s *fakeStore) SetResourceEnabled(_ context.Context, _ int64, enabled bool) error {
	s.resourceEnabled = enabled
	return nil
}

func (s *fakeStore) ListAdminMetricDefinitions(context.Context, string) ([]store.AdminMetricDefinition, error) {
	return nil, nil
}

func (s *fakeStore) UpsertMetricDefinition(context.Context, store.MetricDefinitionInput) (int64, error) {
	return 10, nil
}

func (s *fakeStore) SetMetricDefinitionEnabled(context.Context, int64, bool) error {
	return nil
}

func (s *fakeStore) DeleteMetricDefinition(context.Context, int64) error {
	return nil
}

func (s *fakeStore) ApplyRecommendedMetricSet(context.Context, int64, []store.RecommendedMetric) (int64, error) {
	return 1, nil
}

func TestAdminRequiresBasicAuth(t *testing.T) {
	server, err := NewServer(Config{Store: &fakeStore{}, Username: "admin", Password: "secret", Region: "us-east-1"})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/admin", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
}

func TestAPIResourceEnableUsesBasicAuth(t *testing.T) {
	st := &fakeStore{}
	server, err := NewServer(Config{Store: st, Username: "admin", Password: "secret", Region: "us-east-1"})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	request := httptest.NewRequest(http.MethodPatch, "/api/resources/1/enabled", strings.NewReader(`{"enabled":true}`))
	request.Header.Set("Content-Type", "application/json")
	request.SetBasicAuth("admin", "secret")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", response.Code, response.Body.String())
	}
	if !st.resourceEnabled {
		t.Fatal("expected resource to be enabled")
	}
}

func TestLoadMetricSets(t *testing.T) {
	sets, err := LoadMetricSets(strings.NewReader(`{
	  "version": 1,
	  "metricSets": [
	    {
	      "serviceName": "lambda",
	      "namespace": "AWS/Lambda",
	      "name": "lambda-default",
	      "metrics": [
	        {"metricName": "Errors", "statistic": "Sum", "periodSeconds": 300, "unit": "Count"}
	      ]
	    }
	  ]
	}`))
	if err != nil {
		t.Fatalf("load metric sets: %v", err)
	}
	if len(sets) != 1 || sets[0].Metrics[0].MetricName != "Errors" {
		t.Fatalf("unexpected metric sets: %#v", sets)
	}
}
