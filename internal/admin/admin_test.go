package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"cloud-monitor/internal/store"
)

type fakeStore struct {
	resourceEnabled bool
	selectedMetric  bool
	resourcePublic  store.PublicMetadataInput
	metricPublic    store.PublicMetadataInput
	publicMetrics   []store.PublicMetric
	bulkSelected    int64
	bulkEnabled     bool
	bulkUpdated     int64
	bulkRecommended store.RecommendedMetricSetApplyResult
}

func (s *fakeStore) ListAdminServices(context.Context, string) ([]store.AdminService, error) {
	return []store.AdminService{{
		ServiceName:                "lambda",
		Namespace:                  "AWS/Lambda",
		ResourceCount:              1,
		AvailableMetrics:           3,
		UnselectedAvailableMetrics: 2,
		SelectedMetrics:            1,
		EnabledMetricDefinitions:   4,
		DisabledMetricDefinitions:  5,
	}}, nil
}

func (s *fakeStore) ListAdminResources(context.Context, string) ([]store.AdminResource, error) {
	return []store.AdminResource{{ID: 1, ServiceName: "lambda", Namespace: "AWS/Lambda", ResourceID: "orders-api", Region: "us-east-1", DisplayName: "Orders API"}}, nil
}

func (s *fakeStore) SetResourceEnabled(_ context.Context, _ int64, enabled bool) error {
	s.resourceEnabled = enabled
	return nil
}

func (s *fakeStore) UpdateResourcePublicMetadata(_ context.Context, _ int64, input store.PublicMetadataInput) error {
	s.resourcePublic = input
	return nil
}

func (s *fakeStore) ListAdminMetricCandidates(context.Context, string) ([]store.AdminMetricCandidate, error) {
	return []store.AdminMetricCandidate{{
		ID:                 1,
		ServiceName:        "lambda",
		ResourceIdentifier: "orders-api",
		DisplayName:        "Orders API",
		Namespace:          "AWS/Lambda",
		MetricName:         "Errors",
		Statistic:          "Sum",
		PeriodSeconds:      300,
		Unit:               "Count",
		Region:             "us-east-1",
		AvailabilityStatus: "available",
		ProviderSource:     "lambda-provider",
	}}, nil
}

func (s *fakeStore) CollectionCostEstimate(context.Context, string, int64) (store.CollectionCostEstimate, error) {
	return store.CollectionCostEstimate{
		EnabledMetricCount:             1,
		RegionCount:                    1,
		CollectorIntervalSeconds:       60,
		MonthlyCollectionRunsPerRegion: 43200,
		MonthlyMetricRequests:          43200,
		GetMetricDataPricePerThousand:  0.01,
		EstimatedMonthlyCostUSD:        0.432,
		PricingNote:                    "test estimate",
	}, nil
}

func (s *fakeStore) ListPublicMetrics(context.Context) ([]store.PublicMetric, error) {
	if s.publicMetrics != nil {
		return s.publicMetrics, nil
	}
	return []store.PublicMetric{{
		ID:                  store.PublicMetricID("orders", "errors"),
		ResourceAlias:       "orders",
		ResourceLabel:       "Orders API",
		ResourceDescription: "Public request processing",
		MetricAlias:         "errors",
		MetricLabel:         "Errors",
		MetricDescription:   "Failed requests",
		Unit:                "Count",
		LatestPointAt:       "2026-05-28 12:00:00+00",
		RecentPointCount:    12,
	}}, nil
}

func (s *fakeStore) ListPublicMetricSeries(context.Context, string, int32) ([]store.PublicMetricSeriesPoint, error) {
	return []store.PublicMetricSeriesPoint{{
		Timestamp: time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC),
		Value:     3,
	}}, nil
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

func (s *fakeStore) SetMetricDefinitionsEnabled(_ context.Context, _ string, _ string, enabled bool) (int64, error) {
	s.bulkEnabled = enabled
	s.bulkUpdated = 4
	return s.bulkUpdated, nil
}

