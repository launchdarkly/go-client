package externalauth

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"gopkg.in/mgo.v2/bson"

	"github.com/garyburd/redigo/redis"
	"github.com/golang/groupcache/singleflight"
	"github.com/karlseguin/ccache"
	acc "github.com/launchdarkly/foundation/accounts"
	"github.com/launchdarkly/foundation/dogfood"
	"github.com/launchdarkly/foundation/logger"
	"github.com/launchdarkly/foundation/redis_config"
	"github.com/launchdarkly/go-metrics"
	lddog "gopkg.in/launchdarkly/go-client.v3"
	"gopkg.in/mgo.v2"
)

const (
	lru_cache_entries   = 2000
	lru_cache_time_secs = 60
	prefix              = "acctcache"
	aOrBFlagKey         = "redis_account_cache.redis_backend"
)

var (
	cacheA                *AccountCache
	cacheB                *AccountCache
	emptyAccountListing   = acc.AccountListing{}
	flagged               bool
	redis_cache_time_secs = strconv.Itoa(int((30 * time.Minute) / time.Second))
)

type AccountCache struct {
	name            string
	pool            *redis.Pool
	lru_cache       *ccache.Cache
	getSingleFlight singleflight.Group

	cacheMetrics                       metrics.Registry
	findAccountListingByApiKeyTimer    metrics.Timer
	findAccountListingByMobileKeyTimer metrics.Timer
	findAccountListingByEnvIdTimer     metrics.Timer
	mongoErrorCounter                  metrics.Counter

	redisDialCounter    metrics.Counter
	redisErrorCounter   metrics.Counter
	redisHitCounter     metrics.Counter
	redisMissCounter    metrics.Counter
	redisLookupCounter  metrics.Counter
	setAcctListingTimer metrics.Timer
	getAcctListingTimer metrics.Timer

	memHitCounter  metrics.Counter
	memMissCounter metrics.Counter
}

// Includes optional A/B usage based on redis_account_cache.redis_backend feature flag key. The flag eval user's key is the
// key used for redis, so it's not going to follow the same rollout percentages as other dogfood flags.
// The flagging bit is optional, just omit either config.
// The dogfood (if you're using multiple configs) and logging packages must be initialized before this function is called.
func InitializeRedisAccountCache(configA *redis_config.Config, configB *redis_config.Config) error {
	var err error

	if configA == nil && configB == nil {
		return errors.New("Invalid config! Both A and B configs were nil!!")
	}

	if configA != nil {
		configA.Name = "A"
		cacheA, err = New(*configA)
		if err != nil {
			return err
		}
	}

	if configB != nil {
		configB.Name = "B"
		cacheB, err = New(*configB)
		if err != nil {
			return err
		}
	}

	if cacheA == nil && cacheB != nil {
		logger.Info.Printf("Config A was not found. All redis requests will use the B config: %s", configB.Host)
		flagged = false
		return nil
	}

	if cacheB == nil && cacheA != nil {
		logger.Info.Printf("Config B was not found. All redis requests will use the A config: %s", configA.Host)
		flagged = false
		return nil
	}

	if !dogfood.IsInitialized() {
		flagged = false
		return errors.New("Both A and B redis configs were found, but Dogfood package was not initialized!")
	}
	flagged = true
	return nil
}

