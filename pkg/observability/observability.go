// Package observability provides the application's metrics and tracing boundary.
package observability

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	jaegerconfig "github.com/uber/jaeger-client-go/config"
)

type Config struct {
	ServiceName, JaegerAgentAddress string
	TraceSampleRate                 float64
}

type Observability struct {
	registry                          *prometheus.Registry
	tracer                            opentracing.Tracer
	closer                            func()
	requests                          *prometheus.CounterVec
	duration                          *prometheus.HistogramVec
	inflight, dbOpen, dbIdle, dbInUse prometheus.Gauge
}

func New(cfg Config) (*Observability, error) {
	if cfg.ServiceName == "" {
		cfg.ServiceName = "shopit-api"
	}
	if cfg.TraceSampleRate < 0 || cfg.TraceSampleRate > 1 {
		return nil, fmt.Errorf("trace sample rate must be between 0 and 1")
	}
	registry := prometheus.NewRegistry()
	o := &Observability{registry: registry, tracer: opentracing.NoopTracer{},
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "shopit_http_requests_total", Help: "Total HTTP requests handled."}, []string{"method", "route", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "shopit_http_request_duration_seconds", Help: "HTTP request duration in seconds.", Buckets: prometheus.DefBuckets}, []string{"method", "route", "status"}),
		inflight: prometheus.NewGauge(prometheus.GaugeOpts{Name: "shopit_http_requests_in_flight", Help: "HTTP requests currently being served."}),
		dbOpen:   prometheus.NewGauge(prometheus.GaugeOpts{Name: "shopit_database_open_connections", Help: "Open database connections."}),
		dbIdle:   prometheus.NewGauge(prometheus.GaugeOpts{Name: "shopit_database_idle_connections", Help: "Idle database connections."}),
		dbInUse:  prometheus.NewGauge(prometheus.GaugeOpts{Name: "shopit_database_in_use_connections", Help: "Database connections currently in use."}),
	}
	registry.MustRegister(o.requests, o.duration, o.inflight, o.dbOpen, o.dbIdle, o.dbInUse, prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	if cfg.JaegerAgentAddress != "" {
		tracer, closer, err := jaegerconfig.Configuration{ServiceName: cfg.ServiceName, Sampler: &jaegerconfig.SamplerConfig{Type: "probabilistic", Param: cfg.TraceSampleRate}, Reporter: &jaegerconfig.ReporterConfig{LocalAgentHostPort: cfg.JaegerAgentAddress}}.NewTracer()
		if err != nil {
			return nil, fmt.Errorf("initializing Jaeger tracer: %w", err)
		}
		o.tracer, o.closer = tracer, func() { _ = closer.Close() }
	}
	return o, nil
}

func (o *Observability) Close() {
	if o != nil && o.closer != nil {
		o.closer()
	}
}
func (o *Observability) MetricsHandler() http.Handler {
	return promhttp.HandlerFor(o.registry, promhttp.HandlerOpts{})
}
func (o *Observability) ObserveDatabase(db *sql.DB) {
	if o == nil || db == nil {
		return
	}
	s := db.Stats()
	o.dbOpen.Set(float64(s.OpenConnections))
	o.dbIdle.Set(float64(s.Idle))
	o.dbInUse.Set(float64(s.InUse))
}

// Middleware accepts upstream trace context and emits route-template RED metrics.
func (o *Observability) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := o.startServerSpan(r)
		defer span.Finish()
		r = r.WithContext(ctx)
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		started := time.Now()
		o.inflight.Inc()
		defer o.inflight.Dec()
		next.ServeHTTP(rw, r)
		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = "unmatched"
		}
		status := strconv.Itoa(rw.status)
		o.requests.WithLabelValues(r.Method, route, status).Inc()
		o.duration.WithLabelValues(r.Method, route, status).Observe(time.Since(started).Seconds())
		span.SetTag("http.route", route)
		span.SetTag("http.status_code", rw.status)
		if rw.status >= 500 {
			span.SetTag("error", true)
		}
	})
}

func (o *Observability) startServerSpan(r *http.Request) (context.Context, opentracing.Span) {
	parent, err := o.tracer.Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(r.Header))
	options := []opentracing.StartSpanOption{opentracing.Tag{Key: "span.kind", Value: "server"}, opentracing.Tag{Key: "http.method", Value: r.Method}}
	if err == nil {
		options = append(options, opentracing.ChildOf(parent))
	}
	span := o.tracer.StartSpan("http.request", options...)
	return opentracing.ContextWithSpan(r.Context(), span), span
}

// StartSpan starts an application span beneath the current request span.
func StartSpan(ctx context.Context, operation string) (context.Context, opentracing.Span) {
	// Do not wrap a context that has no request span. Besides avoiding needless
	// allocations for background jobs, this preserves context identity for
	// callers that use it as an interface value (including repository adapters).
	if opentracing.SpanFromContext(ctx) == nil {
		return ctx, opentracing.NoopTracer{}.StartSpan(operation)
	}
	span, childCtx := opentracing.StartSpanFromContext(ctx, operation)
	return childCtx, span
}

// Handler adds a delivery-layer span to an already traced HTTP request.
func Handler(operation string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := StartSpan(r.Context(), operation)
		defer span.Finish()
		next(w, r.WithContext(ctx))
	}
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *responseWriter) Write(b []byte) (int, error) { return w.ResponseWriter.Write(b) }
