package dogfood

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/launchdarkly/foundation/config"
	"github.com/launchdarkly/foundation/logger"
	lddog "gopkg.in/launchdarkly/go-client.v3"
	lddogRedis "gopkg.in/launchdarkly/go-client.v3/redis"
)

type DogfoodConfig struct {
	BaseUri          string
	StreamUri        string
	EventsUri        string
	ApiKey           string
	RedisHost        string
	RedisPort        int
	UseLdd           bool
	SendEvents       *bool
	Offline          bool
	SamplingInterval int32
}

var (
	client      *lddog.LDClient
	configFlags map[string]interface{}
	initialized = false
)

// Initialize sets up the dogfood client
func Initialize(mode config.Mode, cfg DogfoodConfig) error {
	var err error

	if cfg.BaseUri == "" || cfg.ApiKey == "" {
		flagFile, err := mode.DogfoodFlagFileName()

		if err != nil {
			return err
		}

		if mode.IsTest() {
			flagFile, err = config.FindConfigFile(".", flagFile)
			if err != nil {
				return err
			}
		}

		logger.Info.Printf("No remote dogfood server configuration found. Loading dogfood flags from configuration file %s", flagFile)

		file, e := ioutil.ReadFile(flagFile)
		if e != nil {
			return fmt.Errorf("Unable to read dogfood configuration file from %s: %+v", flagFile, e)
		}

		jsonErr := json.Unmarshal(file, &configFlags)
		if jsonErr != nil {
			return fmt.Errorf("Unable to parse dogfood flags from configuration file: %+v", jsonErr)
		}
		initialized = true
		return nil
	}

	config := lddog.DefaultConfig
	config.Logger = log.New(os.Stderr, "[fdogfood]", log.LstdFlags)
	config.BaseUri = cfg.BaseUri
	config.EventsUri = cfg.EventsUri
	config.Timeout = 5 * time.Second
	config.Offline = cfg.Offline
	config.SamplingInterval = cfg.SamplingInterval

	if cfg.SendEvents != nil {
		config.SendEvents = *cfg.SendEvents
	}

	if cfg.StreamUri != "" {
		config.StreamUri = cfg.StreamUri
		config.Stream = true
	} else {
		config.Stream = false
	}

	if cfg.UseLdd {
		config.UseLdd = cfg.UseLdd
		config.Stream = true
	}

	if cfg.RedisHost != "" && cfg.RedisPort != 0 {
		config.FeatureStore = lddogRedis.NewRedisFeatureStore(cfg.RedisHost, cfg.RedisPort, "ld-dogfood", 5*time.Minute, config.Logger)
	}
	client, err = lddog.MakeCustomClient(cfg.ApiKey, config, 10*time.Second)
	if err == nil {
		initialized = true
	}
	return err
}

func IsInitialized() bool {
	return initialized
}

// BoolVariation wraps LDClient's BoolVariation method, but uses local config flags if
// available
func BoolVariation(key string, user lddog.User, defaultVal bool) (bool, error) {

	if client == nil && configFlags != nil {
		if configFlags[key] == nil {
			return defaultVal, fmt.Errorf("Dogfood flag %s does not exist in flag configuration file", key)
		} else if val, ok := configFlags[key].(bool); ok {
			return val, nil
		}
		logger.Error.Printf("Invalid dogfood flag value for flag %s in configuration file: %+v", key, configFlags[key])
		return defaultVal, fmt.Errorf("Dogfood flag %s has a non-boolean value in flag configuration file", key)
	} else if client != nil {
		return client.BoolVariation(key, user, defaultVal)
	}

	return defaultVal, errors.New("Dogfood client is not configured, and no flag configuration file was found. Returning default flag values.")
}

// FloatVariation wraps LDClient's Float64Variation method, but uses local config flags if
// available
func Float64Variation(key string, user lddog.User, defaultVal float64) (float64, error) {
	if client == nil && configFlags != nil {
		if configFlags[key] == nil {
			return defaultVal, fmt.Errorf("Dogfood flag %s does not exist in flag configuration file", key)
		} else if val, ok := configFlags[key].(float64); ok {
			return val, nil
		}
		logger.Error.Printf("Invalid dogfood flag value for flag %s in configuration file: %+v", key, configFlags[key])
		return defaultVal, fmt.Errorf("Dogfood flag %s has a non-float64 value in flag configuration file", key)
	} else if client != nil {
		return client.Float64Variation(key, user, defaultVal)
	}
	return defaultVal, errors.New("Dogfood client is not configured, and no flag configuration file was found. Returning default flag values.")
}

// IntVariation wraps LDClient's IntVariation method, but uses local config flags if
// available
func IntVariation(key string, user lddog.User, defaultVal int) (int, error) {
	if client == nil && configFlags != nil {
		if configFlags[key] == nil {
			return defaultVal, fmt.Errorf("Dogfood flag %s does not exist in flag configuration file", key)
		} else if val, ok := configFlags[key].(float64); ok && float64(int(val)) == val { // json unmarhsalls numbers as float64 so see if this is really an integer
			return int(val), nil
		}
		logger.Error.Printf("Invalid dogfood flag value for flag %s in configuration file: %+v (%T)", key, configFlags[key], configFlags[key])
		return defaultVal, fmt.Errorf("Dogfood flag %s has a non-int (%T) value in flag configuration file", key, configFlags[key])
	} else if client != nil {
		return client.IntVariation(key, user, defaultVal)
	}
	return defaultVal, errors.New("Dogfood client is not configured, and no flag configuration file was found. Returning default flag values.")
}

// StringVariation wraps LDClient's StringVariation method, but uses local config flags if
// available
func StringVariation(key string, user lddog.User, defaultVal string) (string, error) {
	if client == nil && configFlags != nil {
		if configFlags[key] == nil {
			return defaultVal, fmt.Errorf("Dogfood flag %s does not exist in flag configuration file", key)
		} else if val, ok := configFlags[key].(string); ok {
			return val, nil
		}
		logger.Error.Printf("Invalid dogfood flag value for flag %s in configuration file: %+v", key, configFlags[key])
		return defaultVal, fmt.Errorf("Dogfood flag %s has a non-string value in flag configuration file", key)
	} else if client != nil {
		return client.StringVariation(key, user, defaultVal)
	}
	return defaultVal, errors.New("Dogfood client is not configured, and no flag configuration file was found. Returning default flag values.")
}

func JsonVariation(key string, user lddog.User, defaultVal json.RawMessage) (json.RawMessage, error) {
	if client == nil && configFlags != nil {
		if configFlags[key] == nil {
			return defaultVal, nil
		}
		val, err := json.Marshal(configFlags[key])
		if err != nil {
			logger.Error.Printf("Invalid dogfood flag value for flag %s in configuration file: %+v", key, configFlags[key])
			return defaultVal, err
		}
		return val, err
	}

	if client != nil {
		res, err := client.JsonVariation(key, user, defaultVal)
		if err != nil {
			logger.Error.Printf("Encountered error evaluating int flag %s:  %+v", key, err)
			return defaultVal, err
		}
		return res, nil
	}

	return defaultVal, errors.New("Dogfood client is not configured, and no flag configuration file was found. Returning default flag values.")

}

// Identify wraps LDClient's Identify method
func Identify(user lddog.User) error {
	if client != nil {
		return client.Identify(user)
	}
	return nil
}
