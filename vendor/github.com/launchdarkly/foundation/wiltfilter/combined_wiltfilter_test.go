package wiltfilter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type alwaysTrueWiltFilter struct{}

func (alwaysTrueWiltFilter) AlreadySeen(Dedupable) bool {
	return true
}

func TestDoesDedupeWithVariousFilters(t *testing.T) {
	cases := []struct {
		name        string
		wiltFilters []WiltFilter
		d           dedupable
		expected    bool
	}{
		{
			name: "Noop",
			wiltFilters: []WiltFilter{
				NewNoopWiltFilter(),
			},
			d:        newRandomDedupable(),
			expected: false,
		},
		{
			name: "true",
			wiltFilters: []WiltFilter{
				alwaysTrueWiltFilter{},
			},
			d:        newRandomDedupable(),
			expected: true,
		},
		{
			name: "true, noop",
			wiltFilters: []WiltFilter{
				alwaysTrueWiltFilter{},
				NewNoopWiltFilter(),
			},
			d:        newRandomDedupable(),
			expected: true,
		},
		{
			name: "noop, true",
			wiltFilters: []WiltFilter{
				NewNoopWiltFilter(),
				alwaysTrueWiltFilter{},
			},
			d:        newRandomDedupable(),
			expected: true,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			wf := CombineWiltFilters(tt.wiltFilters...)
			assert.Equal(t, tt.expected, wf.AlreadySeen(tt.d))
		})
	}
}
