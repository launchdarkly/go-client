package frollbar

// Mostly copied from https://github.com/stvp/rollbar

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/launchdarkly/foundation/ferror_reporting"
	"github.com/launchdarkly/foundation/logger"
	"github.com/stvp/rollbar"
)

type RollbarConfig struct {
	Token       string
	Environment string
}

const FILTERED = "[FILTERED]"

var FilterFields = regexp.MustCompile("(?i)(password|secret|token)")
var FilterHeaders = regexp.MustCompile("(?i)(Authorization|Cookie)")

// filterParams filters sensitive information like passwords from being sent to
// Rollbar.
func filterParams(values map[string][]string, filter *regexp.Regexp) map[string][]string {
	for key := range values {
		if filter.Match([]byte(key)) {
			values[key] = []string{FILTERED}
		}
	}

	return values
}

func flattenValues(values map[string][]string) map[string]interface{} {
	result := make(map[string]interface{})

	for k, v := range values {
		if len(v) == 1 {
			result[k] = v[0]
		} else {
			result[k] = v
		}
	}

	return result
}

// errorRequest extracts details from a Request in a format that Rollbar
// accepts.
func errorRequest(r *http.Request) map[string]interface{} {
	cleanQuery := filterParams(r.URL.Query(), FilterFields)

	userIp := r.RemoteAddr
	if splitStr := strings.Split(r.RemoteAddr, ":"); len(splitStr) > 0 { // Rollbar expects ips without ports
		userIp = splitStr[0]
	}

	fields := map[string]interface{}{
		"url":     r.URL.String(),
		"method":  r.Method,
		"headers": flattenValues(filterParams(r.Header, FilterHeaders)),

		// GET params
		"query_string": url.Values(cleanQuery).Encode(),
		"GET":          flattenValues(cleanQuery),

		// POST / PUT params
		"POST": flattenValues(filterParams(r.Form, FilterFields)),

		"user_ip": userIp,
	}

	return fields
}

type ErrorRequestContexter struct {
	*http.Request
}

func (r ErrorRequestContexter) ErrorContext() map[string]interface{} {
	fields := map[string]interface{}{"request": errorRequest(r.Request)}
	errCtx := ferror_reporting.RequestErrorContext{r.Request}
	for k, v := range errCtx.ErrorContext() {
		fields[k] = v
	}
	return fields
}

func Initialize(config RollbarConfig) {
	rollbar.Environment = config.Environment
	rollbar.Token = config.Token
	if config.Token != "" {
		logger.Info.Printf("Initializing Rollbar with token ending with %s", config.Token[len(config.Token)-4:len(config.Token)])
	} else {
		logger.Warn.Printf("Initializing Rollbar with empty token")
	}
}

func Notify(calldepth int, level string, ctx ferror_reporting.ErrorContexter, err error) {
	stack := rollbar.BuildStack(calldepth)
	NotifyWithStack(level, ctx, err, stack)
}

func NotifyWithStack(level string, ctx ferror_reporting.ErrorContexter, err error, stack rollbar.Stack) {
	var fields []*rollbar.Field
	for k, v := range ctx.ErrorContext() {
		fields = append(fields, &rollbar.Field{Name: k, Data: v})
	}
	rollbar.ErrorWithStack(level, err, stack, fields...)
}

func AutoNotify(r *http.Request) {
	AutoNotifyWithContext(&r)
}

// Update request to have error context and report context on panics
func AutoNotifyWithContext(r **http.Request) func() {
	newReq := ferror_reporting.WithErrorContext(*r)
	*r = newReq

	return func() {
		if err := recover(); err != nil {
			Reporter{ErrorRequestContexter{newReq}}.notify(err)
			panic(err)
		}
	}
}

type Reporter struct {
	Context ferror_reporting.ErrorContexter
}

func (r Reporter) AutoNotify() {
	if err := recover(); err != nil {
		r.notify(err)
		panic(err)
	}
}

func (r Reporter) notify(err interface{}) {
	if err != nil {
		Notify(3, rollbar.ERR, r.Context, fmt.Errorf("%v", err))
	}
}
