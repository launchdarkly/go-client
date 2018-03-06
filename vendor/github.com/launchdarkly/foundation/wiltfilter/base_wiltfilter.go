package wiltfilter

import (
	"github.com/jmhodges/opposite_of_a_bloom_filter/go/oppobloom"
	l "github.com/launchdarkly/foundation/logger"
	"github.com/launchdarkly/go-metrics"
)

const (
	filterSize = 1 << 25 // ~256 MB per wiltFilter
)

// non-thread-safe, non-refreshing implementation
type wiltFilter struct {
	filter        *oppobloom.Filter
	wiltedCounter metrics.Counter
}

func makeWiltFilter(size int, wiltedCounter metrics.Counter) *wiltFilter {
	filter, err := oppobloom.NewFilter(size)
	if err != nil {
		// The only way errors can happen here is that the size we pass
		// in is out of bounds, but it is hard-coded, so that should never
		// just 'happen'. So, I'm just logging the error, and using the filter.
		l.Error.Printf("Error creating filter: %s", err.Error())
	}
	return &wiltFilter{
		filter:        filter,
		wiltedCounter: wiltedCounter,
	}
}

// Add the user record to the 'seen' set, and indicate if that user had been
// seen before in this environment.
func (d *wiltFilter) AlreadySeen(o Dedupable) (alreadySeen bool) {
	alreadySeen = d.filter.Contains(o.UniqueBytes().Bytes())
	if alreadySeen {
		d.wiltedCounter.Inc(1)
	}
	return
}
