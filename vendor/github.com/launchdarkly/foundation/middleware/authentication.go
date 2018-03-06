package middleware

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	lddog "gopkg.in/launchdarkly/go-client.v3"
	"gopkg.in/mgo.v2/bson"

	"github.com/launchdarkly/foundation/accounts/roles"
	"github.com/launchdarkly/foundation/core"
	"github.com/launchdarkly/foundation/ferror"
	"github.com/launchdarkly/foundation/logger"
)

const (
	REQ_ID_HDR     = core.REQ_ID_HDR
	ACCOUNT_ID_HDR = "X-LD-AccountId"
	ENV_HDR        = "X-LD-EnvId"
	PRJ_HDR        = "X-LD-PrjId"
	MBR_HDR        = "X-LD-MbrId"
	USER_HDR       = "X-LD-User"
	ROLE_HDR       = "X-LD-Role"
	ORIGIN_HDR     = "X-LD-Origin"
	AUTH_KIND_HDR  = "X-LD-AuthKind"
	PRIVATE_HDR    = "X-LD-Private"
	DOG_USER_HDR   = "X-LD-DogUser"

	PRIVATE_ALLOWED = "allowed"
)

type AuthKindType int

const (
	NO_AUTH_HEADER AuthKindType = iota
	SESSION_AUTH
	TOKEN_AUTH
	SNIPPET_AUTH
	SCOPED_ACCESS_TOKEN_AUTH
)

var AuthKinds = []AuthKindType{SESSION_AUTH, TOKEN_AUTH, SNIPPET_AUTH, SCOPED_ACCESS_TOKEN_AUTH, NO_AUTH_HEADER}

var authKindNames = map[AuthKindType]string{
	SESSION_AUTH:             "session",
	TOKEN_AUTH:               "token",
	SNIPPET_AUTH:             "snippet",
	SCOPED_ACCESS_TOKEN_AUTH: "scoped",
	NO_AUTH_HEADER:           "",
}

func (a AuthKindType) String() string {
	return authKindNames[a]
}

// Represents an authentication context that may come from the edge
// or a call from another service.
// The Member may be nil (e.g. if the AuthKind is token). However,
// the AccountId and EnvironmentId must be present.
type AuthContext struct {
	AccountId   bson.ObjectId
	Environment bson.ObjectId
	Project     bson.ObjectId
	Member      *MemberInfo
	AuthKind    AuthKindType
	Origin      string
	RequestId   string
	DogUser     *lddog.User
}

type MemberInfo struct {
	Id       bson.ObjectId
	Username string
	Role     roles.RoleType
}

// Represents an authentication context to a private API, which
// must not come from the edge. All
// parameters except the origin are optional
type PrivateAuthContext struct {
	OnBehalfOfAcct *bson.ObjectId
	Environment    *bson.ObjectId
	Project        *bson.ObjectId
	OnBehalfOfMbr  *MemberInfo
	Origin         string
	RequestId      string
}

// Represents an unauthenticated context
type UnauthenticatedContext struct {
	RequestId string
	Origin    string
	DogUser   *lddog.User
}

type Context interface {
	ProxyAuthHeaders(req *http.Request, origin string, private bool)
}

func (ctx UnauthenticatedContext) ProxyAuthHeaders(req *http.Request, origin string, private bool) {
	req.Header.Set(REQ_ID_HDR, ctx.RequestId)
	req.Header.Set(ORIGIN_HDR, origin)
	if private {
		req.Header.Set(PRIVATE_HDR, PRIVATE_ALLOWED)
	}
	AddDogUserHeader(req.Header, ctx.DogUser)
}

func (ctx AuthContext) GoString() string {
	return fmt.Sprintf("[acct: %s proj: %s env: %s]",
		ctx.AccountId.Hex(), ctx.Project.Hex(), ctx.Environment.Hex())
}

// Adds authentication headers to an HTTP request based on an AuthContext
// This is most useful when making a service-to-service call that
// uses these headers as its primary way of identifying authenticated users
// If private is true, adds the request headers necessary to make calls to private APIs
func (ctx AuthContext) ProxyAuthHeaders(req *http.Request, origin string, private bool) {
	req.Header.Set(REQ_ID_HDR, ctx.RequestId)
	req.Header.Set(ACCOUNT_ID_HDR, ctx.AccountId.Hex())
	if ctx.Member != nil {
		req.Header.Set(USER_HDR, ctx.Member.Username)
		req.Header.Set(MBR_HDR, ctx.Member.Id.Hex())
		req.Header.Set(ROLE_HDR, ctx.Member.Role.String())
	}
	AddDogUserHeader(req.Header, ctx.DogUser)
	req.Header.Set(ORIGIN_HDR, origin)
	req.Header.Set(ENV_HDR, ctx.Environment.Hex())
	req.Header.Set(PRJ_HDR, ctx.Project.Hex())
	if ctx.AuthKind != NO_AUTH_HEADER {
		req.Header.Set(AUTH_KIND_HDR, ctx.AuthKind.String())
	}
	if private {
		req.Header.Set(PRIVATE_HDR, PRIVATE_ALLOWED)
	}
}