func (s *fakeStore) UpdateMetricDefinitionPublicMetadata(_ context.Context, _ int64, input store.PublicMetadataInput) error {
	s.metricPublic = input
	return nil
}

func (s *fakeStore) DeleteMetricDefinition(context.Context, int64) error {
	return nil
}

func (s *fakeStore) ApplyRecommendedMetricSet(context.Context, int64, []store.RecommendedMetric) (int64, error) {
	return 1, nil
}

func (s *fakeStore) ApplyRecommendedMetricSetToResources(context.Context, string, string, []store.RecommendedMetric) (store.RecommendedMetricSetApplyResult, error) {
	s.bulkRecommended = store.RecommendedMetricSetApplyResult{ResourceCount: 2, AppliedCount: 8}
	return s.bulkRecommended, nil
}

func (s *fakeStore) SelectAvailableMetricCandidates(context.Context, string, string) (int64, error) {
	s.bulkSelected = 3
	return s.bulkSelected, nil
}

func (s *fakeStore) SelectMetricCandidate(context.Context, int64) (int64, error) {
	s.selectedMetric = true
	return 10, nil
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

func TestPublicMetricsDoNotRequireBasicAuth(t *testing.T) {
	server, err := NewServer(Config{Store: &fakeStore{}, Username: "admin", Password: "secret", Region: "us-east-1"})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/public/metrics", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, forbidden := range []string{"resourceId", "accountId", "arn", "tags", "sanitizedError", "i-0123456789", "123456789012", "AWS/Lambda"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("public response exposes forbidden identifier %q: %s", forbidden, body)
		}
	}
	if !strings.Contains(body, "resourceAlias") || !strings.Contains(body, "metricAlias") {
		t.Fatalf("public response does not include aliases: %s", body)
	}
}

func TestPublicMetricsEmptyResponseIsArray(t *testing.T) {
	server, err := NewServer(Config{Store: &fakeStore{publicMetrics: []store.PublicMetric{}}, Username: "admin", Password: "secret", Region: "us-east-1"})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/public/metrics", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", response.Code, response.Body.String())
	}
	if strings.TrimSpace(response.Body.String()) != "[]" {
		t.Fatalf("body = %s, want []", response.Body.String())
	}
}

func TestPublicOverviewUsesPublicAPIOnly(t *testing.T) {
	server, err := NewServer(Config{Store: &fakeStore{}, Username: "admin", Password: "secret", Region: "us-east-1"})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/public/overview", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	if !strings.Contains(response.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("content type = %q, want text/html", response.Header().Get("Content-Type"))
	}
	for _, expected := range []string{"Cloud Monitor Portfolio", "/api/public/metrics", "/series"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("public overview does not include %q: %s", expected, body)
		}
	}
	for _, forbidden := range []string{"/admin", "Discovery 실행", "metric-candidates", "metric-definitions", "resourceId", "accountId", "sanitizedError"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("public overview exposes admin or internal field %q: %s", forbidden, body)
		}
	}
}

func TestPublicMetricSeriesIsReadOnly(t *testing.T) {
	server, err := NewServer(Config{Store: &fakeStore{}, Username: "admin", Password: "secret", Region: "us-east-1"})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	id := store.PublicMetricID("orders", "errors")
	request := httptest.NewRequest(http.MethodPost, "/api/public/metrics/"+id+"/series", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", response.Code)
	}
}

