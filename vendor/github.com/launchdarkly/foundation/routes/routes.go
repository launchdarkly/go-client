package routes

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pborman/uuid"
)

type Route struct {
	Verb    string
	Pattern string
	name    string
	router  *mux.Router
}

func MakeRoute(verb, pattern string) Route {
	return Route{
		Verb:    verb,
		Pattern: pattern,
		name:    uuid.NewRandom().String(),
		router:  nil,
	}
}

func (r Route) Register(router *mux.Router, handler http.Handler) {
	router.Handle(r.Pattern, handler).Methods(r.Verb).Name(r.name)
	r.router = router
}

func (r Route) Reverse(params ...string) string {
	if r.router == nil {
		panic("Tried to reverse a route that hasn't been registered yet")
	}

	url, err := r.router.Get(r.name).URL(params...)
	if err != nil {
		panic(err)
	}
	return url.String()
}
