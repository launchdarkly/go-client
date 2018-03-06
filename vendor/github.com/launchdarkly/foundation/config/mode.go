package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// A Mode represents a runtime application and environment context for a LaunchDarkly service.
type Mode struct {
	app string
	env string
}

const (
	production  = "production"
	dogfood     = "dogfood"
	staging     = "staging"
	development = "development"
	managed     = "managed"
)

// NewMode creates a Mode for the given
// application and environment
func NewMode(app, env string) Mode {
	return Mode{
		env: env,
		app: app,
	}
}

// IsProduction returns whether this is an application running in the canonical LaunchDarkly
// Production environment
func (m Mode) IsProduction() bool {
	return m.env == production
}

// IsStaging returns whether this is an application running in the canonical LaunchDarkly
// Staging environment
func (m Mode) IsStaging() bool {
	return m.env == staging
}

// IsDogfood returns whether this is an application running in the canonical LaunchDarkly
// Dogfood environment
func (m Mode) IsDogfood() bool {
	return m.env == dogfood
}

// IsDev returns whether this is an application running in a LaunchDarkly
// Dev environment
func (m Mode) IsDev() bool {
	return m.env == "" || m.env == development
}

// IsTest returns whether this application is running in a special mode for integration tests
func (m Mode) IsTest() bool {
	return m.env == "test"
}

// IsManaged returns whether this application is one of our managed private instances. If
// the environment starts with "managed", then it is a managed private instance
func (m Mode) IsManaged() bool {
	return strings.HasPrefix(m.env, managed)
}

// ConfigFileName returns the name of the configuration file for this Mode
func (m Mode) ConfigFileName() (string, error) {
	if m.IsProduction() {
		return m.app + ".prod.conf", nil
	} else if m.IsStaging() {
		return m.app + ".stg.conf", nil
	} else if m.IsDogfood() {
		return m.app + ".dog.conf", nil
	} else if m.IsTest() {
		return m.app + ".test.conf", nil
	} else if m.IsDev() {
		return m.app + ".local.conf", nil
	} else if m.IsManaged() { // This had better be a managed instance
		return m.app + "." + m.env + ".conf", nil
	}

	return "", errors.New("Unable to determine configuration filename from mode: " + m.env)
}

// DogfoodFlagFileName returns the name of the dogfood flag configuration file for this Mode
func (m Mode) DogfoodFlagFileName() (string, error) {
	if m.IsProduction() {
		return "flags.prod.json", nil
	} else if m.IsStaging() {
		return "flags.stg.json", nil
	} else if m.IsDogfood() {
		return "flags.dog.json", nil
	} else if m.IsTest() {
		return "flags.test.json", nil
	} else if m.IsDev() {
		return "flags.local.json", nil
	} else if m.IsManaged() {
		return "flags." + m.env + ".json", nil
	}

	return "", errors.New("Unable to determine dogfood flag filename from mode: " + m.env)
}

// EnvironmentName returns the name of the environment for this mode
func (m Mode) EnvironmentName() string {
	if m.IsDev() {
		return development
	}
	return m.env
}

// ApplicationName returns the name of the application for this mode
func (m Mode) ApplicationName() string {
	return m.app
}

// String returns a string representation of this mode
func (m Mode) String() string {
	return m.ApplicationName() + "_" + m.EnvironmentName()
}

// When running in test mode, you may need to walk up the directory tree to find the config.
func FindConfigFile(dir, fileName string) (path string, err error) {
	path = filepath.Join(dir, fileName)
	for {
		if _, err = os.Stat(path); err == nil {
			break
		} else {
			if os.IsNotExist(err) {
				if _, err = os.Stat(filepath.Dir(path)); err == nil {
					path = filepath.Join("..", path)
					continue
				}
				err = fmt.Errorf("Could not find test configuration file at or above '%s'", dir)
			}
			return
		}
	}
	return
}
