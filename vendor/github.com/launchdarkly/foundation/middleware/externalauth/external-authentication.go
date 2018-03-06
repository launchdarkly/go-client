package externalauth

import (
	"errors"
	"net/http"
	"regexp"

	acc "github.com/launchdarkly/foundation/accounts"
	"github.com/launchdarkly/foundation/dogfood"
	"github.com/launchdarkly/foundation/ferror"
	"github.com/launchdarkly/foundation/logger"
	mw "github.com/launchdarkly/foundation/middleware"
	ld "gopkg.in/launchdarkly/go-client.v3"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

const (
	OneYearMaxAge      = "max-age=31536000"
	coalesceDogfoodKey = "redis-acct-cache-coalesce"
)

var (
	uuidHeaderPattern = regexp.MustCompile(`^(?:api_key )?((?:[a-z]{3}-)?[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89aAbB][a-f0-9]{3}-[a-f0-9]{12})$`)
	origin            string
	mongoSession      *mgo.Session
)

type SdkContext struct {
	mw.AuthContext
	Database *mgo.Database
}

type WithSdkContext func(http.ResponseWriter, *http.Request, SdkContext) error
type WithMobileSdkContext WithSdkContext

//For bulk event posting, etc where we just proxy the request and don't need a db
type WithAuthContextFromExternal func(http.ResponseWriter, *http.Request, mw.AuthContext) error
type WithAuthContextFromExternalMobile func(http.ResponseWriter, *http.Request, mw.AuthContext) error

func (h WithSdkContext) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	serveHttpWithSdkToken(h, findAccountListingByApiKey, w, req)
}

func (h WithMobileSdkContext) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	serveHttpWithSdkToken(WithSdkContext(h), findAccountListingByMobileKey, w, req)
}

func (h WithAuthContextFromExternal) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	serveHttpWithTokenFromExternal(mw.WithAuthContext(h), findAccountListingByApiKey, w, req)
}

func (h WithAuthContextFromExternalMobile) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	serveHttpWithTokenFromExternal(mw.WithAuthContext(h), findAccountListingByMobileKey, w, req)
}

func SnippetContext(envId bson.ObjectId, reqId string) (*mw.AuthContext, error) {
	sess := mongoSession.Clone()
	sess.SetMode(mgo.Eventual, true)
	db := sess.DB("gonfalon")
	defer db.Session.Close()

	acct, mongoErr := acc.FindAccountByEnvironmentId(db, envId)

	if mongoErr != nil {
		return nil, mongoErr
	}

	env := acct.FindEnvironmentById(envId)

	if env == nil {
		return nil, errors.New("Could not find context")
	}

	u := acct.ToDogfoodUser()
	dogfoodUser := &u

	ctx := mw.AuthContext{
		AccountId:   acct.Id,
		Environment: env.Id,
		Project:     acct.FindProject(*env).Id,
		AuthKind:    mw.SNIPPET_AUTH,
		Origin:      origin,
		RequestId:   reqId,
		DogUser:     dogfoodUser,
	}

	return &ctx, nil
}

func Initialize(appName string, accountsMongoSession *mgo.Session) {
	origin = appName
	mongoSession = accountsMongoSession
}

type findAccountContextByToken func(*mgo.Database, string, string) (acc.AccountListing, *acc.Environment, *ferror.Error)

func FetchAuthToken(req *http.Request) (string, error) {
	authHdr := req.Header.Get("Authorization")
	match := uuidHeaderPattern.FindStringSubmatch(authHdr)

	// successfully matched UUID from header
	if len(match) == 2 {
		return match[1], nil
	}

	return "", errors.New("No valid token found")
}

func shouldCoalesce(token string) bool {
	t := true
	defaultValue := false
	user := ld.User{
		Key:       &token,
		Anonymous: &t,
	}
	ret, err := dogfood.BoolVariation(coalesceDogfoodKey, user, defaultValue)
	if err != nil {
		logger.Error.Printf("Error getting dogfood flag %s: %s", coalesceDogfoodKey, err.Error())
	}
	return ret
}

var findAccountListingByApiKey = func(db *mgo.Database, reqId, token string) (acc.AccountListing, *acc.Environment, *ferror.Error) {
	oldMode := db.Session.Mode()
	defer db.Session.SetMode(oldMode, true)
	db.Session.SetMode(mgo.Eventual, true)
	acct, err := FindAccountListingByApiKey(db, token, shouldCoalesce(token))

	if err != nil {
		return acct, nil, acc.FindAccountError(err, reqId)
	}
	env := acct.FindEnvironmentByApiKey(token)

	return acct, env, nil
}

