package middleware

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/launchdarkly/foundation/config"
	"github.com/launchdarkly/foundation/logger"
	"github.com/launchdarkly/go-metrics"
	"github.com/newrelic/go-agent"
)

const (
	haproxy_useragent   = "HAProxy/HealthCheck"
	histogram_pool_size = 10000
)

var (
	httpMetricsRegistry = metrics.NewPrefixedChildRegistry(metrics.DefaultRegistry, "http.server.")
	app                 newrelic.Application
)

type MetricsRecordingResponseWriter struct {
	http.ResponseWriter
	status    int
	bytesSent int
}

func (w *MetricsRecordingResponseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *MetricsRecordingResponseWriter) Write(data []byte) (int, error) {
	bytesSent, err := w.ResponseWriter.Write(data)
	w.bytesSent = bytesSent
	return bytesSent, err
}

// Creates a metrics prefix for this request based on the mux route name.
func (w *MetricsRecordingResponseWriter) metricsName(req *http.Request) string {
	// In the case of 404s or other unknown route names we don't want to create a new metric for each bogus route,
	// so we use the catchall _unknown_route
	routeName := "unknown_route"
	route := mux.CurrentRoute(req)
	statusCode := w.getStatus()
	if route != nil && route.GetName() != "" {
		routeName = route.GetName()
	} else if statusCode >= 200 && statusCode < 400 {
		//static assets
		if strings.HasPrefix(req.URL.Path, "/s/") {
			split := strings.Split(req.URL.Path, "/")
			routeName = "s_" + split[len(split)-1]
		}
	}
	// We replace the . characters in the name so we can have consistent path lengths which makes for easier
	// Graphite querying because Graphite doesn't support wildcarding multiple path segments.
	routeName = strings.Replace(routeName, ".", "-", -1)
	return fmt.Sprintf("%s.%s.%d.", req.Method, routeName, statusCode)
}

func (w *MetricsRecordingResponseWriter) getStatus() int {
	if w.status == 0 {
		// We never called WriteHeader() which means 200 OK
		return 200
	}
	return w.status
}

func InitNewRelic(mode config.Mode, licenseKey string, newRelicHighSecurity bool) {
	if licenseKey != "" {
		licLen := 5
		if len(licenseKey) < licLen {
			licLen = len(licenseKey)
		}
		logger.Info.Printf("Initializing New Relic with licence key begining with %s***", licenseKey[:licLen])
		hostName, _ := os.Hostname()
		config := newrelic.NewConfig(mode.String(), licenseKey)
		config.HostDisplayName = hostName
		config.HighSecurity = newRelicHighSecurity
		app, _ = newrelic.NewApplication(config)
	} else {
		logger.Info.Printf("NewRelic license key not found. Skipping NewRelic initialization.")
	}
}

func Metrics(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var txn newrelic.Transaction
		//We don't measure HaProxy health checks
		if req.UserAgent() != haproxy_useragent {
			mw := &MetricsRecordingResponseWriter{w, 0, 0}
			start := time.Now()

			if app != nil {
				txn = app.StartTransaction("unknown_route", w, req)
			}

			h.ServeHTTP(mw, req)

			metricsName := mw.metricsName(req)

			if app != nil && txn != nil {
				txn.SetName(metricsName)
				txn.End()
			}

			//The timer gives us a counter for free, so we don't need to add a separate one.
			metrics.GetOrRegisterTimer(metricsName, httpMetricsRegistry).Update(time.Since(start))

			metrics.GetOrRegisterHistogram(metricsName+"responseBytes", httpMetricsRegistry, metrics.NewUniformSample(histogram_pool_size)).
				Update(int64(mw.bytesSent))
			metrics.GetOrRegisterHistogram(metricsName+"requestBytes", httpMetricsRegistry, metrics.NewUniformSample(histogram_pool_size)).
				Update(req.ContentLength)
		}
	})
}
