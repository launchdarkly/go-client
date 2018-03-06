package wiltfilter

type noopWiltFilter struct{}

func (noopWiltFilter) AlreadySeen(o Dedupable) bool {
	return false
}

// NewNoopWiltFilter makes a WiltFilter that never wilts.
func NewNoopWiltFilter() WiltFilter {
	return noopWiltFilter{}
}
