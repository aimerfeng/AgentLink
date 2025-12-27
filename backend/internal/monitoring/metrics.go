package monitoring

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metrics
type Metrics struct {
	// HTTP metrics
	HTTPRequestsTotal    *prometheus.CounterVec
	HTTPRequestDuration  *prometheus.HistogramVec
	HTTPRequestsInFlight prometheus.Gauge

	// AI Provider metrics
	AIProviderLatency    *prometheus.HistogramVec
	AIProviderRequests   *prometheus.CounterVec
	AIProviderErrors     *prometheus.CounterVec

	// Quota metrics
	QuotaRemaining *prometheus.GaugeVec
	QuotaUsed      *prometheus.CounterVec

	// Rate limiting metrics
	RateLimitHits *prometheus.CounterVec

	// Cache metrics
	CacheHits   *prometheus.CounterVec
	CacheMisses *prometheus.CounterVec

	// Database metrics
	DBConnectionsActive prometheus.Gauge
	DBConnectionsIdle   prometheus.Gauge
	DBQueryDuration     *prometheus.HistogramVec

	// Business metrics
	AgentsCreated    prometheus.Counter
	AgentsPublished  prometheus.Counter
	APICallsTotal    *prometheus.CounterVec
	PaymentsTotal    *prometheus.CounterVec
	RevenueTotal     *prometheus.CounterVec

	// Circuit breaker metrics
	CircuitBreakerState *prometheus.GaugeVec
}

var metrics *Metrics

// Init initializes all Prometheus metrics
func Init() *Metrics {
	if metrics != nil {
		return metrics
	}

	metrics = &Metrics{
		// HTTP metrics
		HTTPRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		HTTPRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
			},
			[]string{"method", "path"},
		),
		HTTPRequestsInFlight: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "http_requests_in_flight",
				Help: "Number of HTTP requests currently being processed",
			},
		),

		// AI Provider metrics
		AIProviderLatency: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "ai_provider_latency_seconds",
				Help:    "AI provider response latency in seconds",
				Buckets: []float64{.5, 1, 2, 5, 10, 20, 30, 60},
			},
			[]string{"provider", "model"},
		),
		AIProviderRequests: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ai_provider_requests_total",
				Help: "Total number of requests to AI providers",
			},
			[]string{"provider", "model", "status"},
		),
		AIProviderErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ai_provider_errors_total",
				Help: "Total number of errors from AI providers",
			},
			[]string{"provider", "model", "error_type"},
		),

		// Quota metrics
		QuotaRemaining: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "quota_remaining",
				Help: "Remaining API quota per user",
			},
			[]string{"user_id"},
		),
		QuotaUsed: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "quota_used_total",
				Help: "Total quota used",
			},
			[]string{"user_id", "agent_id"},
		),

		// Rate limiting metrics
		RateLimitHits: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rate_limit_hits_total",
				Help: "Total number of rate limit hits",
			},
			[]string{"user_type"},
		),

		// Cache metrics
		CacheHits: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cache_hits_total",
				Help: "Total number of cache hits",
			},
			[]string{"cache_type"},
		),
		CacheMisses: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cache_misses_total",
				Help: "Total number of cache misses",
			},
			[]string{"cache_type"},
		),

		// Database metrics
		DBConnectionsActive: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "db_connections_active",
				Help: "Number of active database connections",
			},
		),
		DBConnectionsIdle: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "db_connections_idle",
				Help: "Number of idle database connections",
			},
		),
		DBQueryDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "db_query_duration_seconds",
				Help:    "Database query duration in seconds",
				Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
			},
			[]string{"query_type"},
		),

		// Business metrics
		AgentsCreated: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "agents_created_total",
				Help: "Total number of agents created",
			},
		),
		AgentsPublished: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "agents_published_total",
				Help: "Total number of agents published",
			},
		),
		APICallsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "api_calls_total",
				Help: "Total number of API calls",
			},
			[]string{"agent_id", "status"},
		),
		PaymentsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "payments_total",
				Help: "Total number of payments",
			},
			[]string{"method", "status"},
		),
		RevenueTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "revenue_total_usd",
				Help: "Total revenue in USD",
			},
			[]string{"type"},
		),

		// Circuit breaker metrics
		CircuitBreakerState: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "circuit_breaker_state",
				Help: "Circuit breaker state (0=closed, 1=open, 0.5=half-open)",
			},
			[]string{"provider"},
		),
	}

	return metrics
}

