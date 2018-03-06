package redis

import (
	"github.com/garyburd/redigo/redis"
	s "github.com/launchdarkly/foundation/statuschecks"
	"github.com/launchdarkly/foundation/statuschecks/async"
)

func CheckRedis(conn redis.Conn) s.ServiceStatus {
	if _, err := conn.Do("PING"); err != nil {
		return s.DownService()
	} else {
		return s.HealthyService()
	}
}

func FutureCheckRedis(conn redis.Conn) async.FutureServiceStatus {
	return async.MakeFutureServiceStatus(func() s.ServiceStatus {
		return CheckRedis(conn)
	})
}
