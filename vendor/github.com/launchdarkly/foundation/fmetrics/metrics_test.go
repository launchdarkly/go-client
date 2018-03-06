package fmetrics

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/launchdarkly/foundation/concurrent"
	cfg "github.com/launchdarkly/foundation/config"
	"github.com/launchdarkly/go-metrics"
	"github.com/stretchr/testify/assert"
)

func TestHostname(t *testing.T) {
	shortHostname := shortHostname()

	if strings.Contains(shortHostname, ".") {
		t.Errorf("Short hostname should not contain a dot: %s", shortHostname)
	}
}

//This is handy for sanity-checking 2 things:
// 1. End to end metrics reporting/graphing.
// 2. Graphite queries and various related maths.
func TestMetrics(t *testing.T) {
	//Comment out this line to manually test metrics showing up in Graphite
	t.SkipNow()

	config := MetricsConfig{
		GraphiteHost: "graphite.stg.launchdarkly.com",
	}
	mode := cfg.NewMode("_tester_", "test")
	Initialize(config, mode)
	oncePerSecondMeter := metrics.GetOrRegisterMeter("oncePerSecond.Meter", metrics.DefaultRegistry)
	oncePerSecondCounter := metrics.GetOrRegisterCounter("oncePerSecond.Counter", metrics.DefaultRegistry)
	oncePerSecondGauge := metrics.GetOrRegisterGauge("oncePerSecond.Gauge", metrics.DefaultRegistry)
	oncePerSecondOneHundredMillisecondsTimer := metrics.GetOrRegisterTimer("oncePerSecond._100msTimer", metrics.DefaultRegistry)

	tenTimesPerSecondMeter := metrics.GetOrRegisterMeter("tenTimesPerSecond.Meter", metrics.DefaultRegistry)
	tenTimesPerSecondCounter := metrics.GetOrRegisterCounter("tenTimesPerSecond.Counter", metrics.DefaultRegistry)
	tenTimesPerSecondGauge := metrics.GetOrRegisterGauge("tenTimesPerSecond.Gauge", metrics.DefaultRegistry)

	oneHundredMillisecondsTimerTenTimesPerSecond := metrics.GetOrRegisterTimer("tenTimesPerSecond._100msTimer", metrics.DefaultRegistry)

	startMillis := time.Now().UnixNano() / int64(time.Millisecond)

	wg := sync.WaitGroup{}
	wg.Add(1)
	concurrent.GoSafely(func() {
		for range time.Tick(1 * time.Second) {
			oncePerSecondMeter.Mark(1)
			oncePerSecondCounter.Inc(1)
			oncePerSecondGauge.Update((time.Now().UnixNano() / int64(time.Millisecond)) - startMillis)
			oncePerSecondOneHundredMillisecondsTimer.Update(time.Millisecond * 100)
		}
	})
	concurrent.GoSafely(func() {
		for range time.Tick(100 * time.Millisecond) {
			tenTimesPerSecondMeter.Mark(1)
			tenTimesPerSecondCounter.Inc(1)
			tenTimesPerSecondGauge.Update((time.Now().UnixNano() / int64(time.Millisecond)) - startMillis)
			oneHundredMillisecondsTimerTenTimesPerSecond.Update(time.Millisecond * 100)
		}
	})
	wg.Wait()
}

func TestSanitizeUserAgent(t *testing.T) {
	userAgents := []struct {
		in       *string
		expected string
	}{
		{nil, "none.none"},
		{ptr(""), "none.none"},
		{ptr(" "), "none.none"},
		{ptr("bla"), "other.other"},
		{ptr("UserAgent/1.0.0"), "other.other"},
		{ptr("NodeJSClient/2.0.0"), "NodeJSClient.2_0_0"},
		{ptr("JavaClient/4.0.0-SNAPSHOT"), "JavaClient.4_0_0-SNAPSHOT"},
		{ptr("PythonClient/1.0.0b3"), "PythonClient.1_0_0b3"},
		{ptr("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/54.0.2840.71 Safari/537.36"),
			"other.other"},
	}

	for _, u := range userAgents {
		assert.Equal(t, u.expected, SanitizeUserAgent(u.in))
	}
}

func TestSanitizeForGraphite(t *testing.T) {
	testCases := []struct {
		in       string
		expected string
	}{
		{"", ""},
		{" ", ""},
		{"tab\t", "tab"},
		{"1.0.0", "1_0_0"},
		{"A/B", "A_B"},
	}

	for _, u := range testCases {
		assert.Equal(t, u.expected, SanitizeForGraphite(u.in))
	}
}

func ptr(s string) *string {
	return &s
}