func New(config redis_config.Config) (*AccountCache, error) {
	config.FillInMissingFieldsWithDefaults()
	if config.Host == "" || config.Port == 0 {
		errorMsg := fmt.Sprintf("[%s] Could not initialize Redis Account Cache due to missing host or port in config!",
			config.Name)
		return nil, errors.New(errorMsg)
	}
	c := AccountCache{name: config.Name}
	c.lru_cache = ccache.New(ccache.Configure().MaxSize(lru_cache_entries))

	c.pool = &redis.Pool{
		MaxIdle:     config.MaxIdle,
		MaxActive:   config.MaxActive,
		Wait:        true,
		IdleTimeout: time.Duration(config.IdleTimeoutSeconds) * time.Second,
		Dial: func() (conn redis.Conn, err error) {
			conn, err = redis.Dial("tcp", config.Host+":"+strconv.Itoa(config.Port))
			c.redisDialCounter.Inc(1)
			return
		},
		TestOnBorrow: func(conn redis.Conn, t time.Time) error {
			_, err := conn.Do("PING")
			return err
		},
	}

	c.cacheMetrics = metrics.NewPrefixedChildRegistry(metrics.DefaultRegistry, "redis_account_cache."+config.Name+".")
	c.findAccountListingByApiKeyTimer = metrics.GetOrRegisterTimer("mongo.findAccountListingByApiKey", c.cacheMetrics)
	c.findAccountListingByMobileKeyTimer = metrics.GetOrRegisterTimer("mongo.findAccountListingByMobileKey", c.cacheMetrics)
	c.findAccountListingByEnvIdTimer = metrics.GetOrRegisterTimer("mongo.findAccountListingByEnvId", c.cacheMetrics)
	c.mongoErrorCounter = metrics.GetOrRegisterCounter("mongo.errors", c.cacheMetrics)

	c.redisDialCounter = metrics.GetOrRegisterCounter("redis.dials", c.cacheMetrics)
	c.redisErrorCounter = metrics.GetOrRegisterCounter("redis.errors", c.cacheMetrics)
	c.redisLookupCounter = metrics.GetOrRegisterCounter("redis.getAcctListing.lookups", c.cacheMetrics)
	c.redisHitCounter = metrics.GetOrRegisterCounter("redis.getAcctListing.hits", c.cacheMetrics)
	c.redisMissCounter = metrics.GetOrRegisterCounter("redis.getAcctListing.misses", c.cacheMetrics)
	c.setAcctListingTimer = metrics.GetOrRegisterTimer("redis.setAcctListing", c.cacheMetrics)
	c.getAcctListingTimer = metrics.GetOrRegisterTimer("redis.getAcctListing", c.cacheMetrics)

	c.memHitCounter = metrics.GetOrRegisterCounter("mem.getAcctListing.hits", c.cacheMetrics)
	c.memMissCounter = metrics.GetOrRegisterCounter("mem.getAcctListing.misses", c.cacheMetrics)

	metrics.NewRegisteredFunctionalGauge("redis.poolSize", c.cacheMetrics, func() int64 {
		return int64(c.pool.ActiveCount())
	})
	logger.Info.Printf("[%s] Initializing Redis Account Cache with config: %+v", config.Name, config)

	return &c, nil
}

func cacheForKey(key string) *AccountCache {
	if !flagged {
		if cacheA != nil {
			return cacheA
		}
		if cacheB != nil {
			return cacheB
		}
		logger.Error.Printf("Redis Account Cache is in a bad state! both cacheA and cacheB are nil!")
	}
	anon := true
	dogUser := lddog.User{Key: &key, Anonymous: &anon}
	flagValue, err := dogfood.StringVariation(aOrBFlagKey, dogUser, "A")
	if err != nil {
		logger.Error.Printf("Error evaluating dogfood flag: %s", err.Error())
	}
	switch flagValue {
	case "A":
		return cacheA
	case "B":
		return cacheB
	}
	logger.Error.Printf("Got unexpected value from %s flag eval: [%s]. Using redis cache A as default", aOrBFlagKey, flagValue)
	return cacheA
}

func (c *AccountCache) getConn() redis.Conn {
	return c.pool.Get()
}

func mobilePrefix() string {
	return prefix + ":mobile:"
}

func apiPrefix() string {
	return prefix + ":api:"
}

func envIdPrefix() string {
	return prefix + ":env:"
}

func serialize(acct acc.AccountListing) ([]byte, error) {
	buff := new(bytes.Buffer)
	err := gob.NewEncoder(buff).Encode(acct)

	if err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}

func deserialize(value []byte) (acc.AccountListing, error) {
	var acct acc.AccountListing
	buff := bytes.NewBuffer(value)

	err := gob.NewDecoder(buff).Decode(&acct)

	if err != nil {
		return emptyAccountListing, err
	}

	return acct, nil
}

func isEmptyAccountListing(acct acc.AccountListing) bool {
	return reflect.DeepEqual(acct, emptyAccountListing)
}

