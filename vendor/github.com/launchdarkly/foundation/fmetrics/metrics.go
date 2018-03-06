package fmetrics

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/cloudfoundry/gosigar"
	"github.com/launchdarkly/foundation/config"
	"github.com/launchdarkly/foundation/logger"
	"github.com/launchdarkly/go-metrics"
	"github.com/launchdarkly/go-metrics-graphite"
	"github.com/launchdarkly/go-metrics/exp"
)

const (
	defaultGraphitePort = 2003
)

var (
	circuitBreaker              = metrics.NewPrefixedChildRegistry(metrics.DefaultRegistry, "circuitbreaker.")
	freeSystemMemoryGauge       = metrics.GetOrRegisterGauge("sys.mem.free", circuitBreaker)
	actualFreeSystemMemoryGauge = metrics.GetOrRegisterGauge("sys.mem.actualFree", circuitBreaker)
	totalSystemMemory           uint64
	graphiteSanitizeReplacer    = strings.NewReplacer(
		".", "_",
		"/", "_",
		" ", "_",
		"\t", "_",
	)
	knownUserAgents = map[string]bool{
		"AndroidClient": true,
		"DotNetClient":  true,
		"GoClient":      true,
		"iOS":           true,
		"JavaClient":    true,
		"NodeJSClient":  true,
		"PythonClient":  true,
		"PHPClient":     true,
		"RubyClient":    true,
	}
	graphiteConfig graphite.GraphiteConfig
)

type MetricsConfig struct {
	GraphiteHost string
	GraphitePort int
}

// Exports metrics to /debug/metrics
// Sends metrics to graphite once per minute
// Creates an uptime counter for observing app restarts
func Initialize(config MetricsConfig, mode config.Mode) {
	if config.GraphitePort == 0 {
		config.GraphitePort = defaultGraphitePort
	}
	if config.GraphiteHost == "" {
		logger.Warn.Printf("graphiteHost is blank. Skipping publishing to Graphite")
		return
	}

	goMetrics := metrics.NewPrefixedChildRegistry(metrics.DefaultRegistry, "go.")
	metrics.RegisterRuntimeMemStats(goMetrics)
	metrics.RegisterDebugGCStats(goMetrics)

	exp.Exp(metrics.DefaultRegistry)

	//Useful for observing app restarts and correlating problems with uptime.
	uptimeCounter := metrics.GetOrRegisterCounter("uptime.minutes", metrics.DefaultRegistry)

	graphiteAddr := fmt.Sprintf("%s:%d", config.GraphiteHost, config.GraphitePort)
	prefix := metricsPrefix(mode.ApplicationName())
	logger.Info.Printf("Sending Metrics to Graphite at %s once per minute.", graphiteAddr)
	graphiteConfig = graphite.GraphiteConfig{
		Addr:         graphiteAddr,
		Registry:     metrics.DefaultRegistry,
		DurationUnit: time.Millisecond,
		Prefix:       prefix,
		Percentiles:  []float64{0.5, 0.9, 0.95, 0.99, 0.999},
	}

	go func() {
		for range time.Tick(1 * time.Minute) {
			uptimeCounter.Inc(1)
			metrics.CaptureRuntimeMemStatsOnce(goMetrics)
			metrics.CaptureDebugGCStatsOnce(goMetrics)
			Flush()
		}
	}()

	go func() {

	}()

	initCircuitBreakerMetrics()
}

func Flush() {
	err := graphite.GraphiteOnce(graphiteConfig)
	if err != nil {
		logger.Warn.Printf("Error sending metrics to graphite: %s", err.Error())
	}
}

func initCircuitBreakerMetrics() {
	concreteSigar := sigar.ConcreteSigar{}
	mem, err := concreteSigar.GetMem()
	if err != nil {
		logger.Error.Printf("Failed to get system memory stats via sigar: %s", err)
	}
	totalSystemMemory = mem.Total

	// We update these metrics more frequently because we use them in our code to build circuit breakers
	// to avoid exhausting system memory which can easily happen in less than the 60 seconds between metrics publishing.
	go func() {
		for range time.Tick(3 * time.Second) {
			concreteSigar := sigar.ConcreteSigar{}
			mem, err := concreteSigar.GetMem()
			if err != nil {
				logger.Error.Printf("Failed to get system memory stats via sigar: %s", err)
				continue
			}
			freeSystemMemoryGauge.Update(int64(mem.Free))
			actualFreeSystemMemoryGauge.Update(int64(mem.ActualFree))
		}
	}()
}

// Assumes all developers use Macs and local apps are not running in Docker..
// Detects Mac OS and uses a special prefix so we can keep a clean separation between local metrics and metrics
// coming from our servers.
// A typical LD app's metrics prefix will look like this:
//  event-recorder.ip-10-10-1-203.ld.
// A typical locally-running app's prefix will look like this:
//  local_event-recorder_Dans-MacBook-Pro.Dans-MacBook-Pro.ld.

func metricsPrefix(appName string) string {
	if runtime.GOOS == "darwin" {
		prefix := fmt.Sprintf("local_%s.%s.ld.", appName, shortHostname())
		logger.Info.Printf("Detected Mac OS, Using Graphite metrics prefix: %s", prefix)
		return prefix
	}
	prefix := fmt.Sprintf("%s.%s.ld.", appName, shortHostname())
	logger.Info.Printf("Using Graphite metrics prefix: %s", prefix)
	return prefix
}

//Gets this host's name before the first .
func shortHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		logger.Warn.Printf("Couldn't get hostname. Error: %s", err)
		return "_unknown_host"
	}
	logger.Info.Printf("Detected hostname: %s", hostname)
	if strings.Contains(hostname, ".") {
		return strings.Split(hostname, ".")[0]
	}
	return hostname
}

func SanitizeUserAgent(userAgent *string) string {
	if userAgent == nil {
		return "none.none"
	}
	name := strings.TrimSpace(*userAgent)
	if name == "" {
		return "none.none"
	}
	// Split by space and only use the first field
	startSlice := strings.SplitN(name, " ", 2)
	if len(startSlice) > 0 {
		name = startSlice[0]
	}
	split := strings.Split(name, "/")
	if len(split) > 0 {
		if !knownUserAgents[split[0]] {
			return "other.other"
		}
	}
	if len(split) > 1 {
		return SanitizeForGraphite(split[0]) + "." + SanitizeForGraphite(split[1])
	}
	return "other.other"
}

// replaces special characters so we don't get unexpected graphite path segments
func SanitizeForGraphite(c string) string {
	return graphiteSanitizeReplacer.Replace(strings.TrimSpace(c))
}

// Returns the % of system memory not used (0-1.0). This may seem lower than expected because it does not include OS
// buffers.
func GetFreeSystemMemoryPercent() float64 {
	return float64(freeSystemMemoryGauge.Value()) / float64(totalSystemMemory)
}

// Returns the % of system memory not used (0-1.0). This includes any memory used by OS buffers that
// can be freed as needed
func GetActualFreeSystemMemoryPercent() float64 {
	return float64(actualFreeSystemMemoryGauge.Value()) / float64(totalSystemMemory)
}
