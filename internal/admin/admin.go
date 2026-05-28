package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"cloud-monitor/internal/store"
)

type Store interface {
	ListAdminServices(context.Context, string) ([]store.AdminService, error)
	ListAdminResources(context.Context, string) ([]store.AdminResource, error)
	ListAdminMetricCandidates(context.Context, string) ([]store.AdminMetricCandidate, error)
	CollectionCostEstimate(context.Context, string, int64) (store.CollectionCostEstimate, error)
	ListPublicMetrics(context.Context) ([]store.PublicMetric, error)
	ListPublicMetricSeries(context.Context, string, int32) ([]store.PublicMetricSeriesPoint, error)
	SetResourceEnabled(context.Context, int64, bool) error
	UpdateResourcePublicMetadata(context.Context, int64, store.PublicMetadataInput) error
	ListAdminMetricDefinitions(context.Context, string) ([]store.AdminMetricDefinition, error)
	UpsertMetricDefinition(context.Context, store.MetricDefinitionInput) (int64, error)
	SetMetricDefinitionEnabled(context.Context, int64, bool) error
	SetMetricDefinitionsEnabled(context.Context, string, string, bool) (int64, error)
	UpdateMetricDefinitionPublicMetadata(context.Context, int64, store.PublicMetadataInput) error
	DeleteMetricDefinition(context.Context, int64) error
	ApplyRecommendedMetricSet(context.Context, int64, []store.RecommendedMetric) (int64, error)
	ApplyRecommendedMetricSetToResources(context.Context, string, string, []store.RecommendedMetric) (store.RecommendedMetricSetApplyResult, error)
	SelectAvailableMetricCandidates(context.Context, string, string) (int64, error)
	SelectMetricCandidate(context.Context, int64) (int64, error)
}

type DiscoveryRunner interface {
	Run(context.Context) (string, error)
}

type MetricSet struct {
	ServiceName string                    `json:"serviceName"`
	Namespace   string                    `json:"namespace"`
	Name        string                    `json:"name"`
	Metrics     []store.RecommendedMetric `json:"metrics"`
}

type Server struct {
	store      Store
	discovery  DiscoveryRunner
	username   string
	password   string
	region     string
	interval   int64
	metricSets []MetricSet
	templates  *template.Template
}

type Config struct {
	Store      Store
	Discovery  DiscoveryRunner
	Username   string
	Password   string
	Region     string
	Interval   int64
	MetricSets []MetricSet
}

func NewServer(cfg Config) (*Server, error) {
	if cfg.Store == nil {
		return nil, fmt.Errorf("admin store is required")
	}
	if strings.TrimSpace(cfg.Username) == "" {
		return nil, fmt.Errorf("ADMIN_USERNAME is required")
	}
	if strings.TrimSpace(cfg.Password) == "" {
		return nil, fmt.Errorf("ADMIN_PASSWORD is required")
	}
	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = "us-east-1"
	}
	interval := cfg.Interval
	if interval <= 0 {
		interval = 60
	}
	return &Server{
		store:      cfg.Store,
		discovery:  cfg.Discovery,
		username:   cfg.Username,
		password:   cfg.Password,
		region:     region,
		interval:   interval,
		metricSets: cfg.MetricSets,
		templates:  template.Must(template.New("admin").Parse(pageTemplate + publicPageTemplate)),
	}, nil
}

func (s *Server) Handler() http.Handler {
	publicMux := http.NewServeMux()
	publicMux.HandleFunc("/public/overview", s.handlePublicOverview)
	publicMux.HandleFunc("/api/public/metrics", s.handleAPIPublicMetrics)
	publicMux.HandleFunc("/api/public/metrics/", s.handleAPIPublicMetricSeries)

	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/", s.redirectRoot)
	adminMux.HandleFunc("/admin", s.handleAdmin)
	adminMux.HandleFunc("/admin/discovery/run", s.handleRunDiscovery)
	adminMux.HandleFunc("/admin/resources/apply-metric-set", s.handleAdminResourcesApplyMetricSet)
	adminMux.HandleFunc("/admin/resources/", s.handleAdminResourceAction)
	adminMux.HandleFunc("/admin/metric-candidates/select-available", s.handleAdminMetricCandidatesSelectAvailable)
	adminMux.HandleFunc("/admin/metric-candidates/", s.handleAdminMetricCandidateAction)
	adminMux.HandleFunc("/admin/metric-definitions", s.handleAdminMetricDefinitions)
	adminMux.HandleFunc("/admin/metric-definitions/bulk-enabled", s.handleAdminMetricDefinitionsBulkEnabled)
	adminMux.HandleFunc("/admin/metric-definitions/", s.handleAdminMetricDefinitionAction)
	adminMux.HandleFunc("/api/services", s.handleAPIServices)
	adminMux.HandleFunc("/api/cost-estimate", s.handleAPICostEstimate)
	adminMux.HandleFunc("/api/resources", s.handleAPIResources)
	adminMux.HandleFunc("/api/resources/apply-metric-set", s.handleAPIResourcesApplyMetricSet)
	adminMux.HandleFunc("/api/resources/", s.handleAPIResourceAction)
	adminMux.HandleFunc("/api/metric-candidates", s.handleAPIMetricCandidates)
	adminMux.HandleFunc("/api/metric-candidates/select-available", s.handleAPIMetricCandidatesSelectAvailable)
	adminMux.HandleFunc("/api/metric-candidates/", s.handleAPIMetricCandidateAction)
	adminMux.HandleFunc("/api/metric-definitions", s.handleAPIMetricDefinitions)
	adminMux.HandleFunc("/api/metric-definitions/bulk-enabled", s.handleAPIMetricDefinitionsBulkEnabled)
	adminMux.HandleFunc("/api/metric-definitions/", s.handleAPIMetricDefinitionAction)

	root := http.NewServeMux()
	root.Handle("/public/overview", publicMux)
	root.Handle("/api/public/metrics", publicMux)
	root.Handle("/api/public/metrics/", publicMux)
	root.Handle("/", s.basicAuth(adminMux))
	return root
}