// Adds authentication headers to an HTTP request based on a PrivateAuthContext
// This is most useful when making a service-to-service call that
// uses these headers as its primary way of identifying authenticated users
func (ctx PrivateAuthContext) ProxyAuthHeaders(req *http.Request, origin string, private bool) {
	req.Header.Set(REQ_ID_HDR, ctx.RequestId)
	if ctx.OnBehalfOfAcct != nil {
		req.Header.Set(ACCOUNT_ID_HDR, ctx.OnBehalfOfAcct.Hex())
	}
	if ctx.OnBehalfOfMbr != nil {
		req.Header.Set(USER_HDR, ctx.OnBehalfOfMbr.Username)
		req.Header.Set(MBR_HDR, ctx.OnBehalfOfMbr.Id.Hex())
		req.Header.Set(ROLE_HDR, ctx.OnBehalfOfMbr.Role.String())
	}

	req.Header.Set(ORIGIN_HDR, origin)
	if ctx.Environment != nil {
		req.Header.Set(ENV_HDR, ctx.Environment.Hex())
	}
	if ctx.Project != nil {
		req.Header.Set(PRJ_HDR, ctx.Project.Hex())
	}
	if private {
		req.Header.Set(PRIVATE_HDR, PRIVATE_ALLOWED)
	}
}

type WithAuthContext func(http.ResponseWriter, *http.Request, AuthContext) error
type WithPrivateAuthContext func(http.ResponseWriter, *http.Request, PrivateAuthContext) error
type WithUnauthenticatedContext func(http.ResponseWriter, *http.Request, UnauthenticatedContext) error

func MakeAuthKind(authKind string) (AuthKindType, error) {
	for _, k := range AuthKinds {
		if k.String() == authKind {
			return k, nil
		}
	}
	return AuthKindType(TOKEN_AUTH), fmt.Errorf("Invalid auth kind: %s", authKind)
}

func getMember(req *http.Request) (*MemberInfo, error) {
	user := req.Header.Get(USER_HDR)
	roleHdr := req.Header.Get(ROLE_HDR)
	mbrId := req.Header.Get(MBR_HDR)

	if user == "" {
		return nil, nil
	}

	if !bson.IsObjectIdHex(mbrId) {
		return nil, errors.New(fmt.Sprintf("Invalid member ID %s", mbrId))
	}

	if role, err := roles.MakeRole(roleHdr); err == nil {
		mbr := &MemberInfo{
			Id:       bson.ObjectIdHex(mbrId),
			Username: user,
			Role:     role,
		}
		return mbr, nil
	} else {
		return nil, err
	}
}

func (h WithUnauthenticatedContext) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	reqId := req.Header.Get(REQ_ID_HDR)
	origin := req.Header.Get(ORIGIN_HDR)

	ctx := UnauthenticatedContext{
		RequestId: reqId,
		Origin:    origin,
	}

	err := ferror.FromError(h(w, req, ctx), reqId)

	if err != nil {
		if err.StatusCode >= http.StatusInternalServerError {
			logger.Error.Println(err.Log())
		}
		w.WriteHeader(err.StatusCode)
		return
	}
}

