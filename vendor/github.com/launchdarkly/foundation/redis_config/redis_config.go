package redis_config

import (
	"fmt"
)

const (
	default_max_active           = 2048
	default_max_idle             = 20
	default_idle_timeout_seconds = 300
	default_connect_timeout_ms   = 250
	default_read_timeout_ms      = 250
	default_write_timeout_ms     = 250
	default_pool_size            = 512
	default_max_retries          = 2
	default_max_retry_backoff_ms = 5000
)

type Config struct {
	Name               string
	Host               string
	Port               int
	MaxActive          int // for old redigo only
	MaxIdle            int // for old redigo only
	IdleTimeoutSeconds int
	ConnectTimeoutMs   int
	ReadTimeoutMs      int
	WriteTimeoutMs     int
	PoolSize           int // for new go-redis client
	MaxRetries         int // for new go-redis client
	MaxRetryBackoffMs  int // for new go-redis client
	TestOnBorrowMinMs  int // for redigo
}

// Populates all but Host and Port fields with reasonable defaults if they're zero values.
func (c *Config) FillInMissingFieldsWithDefaults() {
	if c.MaxIdle == 0 {
		c.MaxIdle = default_max_idle
	}
	if c.MaxActive == 0 {
		c.MaxActive = default_max_active
	}
	if c.IdleTimeoutSeconds == 0 {
		c.IdleTimeoutSeconds = default_idle_timeout_seconds
	}
	if c.ConnectTimeoutMs == 0 {
		c.ConnectTimeoutMs = default_connect_timeout_ms
	}
	if c.ReadTimeoutMs == 0 {
		c.ReadTimeoutMs = default_read_timeout_ms
	}
	if c.WriteTimeoutMs == 0 {
		c.WriteTimeoutMs = default_write_timeout_ms
	}
	if c.PoolSize == 0 {
		c.PoolSize = default_pool_size
	}
	if c.MaxRetries == 0 {
		c.MaxRetries = default_max_retries
	}
	if c.MaxRetryBackoffMs == 0 {
		c.MaxRetryBackoffMs = default_max_retry_backoff_ms
	}
}

func (c Config) String() string {
	// type aliasing avoids recursion
	type alias Config
	return fmt.Sprintf("%+v", alias(c))
}
