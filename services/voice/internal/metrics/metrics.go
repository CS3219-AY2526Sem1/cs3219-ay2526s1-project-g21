package metrics

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "peerprep",
		Name:      "http_requests_total",
		Help:      "Total number of HTTP requests received",
	}, []string{"service", "method", "path", "status"})

	httpLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "peerprep",
		Name:      "http_request_duration_seconds",
		Help:      "Duration of HTTP requests in seconds",
		Buckets:   prometheus.DefBuckets,
	}, []string{"service", "method", "path", "status"})

	httpInFlight = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "peerprep",
		Name:      "http_in_flight_requests",
		Help:      "Current number of in-flight HTTP requests",
	}, []string{"service"})

	httpResponseSize = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "peerprep",
		Name:      "http_response_size_bytes",
		Help:      "Size of HTTP responses in bytes",
		Buckets:   prometheus.ExponentialBuckets(200, 2, 8),
	}, []string{"service", "method", "path", "status"})

	httpRequestSize = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "peerprep",
		Name:      "http_request_size_bytes",
		Help:      "Size of HTTP requests in bytes",
		Buckets:   prometheus.ExponentialBuckets(200, 2, 8),
	}, []string{"service", "method", "path", "status"})
)

type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

func (r *responseRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (r *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := r.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, fmt.Errorf("voice metrics: underlying ResponseWriter does not support hijacking")
}

func (r *responseRecorder) Push(target string, opts *http.PushOptions) error {
	if p, ok := r.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}

// Middleware records request metrics with Prometheus labels.
func Middleware(service string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
			start := time.Now()

			httpInFlight.WithLabelValues(service).Inc()
			defer httpInFlight.WithLabelValues(service).Dec()

			next.ServeHTTP(rec, r)

			labels := prometheus.Labels{
				"service": service,
				"method":  r.Method,
				"path":    r.URL.Path,
				"status":  strconv.Itoa(rec.status),
			}

			if r.ContentLength > 0 {
				httpRequestSize.With(labels).Observe(float64(r.ContentLength))
			}

			httpRequests.With(labels).Inc()
			httpLatency.With(labels).Observe(time.Since(start).Seconds())
			httpResponseSize.With(labels).Observe(float64(rec.bytes))
		})
	}
}

// Handler exposes the default Prometheus metrics endpoint.
func Handler() http.Handler {
	return promhttp.Handler()
}