func (h WithAuthContext) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	accountId := req.Header.Get(ACCOUNT_ID_HDR)
	origin := req.Header.Get(ORIGIN_HDR)
	envId := req.Header.Get(ENV_HDR)
	projId := req.Header.Get(PRJ_HDR)
	user64 := req.Header.Get(DOG_USER_HDR)
	authKindHdr := req.Header.Get(AUTH_KIND_HDR)
	var dogUser *lddog.User

	if accountId == "" || envId == "" || projId == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if !bson.IsObjectIdHex(accountId) || !bson.IsObjectIdHex(envId) || !bson.IsObjectIdHex(projId) {
		logger.Debug.Printf("Invalid account (%s), project (%s), or environment (%s) ID", accountId, projId, envId)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	mbr, mbrErr := getMember(req)

	if mbrErr != nil {
		logger.Debug.Printf("Failed to create member info: %+v", mbrErr)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if user64 != "" {
		var dogUserErr error
		dogUser, dogUserErr = FromDogUserHeader(user64)

		if dogUserErr != nil {
			logger.Error.Printf("Error (%+v) decoding dogfood user from header. User: %s", dogUserErr, user64)
		}
	}

	var authKind AuthKindType
	authKind, err := MakeAuthKind(authKindHdr)
	if err != nil {
		logger.Error.Printf("Error decoding auth header '%s' for account '%s', member '%s', origin '%s': %s", authKindHdr, accountId, mbr.Id, origin, err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	reqId := req.Header.Get(REQ_ID_HDR)

	ctx := AuthContext{
		Member:      mbr,
		RequestId:   reqId,
		AccountId:   bson.ObjectIdHex(accountId),
		AuthKind:    authKind,
		Origin:      origin,
		Environment: bson.ObjectIdHex(envId),
		Project:     bson.ObjectIdHex(projId),
		DogUser:     dogUser,
	}

	if err := ferror.FromError(h(w, req, ctx), reqId); err != nil {
		if err.StatusCode >= http.StatusInternalServerError {
			logger.Error.Println(err.Log())
		}
		w.WriteHeader(err.StatusCode)
		return
	}
}

func (ctx AuthContext) ErrorContext() map[string]interface{} {
	errorContext := map[string]interface{}{}
	errorContext["authKind"] = ctx.AuthKind
	errorContext["account.id"] = ctx.AccountId.Hex()
	errorContext["environment.id"] = ctx.Environment.Hex()
	errorContext["proj.id"] = ctx.Project.Hex()
	if ctx.Member != nil {
		errorContext["member.id"] = ctx.Member.Id.Hex()
		errorContext["member.role"] = ctx.Member.Role
		errorContext["person"] = map[string]interface{}{
			"id":       ctx.Member.Id.Hex(),
			"username": ctx.Member.Username,
		}
	}
	return errorContext
}

func (h WithPrivateAuthContext) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var mbr *MemberInfo
	var acctPtr, envPtr, projPtr *bson.ObjectId

	accountId := req.Header.Get(ACCOUNT_ID_HDR)
	envId := req.Header.Get(ENV_HDR)
	projId := req.Header.Get(PRJ_HDR)
	origin := req.Header.Get(ORIGIN_HDR)
	private := req.Header.Get(PRIVATE_HDR)

	if private != PRIVATE_ALLOWED {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if accountId != "" {
		if !bson.IsObjectIdHex(accountId) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		acct := bson.ObjectIdHex(accountId)
		acctPtr = &acct
	}

	if envId != "" {
		if !bson.IsObjectIdHex(envId) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		env := bson.ObjectIdHex(envId)
		envPtr = &env
	}

	if projId != "" {
		if !bson.IsObjectIdHex(projId) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		proj := bson.ObjectIdHex(projId)
		projPtr = &proj
	}

	mbr, mbrErr := getMember(req)

	if mbrErr != nil {
		logger.Debug.Printf("Failed to create member info: %+v", mbrErr)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	reqId := req.Header.Get(REQ_ID_HDR)

	ctx := PrivateAuthContext{
		RequestId:      reqId,
		OnBehalfOfAcct: acctPtr,
		Environment:    envPtr,
		Project:        projPtr,
		OnBehalfOfMbr:  mbr,
		Origin:         origin,
	}

	err := ferror.FromError(h(w, req, ctx), reqId)

	if err != nil {
		if err.StatusCode >= http.StatusInternalServerError {
			logger.Error.Println(err.Log())
		}
		w.WriteHeader(err.StatusCode)
		return
	}
}

func (ctx PrivateAuthContext) ErrorContext() map[string]interface{} {
	errorContext := map[string]interface{}{}
	if ctx.OnBehalfOfAcct != nil {
		errorContext["account.id"] = ctx.OnBehalfOfAcct.Hex()
	}
	if ctx.Environment != nil {
		errorContext["environment.id"] = ctx.Environment.Hex()
	}
	if ctx.Project != nil {
		errorContext["proj.id"] = ctx.Project.Hex()
	}
	if ctx.OnBehalfOfMbr != nil {
		errorContext["member.id"] = ctx.OnBehalfOfMbr.Id.Hex()
		errorContext["member.role"] = ctx.OnBehalfOfMbr.Role
		errorContext["person"] = map[string]interface{}{
			"id":       ctx.OnBehalfOfMbr.Id.Hex(),
			"username": ctx.OnBehalfOfMbr.Username,
		}
	}
	return errorContext
}

func ToDogUserHeader(u *lddog.User) string {
	if u != nil {
		data, _ := json.Marshal(*u)
		return base64.StdEncoding.EncodeToString(data)
	}
	return ""
}

func FromDogUserHeader(user64 string) (*lddog.User, error) {
	if user64 == "" {
		return nil, nil
	}

	data, err := base64.StdEncoding.DecodeString(user64)

	if err != nil {
		return nil, err
	}

	var dogUser lddog.User
	err = json.Unmarshal(data, &dogUser)

	return &dogUser, err
}

func AddDogUserHeader(h http.Header, u *lddog.User) {
	if u != nil {
		h.Add(DOG_USER_HDR, ToDogUserHeader(u))
	}
}

func GetDogUserFromHeader(h http.Header) (*lddog.User, error) {
	return FromDogUserHeader(h.Get(DOG_USER_HDR))
}
