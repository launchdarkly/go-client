package wiltfilter

import (
	"fmt"
	"math/rand"
	"sync"

	"github.com/jasonlvhit/gocron"
	"github.com/launchdarkly/foundation/logger"
	"github.com/launchdarkly/go-metrics"
)

type RefreshInterval string

const (
	Daily    RefreshInterval = "day"
	Hourly   RefreshInterval = "hour"
	Minutely RefreshInterval = "min"
	Secondly RefreshInterval = "sec"
	Manually RefreshInterval = "manual"
)

type RefreshConfig struct {
	Interval           RefreshInterval
	Frequency          uint64
	RandomizeDailyTime bool
}

var DefaultDailyRefreshConfig = RefreshConfig{
	Interval:           Daily,
	Frequency:          1,
	RandomizeDailyTime: true,
}

type RefreshingWiltFilter interface {
	WiltFilter
	Refresh()
}

// This wrapper will recreate the WiltFilter state from a clean slate either at the specified
// interval, or when Refresh() is called.  The time of the refresh is randomized when the
// WiltFilter is created, to avoid all nodes starting with a cold cache at the same time.
type refreshingWiltFilter struct {
	genWiltFilter func() WiltFilter
	impl          WiltFilter
	m             sync.RWMutex
	name          string
}

func (d *refreshingWiltFilter) Refresh() {
	d.m.Lock()
	defer d.m.Unlock()
	logger.Debug.Printf("[%s] Refreshing WiltFilter state", d.name)
	metrics.GetOrRegisterCounter(fmt.Sprintf("wiltfilter.%s.refresh", d.name), metrics.DefaultRegistry).Inc(1)
	d.impl = d.genWiltFilter()
}

func (d *refreshingWiltFilter) AlreadySeen(o Dedupable) bool {
	d.m.RLock()
	defer d.m.RUnlock()
	return d.impl.AlreadySeen(o)
}

func makeJobFromConfig(conf RefreshConfig) (job *gocron.Job, logMessage string) {
	if conf.Interval == Manually {
		return nil, ""
	}
	if conf.Frequency < 1 {
		logger.Warn.Println("Found an invalid value for RefreshConfig Frequency, setting it to 1")
		conf.Frequency = 1
	}
	logMessage = fmt.Sprintf("Created refreshing wilt filter, refreshing every %d ", conf.Frequency)
	job = gocron.Every(conf.Frequency)
	switch conf.Interval {
	case Hourly:
		job = job.Hours()
		logMessage = logMessage + string(Hourly)
	case Minutely:
		job = job.Minutes()
		logMessage = logMessage + string(Minutely)
	case Secondly:
		job = job.Seconds()
		logMessage = logMessage + string(Secondly)
	default: // Daily is the default
		job = job.Day()
		logMessage = logMessage + string(Daily)
		if conf.RandomizeDailyTime {
			refreshHour := rand.Intn(24)
			refreshMinute := rand.Intn(60)
			refeshTime := fmt.Sprintf("%02d:%02d", refreshHour, refreshMinute)
			job = job.At(refeshTime)
			logMessage = fmt.Sprintf("%s at %s", logMessage, refeshTime)
		}
	}
	return job, logMessage
}

func makeRefreshingWiltFilter(gen func() WiltFilter, conf RefreshConfig, name string) *refreshingWiltFilter {
	ret := refreshingWiltFilter{genWiltFilter: gen, name: name}
	ret.Refresh()
	job, logMessage := makeJobFromConfig(conf)
	if job != nil {
		job.Do(ret.Refresh)
		gocron.Start()
	}
	logger.Info.Printf("[%s] %s", name, logMessage)
	return &ret
}
