package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Collector struct {
	registry           *prometheus.Registry
	httpRequestsTotal  *prometheus.CounterVec
	httpRequestLatency *prometheus.HistogramVec
	rateLimitExceeded  *prometheus.CounterVec
	webhookDeliveries  *prometheus.CounterVec
}

func NewCollector() *Collector {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		collectors.NewGoCollector(),
	)

	collector := &Collector{
		registry: registry,
		httpRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "go_hermes_http_requests_total",
			Help: "Total HTTP requests handled by the API.",
		}, []string{"method", "route", "status"}),
		httpRequestLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "go_hermes_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "route", "status"}),
		rateLimitExceeded: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "go_hermes_rate_limit_exceeded_total",
			Help: "Total requests rejected by the rate limiter.",
		}, []string{"scope"}),
		webhookDeliveries: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "go_hermes_webhook_delivery_events_total",
			Help: "Webhook delivery lifecycle events.",
		}, []string{"event_type", "outcome"}),
	}

	registry.MustRegister(
		collector.httpRequestsTotal,
		collector.httpRequestLatency,
		collector.rateLimitExceeded,
		collector.webhookDeliveries,
	)

	return collector
}

func (c *Collector) ObserveHTTPRequest(method, route string, statusCode int, duration time.Duration) {
	if c == nil {
		return
	}

	status := strconv.Itoa(statusCode)
	c.httpRequestsTotal.WithLabelValues(method, route, status).Inc()
	c.httpRequestLatency.WithLabelValues(method, route, status).Observe(duration.Seconds())
}

func (c *Collector) IncrementRateLimitExceeded(scope string) {
	if c == nil {
		return
	}

	c.rateLimitExceeded.WithLabelValues(scope).Inc()
}

func (c *Collector) ObserveWebhookDelivery(eventType, outcome string) {
	if c == nil {
		return
	}

	c.webhookDeliveries.WithLabelValues(eventType, outcome).Inc()
}

func (c *Collector) HTTPHandler() http.Handler {
	if c == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
	}
	return promhttp.HandlerFor(c.registry, promhttp.HandlerOpts{})
}

func (c *Collector) FiberHandler() fiber.Handler {
	return adaptor.HTTPHandler(c.HTTPHandler())
}
