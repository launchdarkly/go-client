package notify

import (
	"fmt"
	"regexp"

	"github.com/launchdarkly/foundation/ferror_reporting"
	"github.com/mohae/deepcopy"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// Notifier uses the contained Config to route Notifications to Sinks.
type Notifier struct {
	config Config
}

func NewNotifier(userConfig Config) (*Notifier, error) {
	config := deepcopy.Copy(userConfig).(Config)
	err := config.compileRegexps()
	if err != nil {
		return nil, err
	}
	return &Notifier{config}, nil
}

// Notify actually sends the notification to the appropriate sinks.
// When notify gets a notification, it attempts to match it against each route.
// When it finds a matching Route, it passes the notification to that Route's
// sinks and stops.
func (n *Notifier) Notify(level Level, notification Notification) {
	for _, route := range n.config {
		if route.Match(level, notification) {
			for _, sink := range route.Sinks {
				sink.Output(2, level, notification)
			}
			break
		}
	}
}

// LevelNotifier is just a wrapper around notifier that already has a level specified.
type LevelNotifier struct {
	notifier *Notifier
	level    Level
}

// Notify just delegates to the wrapped Notifier's Notify.
func (ln LevelNotifier) Notify(n Notification) {
	ln.notifier.Notify(ln.level, n)
}

func (ln LevelNotifier) Err(ctx ferror_reporting.ErrorContexter, err error) {
	ln.Notify(NewErrorNotification(ctx, err))
}

func (ln LevelNotifier) Fmt(ctx ferror_reporting.ErrorContexter, format string, args ...interface{}) {
	ln.Notify(NewMessageNotification(ctx, fmt.Sprintf(format, args...)))
}

// DefaultNotifier is a singleton Notifier created by Initialize which handles calls
// to the global methods of this package.
var DefaultNotifier *Notifier

func Notify(level Level, n Notification) {
	DefaultNotifier.Notify(level, n)
}

var Debug, Info, Warn, Error LevelNotifier

// Sinks are destinations for notifications. Current examples are logs and rollbar.
type Sink interface {
	Output(calldepth int, level Level, notification Notification)
}

// Routes match errors by level or message regex, and specify sinks for matched errors.
type Route struct {
	Pattern  string
	MinLevel Level
	Sinks    []Sink

	compiledPattern *regexp.Regexp
}

func (r Route) Match(level Level, n Notification) bool {
	if level < r.MinLevel {
		return false
	}

	if r.compiledPattern != nil && !r.compiledPattern.MatchString(n.String()) {
		return false
	}

	// TODO also provide some matching on ctx, or other parts of the notification?

	return true
}

type Config []Route

func init() {
	Initialize(Config{
		{
			Sinks: []Sink{Console{}},
		},
	})
}

// Sets the config specifying what to do with messages.
func Initialize(userConfig Config) error {
	var err error
	DefaultNotifier, err = NewNotifier(userConfig)
	if err != nil {
		return err
	}

	// Set up global level notifiers pointing to DefaultNotifier
	Debug = LevelNotifier{DefaultNotifier, LevelDebug}
	Info = LevelNotifier{DefaultNotifier, LevelInfo}
	Warn = LevelNotifier{DefaultNotifier, LevelWarn}
	Error = LevelNotifier{DefaultNotifier, LevelError}

	return nil
}

// Go through the current config and compile all of the regexp strings into objects.
func (c Config) compileRegexps() error {
	for i := range c {
		if c[i].Pattern != "" {
			compiledPattern, err := regexp.Compile(c[i].Pattern)
			if err != nil {
				return err
			}
			c[i].compiledPattern = compiledPattern
		}
	}
	return nil
}