func (s *Server) basicAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || username != s.username || password != s.password {
			w.Header().Set("WWW-Authenticate", `Basic realm="cloud-monitor-admin"`)
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) redirectRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusFound)
}

func (s *Server) handleAdmin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	services, resources, candidates, definitions, cost, err := s.dashboardData(r.Context(), r.URL.Query().Get("region"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := struct {
		Region      string
		Services    []store.AdminService
		Resources   []store.AdminResource
		Candidates  []store.AdminMetricCandidate
		Definitions []store.AdminMetricDefinition
		Cost        store.CollectionCostEstimate
		MetricSets  []MetricSet
	}{
		Region:      s.requestRegion(r),
		Services:    services,
		Resources:   resources,
		Candidates:  candidates,
		Definitions: definitions,
		Cost:        cost,
		MetricSets:  s.metricSets,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "page", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handlePublicOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "public", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleAPIPublicMetrics(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/public/metrics" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	metrics, err := s.store.ListPublicMetrics(r.Context())
	if err != nil {
		http.Error(w, "public metrics are unavailable", http.StatusInternalServerError)
		return
	}
	writeJSON(w, metrics)
}

func (s *Server) handleAPIPublicMetricSeries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	id, ok := strings.CutPrefix(strings.Trim(r.URL.Path, "/"), "api/public/metrics/")
	if !ok {
		http.NotFound(w, r)
		return
	}
	id, ok = strings.CutSuffix(id, "/series")
	if !ok || strings.TrimSpace(id) == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}
	limit := int32(288)
	if value := strings.TrimSpace(r.URL.Query().Get("limit")); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 32)
		if err != nil || parsed <= 0 {
			http.Error(w, "limit must be a positive number", http.StatusBadRequest)
			return
		}
		limit = int32(parsed)
	}
	points, err := s.store.ListPublicMetricSeries(r.Context(), id, limit)
	if err != nil {
		http.Error(w, "public metric series is unavailable", http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{
		"id":     id,
		"points": points,
	})
}

func (s *Server) dashboardData(ctx context.Context, region string) ([]store.AdminService, []store.AdminResource, []store.AdminMetricCandidate, []store.AdminMetricDefinition, store.CollectionCostEstimate, error) {
	region = strings.TrimSpace(region)
	if region == "" {
		region = s.region
	}
	services, err := s.store.ListAdminServices(ctx, region)
	if err != nil {
		return nil, nil, nil, nil, store.CollectionCostEstimate{}, err
	}
	resources, err := s.store.ListAdminResources(ctx, region)
	if err != nil {
		return nil, nil, nil, nil, store.CollectionCostEstimate{}, err
	}
	candidates, err := s.store.ListAdminMetricCandidates(ctx, region)
	if err != nil {
		return nil, nil, nil, nil, store.CollectionCostEstimate{}, err
	}
	definitions, err := s.store.ListAdminMetricDefinitions(ctx, region)
	if err != nil {
		return nil, nil, nil, nil, store.CollectionCostEstimate{}, err
	}
	cost, err := s.store.CollectionCostEstimate(ctx, region, s.interval)
	if err != nil {
		return nil, nil, nil, nil, store.CollectionCostEstimate{}, err
	}
	return services, resources, candidates, definitions, cost, nil
}

func (s *Server) handleRunDiscovery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if s.discovery == nil {
		http.Error(w, "discovery runner is not configured", http.StatusServiceUnavailable)
		return
	}
	message, err := s.discovery.Run(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]string{"status": "ok", "message": message})
}

func (s *Server) handleAdminResourceAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	id, action, err := parseIDAction(r.URL.Path, "/admin/resources/")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	switch action {
	case "enable":
		err = s.store.SetResourceEnabled(r.Context(), id, true)
	case "disable":
		err = s.store.SetResourceEnabled(r.Context(), id, false)
	case "apply-metric-set":
		metricSet, ok := s.metricSetByName(r.FormValue("metric_set"))
		if !ok {
			http.Error(w, "metric set not found", http.StatusBadRequest)
			return
		}
		_, err = s.store.ApplyRecommendedMetricSet(r.Context(), id, metricSet.Metrics)
	case "public":
		input, inputErr := publicMetadataInputFromRequest(r)
		if inputErr != nil {
			http.Error(w, inputErr.Error(), http.StatusBadRequest)
			return
		}
		err = s.store.UpdateResourcePublicMetadata(r.Context(), id, input)
	default:
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/admin?region="+s.requestRegion(r), http.StatusSeeOther)
}

func (s *Server) handleAdminResourcesApplyMetricSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	metricSet, ok := s.metricSetByName(r.FormValue("metric_set"))
	if !ok {
		http.Error(w, "metric set not found", http.StatusBadRequest)
		return
	}
	if _, err := s.store.ApplyRecommendedMetricSetToResources(r.Context(), s.requestRegion(r), metricSet.ServiceName, metricSet.Metrics); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/admin?region="+s.requestRegion(r), http.StatusSeeOther)
}

func (s *Server) handleAdminMetricCandidateAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	id, action, err := parseIDAction(r.URL.Path, "/admin/metric-candidates/")
	if err != nil || action != "select" {
		http.NotFound(w, r)
		return
	}
	if _, err := s.store.SelectMetricCandidate(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/admin?region="+s.requestRegion(r), http.StatusSeeOther)
}

func (s *Server) handleAdminMetricCandidatesSelectAvailable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if _, err := s.store.SelectAvailableMetricCandidates(r.Context(), s.requestRegion(r), strings.TrimSpace(r.FormValue("service_name"))); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/admin?region="+s.requestRegion(r), http.StatusSeeOther)
}

func (s *Server) handleAdminMetricDefinitions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	input, err := metricDefinitionInputFromRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if _, err := s.store.UpsertMetricDefinition(r.Context(), input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/admin?region="+s.requestRegion(r), http.StatusSeeOther)
}

func (s *Server) handleAdminMetricDefinitionsBulkEnabled(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	enabled, err := enabledFromRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if _, err := s.store.SetMetricDefinitionsEnabled(r.Context(), s.requestRegion(r), strings.TrimSpace(r.FormValue("service_name")), enabled); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/admin?region="+s.requestRegion(r), http.StatusSeeOther)
}

func (s *Server) handleAdminMetricDefinitionAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	id, action, err := parseIDAction(r.URL.Path, "/admin/metric-definitions/")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	switch action {
	case "enable":
		err = s.store.SetMetricDefinitionEnabled(r.Context(), id, true)
	case "disable":
		err = s.store.SetMetricDefinitionEnabled(r.Context(), id, false)
	case "delete":
		err = s.store.DeleteMetricDefinition(r.Context(), id)
	case "public":
		input, inputErr := publicMetadataInputFromRequest(r)
		if inputErr != nil {
			http.Error(w, inputErr.Error(), http.StatusBadRequest)
			return
		}
		err = s.store.UpdateMetricDefinitionPublicMetadata(r.Context(), id, input)
	default:
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/admin?region="+s.requestRegion(r), http.StatusSeeOther)
}

func (s *Server) handleAPIServices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	services, err := s.store.ListAdminServices(r.Context(), s.requestRegion(r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, services)
}

func (s *Server) handleAPICostEstimate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	cost, err := s.store.CollectionCostEstimate(r.Context(), s.requestRegion(r), s.interval)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, cost)
}

func (s *Server) handleAPIResources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	resources, err := s.store.ListAdminResources(r.Context(), s.requestRegion(r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, resources)
}

func (s *Server) handleAPIResourcesApplyMetricSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	metricSetName := strings.TrimSpace(r.URL.Query().Get("metricSet"))
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var payload struct {
			MetricSet string `json:"metricSet"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		metricSetName = strings.TrimSpace(payload.MetricSet)
	}
	metricSet, ok := s.metricSetByName(metricSetName)
	if !ok {
		http.Error(w, "metric set not found", http.StatusBadRequest)
		return
	}
	result, err := s.store.ApplyRecommendedMetricSetToResources(r.Context(), s.requestRegion(r), metricSet.ServiceName, metricSet.Metrics)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{
		"metricSet":     metricSet.Name,
		"serviceName":   metricSet.ServiceName,
		"resourceCount": result.ResourceCount,
		"appliedCount":  result.AppliedCount,
	})
}

func (s *Server) handleAPIResourceAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch && r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	id, action, err := parseIDAction(r.URL.Path, "/api/resources/")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	switch action {
	case "enabled":
		enabled, err := enabledFromRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := s.store.SetResourceEnabled(r.Context(), id, enabled); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"id": id, "enabled": enabled})
	case "public":
		input, err := publicMetadataInputFromRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := s.store.UpdateResourcePublicMetadata(r.Context(), id, input); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"id": id, "publicEnabled": input.PublicEnabled})
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleAPIMetricDefinitions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		definitions, err := s.store.ListAdminMetricDefinitions(r.Context(), s.requestRegion(r))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, definitions)
	case http.MethodPost:
		input, err := metricDefinitionInputFromRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		id, err := s.store.UpsertMetricDefinition(r.Context(), input)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"id": id})
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleAPIMetricDefinitionsBulkEnabled(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPatch {
		methodNotAllowed(w)
		return
	}
	serviceName := strings.TrimSpace(r.URL.Query().Get("serviceName"))
	var enabled bool
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var payload struct {
			Enabled     bool   `json:"enabled"`
			ServiceName string `json:"serviceName"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		enabled = payload.Enabled
		serviceName = strings.TrimSpace(payload.ServiceName)
	} else {
		var err error
		enabled, err = enabledFromRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	updated, err := s.store.SetMetricDefinitionsEnabled(r.Context(), s.requestRegion(r), serviceName, enabled)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"updated": updated, "enabled": enabled, "serviceName": serviceName})
}

func (s *Server) handleAPIMetricCandidates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	candidates, err := s.store.ListAdminMetricCandidates(r.Context(), s.requestRegion(r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, candidates)
}

