package metrics

import (
    "time"
    
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
    // API metrics
    HTTPRequestDuration *prometheus.HistogramVec
    HTTPRequestTotal    *prometheus.CounterVec
    HTTPRequestSize     prometheus.Histogram
    HTTPResponseSize    prometheus.Histogram
    
    // Cache metrics
    CacheHits         prometheus.Counter
    CacheMisses       prometheus.Counter
    CacheSetDuration  prometheus.Histogram
    CacheGetDuration  prometheus.Histogram
    
    // Event metrics
    EventsPublished   *prometheus.CounterVec
    EventsConsumed    *prometheus.CounterVec
    EventProcessingDuration *prometheus.HistogramVec
    EventLag          *prometheus.GaugeVec
    
    // Database metrics
    DBQueryDuration *prometheus.HistogramVec
    DBConnections   *prometheus.GaugeVec
    
    // WebSocket metrics
    WSConnections     prometheus.Gauge
    WSMessagesIn      prometheus.Counter
    WSMessagesOut     prometheus.Counter
    WSMessageDropped  prometheus.Counter
}

func New(namespace string) *Metrics {
    return &Metrics{
        HTTPRequestDuration: promauto.NewHistogramVec(
            prometheus.HistogramOpts{
                Namespace: namespace,
                Name:      "http_request_duration_seconds",
                Help:      "HTTP request duration in seconds",
                Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
            },
            []string{"method", "endpoint", "status"},
        ),
        HTTPRequestTotal: promauto.NewCounterVec(
            prometheus.CounterOpts{
                Namespace: namespace,
                Name:      "http_requests_total",
                Help:      "Total HTTP requests",
            },
            []string{"method", "endpoint", "status"},
        ),
        HTTPRequestSize: promauto.NewHistogram(
            prometheus.HistogramOpts{
                Namespace: namespace,
                Name:      "http_request_size_bytes",
                Help:      "HTTP request size in bytes",
                Buckets:   prometheus.ExponentialBuckets(100, 10, 8),
            },
        ),
        HTTPResponseSize: promauto.NewHistogram(
            prometheus.HistogramOpts{
                Namespace: namespace,
                Name:      "http_response_size_bytes",
                Help:      "HTTP response size in bytes",
                Buckets:   prometheus.ExponentialBuckets(100, 10, 8),
            },
        ),
        CacheHits: promauto.NewCounter(
            prometheus.CounterOpts{
                Namespace: namespace,
                Name:      "cache_hits_total",
                Help:      "Total cache hits",
            },
        ),
        CacheMisses: promauto.NewCounter(
            prometheus.CounterOpts{
                Namespace: namespace,
                Name:      "cache_misses_total",
                Help:      "Total cache misses",
            },
        ),
        CacheSetDuration: promauto.NewHistogram(
            prometheus.HistogramOpts{
                Namespace: namespace,
                Name:      "cache_set_duration_seconds",
                Help:      "Cache SET operation duration",
                Buckets:   []float64{.001, .005, .01, .025, .05, .1},
            },
        ),
        CacheGetDuration: promauto.NewHistogram(
            prometheus.HistogramOpts{
                Namespace: namespace,
                Name:      "cache_get_duration_seconds",
                Help:      "Cache GET operation duration",
                Buckets:   []float64{.001, .005, .01, .025, .05, .1},
            },
        ),
        EventsPublished: promauto.NewCounterVec(
            prometheus.CounterOpts{
                Namespace: namespace,
                Name:      "events_published_total",
                Help:      "Total events published",
            },
            []string{"topic", "status"},
        ),
        EventsConsumed: promauto.NewCounterVec(
            prometheus.CounterOpts{
                Namespace: namespace,
                Name:      "events_consumed_total",
                Help:      "Total events consumed",
            },
            []string{"topic", "status"},
        ),
        EventProcessingDuration: promauto.NewHistogramVec(
            prometheus.HistogramOpts{
                Namespace: namespace,
                Name:      "event_processing_duration_seconds",
                Help:      "Event processing duration",
                Buckets:   []float64{.01, .05, .1, .5, 1, 2, 5, 10},
            },
            []string{"topic", "handler"},
        ),
        EventLag: promauto.NewGaugeVec(
            prometheus.GaugeOpts{
                Namespace: namespace,
                Name:      "event_lag",
                Help:      "Current event consumer lag",
            },
            []string{"topic", "partition"},
        ),
        DBQueryDuration: promauto.NewHistogramVec(
            prometheus.HistogramOpts{
                Namespace: namespace,
                Name:      "db_query_duration_seconds",
                Help:      "Database query duration",
                Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
            },
            []string{"operation", "table"},
        ),
        DBConnections: promauto.NewGaugeVec(
            prometheus.GaugeOpts{
                Namespace: namespace,
                Name:      "db_connections",
                Help:      "Current database connections",
            },
            []string{"type"}, // primary, replica
        ),
        WSConnections: promauto.NewGauge(
            prometheus.GaugeOpts{
                Namespace: namespace,
                Name:      "websocket_connections",
                Help:      "Current WebSocket connections",
            },
        ),
        WSMessagesIn: promauto.NewCounter(
            prometheus.CounterOpts{
                Namespace: namespace,
                Name:      "websocket_messages_in_total",
                Help:      "Total WebSocket messages received",
            },
        ),
        WSMessagesOut: promauto.NewCounter(
            prometheus.CounterOpts{
                Namespace: namespace,
                Name:      "websocket_messages_out_total",
                Help:      "Total WebSocket messages sent",
            },
        ),
        WSMessageDropped: promauto.NewCounter(
            prometheus.CounterOpts{
                Namespace: namespace,
                Name:      "websocket_messages_dropped_total",
                Help:      "Total WebSocket messages dropped due to backpressure",
            },
        ),
    }
}

// ObserveHTTP records HTTP request metrics
func (m *Metrics) ObserveHTTP(method, endpoint, status string, duration time.Duration, reqSize, respSize int) {
    m.HTTPRequestDuration.WithLabelValues(method, endpoint, status).Observe(duration.Seconds())
    m.HTTPRequestTotal.WithLabelValues(method, endpoint, status).Inc()
    m.HTTPRequestSize.Observe(float64(reqSize))
    m.HTTPResponseSize.Observe(float64(respSize))
}