var findAccountListingByMobileKey = func(db *mgo.Database, reqId, token string) (acc.AccountListing, *acc.Environment, *ferror.Error) {
	oldMode := db.Session.Mode()
	defer db.Session.SetMode(oldMode, true)
	db.Session.SetMode(mgo.Eventual, true)
	coalesce := shouldCoalesce(token)
	acct, err := FindAccountListingByMobileKey(db, token, coalesce)

	if err != nil { // Could not find by mobile key
		var ferr *ferror.Error
		if acct, err = FindAccountListingByApiKey(db, token, coalesce); err == nil {
			// an API key was used on a mobile resource. That's a paddlin'
			logger.Info.Printf("Account %s tried to use api key to access mobile resource", acct.Id.Hex())
			ferr = ferror.NewUnauthorizedError(
				"You must use a mobile key to access mobile resources, not an api key. Please consult http://docs.launchdarkly.com for more information or contact support@launchdarkly.com.",
				err,
				reqId)
		} else {
			ferr = acc.FindAccountError(err, reqId)
		}
		return acct, nil, ferr

	}
	env := acct.FindEnvironmentByMobileKey(token)

	return acct, env, nil
}

func serveHttpWithSdkToken(h WithSdkContext, accountFinder findAccountContextByToken, w http.ResponseWriter, req *http.Request) {
	var authKind mw.AuthKindType
	token, tokenErr := FetchAuthToken(req)
	var acct acc.AccountListing
	var env *acc.Environment
	reqId := req.Header.Get(mw.REQ_ID_HDR)
	if tokenErr != nil {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", OneYearMaxAge)
		w.WriteHeader(http.StatusUnauthorized)
		return
	} else {
		sess := mongoSession.Clone()
		db := sess.DB("gonfalon")
		defer db.Session.Close()

		if tokenErr == nil {
			var ferr *ferror.Error
			acct, env, ferr = accountFinder(db, reqId, token)
			if ferr != nil {
				if ferr.StatusCode == http.StatusUnauthorized {
					w.Header().Set("Cache-Control", OneYearMaxAge)
				}
				mw.LogAndRenderJsonServerError(w, req, ferr)
				return
			}

			authKind = mw.TOKEN_AUTH
		}
		u := acct.ToDogfoodUser()
		dogfoodUser := &u
		ctx := SdkContext{
			mw.AuthContext{
				AccountId:   acct.Id,
				Environment: env.Id,
				Project:     env.ProjectId,
				Member:      nil,
				AuthKind:    authKind,
				RequestId:   reqId,
				Origin:      origin,
				DogUser:     dogfoodUser,
			},
			db,
		}
		ferr := ferror.FromError(h(w, req, ctx), reqId)
		if ferr != nil {
			mw.LogAndRenderJsonServerError(w, req, ferr)
		}
	}

}

func serveHttpWithTokenFromExternal(h mw.WithAuthContext, accountFinder findAccountContextByToken, w http.ResponseWriter, req *http.Request) {
	var ctx mw.AuthContext
	var authKind mw.AuthKindType
	token, tokenErr := FetchAuthToken(req)
	var acct acc.AccountListing
	var env *acc.Environment
	reqId := req.Header.Get(mw.REQ_ID_HDR)
	if tokenErr != nil {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", OneYearMaxAge)
		w.WriteHeader(http.StatusUnauthorized)
		return
	} else {
		sess := mongoSession.Clone()
		sess.SetMode(mgo.Eventual, true)
		db := sess.DB("gonfalon")
		defer db.Session.Close()

		if tokenErr == nil {
			var ferr *ferror.Error
			acct, env, ferr = accountFinder(db, reqId, token)
			if ferr != nil {
				if ferr.StatusCode == http.StatusUnauthorized {
					w.Header().Set("Cache-Control", OneYearMaxAge)
				}
				mw.LogAndRenderJsonServerError(w, req, ferr)
				return
			}

			authKind = mw.TOKEN_AUTH
		}
		u := acct.ToDogfoodUser()
		dogfoodUser := &u
		ctx = mw.AuthContext{
			AccountId: acct.Id,
			Member:    nil,
			AuthKind:  authKind,
			RequestId: reqId,
			Origin:    origin,
			DogUser:   dogfoodUser,
		}
		if env != nil {
			ctx.Environment = env.Id
			ctx.Project = env.ProjectId
		}
		ferr := ferror.FromError(h(w, req, ctx), reqId)
		if ferr != nil {
			mw.LogAndRenderJsonServerError(w, req, ferr)
		}
	}
}
