package server

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "claw_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)
	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "claw_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
	activeStreams = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "claw_active_streams",
			Help: "Number of currently active SSE streams",
		},
	)
	tokenUsage = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "claw_token_usage_total",
			Help: "Total token usage by model and type",
		},
		[]string{"model", "type"},
	)
	activeSessions = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "claw_active_sessions",
			Help: "Number of active sessions",
		},
	)
)

func init() {
	prometheus.MustRegister(requestsTotal)
	prometheus.MustRegister(requestDuration)
	prometheus.MustRegister(activeStreams)
	prometheus.MustRegister(tokenUsage)
	prometheus.MustRegister(activeSessions)
}

func metricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		c.Next()

		status := strconv.Itoa(c.Writer.Status())
		requestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		requestDuration.WithLabelValues(c.Request.Method, path).Observe(time.Since(start).Seconds())
	}
}

func metricsHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

func RecordTokenUsage(model string, inputTokens int, outputTokens int) {
	if inputTokens > 0 {
		tokenUsage.WithLabelValues(model, "input").Add(float64(inputTokens))
	}
	if outputTokens > 0 {
		tokenUsage.WithLabelValues(model, "output").Add(float64(outputTokens))
	}
}

func RecordStreamStart() {
	activeStreams.Inc()
}

func RecordStreamEnd() {
	activeStreams.Dec()
}
