package wiltfilter

import (
	"strconv"
	"time"

	"github.com/launchdarkly/foundation/redis_config"

	"github.com/garyburd/redigo/redis"
	"github.com/launchdarkly/foundation/logger"
	"github.com/launchdarkly/go-metrics"
)

// RedisWiltFilterConfig is a redis_config.Config with a timeout (in ms) for the
// key to have in redis. The Name will be used as a namespace for keys stored in the WiltFilter.
type RedisWiltFilterConfig struct {
	redis_config.Config
	TimeoutMilliseconds int
}

type redisWiltFilter struct {
	pool                *redis.Pool
	timeoutMilliseconds int
	name                string
	wiltCounter         metrics.Counter
	redisCommandTimer   metrics.Timer
}

func (rwf redisWiltFilter) AlreadySeen(o Dedupable) (alreadySeen bool) {
	var ok interface{}
	var err error
	rwf.redisCommandTimer.Time(func() {
		c := rwf.pool.Get()
		defer c.Close()
		ok, err = c.Do("SET", rwf.name+o.UniqueBytes().String(), "t", "NX", "PX", rwf.timeoutMilliseconds)
	})
	if err != nil {
		logger.Error.Printf("Redis wiltfilter error: %s:", err.Error())
		return false
	}
	switch ok := ok.(type) {
	case string:
		alreadySeen = ok != "OK"
	default:
		alreadySeen = true
	}
	if alreadySeen {
		rwf.wiltCounter.Inc(1)
	}
	return
}

// NewRedisWiltFilter creates a new wilt filter backed by a redis store
// specified by cfg. Take care when configuring this redis store. This will
// create an arbitrary amount of keys. It's best if you configure an eviction
// strategy, like allkeys_lru for the redis that you're using. See also
// https://redis.io/topics/lru-cache#eviction-policies
func NewRedisWiltFilter(cfg RedisWiltFilterConfig, name string, registry metrics.Registry) WiltFilter {
	cfg.FillInMissingFieldsWithDefaults()

	dialCounter := metrics.GetOrRegisterCounter("wiltfilter."+name+".redis.dial", registry)
	redisCommandTimer := metrics.GetOrRegisterTimer("wiltfilter."+name+".redis.command", registry)
	pool := &redis.Pool{
		MaxIdle:     cfg.MaxIdle,
		MaxActive:   cfg.MaxActive,
		Wait:        true,
		IdleTimeout: time.Duration(cfg.IdleTimeoutSeconds) * time.Second,
		Dial: func() (c redis.Conn, err error) {
			c, err = redis.Dial("tcp", cfg.Host+":"+strconv.Itoa(cfg.Port),
				redis.DialConnectTimeout(time.Duration(cfg.ConnectTimeoutMs)*time.Millisecond),
				redis.DialReadTimeout(time.Duration(cfg.ReadTimeoutMs)*time.Millisecond),
				redis.DialWriteTimeout(time.Duration(cfg.WriteTimeoutMs)*time.Millisecond))
			dialCounter.Inc(1)
			return
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			if int64(time.Since(t)*time.Millisecond) >= int64(cfg.TestOnBorrowMinMs) {
				_, err := c.Do("PING")
				return err
			}
			return nil
		},
	}
	metrics.NewRegisteredFunctionalGauge("wiltfilter."+name+".redis.pool.active", registry, func() int64 {
		return int64(pool.ActiveCount())
	})
	return redisWiltFilter{
		pool:                pool,
		name:                name,
		timeoutMilliseconds: cfg.TimeoutMilliseconds,
		redisCommandTimer:   redisCommandTimer,
		wiltCounter:         wiltFilterCounter(name, registry),
	}
}
