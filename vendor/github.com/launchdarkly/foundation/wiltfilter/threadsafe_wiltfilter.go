package wiltfilter

import (
	"sync"
)

type threadsafeWiltFilter struct {
	d WiltFilter
	m sync.Mutex
}

func (tsd *threadsafeWiltFilter) AlreadySeen(o Dedupable) bool {
	tsd.m.Lock()
	defer tsd.m.Unlock()
	return tsd.d.AlreadySeen(o)
}

func makeThreadsafeWiltFilter(d WiltFilter) *threadsafeWiltFilter {
	return &threadsafeWiltFilter{d: d}
}