// Fetch an account listing from redis cache. This will return redis.ErrNil
// if the key does not exist, mgo.ErrNotFound if the key is negative cached (invalid)
// or the account otherwise
func (c *AccountCache) getAcctListing(preFn func() string, key string, coalesce bool) (*acc.AccountListing, error) {
	fullKey := preFn() + key
	// Check the lru cache first
	item := c.lru_cache.Get(fullKey)

	if item != nil && !item.Expired() {
		acct := item.Value().(*acc.AccountListing)

		if isEmptyAccountListing(*acct) {
			return nil, mgo.ErrNotFound
		}
		c.memHitCounter.Inc(1)
		return acct, nil
	}
	c.memMissCounter.Inc(1)

	getContent := func() (interface{}, error) {
		conn := c.getConn()
		defer conn.Close()
		var err error
		var data []byte
		c.getAcctListingTimer.Time(func() {
			data, err = redis.Bytes(conn.Do("GET", fullKey))
		})
		if err != nil {
			if err == redis.ErrNil {
				c.redisMissCounter.Inc(1)
			} else {
				c.redisErrorCounter.Inc(1)
			}
			return nil, err
		}
		acct, err := deserialize(data)
		if err != nil {
			return nil, err
		}
		return acct, err
	}

	var err error
	var acctUntyped interface{}
	c.redisLookupCounter.Inc(1)
	if coalesce {
		acctUntyped, err = c.getSingleFlight.Do(fullKey, getContent)
	} else {
		acctUntyped, err = getContent()
	}
	if err != nil {
		return nil, err
	}

	if acct, ok := acctUntyped.(acc.AccountListing); ok {
		// Set the lru cache now that we got a fresh account
		c.lru_cache.Set(preFn()+key, &acct, time.Duration(lru_cache_time_secs)*time.Second)

		if isEmptyAccountListing(acct) {
			return nil, mgo.ErrNotFound
		}
		c.redisHitCounter.Inc(1)
		return &acct, nil
	}
	return nil, fmt.Errorf("Error converting acct: %+v", acctUntyped)
}

func (c *AccountCache) setAcctListing(preFn func() string, key string, acct acc.AccountListing) error {
	data, err := serialize(acct)

	if err != nil {
		return err
	}
	conn := c.getConn()
	defer conn.Close()
	c.setAcctListingTimer.Time(func() {
		_, err = conn.Do("SET", preFn()+key, data, "NX", "EX", redis_cache_time_secs)
	})
	if err == nil {
		c.lru_cache.Set(preFn()+key, &acct, time.Duration(lru_cache_time_secs)*time.Second)
	} else {
		c.redisErrorCounter.Inc(1)
	}

	return err
}

func PurgeAcctCacheForApiKey(key string) error {
	return cacheForKey(key).purgeForApiKey(key)
}

func (c *AccountCache) purgeForApiKey(key string) error {
	conn := c.getConn()
	defer conn.Close()

	_, err := conn.Do("DEL", apiPrefix()+key)

	if err != nil && err != redis.ErrNil {
		c.redisErrorCounter.Inc(1)
		return err
	}

	c.lru_cache.Delete(apiPrefix() + key)

	return nil
}

func PurgeAcctCacheForMobileKey(key string) error {
	return cacheForKey(key).purgeForMobileKey(key)
}

func (c *AccountCache) purgeForMobileKey(key string) error {
	conn := c.getConn()
	defer conn.Close()

	_, err := conn.Do("DEL", mobilePrefix()+key)

	if err != nil && err != redis.ErrNil {
		c.redisErrorCounter.Inc(1)
		return err
	}

	c.lru_cache.Delete(apiPrefix() + key)

	return nil
}

func FindAccountListingByEnvironmentId(db *mgo.Database, envId bson.ObjectId, coalesce bool) (acc.AccountListing, error) {
	return cacheForKey(envId.Hex()).findAccountListingByEnvironmentId(db, envId, coalesce)
}

