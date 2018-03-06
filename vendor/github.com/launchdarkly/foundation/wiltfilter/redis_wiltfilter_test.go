package wiltfilter

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/launchdarkly/go-metrics"

	"github.com/launchdarkly/foundation/redis_config"

	"github.com/stretchr/testify/assert"
)

type dedupable struct {
	id []byte
}

const wiltFilterName = "test"

func newRandomDedupable() dedupable {
	b := make([]byte, 10)
	rand.Read(b)
	return dedupable{b}
}

func (d dedupable) waitOnWiltExpire() chan error {
	resultChan := make(chan error)
	go func() {
		time.Sleep(1 * time.Second)
		resultChan <- fmt.Errorf("Key did not expire after 1 second")
	}()
	go func() {
		c, err := redis.Dial("tcp", ":6379")
		if err != nil {
			resultChan <- err
		}
		c.Send("CONFIG", "SET", "notify-keyspace-events", "Kxe")
		c.Send("SUBSCRIBE", "__keyspace@0__:"+wiltFilterName+d.UniqueBytes().String())
		c.Flush()
		for {
			reply, err := c.Receive()
			if err != nil {
				resultChan <- err
			}
			if args, ok := reply.([]interface{}); ok {
				for _, item := range args {
					if arg, ok := item.([]uint8); ok && string(arg) == "expired" {
						resultChan <- nil
					}
				}
			}
		}
	}()
	return resultChan
}

func (d dedupable) UniqueBytes() *bytes.Buffer {
	return bytes.NewBuffer(d.id)
}

func TestDoesDedupe(t *testing.T) {
	d := newRandomDedupable()
	wait := d.waitOnWiltExpire()
	wf := NewRedisWiltFilter(RedisWiltFilterConfig{
		Config: redis_config.Config{
			Host: "localhost",
			Port: 6379,
		},
		TimeoutMilliseconds: 1,
	}, wiltFilterName, metrics.DefaultRegistry)

	assert.False(t, wf.AlreadySeen(d))
	assert.True(t, wf.AlreadySeen(d))
	assert.NoError(t, <-wait)
	assert.False(t, wf.AlreadySeen(d))
}

func TestRedisError(t *testing.T) {
	wf := NewRedisWiltFilter(RedisWiltFilterConfig{
		Config: redis_config.Config{
			Host: "localhost",
			Port: 99999999, // there is no redis here
		},
		TimeoutMilliseconds: 1,
	}, wiltFilterName, metrics.DefaultRegistry)
	d := newRandomDedupable()

	assert.False(t, wf.AlreadySeen(d))
}

func TestNamespaceInterference(t *testing.T) {
	wf1 := NewRedisWiltFilter(RedisWiltFilterConfig{
		Config: redis_config.Config{
			Host: "localhost",
			Port: 6379,
		},
		TimeoutMilliseconds: 1,
	}, "cool", metrics.DefaultRegistry)
	wf2 := NewRedisWiltFilter(RedisWiltFilterConfig{
		Config: redis_config.Config{
			Host: "localhost",
			Port: 6379,
		},
		TimeoutMilliseconds: 1,
	}, "lame", metrics.DefaultRegistry)
	d := newRandomDedupable()

	assert.False(t, wf1.AlreadySeen(d))
	assert.True(t, wf1.AlreadySeen(d))
	assert.False(t, wf2.AlreadySeen(d))
}