func TestPublicMetricSeriesReturnsOnlyPoints(t *testing.T) {
	server, err := NewServer(Config{Store: &fakeStore{}, Username: "admin", Password: "secret", Region: "us-east-1"})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	id := store.PublicMetricID("orders", "errors")
	request := httptest.NewRequest(http.MethodGet, "/api/public/metrics/"+id+"/series?limit=10", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", response.Code, response.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode public series response: %v", err)
	}
	if _, ok := payload["points"]; !ok {
		t.Fatalf("series response does not include points: %s", response.Body.String())
	}
	for _, forbidden := range []string{"resourceId", "accountId", "arn", "tags", "sanitizedError"} {
		if strings.Contains(response.Body.String(), forbidden) {
			t.Fatalf("public series exposes forbidden identifier %q: %s", forbidden, response.Body.String())
		}
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

func TestAdminBulkActionsRequireConfirmation(t *testing.T) {
	server, err := NewServer(Config{Store: &fakeStore{}, Username: "admin", Password: "secret", Region: "us-east-1"})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/admin", nil)
	request.SetBasicAuth("admin", "secret")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, expected := range []string{"추천 세트 일괄 적용", "Advanced candidate bulk actions", "Advanced public resource metadata", "Advanced public metric metadata", "Available 일괄 수집 시작", "일괄 Enable", "일괄 Disable", "confirm(", "Bulk preview", "수집 시작 2", "enable 5 / disable 4"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("admin page does not include bulk action confirmation marker %q", expected)
		}
	}
	if strings.Index(body, "추천 세트 일괄 적용") > strings.Index(body, "Advanced candidate bulk actions") {
		t.Fatal("recommended bulk setup should appear before advanced available candidate bulk action")
	}
	if strings.Index(body, "Advanced public resource metadata") < strings.Index(body, "추천 세트 일괄 적용") {
		t.Fatal("public resource metadata should not appear before recommended setup")
	}
	if strings.Index(body, "Advanced public metric metadata") < strings.Index(body, "일괄 Enable") {
		t.Fatal("public metric metadata should not appear before collection controls")
	}
}

func TestAPIMetricCandidatesIncludesAvailability(t *testing.T) {
	server, err := NewServer(Config{Store: &fakeStore{}, Username: "admin", Password: "secret", Region: "us-east-1"})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/metric-candidates", nil)
	request.SetBasicAuth("admin", "secret")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "availabilityStatus") {
		t.Fatalf("response does not include availability status: %s", response.Body.String())
	}
}

func TestAPIServicesUsesBasicAuth(t *testing.T) {
	server, err := NewServer(Config{Store: &fakeStore{}, Username: "admin", Password: "secret", Region: "us-east-1"})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/services", nil)
	request.SetBasicAuth("admin", "secret")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "resourceCount") {
		t.Fatalf("response does not include service summary: %s", response.Body.String())
	}
}

func TestAPICostEstimateUsesBasicAuth(t *testing.T) {
	server, err := NewServer(Config{Store: &fakeStore{}, Username: "admin", Password: "secret", Region: "us-east-1"})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/cost-estimate", nil)
	request.SetBasicAuth("admin", "secret")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "estimatedMonthlyCostUsd") {
		t.Fatalf("response does not include cost estimate: %s", response.Body.String())
	}
}

func TestAPIMetricCandidateSelect(t *testing.T) {
	st := &fakeStore{}
	server, err := NewServer(Config{Store: st, Username: "admin", Password: "secret", Region: "us-east-1"})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/metric-candidates/1/select", nil)
	request.SetBasicAuth("admin", "secret")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", response.Code, response.Body.String())
	}
	if !st.selectedMetric {
		t.Fatal("expected metric candidate selection")
	}
}