func (s *Server) handleAPIMetricCandidatesSelectAvailable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	serviceName := strings.TrimSpace(r.URL.Query().Get("serviceName"))
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var payload struct {
			ServiceName string `json:"serviceName"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		serviceName = strings.TrimSpace(payload.ServiceName)
	}
	selected, err := s.store.SelectAvailableMetricCandidates(r.Context(), s.requestRegion(r), serviceName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"selected": selected, "serviceName": serviceName})
}

func (s *Server) handleAPIMetricCandidateAction(w http.ResponseWriter, r *http.Request) {
	id, action, err := parseIDAction(r.URL.Path, "/api/metric-candidates/")
	if err != nil || action != "select" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	definitionID, err := s.store.SelectMetricCandidate(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"id": id, "metricDefinitionId": definitionID, "selected": true})
}

func (s *Server) handleAPIMetricDefinitionAction(w http.ResponseWriter, r *http.Request) {
	id, action, err := parseIDAction(r.URL.Path, "/api/metric-definitions/")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	switch {
	case action == "enabled" && (r.Method == http.MethodPatch || r.Method == http.MethodPost):
		enabled, err := enabledFromRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := s.store.SetMetricDefinitionEnabled(r.Context(), id, enabled); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"id": id, "enabled": enabled})
	case action == "" && r.Method == http.MethodDelete:
		if err := s.store.DeleteMetricDefinition(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"id": id, "deleted": true})
	case action == "public" && (r.Method == http.MethodPatch || r.Method == http.MethodPost):
		input, err := publicMetadataInputFromRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := s.store.UpdateMetricDefinitionPublicMetadata(r.Context(), id, input); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"id": id, "publicEnabled": input.PublicEnabled})
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) requestRegion(r *http.Request) string {
	region := strings.TrimSpace(r.URL.Query().Get("region"))
	if region == "" {
		region = strings.TrimSpace(r.FormValue("region"))
	}
	if region == "" {
		region = s.region
	}
	return region
}

func (s *Server) metricSetByName(name string) (MetricSet, bool) {
	for _, metricSet := range s.metricSets {
		if metricSet.Name == name {
			return metricSet, true
		}
	}
	return MetricSet{}, false
}

func metricDefinitionInputFromRequest(r *http.Request) (store.MetricDefinitionInput, error) {
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var input store.MetricDefinitionInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			return store.MetricDefinitionInput{}, err
		}
		if err := validateMetricDefinitionInput(input); err != nil {
			return store.MetricDefinitionInput{}, err
		}
		return input, nil
	}

	if err := r.ParseForm(); err != nil {
		return store.MetricDefinitionInput{}, err
	}
	period, err := strconv.ParseInt(defaultString(r.FormValue("period_seconds"), "300"), 10, 32)
	if err != nil {
		return store.MetricDefinitionInput{}, fmt.Errorf("period_seconds must be a number")
	}
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	input := store.MetricDefinitionInput{
		ID:             id,
		ServiceName:    strings.TrimSpace(r.FormValue("service_name")),
		Namespace:      strings.TrimSpace(r.FormValue("namespace")),
		MetricName:     strings.TrimSpace(r.FormValue("metric_name")),
		ResourceID:     strings.TrimSpace(r.FormValue("resource_id")),
		Region:         strings.TrimSpace(r.FormValue("region")),
		DimensionsJSON: strings.TrimSpace(defaultString(r.FormValue("dimensions"), "[]")),
		Statistic:      strings.TrimSpace(r.FormValue("statistic")),
		PeriodSeconds:  int32(period),
		Unit:           strings.TrimSpace(r.FormValue("unit")),
		Enabled:        r.FormValue("enabled") == "true" || r.FormValue("enabled") == "on",
	}
	if err := validateMetricDefinitionInput(input); err != nil {
		return store.MetricDefinitionInput{}, err
	}
	return input, nil
}

func validateMetricDefinitionInput(input store.MetricDefinitionInput) error {
	if input.ServiceName == "" || input.Namespace == "" || input.MetricName == "" || input.ResourceID == "" || input.Region == "" || input.Statistic == "" {
		return fmt.Errorf("service_name, namespace, metric_name, resource_id, region, statistic are required")
	}
	if input.PeriodSeconds <= 0 {
		return fmt.Errorf("period_seconds must be positive")
	}
	if input.DimensionsJSON == "" {
		return nil
	}
	var dimensions []store.Dimension
	if err := json.Unmarshal([]byte(input.DimensionsJSON), &dimensions); err != nil {
		return fmt.Errorf("dimensions must be JSON")
	}
	return nil
}

func publicMetadataInputFromRequest(r *http.Request) (store.PublicMetadataInput, error) {
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var input store.PublicMetadataInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			return store.PublicMetadataInput{}, err
		}
		if input.PublicEnabled && strings.TrimSpace(input.PublicLabel) == "" {
			return store.PublicMetadataInput{}, fmt.Errorf("public_label is required when public_enabled is true")
		}
		return input, nil
	}

	if err := r.ParseForm(); err != nil {
		return store.PublicMetadataInput{}, err
	}
	sortOrder, err := strconv.ParseInt(defaultString(r.FormValue("public_sort_order"), "0"), 10, 32)
	if err != nil {
		return store.PublicMetadataInput{}, fmt.Errorf("public_sort_order must be a number")
	}
	input := store.PublicMetadataInput{
		PublicEnabled:     r.FormValue("public_enabled") == "true" || r.FormValue("public_enabled") == "on",
		PublicDisplayName: strings.TrimSpace(r.FormValue("public_display_name")),
		PublicDescription: strings.TrimSpace(r.FormValue("public_description")),
		PublicLabel:       strings.TrimSpace(r.FormValue("public_label")),
		PublicSortOrder:   int32(sortOrder),
	}
	if input.PublicEnabled && input.PublicLabel == "" {
		return store.PublicMetadataInput{}, fmt.Errorf("public_label is required when public_enabled is true")
	}
	return input, nil
}

func enabledFromRequest(r *http.Request) (bool, error) {
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var payload struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			return false, err
		}
		return payload.Enabled, nil
	}
	if err := r.ParseForm(); err != nil {
		return false, err
	}
	value := r.FormValue("enabled")
	return value == "true" || value == "on" || value == "1", nil
}

func parseIDAction(path string, prefix string) (int64, string, error) {
	rest := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if rest == "" || rest == path {
		return 0, "", fmt.Errorf("missing id")
	}
	parts := strings.Split(rest, "/")
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, "", err
	}
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}
	return id, action, nil
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func methodNotAllowed(w http.ResponseWriter) {
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

const pageTemplate = `{{define "page"}}<!doctype html>
<html lang="ko">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Cloud Monitor Admin</title>
  <script src="https://unpkg.com/htmx.org@1.9.12"></script>
  <style>
    body { font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 0; color: #17202a; background: #f6f8fb; }
    header { background: #17324d; color: #fff; padding: 18px 28px; }
    main { max-width: 1180px; margin: 0 auto; padding: 24px; }
    section { margin-bottom: 28px; }
    h1 { font-size: 22px; margin: 0; }
    h2 { font-size: 18px; margin: 0 0 12px; }
    table { width: 100%; border-collapse: collapse; background: #fff; border: 1px solid #d9e1ea; }
    th, td { padding: 10px 12px; border-bottom: 1px solid #e5ebf1; text-align: left; font-size: 14px; vertical-align: top; }
    th { background: #edf2f7; font-weight: 650; }
    input, select, textarea, button { font: inherit; }
    input, select, textarea { border: 1px solid #c8d3df; border-radius: 6px; padding: 8px 9px; background: #fff; }
    textarea { min-height: 68px; min-width: 280px; }
    button { border: 1px solid #17324d; background: #17324d; color: #fff; border-radius: 6px; padding: 8px 10px; cursor: pointer; }
    button.secondary { color: #17324d; background: #fff; }
    form.inline { display: inline-flex; gap: 6px; align-items: center; margin: 2px 0; }
    .toolbar { display: flex; gap: 10px; align-items: center; flex-wrap: wrap; margin-bottom: 12px; }
    .grid { display: grid; grid-template-columns: repeat(3, minmax(180px, 1fr)); gap: 10px; background: #fff; border: 1px solid #d9e1ea; padding: 14px; }
    .wide { grid-column: 1 / -1; }
    .muted { color: #5b6b7d; }
    .status { font-weight: 650; }
    .warning { color: #8a4b00; }
    details { margin-bottom: 16px; }
  </style>
</head>
<body>
  <header><h1>Cloud Monitor Admin</h1></header>
  <main>
    <section>
      <div class="toolbar">
        <form method="get" action="/admin" class="inline">
          <label>Region <input name="region" value="{{.Region}}"></label>
          <button type="submit">조회</button>
        </form>
        <form hx-post="/admin/discovery/run" hx-target="#discovery-result" hx-swap="innerHTML" method="post" class="inline">
          <button type="submit" class="secondary">Discovery 실행</button>
        </form>
        <span id="discovery-result" class="muted"></span>
      </div>
    </section>
    <section>
      <h2>서비스</h2>
      <div class="toolbar">
        <span>Enabled metrics {{.Cost.EnabledMetricCount}}</span>
        <span>Regions {{.Cost.RegionCount}}</span>
        <span>Interval {{.Cost.CollectorIntervalSeconds}}s</span>
        <span>Monthly requests {{.Cost.MonthlyMetricRequests}}</span>
        <span>Estimated GetMetricData ${{printf "%.4f" .Cost.EstimatedMonthlyCostUSD}}</span>
        {{if .Cost.CostWarningMetricCount}}<span class="warning">Cost warning metrics {{.Cost.CostWarningMetricCount}}</span>{{end}}
      </div>
      <p class="muted">{{.Cost.PricingNote}}</p>
      <table>
        <thead><tr><th>서비스</th><th>리소스</th><th>Metric 후보</th><th>선택</th><th>Bulk preview</th></tr></thead>
        <tbody>{{range .Services}}
          <tr>
            <td>{{.ServiceName}}<br><span class="muted">{{.Namespace}}</span></td>
            <td>{{.ResourceCount}}</td>
            <td>available {{.AvailableMetrics}} / setup {{.RequiresSetup}} / unsupported {{.UnsupportedMetrics}}</td>
            <td>{{.SelectedMetrics}}</td>
            <td>수집 시작 {{.UnselectedAvailableMetrics}} / enable {{.DisabledMetricDefinitions}} / disable {{.EnabledMetricDefinitions}}</td>
          </tr>
        {{else}}<tr><td colspan="5" class="muted">Discovery를 실행하면 서비스가 표시됩니다.</td></tr>{{end}}</tbody>
      </table>
    </section>
    <section>
      <h2>리소스</h2>
      <div class="toolbar">
        <form method="post" action="/admin/resources/apply-metric-set?region={{.Region}}" class="inline" onsubmit="return confirm('선택한 추천 metric set을 해당 서비스의 모든 리소스에 적용할까요? 서비스 표의 리소스 수와 Bulk preview를 확인하세요.');">
          <select name="metric_set">{{range .MetricSets}}<option value="{{.Name}}">{{.Name}} - {{len .Metrics}} metrics</option>{{end}}</select>
          <button type="submit">추천 세트 일괄 적용</button>
        </form>
      </div>
      <table>
        <thead><tr><th>상태</th><th>서비스</th><th>표시 이름</th><th>Region</th><th>Metric</th><th>Public metadata</th><th>작업</th></tr></thead>
        <tbody>{{range .Resources}}
          <tr>
            <td class="status">{{if .Enabled}}enabled{{else}}disabled{{end}}</td>
            <td>{{.ServiceName}}<br><span class="muted">{{.Namespace}}</span><br><span class="muted">{{.ProviderSource}}</span></td>
            <td>{{.DisplayName}}<br><span class="muted">{{.ResourceID}}</span></td>
            <td>{{.Region}}</td>
            <td>후보 {{.DiscoveredMetrics}} / available {{.AvailableMetrics}} / 선택 {{.SelectedMetrics}} / 수집 {{.MetricDefinitions}}</td>
            <td>
              <form method="post" action="/admin/resources/{{.ID}}/public?region={{$.Region}}" class="inline">
                <label><input type="checkbox" name="public_enabled" {{if .PublicEnabled}}checked{{end}}> public</label>
                <input name="public_display_name" value="{{.PublicDisplayName}}" placeholder="display">
                <input name="public_label" value="{{.PublicLabel}}" placeholder="label">
                <input name="public_sort_order" value="{{.PublicSortOrder}}" placeholder="sort">
                <button class="secondary">저장</button>
              </form>
            </td>
            <td>
              {{if .Enabled}}
              <form method="post" action="/admin/resources/{{.ID}}/disable?region={{$.Region}}" class="inline"><button class="secondary">Disable</button></form>
              {{else}}
              <form method="post" action="/admin/resources/{{.ID}}/enable?region={{$.Region}}" class="inline"><button>Enable</button></form>
              {{end}}
              <form method="post" action="/admin/resources/{{.ID}}/apply-metric-set?region={{$.Region}}" class="inline">
                <select name="metric_set">{{range $.MetricSets}}<option value="{{.Name}}">{{.Name}}</option>{{end}}</select>
                <button class="secondary">추천 적용</button>
              </form>
            </td>
          </tr>
        {{else}}<tr><td colspan="7" class="muted">발견된 리소스가 없습니다.</td></tr>{{end}}</tbody>
      </table>
    </section>
    <section>
      <h2>Metric 후보</h2>
      <div class="toolbar">
        <form method="post" action="/admin/metric-candidates/select-available?region={{.Region}}" class="inline" onsubmit="return confirm('선택한 범위의 available metric 후보를 일괄 수집 시작할까요? 서비스 표의 Bulk preview에서 예상 개수를 확인하세요.');">
          <select name="service_name">
            <option value="">전체 서비스</option>
            {{range .Services}}<option value="{{.ServiceName}}">{{.ServiceName}} - 수집 시작 {{.UnselectedAvailableMetrics}}</option>{{end}}
          </select>
          <button type="submit">Available 일괄 수집 시작</button>
        </form>
      </div>
      <table>
        <thead><tr><th>상태</th><th>리소스</th><th>Metric</th><th>Provider</th><th>Reason</th><th>작업</th></tr></thead>
        <tbody>{{range .Candidates}}
          <tr>
            <td class="status">{{if .Selected}}selected{{else}}{{.AvailabilityStatus}}{{end}}</td>
            <td>{{.DisplayName}}<br><span class="muted">{{.ServiceName}} / {{.Region}}</span></td>
            <td>{{.MetricName}}<br><span class="muted">{{.Namespace}} / {{.Statistic}} / {{.PeriodSeconds}}s / {{.Unit}}</span></td>
            <td>{{.ProviderSource}}</td>
            <td><span class="{{if ne .AvailabilityStatus "available"}}warning{{else}}muted{{end}}">{{.AvailabilityReason}}</span><br><span class="muted">{{.Prerequisite}} {{.CostWarning}}</span></td>
            <td>
              {{if eq .AvailabilityStatus "available"}}
                {{if .Selected}}<span class="muted">적용됨</span>{{else}}<form method="post" action="/admin/metric-candidates/{{.ID}}/select?region={{$.Region}}" class="inline"><button>수집 시작</button></form>{{end}}
              {{else}}
                <span class="muted">설정 필요</span>
              {{end}}
            </td>
          </tr>
        {{else}}<tr><td colspan="6" class="muted">Metric 후보가 없습니다.</td></tr>{{end}}</tbody>
      </table>
    </section>
    <section>
      <h2>Metric Definition</h2>
      <div class="toolbar">
        <form method="post" action="/admin/metric-definitions/bulk-enabled?region={{.Region}}" class="inline" onsubmit="return confirm('선택한 범위의 metric definition 상태를 일괄 변경할까요? 서비스 표의 Bulk preview에서 예상 개수를 확인하세요.');">
          <select name="service_name">
            <option value="">전체 서비스</option>
            {{range .Services}}<option value="{{.ServiceName}}">{{.ServiceName}} - enable {{.DisabledMetricDefinitions}} / disable {{.EnabledMetricDefinitions}}</option>{{end}}
          </select>
          <button type="submit" name="enabled" value="true">일괄 Enable</button>
          <button type="submit" name="enabled" value="false" class="secondary">일괄 Disable</button>
        </form>
      </div>
      <table>
        <thead><tr><th>상태</th><th>서비스</th><th>Metric</th><th>Resource</th><th>Diagnostics</th><th>Dimensions</th><th>Public metadata</th><th>작업</th></tr></thead>
        <tbody>{{range .Definitions}}
          <tr>
            <td class="status">{{if .Enabled}}enabled{{else}}disabled{{end}}</td>
            <td>{{.ServiceName}}<br><span class="muted">{{.Namespace}}</span></td>
            <td>{{.MetricName}}<br><span class="muted">{{.Statistic}} / {{.PeriodSeconds}}s / {{.Unit}}</span></td>
            <td>{{.ResourceID}}<br><span class="muted">{{.Region}}</span></td>
            <td>
              <span class="status">{{.LastRunStatus}}</span><br>
              <span class="muted">fetched {{.FetchedPointCount}} / inserted {{.InsertedPointCount}} / recent {{.RecentPointCount}}</span><br>
              <span class="muted">latest {{.LatestPointAt}}</span><br>
              {{if .SanitizedError}}<span class="warning">{{.SanitizedError}}</span>{{end}}
            </td>
            <td><code>{{.DimensionsJSON}}</code></td>
            <td>
              <form method="post" action="/admin/metric-definitions/{{.ID}}/public?region={{$.Region}}" class="inline">
                <label><input type="checkbox" name="public_enabled" {{if .PublicEnabled}}checked{{end}}> public</label>
                <input name="public_label" value="{{.PublicLabel}}" placeholder="label">
                <button class="secondary">저장</button>
              </form>
            </td>
            <td>
              {{if .Enabled}}
              <form method="post" action="/admin/metric-definitions/{{.ID}}/disable?region={{$.Region}}" class="inline"><button class="secondary">Disable</button></form>
              {{else}}
              <form method="post" action="/admin/metric-definitions/{{.ID}}/enable?region={{$.Region}}" class="inline"><button>Enable</button></form>
              {{end}}
              <form method="post" action="/admin/metric-definitions/{{.ID}}/delete?region={{$.Region}}" class="inline"><button class="secondary">삭제</button></form>
            </td>
          </tr>
        {{else}}<tr><td colspan="8" class="muted">등록된 metric definition이 없습니다.</td></tr>{{end}}</tbody>
      </table>
      <details>
        <summary>Advanced manual metric definition</summary>
        <form method="post" action="/admin/metric-definitions?region={{.Region}}" class="grid">
          <input name="service_name" placeholder="service_name">
          <input name="namespace" placeholder="namespace">
          <input name="metric_name" placeholder="metric_name">
          <input name="resource_id" placeholder="resource_id">
          <input name="region" value="{{.Region}}" placeholder="region">
          <input name="statistic" placeholder="statistic">
          <input name="period_seconds" value="300" placeholder="period_seconds">
          <input name="unit" placeholder="unit">
          <label><input type="checkbox" name="enabled" checked> enabled</label>
          <textarea class="wide" name="dimensions" placeholder='[{"name":"InstanceId","value":"..."}]'>[]</textarea>
          <button type="submit">추가</button>
        </form>
      </details>
    </section>
  </main>
</body>
</html>{{end}}`

const publicPageTemplate = `{{define "public"}}<!doctype html>
<html lang="ko">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Cloud Monitor Portfolio</title>
  <style>
    :root { color-scheme: light; --ink: #16202a; --muted: #526170; --line: #d7e0e8; --panel: #ffffff; --wash: #f3f7fa; --accent: #25636f; --accent-2: #6f4f1f; }
    * { box-sizing: border-box; }
    body { margin: 0; font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; color: var(--ink); background: var(--wash); }
    header { background: #ffffff; border-bottom: 1px solid var(--line); }
    .wrap { width: min(1120px, calc(100% - 32px)); margin: 0 auto; }
    .top { min-height: 150px; display: flex; align-items: flex-end; justify-content: space-between; gap: 24px; padding: 36px 0 28px; }
    h1 { margin: 0; font-size: 34px; line-height: 1.05; letter-spacing: 0; }
    .summary { color: var(--muted); margin-top: 10px; max-width: 640px; line-height: 1.55; }
    .pill { display: inline-flex; align-items: center; gap: 8px; border: 1px solid var(--line); background: #fff; padding: 8px 10px; font-size: 13px; color: var(--muted); }
    main { padding: 26px 0 42px; }
    .toolbar { display: flex; justify-content: space-between; align-items: center; gap: 12px; margin-bottom: 16px; }
    .metric-count { font-size: 14px; color: var(--muted); }
    button { appearance: none; border: 1px solid var(--accent); background: var(--accent); color: white; border-radius: 6px; padding: 8px 11px; font: inherit; cursor: pointer; }
    button.secondary { background: #fff; color: var(--accent); }
    button:disabled { cursor: default; opacity: .6; }
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(260px, 1fr)); gap: 14px; align-items: stretch; }
    article { background: var(--panel); border: 1px solid var(--line); border-radius: 8px; padding: 16px; min-height: 190px; display: flex; flex-direction: column; gap: 14px; }
    .resource { display: flex; justify-content: space-between; gap: 14px; align-items: flex-start; }
    h2 { margin: 0; font-size: 18px; line-height: 1.25; letter-spacing: 0; }
    .alias { color: var(--accent-2); font-size: 13px; white-space: nowrap; }
    .metric { margin: 0; color: var(--muted); line-height: 1.45; }
    .meta { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; margin-top: auto; }
    .meta div { border-top: 1px solid var(--line); padding-top: 8px; min-width: 0; }
    .label { display: block; font-size: 12px; color: var(--muted); }
    .value { display: block; margin-top: 2px; font-weight: 650; overflow-wrap: anywhere; }
    .series { width: 100%; min-height: 56px; border: 1px solid var(--line); background: #f8fafc; padding: 8px; font-size: 12px; color: var(--muted); overflow: auto; }
    .empty, .error { border: 1px solid var(--line); background: #fff; padding: 18px; color: var(--muted); }
    .error { border-color: #c56b6b; color: #8a2f2f; }
    @media (max-width: 640px) {
      .top, .toolbar { align-items: flex-start; flex-direction: column; }
      h1 { font-size: 28px; }
      .wrap { width: min(100% - 24px, 1120px); }
    }
  </style>
</head>
<body>
  <header>
    <div class="wrap top">
      <div>
        <h1>Cloud Monitor Portfolio</h1>
        <p class="summary">Public aliases expose selected service health signals without operational identifiers.</p>
      </div>
      <span class="pill" id="updated-at">Loading</span>
    </div>
  </header>
  <main class="wrap">
    <div class="toolbar">
      <span class="metric-count" id="metric-count">0 metrics</span>
      <button type="button" class="secondary" id="refresh">Refresh</button>
    </div>
    <section class="grid" id="metrics" aria-live="polite"></section>
  </main>
  <script>
    const forbiddenKeys = ["resource" + "Id", "account" + "Id", "a" + "rn", "ta" + "gs", "sanitized" + "Error", "name" + "space", "re" + "gion"];
    const metricsEl = document.querySelector("#metrics");
    const countEl = document.querySelector("#metric-count");
    const updatedEl = document.querySelector("#updated-at");
    const refreshEl = document.querySelector("#refresh");

    function text(value) {
      return value === undefined || value === null || value === "" ? "-" : String(value);
    }

    function assertPublicPayload(value) {
      const encoded = JSON.stringify(value);
      for (const key of forbiddenKeys) {
        if (encoded.includes("\"" + key + "\"")) {
          throw new Error("public payload contains an internal field");
        }
      }
    }

    async function loadSeries(metric) {
      const response = await fetch("/api/public/metrics/" + encodeURIComponent(metric.id) + "/series?limit=12", { headers: { "Accept": "application/json" } });
      if (!response.ok) {
        return [];
      }
      const payload = await response.json();
      assertPublicPayload(payload);
      return Array.isArray(payload.points) ? payload.points : [];
    }

    function renderSeries(points) {
      if (!points.length) {
        return "No recent points";
      }
      return points.slice().reverse().map((point) => {
        const time = new Date(point.timestamp).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
        return time + " " + Number(point.value).toLocaleString();
      }).join(" | ");
    }

	    function card(metric) {
	      const el = document.createElement("article");
	      el.innerHTML = ""
	        + "<div class=\"resource\">"
	        + "<div><h2></h2><p class=\"metric\"></p></div>"
	        + "<span class=\"alias\"></span>"
	        + "</div>"
	        + "<div class=\"series\">Loading series</div>"
	        + "<div class=\"meta\">"
	        + "<div><span class=\"label\">Metric</span><span class=\"value metric-label\"></span></div>"
	        + "<div><span class=\"label\">Unit</span><span class=\"value unit\"></span></div>"
	        + "<div><span class=\"label\">Latest</span><span class=\"value latest\"></span></div>"
	        + "<div><span class=\"label\">Recent points</span><span class=\"value points\"></span></div>"
	        + "</div>";
      el.querySelector("h2").textContent = text(metric.resourceLabel);
      el.querySelector(".metric").textContent = text(metric.resourceDescription);
      el.querySelector(".alias").textContent = text(metric.resourceAlias);
      el.querySelector(".metric-label").textContent = text(metric.metricLabel);
      el.querySelector(".unit").textContent = text(metric.unit);
      el.querySelector(".latest").textContent = text(metric.latestPointAt);
      el.querySelector(".points").textContent = text(metric.recentPointCount);
      loadSeries(metric).then((points) => {
        el.querySelector(".series").textContent = renderSeries(points);
      }).catch(() => {
        el.querySelector(".series").textContent = "Series unavailable";
      });
      return el;
    }

    async function loadMetrics() {
      refreshEl.disabled = true;
      metricsEl.innerHTML = "";
      try {
        const response = await fetch("/api/public/metrics", { headers: { "Accept": "application/json" } });
        if (!response.ok) {
          throw new Error("metrics unavailable");
        }
        const metrics = await response.json();
        assertPublicPayload(metrics);
	        countEl.textContent = metrics.length + (metrics.length === 1 ? " metric" : " metrics");
	        updatedEl.textContent = new Date().toLocaleString();
	        if (!metrics.length) {
	          metricsEl.innerHTML = "<div class=\"empty\">No public metrics</div>";
	          return;
	        }
        for (const metric of metrics) {
          metricsEl.appendChild(card(metric));
        }
	      } catch (error) {
	        countEl.textContent = "0 metrics";
	        metricsEl.innerHTML = "<div class=\"error\">Public metrics unavailable</div>";
	      } finally {
        refreshEl.disabled = false;
      }
    }

    refreshEl.addEventListener("click", loadMetrics);
    loadMetrics();
  </script>
</body>
</html>{{end}}`
