package ftime

import (
	"encoding/json"
	"strconv"
	"time"
)

type UnixMillis int64

func ParseUnixMillis(ms string) (UnixMillis, error) {
	um, err := strconv.ParseInt(ms, 10, 64)

	if err != nil {
		return 0, err
	}

	return UnixMillis(um), nil
}

func Now() UnixMillis {
	return ToUnixMillis(time.Now().UTC())
}

func ToUnixMillis(t time.Time) UnixMillis {
	ms := t.UnixNano() / 1000000

	return UnixMillis(ms)
}

func (um UnixMillis) ToString() string {
	return strconv.FormatInt(int64(um), 10)
}

func (um UnixMillis) ToTime() time.Time {
	ns := um * 1000000
	return time.Unix(0, int64(ns)).UTC()
}

func (um UnixMillis) Add(d time.Duration) UnixMillis {
	return um + UnixMillis((d.Nanoseconds() / 1e6))
}

func (um UnixMillis) Equals(um2 UnixMillis) bool {
	return int64(um) == int64(um2)
}

func (z *UnixMillis) UnmarshalJSON(text []byte) error {
	var f float64
	if err := json.Unmarshal(text, &f); err == nil {
		t := UnixMillis(f)
		*z = t
		return nil
	} else {
		return err
	}
}
