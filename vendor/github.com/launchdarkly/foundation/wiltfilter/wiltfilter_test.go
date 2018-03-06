package wiltfilter

import (
	"bytes"
	"crypto/md5"
	"testing"

	l "github.com/launchdarkly/foundation/logger"
	"github.com/launchdarkly/foundation/usertests"
	ld "github.com/launchdarkly/go-client-private"
	"github.com/launchdarkly/go-metrics"
	"github.com/ugorji/go/codec"
	"gopkg.in/mgo.v2/bson"
)

func TestNewUserIsNotSeen(t *testing.T) {
	d := NewWiltFilter("TestNewUserIsNotSeen", metrics.DefaultRegistry)
	seen := d.AlreadySeen(&userEnvDedupable{bson.NewObjectId(), usertests.GenUser()})
	if seen {
		t.Error("WiltFilter said the user was seen, but it should not have")
	}
}

func TestRepeatUserIsSeen(t *testing.T) {
	envId := bson.NewObjectId()
	u := usertests.GenUser()

	d := NewWiltFilter("TestRepeatUserIsSeen", metrics.DefaultRegistry)
	d.AlreadySeen(&userEnvDedupable{envId, u})
	seen := d.AlreadySeen(&userEnvDedupable{envId, u})
	if !seen {
		t.Error("WiltFilter said the user was not seen, but it should have")
	}
}

func TestRepeatUserButDifferentDataIsNotSeen(t *testing.T) {
	envId := bson.NewObjectId()
	u := usertests.GenUser()
	slimUser := ld.User{Key: u.Key}

	d := NewWiltFilter("TestRepeatUserButDifferentDataIsNotSeen", metrics.DefaultRegistry)
	d.AlreadySeen(&userEnvDedupable{envId, slimUser})
	seen := d.AlreadySeen(&userEnvDedupable{envId, u})
	if seen {
		t.Error("WiltFilter said the user was seen, but it should not have")
	}
}

func TestRepeatUserIsNotSeenAfterRefresh(t *testing.T) {
	envId := bson.NewObjectId()
	u := usertests.GenUser()

	d := makeRefreshingWiltFilter(func() WiltFilter {
		return makeThreadsafeWiltFilter(makeWiltFilter(filterSize, metrics.NewCounter()))
	}, DefaultDailyRefreshConfig,
		"TestRepeatUserIsNotSeenAfterRefresh")
	d.AlreadySeen(&userEnvDedupable{envId, u})
	d.refresh()
	seen := d.AlreadySeen(&userEnvDedupable{envId, u})
	if seen {
		t.Error("WiltFilter said the user was seen, but it should not have")
	}
}

func BenchmarkWiltFilter(b *testing.B) {
	d := makeWiltFilter(filterSize, metrics.NewCounter())
	benchmarkWiltFilter(b, d)
}

func BenchmarkThreadsafeWiltFilter(b *testing.B) {
	d := makeThreadsafeWiltFilter(makeWiltFilter(filterSize, metrics.NewCounter()))
	benchmarkWiltFilter(b, d)
}

func BenchmarkRefreshingThreadsafeWiltFilter(b *testing.B) {
	d := makeRefreshingWiltFilter(func() WiltFilter {
		return makeThreadsafeWiltFilter(makeWiltFilter(filterSize, metrics.NewCounter()))
	}, DefaultDailyRefreshConfig,
		"BenchmarkRefreshingThreadsafeWiltFilter")
	benchmarkWiltFilter(b, d)
}

func benchmarkWiltFilter(b *testing.B, d WiltFilter) {
	for i := 0; i < b.N; i++ {
		envId := bson.NewObjectId()
		u := usertests.GenUser()

		d.AlreadySeen(&userEnvDedupable{envId, u})
	}
}

type userEnvDedupable struct {
	envId bson.ObjectId
	u     ld.User
}

func (d *userEnvDedupable) UniqueBytes() *bytes.Buffer {
	codecHandle := new(codec.MsgpackHandle)
	codecHandle.Canonical = true
	userBytes := bytes.NewBuffer(d.envId.Machine())
	// the deterministic hashed bytes are:
	// user key
	// md5(user bytes)
	// this ensures that two distinct users can never have the same 'deterministic hashed bytes'
	// even if they collide on the md5 hash, while keeping the byte size somewhat constrained.
	var bytesToHash bytes.Buffer
	enc := codec.NewEncoder(userBytes, codecHandle)
	if err := enc.Encode(d.u); err != nil {
		l.Error.Printf("Error encoding user bytes for deduping: %s", err.Error())
	}
	hash := md5.Sum(bytesToHash.Bytes())
	if d.u.Key != nil {
		userBytes.WriteString(*d.u.Key)
	}
	userBytes.Write(hash[:])
	return userBytes
}
