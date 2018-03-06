package wiltfilter

import (
	"bytes"

	"github.com/launchdarkly/go-metrics"
)

type Dedupable interface {
	// the object to be deduped, distilled down to the bytes that uniquely identify a duplicate
	UniqueBytes() *bytes.Buffer
}

type WiltFilter interface {
	AlreadySeen(o Dedupable) bool
}

func wiltFilterCounter(name string, registry metrics.Registry) metrics.Counter {
	return metrics.GetOrRegisterCounter("wiltfilter."+name+".wilted", registry)
}

func NewWiltFilter(name string, registry metrics.Registry) WiltFilter {
	return NewWiltFilterWithRefreshConfig(DefaultDailyRefreshConfig, name, registry)
}

func NewWiltFilterWithRefreshConfig(conf RefreshConfig, name string, registry metrics.Registry) WiltFilter {
	return makeRefreshingWiltFilter(func() WiltFilter {
		return makeThreadsafeWiltFilter(makeWiltFilter(filterSize, wiltFilterCounter(name, registry)))
	}, conf, name)
}