func TestAPIResourcesApplyMetricSet(t *testing.T) {
	st := &fakeStore{}
	server, err := NewServer(Config{
		Store:    st,
		Username: "admin",
		Password: "secret",
		Region:   "us-east-1",
		MetricSets: []MetricSet{{
			ServiceName: "lambda",
			Namespace:   "AWS/Lambda",
			Name:        "lambda-default",
			Metrics:     []store.RecommendedMetric{{MetricName: "Errors", Statistic: "Sum", PeriodSeconds: 300, Unit: "Count"}},
		}},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/resources/apply-metric-set", strings.NewReader(`{"metricSet":"lambda-default"}`))
	request.Header.Set("Content-Type", "application/json")
	request.SetBasicAuth("admin", "secret")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", response.Code, response.Body.String())
	}
	if st.bulkRecommended.ResourceCount != 2 || st.bulkRecommended.AppliedCount != 8 {
		t.Fatalf("bulk recommended = %#v, want 2 resources and 8 applied", st.bulkRecommended)
	}
	if !strings.Contains(response.Body.String(), `"metricSet":"lambda-default"`) || !strings.Contains(response.Body.String(), `"appliedCount":8`) {
		t.Fatalf("response does not include recommended metric set result: %s", response.Body.String())
	}
}

func TestAPIMetricCandidatesSelectAvailable(t *testing.T) {
	st := &fakeStore{}
	server, err := NewServer(Config{Store: st, Username: "admin", Password: "secret", Region: "us-east-1"})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/metric-candidates/select-available?serviceName=lambda", nil)
	request.SetBasicAuth("admin", "secret")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", response.Code, response.Body.String())
	}
	if st.bulkSelected != 3 {
		t.Fatalf("bulkSelected = %d, want 3", st.bulkSelected)
	}
	if !strings.Contains(response.Body.String(), `"selected":3`) {
		t.Fatalf("response does not include selected count: %s", response.Body.String())
	}
}

func TestAPIMetricDefinitionsBulkEnabled(t *testing.T) {
	st := &fakeStore{}
	server, err := NewServer(Config{Store: st, Username: "admin", Password: "secret", Region: "us-east-1"})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	request := httptest.NewRequest(http.MethodPatch, "/api/metric-definitions/bulk-enabled", strings.NewReader(`{"enabled":true,"serviceName":"lambda"}`))
	request.Header.Set("Content-Type", "application/json")
	request.SetBasicAuth("admin", "secret")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", response.Code, response.Body.String())
	}
	if !st.bulkEnabled || st.bulkUpdated != 4 {
		t.Fatalf("bulk enabled=%v updated=%d, want true/4", st.bulkEnabled, st.bulkUpdated)
	}
	if !strings.Contains(response.Body.String(), `"updated":4`) {
		t.Fatalf("response does not include updated count: %s", response.Body.String())
	}
}

func TestAPIResourcePublicMetadataRequiresLabelWhenEnabled(t *testing.T) {
	st := &fakeStore{}
	server, err := NewServer(Config{Store: st, Username: "admin", Password: "secret", Region: "us-east-1"})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	request := httptest.NewRequest(http.MethodPatch, "/api/resources/1/public", strings.NewReader(`{"publicEnabled":true}`))
	request.Header.Set("Content-Type", "application/json")
	request.SetBasicAuth("admin", "secret")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
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

func TestLoadMetricSetsFromProductCatalog(t *testing.T) {
	sets, err := LoadMetricSetsFromProductCatalog(strings.NewReader(`{
	  "version": 1,
	  "metrics": [
	    {
	      "key": "lambda.errors",
	      "serviceName": "lambda",
	      "namespace": "AWS/Lambda",
	      "metricName": "Errors",
	      "statistic": "Sum",
	      "periodSeconds": 300,
	      "unit": "Count",
	      "requiredDimensions": ["FunctionName"],
	      "recommended": true,
	      "axis": {"min": 0},
	      "prerequisite": "",
	      "costWarning": ""
	    },
	    {
	      "key": "lambda.duration",
	      "serviceName": "lambda",
	      "namespace": "AWS/Lambda",
	      "metricName": "Duration",
	      "statistic": "Average",
	      "periodSeconds": 300,
	      "unit": "Milliseconds",
	      "requiredDimensions": ["FunctionName"],
	      "recommended": false,
	      "axis": {"min": 0},
	      "prerequisite": "",
	      "costWarning": ""
	    }
	  ]
	}`))
	if err != nil {
		t.Fatalf("load metric sets from product catalog: %v", err)
	}
	if len(sets) != 1 || sets[0].Name != "lambda-default" {
		t.Fatalf("unexpected metric sets: %#v", sets)
	}
	if len(sets[0].Metrics) != 1 || sets[0].Metrics[0].MetricName != "Errors" {
		t.Fatalf("unexpected recommended metrics: %#v", sets[0].Metrics)
	}
}
