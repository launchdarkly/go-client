package middleware

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

func OptionsHandler(siteMap *map[string][]string, pattern string, domains []string) http.Handler {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Allow", strings.Join((*siteMap)[pattern], ", "))
		w.WriteHeader(http.StatusOK)
	}

	if len(domains) != 0 {
		return CorsDomainsMiddleware(domains, siteMap, pattern, http.HandlerFunc(handler))
	} else {
		return CorsHeadersMiddleware(siteMap, pattern, http.HandlerFunc(handler))
	}
}

func AddDefaultHandler(siteMap *map[string][]string, pattern string, router *mux.Router) {
	matcher := mux.MatcherFunc(func(req *http.Request, m *mux.RouteMatch) bool {
		methods := (*siteMap)[pattern]
		for _, method := range methods {
			if method == req.Method {
				return false
			}
		}
		return true
	})
	route := router.Path(pattern).MatcherFunc(matcher)
	route.Handler(CorsHeadersMiddleware(siteMap, pattern, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Allow", strings.Join((*siteMap)[pattern], ", "))
		w.WriteHeader(http.StatusMethodNotAllowed)
	})))
}

func CorsHeadersMiddleware(siteMap *map[string][]string, pattern string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := "*"
		if r.Header.Get("Origin") != "" {
			origin = r.Header.Get("Origin")
		}
		setCorsHeaders(w, origin, (*siteMap)[pattern])
		h.ServeHTTP(w, r)
	})
}

func CorsDomainsMiddleware(domains []string, siteMap *map[string][]string, pattern string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, d := range domains {
			if r.Header.Get("Origin") == d {
				setCorsHeaders(w, d, (*siteMap)[pattern])
				break
			}
		}
		h.ServeHTTP(w, r)
	})
}

func setCorsHeaders(w http.ResponseWriter, origin string, methods []string) {
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Max-Age", "300")
	w.Header().Set("Access-Control-Allow-Methods", strings.Join(methods, ", "))
	w.Header().Set("Access-Control-Allow-Headers",
		"Accept, Content-Type, Content-Length, Accept-Encoding, Authorization, X-Requested-With,"+PRIVATE_HDR+","+ACCOUNT_ID_HDR+","+ENV_HDR+","+PRJ_HDR)
}