// Get returns the global metrics instance
func Get() *Metrics {
	if metrics == nil {
		return Init()
	}
	return metrics
}

// Handler returns the Prometheus HTTP handler
func Handler() http.Handler {
	return promhttp.Handler()
}

// GinHandler returns a Gin-compatible handler for Prometheus metrics
func GinHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}


// MetricsMiddleware is a Gin middleware for collecting HTTP metrics
func MetricsMiddleware() gin.HandlerFunc {
	m := Get()
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		method := c.Request.Method

		// Track in-flight requests
		m.HTTPRequestsInFlight.Inc()
		defer m.HTTPRequestsInFlight.Dec()

		// Process request
		c.Next()

		// Record metrics
		status := strconv.Itoa(c.Writer.Status())
		duration := time.Since(start).Seconds()

		m.HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
		m.HTTPRequestDuration.WithLabelValues(method, path).Observe(duration)
	}
}

// RecordAIProviderLatency records AI provider latency
func RecordAIProviderLatency(provider, model string, duration time.Duration) {
	Get().AIProviderLatency.WithLabelValues(provider, model).Observe(duration.Seconds())
}

// RecordAIProviderRequest records an AI provider request
func RecordAIProviderRequest(provider, model, status string) {
	Get().AIProviderRequests.WithLabelValues(provider, model, status).Inc()
}

// RecordAIProviderError records an AI provider error
func RecordAIProviderError(provider, model, errorType string) {
	Get().AIProviderErrors.WithLabelValues(provider, model, errorType).Inc()
}

// RecordQuotaUsage records quota usage
func RecordQuotaUsage(userID, agentID string, amount float64) {
	Get().QuotaUsed.WithLabelValues(userID, agentID).Add(amount)
}

// SetQuotaRemaining sets the remaining quota for a user
func SetQuotaRemaining(userID string, remaining float64) {
	Get().QuotaRemaining.WithLabelValues(userID).Set(remaining)
}

// RecordRateLimitHit records a rate limit hit
func RecordRateLimitHit(userType string) {
	Get().RateLimitHits.WithLabelValues(userType).Inc()
}

// RecordCacheHit records a cache hit
func RecordCacheHit(cacheType string) {
	Get().CacheHits.WithLabelValues(cacheType).Inc()
}

// RecordCacheMiss records a cache miss
func RecordCacheMiss(cacheType string) {
	Get().CacheMisses.WithLabelValues(cacheType).Inc()
}

// RecordDBQuery records a database query duration
func RecordDBQuery(queryType string, duration time.Duration) {
	Get().DBQueryDuration.WithLabelValues(queryType).Observe(duration.Seconds())
}

// SetDBConnections sets database connection metrics
func SetDBConnections(active, idle int) {
	Get().DBConnectionsActive.Set(float64(active))
	Get().DBConnectionsIdle.Set(float64(idle))
}

// RecordAgentCreated records an agent creation
func RecordAgentCreated() {
	Get().AgentsCreated.Inc()
}

// RecordAgentPublished records an agent publication
func RecordAgentPublished() {
	Get().AgentsPublished.Inc()
}

// RecordAPICall records an API call
func RecordAPICall(agentID, status string) {
	Get().APICallsTotal.WithLabelValues(agentID, status).Inc()
}

// RecordPayment records a payment
func RecordPayment(method, status string) {
	Get().PaymentsTotal.WithLabelValues(method, status).Inc()
}

// RecordRevenue records revenue
func RecordRevenue(revenueType string, amount float64) {
	Get().RevenueTotal.WithLabelValues(revenueType).Add(amount)
}

// SetCircuitBreakerState sets the circuit breaker state
// state: 0=closed, 1=open, 0.5=half-open
func SetCircuitBreakerState(provider string, state float64) {
	Get().CircuitBreakerState.WithLabelValues(provider).Set(state)
}
