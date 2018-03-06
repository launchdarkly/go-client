package wiltfilter

import (
	"reflect"
	"strings"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/jasonlvhit/gocron"
	"github.com/stretchr/testify/assert"
)

func TestMakeJobFromConfig(t *testing.T) {
	cases := []struct {
		name     string
		input    RefreshConfig
		expected *gocron.Job
	}{
		{
			name: "Secondly",
			input: RefreshConfig{
				Frequency: 1,
				Interval:  Secondly,
			},
			expected: gocron.Every(1).Seconds(),
		},
		{
			name: "Minutely",
			input: RefreshConfig{
				Frequency: 1,
				Interval:  Minutely,
			},
			expected: gocron.Every(1).Minutes(),
		},
		{
			name: "BiMinutely",
			input: RefreshConfig{
				Frequency: 2,
				Interval:  Minutely,
			},
			expected: gocron.Every(2).Minutes(),
		},
		{
			name: "Hourly",
			input: RefreshConfig{
				Frequency: 1,
				Interval:  Hourly,
			},
			expected: gocron.Every(1).Hours(),
		},
		{
			name: "Daily (non-randomized)",
			input: RefreshConfig{
				Frequency: 1,
				Interval:  Daily,
			},
			expected: gocron.Every(1).Days(),
		},
		{
			name:     "Empty config",
			input:    RefreshConfig{},
			expected: gocron.Every(1).Days(),
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			actual, _ := makeJobFromConfig(tt.input)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestMakeJobFromConfigDailyRandomized(t *testing.T) {
	conf := RefreshConfig{
		Frequency:          1,
		Interval:           Daily,
		RandomizeDailyTime: true,
	}
	actual, logMessage := makeJobFromConfig(conf)
	randomTime := strings.Replace(logMessage, "Created refreshing wilt filter, refreshing every 1 day at ", "", 1)
	expected := gocron.Every(1).Days().At(randomTime)
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Job is not as expected: expected: %s, actual: %s", spew.Sdump(expected), spew.Sdump(actual))
	}
}