func (c *AccountCache) findAccountListingByEnvironmentId(db *mgo.Database, envId bson.ObjectId, coalesce bool) (acc.AccountListing, error) {
	acct, redisErr := c.getAcctListing(envIdPrefix, envId.Hex(), coalesce)

	if redisErr == mgo.ErrNotFound {
		return emptyAccountListing, mgo.ErrNotFound
	} else if redisErr != nil && redisErr != redis.ErrNil {
		logger.Error.Printf("[%s] Error fetching envId account listing from redis: %+v", c.name, redisErr)
	} else if acct != nil {
		return *acct, nil
	}

	var acctMongo acc.AccountListing
	var err error
	c.findAccountListingByEnvIdTimer.Time(func() {
		acctMongo, err = acc.FindAccountListingByEnvironmentId(db, envId)
	})
	if err != nil {
		// Negative cache the empty account object
		if err == mgo.ErrNotFound {
			acctMongo = emptyAccountListing
		} else {
			c.mongoErrorCounter.Inc(1)
		}
	}

	if (err == nil || err == mgo.ErrNotFound) && (redisErr == nil || redisErr == redis.ErrNil) {
		setErr := c.setAcctListing(envIdPrefix, envId.Hex(), acctMongo)
		if setErr != nil {
			logger.Error.Printf("[%s] Error storing mobile account listing to redis: %+v", c.name, setErr)
		}
	}

	return acctMongo, err
}

func FindAccountListingByMobileKey(db *mgo.Database, key string, coalesce bool) (acc.AccountListing, error) {
	return cacheForKey(key).findAccountListingByMobileKey(db, key, coalesce)
}

func (c *AccountCache) findAccountListingByMobileKey(db *mgo.Database, key string, coalesce bool) (acc.AccountListing, error) {
	acct, redisErr := c.getAcctListing(mobilePrefix, key, coalesce)

	if redisErr == mgo.ErrNotFound {
		return emptyAccountListing, mgo.ErrNotFound
	} else if redisErr != nil && redisErr != redis.ErrNil {
		logger.Error.Printf("[%s] Error fetching mobile account listing from redis: %+v", c.name, redisErr)
	} else if acct != nil {
		return *acct, nil
	}

	var acctMongo acc.AccountListing
	var err error
	c.findAccountListingByMobileKeyTimer.Time(func() {
		acctMongo, err = acc.FindAccountListingByMobileKey(db, key)
	})
	if err != nil {
		// Negative cache the empty account object
		if err == mgo.ErrNotFound {
			acctMongo = emptyAccountListing
		} else {
			c.mongoErrorCounter.Inc(1)
		}
	}

	if (err == nil || err == mgo.ErrNotFound) && (redisErr == nil || redisErr == redis.ErrNil) {
		setErr := c.setAcctListing(mobilePrefix, key, acctMongo)
		if setErr != nil {
			logger.Error.Printf("[%s] Error storing mobile account listing to redis: %+v", c.name, setErr)
		}
	}

	return acctMongo, err
}

func FindAccountListingByApiKey(db *mgo.Database, key string, coalesce bool) (acc.AccountListing, error) {
	return cacheForKey(key).findAccountListingByApiKey(db, key, coalesce)
}

func (c *AccountCache) findAccountListingByApiKey(db *mgo.Database, key string, coalesce bool) (acc.AccountListing, error) {
	acct, redisErr := c.getAcctListing(apiPrefix, key, coalesce)

	if redisErr == mgo.ErrNotFound {
		return emptyAccountListing, mgo.ErrNotFound
	} else if redisErr != nil && redisErr != redis.ErrNil {
		logger.Error.Printf("[%s] Error fetching account listing from redis: %+v", c.name, redisErr)
	} else if acct != nil {
		return *acct, nil
	}

	var acctMongo acc.AccountListing
	var err error
	c.findAccountListingByApiKeyTimer.Time(func() {
		acctMongo, err = acc.FindAccountListingByApiKey(db, key)
	})
	if err != nil {
		// Negative cache the empty account object
		if err == mgo.ErrNotFound {
			acctMongo = emptyAccountListing
		} else {
			c.mongoErrorCounter.Inc(1)
		}
	}

	if (err == nil || err == mgo.ErrNotFound) && (redisErr == nil || redisErr == redis.ErrNil) {
		setErr := c.setAcctListing(apiPrefix, key, acctMongo)
		if setErr != nil {
			logger.Error.Printf("[%s] Error storing account listing to redis: %+v", c.name, setErr)
		}
	}

	return acctMongo, err
}
