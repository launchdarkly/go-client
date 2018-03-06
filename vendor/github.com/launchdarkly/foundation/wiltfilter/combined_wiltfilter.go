package wiltfilter

type combinedWiltFilter struct {
	wfs []WiltFilter
}

func (c combinedWiltFilter) AlreadySeen(d Dedupable) bool {
	for _, wf := range c.wfs {
		if wf.AlreadySeen(d) {
			return true
		}
	}
	return false
}

// CombineWiltFilters creates a wiltfilter which executes each provided
// wiltfilter, in order, until it finds that something has already been seen or
// that it is not present in any WiltFilters.
func CombineWiltFilters(wfs ...WiltFilter) WiltFilter {
	return combinedWiltFilter{wfs}
}
