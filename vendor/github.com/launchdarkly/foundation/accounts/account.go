package accounts

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/pborman/uuid"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"

	lddog "gopkg.in/launchdarkly/go-client.v3"

	"github.com/launchdarkly/foundation/accounts/roles"
	"github.com/launchdarkly/foundation/ferror"
	"github.com/launchdarkly/foundation/ftime"
	"github.com/launchdarkly/foundation/logger"
)

type AccountListing struct {
	Id           bson.ObjectId `bson:"_id,omitempty"`
	Environments []Environment
	Organization *string
	Subscription Subscription
	PostV2Signup bool
	SignupDate   *time.Time `bson:"signup_date,omitempty"`
}

type Account struct {
	Id             bson.ObjectId `bson:"_id,omitempty"`
	Organization   *string
	Members        []Member
	Projects       []Project
	Environments   []Environment
	SignupDate     *time.Time `bson:"signup_date,omitempty"`
	StripeId       *string    `bson:"stripeId,omitempty"`
	Subscription   Subscription
	Tokens         []string `bson:"tokens"`
	BillingContact BillingContact
	Version        int
	PostV2Signup   bool
	RequireMFA     bool        `bson:"require_mfa"`
	SamlConfig     *SamlConfig `bson:"saml,omitempty"`
	SessionCfg     SessionCfg  `bson:"session_cfg"`
}

type SessionCfg struct {
	// How long sessions should last
	Duration *time.Duration `bson:"duration,omitempty"`
	// Whether or not we should refresh sessions automatically
	Refresh *bool `bson:"refresh"`
	// The last time all sessions were revoked. Any sessions issued
	// before this date are invalid
	LastRevoked *ftime.UnixMillis `bson:"last_revoked,omitempty"`
}

func (acc Account) ToListing() AccountListing {
	return AccountListing{
		Id:           acc.Id,
		Organization: acc.Organization,
		Subscription: acc.Subscription,
		Environments: acc.Environments,
		PostV2Signup: acc.PostV2Signup,
		SignupDate:   acc.SignupDate,
	}
}

func (acc *Account) EnvIds() []bson.ObjectId {
	envIds := make([]bson.ObjectId, len(acc.Environments))

	for i, e := range acc.Environments {
		envIds[i] = e.Id
	}

	return envIds
}

func (acc *Account) Owner() *Member {
	for _, member := range acc.Members {
		if member.Role == roles.OwnerRole {
			return &member
		}
	}
	logger.Error.Printf("Account %s has no owner", acc.Id.Hex())
	return nil
}

type BillingContact struct {
	Name       *string `bson:"name,omitempty" json:"name,omitempty"`
	Email      *string `bson:"email,omitempty" json:"email,omitempty"`
	Address1   *string `bson:"address1,omitempty" json:"address1,omitempty"`
	Address2   *string `bson:"address2,omitempty" json:"address2,omitempty"`
	City       *string `bson:"city,omitempty" json:"city,omitempty"`
	State      *string `bson:"state,omitempty" json:"state,omitempty"`
	PostalCode *string `bson:"postalCode,omitempty" json:"postalCode,omitempty"`
	Country    *string `bson:"country,omitempty" json:"country,omitempty"`
	PoNumber   *string `bson:"poNumber,omitempty" json:"poNumber,omitempty"`
}

// returns as complete a billing contact as we can get. This will prefer any
// explicit billing contact details, and fall back to the account owner details.
func (acc *Account) GetBillingContact() BillingContact {
	var name string
	var email string
	if acc.BillingContact.Name != nil {
		name = *acc.BillingContact.Name
	} else {
		name = acc.Owner().DisplayName()
	}

	if acc.BillingContact.Email != nil {
		email = *acc.BillingContact.Email
	} else {
		email = acc.Owner().Email
	}

	return BillingContact{
		Name:       &name,
		Email:      &email,
		Address1:   acc.BillingContact.Address1,
		Address2:   acc.BillingContact.Address2,
		City:       acc.BillingContact.City,
		State:      acc.BillingContact.State,
		PostalCode: acc.BillingContact.PostalCode,
		Country:    acc.BillingContact.Country,
		PoNumber:   acc.BillingContact.PoNumber,
	}
}

func (m Member) IsOwner() bool {
	return m.Role == roles.OwnerRole
}

func (m Member) IsAdmin() bool {
	return m.Role == roles.AdminRole
}

func (m Member) IsReader() bool {
	return m.Role == roles.ReaderRole
}

func (m Member) IsWriter() bool {
	return m.Role == roles.WriterRole
}

func (m Member) HasAdminRights() bool {
	return m.IsAdmin() || m.IsOwner()
}

func (m Member) HasWriterRights() bool {
	return m.HasAdminRights() || m.IsWriter()
}

func (m Member) HasOwnerRights() bool {
	return m.IsOwner()
}

func (m Member) HasCustomRoles() bool {
	return len(m.CustomRoleIds) > 0
}

func (acc *Account) HasPaymentCardOnFile(db *mgo.Database) bool {
	ret, err := GetPaymentCard(db, *acc)
	return err == nil && ret != nil
}

// Compute the max-age header for an environment
// We set Cache-Control: max-age=X when a feature is already live.
// In that case, we can always tell caches to keep the cached value for Ttl minutes
func (e Environment) MaxAge() string {
	return fmt.Sprintf("max-age=%d", e.DefaultTtl*60)
}

type Mfa struct {
	Secret       string
	RecoveryCode *string `bson:"recoveryCode"`
}

type Member struct {
	Id            bson.ObjectId
	Email         string
	FirstName     *string
	LastName      *string
	Password      []byte
	Role          roles.RoleType
	CustomRoleIds []bson.ObjectId
	ForgotPw      *PasswordToken `bson:"forgot_pw,omitempty"`
	Invite        *PasswordToken `bson:"invite,omitempty"`
	Mfa           *Mfa           `bson:"mfa"`
	Version       int
	Onboarding    *Onboarding       `bson:"onboarding,omitempty"`
	LastLogout    *ftime.UnixMillis `bson:"last_logout,omitempty"`
}

type Onboarding struct {
	Onboarded *bool `json:"onboarded" bson:"onboarded,omitempty"`
}

// Returns true if the Member's Onboarding field is nil, or if the Onboarding's OnBoarded field is nil.
func (m *Member) IsOnboarded() bool {
	if m.Onboarding != nil && m.Onboarding.Onboarded != nil {
		return *m.Onboarding.Onboarded
	}
	return true
}

type PasswordToken struct {
	Token  string
	Expiry time.Time
}

func (token PasswordToken) IsValid() bool {
	return token.Expiry.UTC().After(time.Now().UTC())
}

func MakeForgotPwToken() PasswordToken {
	return PasswordToken{
		Token:  GenerateInviteToken(),
		Expiry: time.Now().UTC().Add(1 * time.Hour),
	}
}

func MakeInviteToken() PasswordToken {
	return PasswordToken{
		Token:  GenerateInviteToken(),
		Expiry: time.Now().UTC().Add(24 * 7 * time.Hour),
	}
}

type ExpiringApiKey struct {
	ApiKey string           `json:"apiKey" bson:"apiKey"`
	Expiry ftime.UnixMillis `json:"expiry" bson:"expiry"`
}

func (e ExpiringApiKey) IsExpired() bool {
	return e.Expiry < ftime.Now()
}

type Environment struct {
	Id             bson.ObjectId
	ProjectId      bson.ObjectId `json:"projectId" bson:"projectId"`
	Name           string
	Key            string
	ApiKey         string          `json:"apiKey" bson:"apiKey"`
	ExpiringApiKey *ExpiringApiKey `json:"expiringApiKey" bson:"expiringApiKey"`
	MobileKey      string          `json:"mobileKey" bson:"mobileKey"`
	Color          string
	DefaultTtl     int  `json:"defaultTtl" bson:"defaultTtl"`
	SecureMode     bool `json:"secureMode" bson:"secureMode"`
	Version        int
}

type Project struct {
	Id                        bson.ObjectId
	Name                      string
	Key                       string
	IncludeInSnippetByDefault bool         `json:"includeInSnippetByDefault" bson:"includeInSnippetByDefault"`
	Integrations              Integrations `json:"integrations" bson:"integrations"`
	Version                   int
}

type Integrations struct {
	Optimizely *Optimizely
}

type Optimizely struct {
	Token string
}

type SamlConfig struct {
	Enabled    bool   `json:"enabled" bson:"enabled"`
	SsoUrl     string `json:"ssoUrl" bson:"ssoUrl"`
	X509Cert   string `json:"x509Certificate" bson:"x509cert"`
	RequireSso bool   `json:"requireSso" bson:"requireSso"`
}

// A Fastly surrogate key for all features in the environment
func (env Environment) FeaturesSurrogateKey() string {
	return fmt.Sprintf("features_%s", env.Id.Hex())
}

func (env Environment) FeatureFlagsSurrogateKey() string {
	return FeatureFlagsSurrogateKeyForEnvId(env.Id)
}

func FeatureFlagsSurrogateKeyForEnvId(envId bson.ObjectId) string {
	return fmt.Sprintf("flags_%s", envId.Hex())
}

func accountIndices(db *mgo.Database) (err error) {
	err = accounts(db).EnsureIndex(mgo.Index{
		Key:        []string{"members.id"},
		Unique:     true,
		DropDups:   false,
		Background: true,
		Sparse:     false,
	})

	if err != nil {
		return
	}

	err = accounts(db).EnsureIndex(mgo.Index{
		Key:        []string{"members.email"},
		Unique:     true,
		DropDups:   false,
		Background: true,
		Sparse:     false,
	})

	if err != nil {
		return
	}

	err = accounts(db).EnsureIndex(mgo.Index{
		Key:        []string{"environments.id", "environments", "organization", "subscription"},
		Unique:     true,
		DropDups:   false,
		Background: true,
		Sparse:     false,
	})

	if err != nil {
		return
	}

	err = accounts(db).EnsureIndex(mgo.Index{
		Key:        []string{"environments.apiKey", "environments", "organization", "subscription"},
		Unique:     true,
		DropDups:   false,
		Background: true,
		Sparse:     false,
	})

	if err = accounts(db).EnsureIndex(mgo.Index{
		Key:        []string{"tokens", "_id"}, // Trivial additional key to avoid conflict with original (non-sparse) token index
		Unique:     true,
		DropDups:   false,
		Background: true,
		Sparse:     true,
	}); err != nil {
		return
	}

	accounts(db).DropIndex("tokens") // Drop the old non-sparse index (this can go away eventually)

	err = accounts(db).EnsureIndex(mgo.Index{
		Key:        []string{"environments.mobileKey", "environments", "organization", "subscription"},
		Unique:     true,
		DropDups:   false,
		Background: true,
		Sparse:     false,
	})

	if err != nil {
		return
	}

	err = accounts(db).EnsureIndex(mgo.Index{
		Key:        []string{"members.forgot_pw.token"},
		Unique:     false,
		DropDups:   false,
		Background: true,
		Sparse:     false,
	})

	if err != nil {
		return
	}

	err = accounts(db).EnsureIndex(mgo.Index{
		Key:        []string{"members.invite.token"},
		Unique:     false,
		DropDups:   false,
		Background: true,
		Sparse:     false,
	})

	if err != nil {
		return
	}

	err = accounts(db).EnsureIndex(mgo.Index{
		Key:        []string{"stripeId"},
		Unique:     true,
		DropDups:   false,
		Background: true,
		Sparse:     true,
	})

	err = accounts(db).EnsureIndex(mgo.Index{
		Key:        []string{"environments.expiringApiKey.apiKey", "environments", "organization", "subscription"},
		Unique:     true,
		DropDups:   false,
		Background: true,
		Sparse:     true,
	})

	return
}

func accounts(db *mgo.Database) *mgo.Collection {
	return db.C("accounts")
}

var sanitize = regexp.MustCompile(`([.*+?^${}()|\[\]/\\])`)

func sanitizeRegEx(src string) string {
	return sanitize.ReplaceAllString(src, "\\$1")
}

func getAccounts(db *mgo.Database) (accts []Account, err error) {
	err = accounts(db).Find(bson.M{}).All(&accts)
	return
}

func searchAccounts(db *mgo.Database, term string) (accts []Account, err error) {
	memberEmailRegex := bson.M{"$regex": bson.RegEx{sanitizeRegEx(term), "i"}}
	memberEmailClause := bson.D{{"members.email", memberEmailRegex}}

	organizationRegex := bson.M{"$regex": bson.RegEx{sanitizeRegEx(term), "i"}}
	organizationClause := bson.D{{"organization", organizationRegex}}

	clauses := []bson.D{
		memberEmailClause,
		organizationClause,
	}

	if bson.IsObjectIdHex(term) {
		idClause := bson.D{{"_id", bson.ObjectIdHex(term)}}
		clauses = append(clauses, idClause)
	}

	queryOr := bson.DocElem{"$or", clauses}
	query := bson.D{queryOr}
	err = accounts(db).Find(query).All(&accts)
	return
}

// FindAccounts returns accounts for which term matches either the
// account id, the organization name, or a member email.
func FindAccounts(db *mgo.Database, term string) (accts []Account, err error) {
	if term == "" {
		accts, err = getAccounts(db)
	} else {
		accts, err = searchAccounts(db, term)
	}

	return
}

func FindAccountByStripeId(db *mgo.Database, stripeId string) (account Account, err error) {
	err = accounts(db).Find(bson.M{"stripeId": stripeId}).One(&account)
	return
}

func FindAccountByMemberEmail(db *mgo.Database, email string) (account Account, err error) {
	err = accounts(db).Find(bson.M{"members": bson.M{"$elemMatch": bson.M{"email": email}}}).One(&account)
	return
}

func FindAccountByMemberId(db *mgo.Database, id string) (account Account, err error) {
	if !bson.IsObjectIdHex(id) {
		err = errors.New("Invalid member ID")
		return
	}

	err = accounts(db).Find(bson.M{"members": bson.M{"$elemMatch": bson.M{"id": bson.ObjectIdHex(id)}}}).One(&account)
	return
}

func DeleteAccount(db *mgo.Database, id bson.ObjectId) error {
	return accounts(db).Remove(bson.M{"_id": id})
}

func FindAccountByApiKey(db *mgo.Database, key string) (account Account, err error) {
	err = accounts(db).Find(bson.M{"environments": bson.M{"$elemMatch": bson.M{"apiKey": key}}}).One(&account)

	// If we couldn't find an account with that API key, then we look for an expiring API key
	// whose expiry is in the future
	if err == mgo.ErrNotFound {
		now := ftime.Now()
		err = accounts(db).Find(bson.M{"environments": bson.M{"$elemMatch": bson.M{"expiringApiKey.apiKey": key, "expiringApiKey.expiry": bson.M{"$gte": now}}}}).One(&account)
	}

	return
}

func FindAccountByToken(db *mgo.Database, token string) (account Account, err error) {
	err = accounts(db).Find(bson.D{{"tokens", token}}).One(&account)
	return
}

func FindAccountListingByApiKey(db *mgo.Database, key string) (account AccountListing, err error) {
	err = accounts(db).Find(bson.M{"environments.apiKey": key}).Select(bson.M{
		"_id":            1,
		"environments.$": 1,
		"organization":   1,
		"subscription":   1,
		"postv2signup":   1,
		"signup_date":    1,
	}).One(&account)

	if err == mgo.ErrNotFound {
		now := ftime.Now()
		err = accounts(db).Find(bson.M{"environments": bson.M{"$elemMatch": bson.M{"expiringApiKey.apiKey": key, "expiringApiKey.expiry": bson.M{"$gte": now}}}}).Select(bson.M{
			"_id":            1,
			"environments.$": 1,
			"organization":   1,
			"subscription":   1,
			"postv2signup":   1,
			"signup_date":    1,
		}).One(&account)
	}

	return
}

func FindAccountByMobileKey(db *mgo.Database, key string) (account Account, err error) {
	err = accounts(db).Find(bson.M{"environments": bson.M{"$elemMatch": bson.M{"mobileKey": key}}}).One(&account)
	return
}

func FindAccountListingByMobileKey(db *mgo.Database, key string) (account AccountListing, err error) {
	err = accounts(db).Find(bson.M{"environments.mobileKey": key}).Select(bson.M{
		"_id":            1,
		"environments.$": 1,
		"organization":   1,
		"subscription":   1,
		"postv2signup":   1,
		"signup_date":    1,
	}).One(&account)
	return
}

func FindAccountById(db *mgo.Database, id bson.ObjectId) (account Account, err error) {
	err = accounts(db).FindId(id).One(&account)
	return
}

// Many efficiency gains are lost here because when you can't identify the environment, the AccountListing
// is not much smaller than the Account.
func FindAccountListingById(db *mgo.Database, id bson.ObjectId) (account AccountListing, err error) {
	err = accounts(db).FindId(id).Select(bson.M{
		"_id":          1,
		"environments": 1,
		"organization": 1,
		"subscription": 1,
		"postv2signup": 1,
		"signup_date":  1,
	}).One(&account)
	return
}

func FindAccountByEnvironmentId(db *mgo.Database, id bson.ObjectId) (account Account, err error) {
	err = accounts(db).Find(bson.M{"environments": bson.M{"$elemMatch": bson.M{"id": id}}}).One(&account)
	return
}

func FindAccountListingByEnvironmentId(db *mgo.Database, id bson.ObjectId) (account AccountListing, err error) {
	err = accounts(db).Find(bson.M{"environments": bson.M{"$elemMatch": bson.M{"id": id}}}).Select(bson.M{
		"_id":          1,
		"environments": 1,
		"organization": 1,
		"subscription": 1,
		"postv2signup": 1,
		"signup_date":  1,
	}).One(&account)
	return
}

func FindTtlByEnvironmentId(db *mgo.Database, envId bson.ObjectId) (int, error) {
	var account Account
	err := accounts(db).Find(bson.M{"environments.id": envId}).Select(bson.M{
		"environments.$": 1,
	}).One(&account)

	if err != nil || len(account.Environments) == 0 {
		return 0, err
	}

	return account.Environments[0].DefaultTtl, nil
}

func FindAccountByForgotPwToken(db *mgo.Database, token string) (account Account, err error) {
	err = accounts(db).Find(bson.M{"members": bson.M{"$elemMatch": bson.M{"forgot_pw.token": token}}}).One(&account)

	if err != nil {
		return
	}

	// Also validate that the token isn't expired
	if account.FindMemberByForgotPwToken(token) == nil {
		return account, errors.New("Token is expired")
	}
	return
}

func FindAccountByInviteToken(db *mgo.Database, token string) (account Account, err error) {
	err = accounts(db).Find(bson.M{"members": bson.M{"$elemMatch": bson.M{"invite.token": token}}}).One(&account)

	if err != nil {
		return
	}

	// Also validate that the token isn't expired
	if account.FindMemberByInviteToken(token) == nil {
		return account, errors.New("Token is expired")
	}
	return
}

func FindAccountError(err error, reqId string) *ferror.Error {
	if err == mgo.ErrNotFound {
		ferr := ferror.NewUnauthorizedError(
			"Invalid key",
			err,
			reqId)
		return ferr
	} else {
		ferr := ferror.NewInternalError("Internal error", err, reqId)
		return ferr
	}
}

func setPwToken(db *mgo.Database, email string, invite bool) (token PasswordToken, err error) {
	var acct, newAcct Account
	acct, err = FindAccountByMemberEmail(db, email)

	if err != nil {
		return
	}

	member := acct.FindMemberByEmail(email)

	if member == nil {
		err = errors.New("Could not find member " + email)
	}

	newMember := *member

	if invite {
		token = MakeInviteToken()
		newMember.Invite = &token
	} else {
		token = MakeForgotPwToken()
		newMember.ForgotPw = &token
	}

	if newAcct, err = acct.UpdateMember(*member, newMember); err != nil {
		logger.Error.Printf("Invariant failed-- setting password tokens should never change e-mail addresses")
		return
	}

	err = UpdateAccount(db, acct, newAcct)

	return
}

func SetForgotPwToken(db *mgo.Database, email string) (token PasswordToken, err error) {
	return setPwToken(db, email, false)
}

func (m Member) DisplayName() string {
	if m.FirstName != nil && m.LastName != nil {
		name := *m.FirstName + " " + *m.LastName
		name = strings.TrimSpace(name)
		if name == "" {
			return m.Email
		} else {
			return name
		}
	} else {
		return m.Email
	}
}

func (acct Account) IsCustomRoleInUse(roleId bson.ObjectId) bool {
	for _, mbr := range acct.Members {
		if mbr.HasCustomRole(roleId) {
			return true
		}
	}

	return false
}

func (mbr Member) HasCustomRole(roleId bson.ObjectId) bool {
	for _, id := range mbr.CustomRoleIds {
		if id == roleId {
			return true
		}
	}

	return false
}

func (acct Account) TrialDaysRemaining() int64 {
	trialEndDate := acct.Subscription.TrialEndDate.UTC()
	now := time.Now().UTC()

	if trialEndDate.Before(now) {
		return 0
	}

	return int64(trialEndDate.Sub(now).Hours() / 24)
}

func (acct Account) UpdateMember(o, n Member) (Account, error) {
	// Check whether a user with the same e-mail address exists
	for i, member := range acct.Members {
		if o.Id == member.Id {
			acct.Members[i] = n
		} else if member.Email == n.Email {
			return acct, errors.New("E-mail address already exists in this account")
		}
	}
	return acct, nil
}

func (acct Account) UpdateEnvironment(o, n Environment) Account {
	for i, env := range acct.Environments {
		if o.Id == env.Id {
			acct.Environments[i] = n
		}
	}
	return acct
}

func (acct Account) UpdateProject(o, n Project) Account {
	for i, proj := range acct.Projects {
		if o.Id == proj.Id {
			acct.Projects[i] = n
		}
	}
	return acct
}

func (acct Account) FindProject(env Environment) (project *Project) {
	for _, p := range acct.Projects {
		if p.Id == env.ProjectId {
			project = &p
			return
		}
	}

	return
}

func (acct Account) FindProjectByKey(projKey string) (project *Project) {
	for _, p := range acct.Projects {
		if p.Key == projKey {
			project = &p
			return
		}
	}

	return
}

func (acct Account) FindProjectByEnvId(envId bson.ObjectId) (project *Project) {
	env := acct.FindEnvironmentById(envId)

	if env == nil {
		return
	}

	return acct.FindProject(*env)
}

func (acct Account) AddEnvironment(e Environment, position int) Account {
	envs := make([]Environment, 0)

	for i, env := range acct.Environments {
		if position == i {
			envs = append(envs, e)
		}
		envs = append(envs, env)
	}
	acct.Environments = envs
	return acct
}

func (acct Account) AppendEnvironment(e Environment) Account {
	return acct.AddEnvironment(e, 0)
}

func (acct Account) TrialExpirationDate() int64 {
	return acct.Subscription.TrialEndDate.Unix()
}

func (acct Account) FindMemberByEmail(email string) *Member {
	for _, member := range acct.Members {
		if member.Email == email {
			return &member
		}
	}
	return nil
}

func (acct Account) FindMemberByForgotPwToken(token string) *Member {
	for _, member := range acct.Members {
		if member.ForgotPw != nil && (*member.ForgotPw).Token == token && (*member.ForgotPw).IsValid() {
			return &member
		}
	}
	return nil
}

func (acct Account) FindMemberByInviteToken(token string) *Member {
	for _, member := range acct.Members {
		if member.Invite != nil && (*member.Invite).Token == token && (*member.Invite).IsValid() {
			return &member
		}
	}
	return nil
}

func (acct Account) FindMemberById(id string) *Member {
	if !bson.IsObjectIdHex(id) {
		return nil
	}

	oid := bson.ObjectIdHex(id)

	for _, member := range acct.Members {
		if member.Id == oid {
			return &member
		}
	}
	return nil
}

func (acct Account) FindProjectById(oid bson.ObjectId) *Project {
	for _, project := range acct.Projects {
		if project.Id == oid {
			return &project
		}
	}
	return nil
}

func (acct Account) FindEnvironmentById(oid bson.ObjectId) *Environment {
	for _, env := range acct.Environments {
		if env.Id == oid {
			return &env
		}
	}
	return nil
}

func (acct Account) FindEnvironmentByProjectIdAndKey(pId bson.ObjectId, key string) *Environment {
	for _, env := range acct.Environments {
		if env.Key == key && env.ProjectId == pId {
			return &env
		}
	}
	return nil
}

func (acct Account) FindEnvironmentByApiKey(apiKey string) *Environment {
	for _, env := range acct.Environments {
		if env.ApiKey == apiKey {
			return &env
		}

		if env.ExpiringApiKey != nil && env.ExpiringApiKey.ApiKey == apiKey && !env.ExpiringApiKey.IsExpired() {
			return &env
		}
	}

	return nil
}

func (acct Account) FindEnvironmentByMobileKey(mobileKey string) *Environment {
	for _, env := range acct.Environments {
		if env.MobileKey == mobileKey {
			return &env
		}
	}
	return nil
}

func (a Account) ToDogfoodUser() lddog.User {
	ret := a.ToListing().ToDogfoodUser()
	if a.Owner() != nil {
		email := a.Owner().Email
		ret.Email = &email
		ret.FirstName = a.Owner().FirstName
		ret.LastName = a.Owner().LastName
	}
	return ret
}

func (acct AccountListing) FindEnvironmentById(oid bson.ObjectId) *Environment {
	for _, env := range acct.Environments {
		if env.Id == oid {
			return &env
		}
	}
	return nil
}

func (acct AccountListing) FindEnvironmentByApiKey(apiKey string) *Environment {
	for _, env := range acct.Environments {
		if env.ApiKey == apiKey {
			return &env
		}

		if env.ExpiringApiKey != nil && env.ExpiringApiKey.ApiKey == apiKey && !env.ExpiringApiKey.IsExpired() {
			return &env
		}
	}
	return nil
}

func (acct AccountListing) FindEnvironmentByMobileKey(mobileKey string) *Environment {
	for _, env := range acct.Environments {
		if env.MobileKey == mobileKey {
			return &env
		}
	}
	return nil
}

func (a AccountListing) ToDogfoodUser() lddog.User {
	custom := map[string]interface{}{
		"organization": a.Organization,
		"accountId":    a.Id.Hex(),
		"isTrial":      a.Subscription.InTrial(),
		"isLapsed":     a.Subscription.IsLapsed(),
		"isBeta":       a.Subscription.IsBetaAccountSubscription(),
		"postV2Signup": a.PostV2Signup,
	}

	if a.SignupDate != nil {
		custom["signupDate"] = ftime.ToUnixMillis(*a.SignupDate)
	}

	if a.Subscription.Plan != nil {
		custom["planName"] = a.Subscription.Plan.Name
		custom["planNameVersion"] = fmt.Sprintf("%s-%d", a.Subscription.Plan.Name, a.Subscription.Plan.Version)
		custom["isPlanPubliclyAvailable"] = a.Subscription.Plan.PubliclyAvailable
	}

	id := a.Id.Hex()
	return lddog.User{Key: &id, Custom: &custom}
}

func (acct Account) DefaultEnvironment() *Environment {
	return &acct.Environments[len(acct.Environments)-1]
}

func (acct Account) HasSubscription() bool {
	return acct.Subscription.Plan != nil && acct.Subscription.StripeSubscriptionId != nil
}

func (acct Account) FindEnvironmentsForProject(p Project) []Environment {
	envs := make([]Environment, 0)
	for _, env := range acct.Environments {
		if env.ProjectId == p.Id {
			envs = append(envs, env)
		}
	}
	return envs
}

func CreateMember(email string, password string, first *string, last *string, role roles.RoleType, customRoles []bson.ObjectId) Member {
	pw, err := EncryptPassword(password)
	if err != nil {
		panic("Unable to encrypt password")
	}
	onboardedDefaultValue := false
	member := Member{
		Id:            bson.NewObjectId(),
		Email:         email,
		FirstName:     first,
		LastName:      last,
		Password:      pw,
		ForgotPw:      nil,
		Invite:        nil,
		Role:          role,
		CustomRoleIds: customRoles,
		Onboarding: &Onboarding{
			Onboarded: &onboardedDefaultValue,
		},
	}
	return member
}

func createDefaultProject() Project {
	return Project{
		Id:   bson.NewObjectId(),
		Name: "Default Project",
		Key:  "default",
		Integrations: Integrations{
			Optimizely: nil,
		},
	}
}

func createDefaultEnvironments(projectId bson.ObjectId) []Environment {
	test := Environment{
		Id:         bson.NewObjectId(),
		Name:       "Test",
		Key:        "test",
		ApiKey:     GenerateApiKey(),
		MobileKey:  GenerateMobileKey(),
		ProjectId:  projectId,
		Color:      "F5A623",
		DefaultTtl: 0,
	}
	prod := Environment{
		Id:         bson.NewObjectId(),
		Name:       "Production",
		Key:        "production",
		ApiKey:     GenerateApiKey(),
		MobileKey:  GenerateMobileKey(),
		ProjectId:  projectId,
		Color:      "417505",
		DefaultTtl: 0,
	}

	return []Environment{test, prod}
}

func CreateAccount(db *mgo.Database, organization *string, email string, password string, first *string, last *string, coupon *string, generateAccountToken bool) (account Account, err error) {
	now := time.Now().UTC()

	project := createDefaultProject()

	tokens := []string{}
	if generateAccountToken {
		tokens = append(tokens, GenerateAccountToken())
	}

	account = Account{
		Organization: organization,
		Members:      []Member{CreateMember(email, password, first, last, roles.OwnerRole, nil)},
		Projects:     []Project{project},
		Environments: createDefaultEnvironments(project.Id),
		SignupDate:   &now,
		Subscription: Subscription{
			TrialStartDate: now,
			TrialEndDate:   add30Days(now),
			Coupon:         coupon,
		},
		Tokens:       tokens,
		PostV2Signup: true,
	}

	err = accounts(db).Insert(&account)

	if err == nil {
		return FindAccountByMemberEmail(db, email)
	}

	return
}

func UpdateAccount(db *mgo.Database, o Account, n Account) (err error) {
	err = accounts(db).Update(bson.M{"_id": o.Id}, n)
	return
}

func AddMember(db *mgo.Database, acct Account, m Member) error {
	return AddMembers(db, acct, []Member{m})
}

type AddMembersErrorCode int

const (
	ERR_DUPLICATE_EMAILS AddMembersErrorCode = iota
	ERR_ALREADY_EXISTS
	ERR_EXISTS_IN_ANOTHER_ACCOUNT
)

var AddMemberErrorMessages = map[AddMembersErrorCode]string{
	ERR_DUPLICATE_EMAILS:          "The following email addresses are duplicated in the list of new members:",
	ERR_ALREADY_EXISTS:            "Members with the following e-mail addresses already exist in this account:",
	ERR_EXISTS_IN_ANOTHER_ACCOUNT: "The following email addresses belong to members of other LaunchDarkly accounts:",
}

type AddMembersError struct {
	Emails []string
	Code   AddMembersErrorCode
}

func (err AddMembersError) Error() string {
	return AddMemberErrorMessages[err.Code] + " " + strings.Join(err.Emails, ", ")
}

func AddMembers(db *mgo.Database, acct Account, newMembers []Member) error {
	newEmails := getEmailsForMembers(newMembers)

	duplicateNewEmails := findDuplicateStrings(newEmails)
	if len(duplicateNewEmails) > 0 {
		return AddMembersError{Code: ERR_DUPLICATE_EMAILS, Emails: duplicateNewEmails}
	}

	existingEmails := getEmailsForMembers(acct.Members)

	existingIds := map[bson.ObjectId]bool{}
	for _, member := range newMembers {
		if _, exists := existingIds[member.Id]; exists {
			return fmt.Errorf("A member with ID '%s' already exists in this account", member.Id)
		}
	}

	duplicateEmails := findDuplicateStrings(append(newEmails, uniqueStrings(existingEmails)...)) // Don't prevent adding when there are already duplicates
	if len(duplicateEmails) > 0 {
		return AddMembersError{Code: ERR_ALREADY_EXISTS, Emails: duplicateEmails}
	}

	// Use an update selector to ensure atomically that we don't create duplicate users in mongodb
	updateSelector := bson.M{
		"_id": acct.Id,
		"members": bson.M{
			"$not": bson.M{
				"$elemMatch": bson.M{
					"email": bson.M{"$in": newEmails},
				},
			},
		},
	}
	err := accounts(db).Update(updateSelector, bson.M{"$push": bson.M{"members": bson.M{"$each": newMembers}}})
	if err != nil {
		if mgo.IsDup(err) {
			emailsInOtherAccounts, err := FilterEmailsInAnyExistingAccount(db, newEmails)
			if err != nil {
				return err
			}
			if len(emailsInOtherAccounts) > 0 {
				return AddMembersError{Code: ERR_EXISTS_IN_ANOTHER_ACCOUNT, Emails: emailsInOtherAccounts}
			}
		}
	}

	return err
}

func FilterEmailsInAnyExistingAccount(db *mgo.Database, emails []string) ([]string, error) {
	pipeline := []bson.M{
		{"$match": bson.M{"members.email": bson.M{"$in": emails}}},
		{"$project": bson.M{"members.email": true}},
		{"$unwind": "$members"},
		{"$match": bson.M{"members.email": bson.M{"$in": emails}}},
		{"$group": bson.M{"_id": nil, "emails": bson.M{"$addToSet": "$members.email"}}},
	}

	var result struct {
		Emails []string
	}
	err := accounts(db).Pipe(pipeline).One(&result)
	return result.Emails, err
}

func RemoveMember(db *mgo.Database, acct Account, m Member) (err error) {
	err = accounts(db).UpdateId(acct.Id, bson.M{"$pull": bson.M{"members": bson.M{"id": m.Id}}})
	return
}

func RemoveEnvironment(db *mgo.Database, acct Account, e Environment) (err error) {
	err = accounts(db).UpdateId(acct.Id, bson.M{"$pull": bson.M{"environments": bson.M{"id": e.Id}}})
	return
}

func UpdateEnvironment(db *mgo.Database, acct Account, e Environment) (err error) {
	err = accounts(db).Update(bson.M{"environments.id": e.Id}, bson.M{"$set": bson.M{"environments.$": e}})
	return
}

func AddEnvironment(db *mgo.Database, acct Account, project Project, name, key string, color string, ttl int) (env Environment, err error) {
	env = Environment{
		Id:         bson.NewObjectId(),
		Name:       name,
		Key:        key,
		Color:      color,
		ApiKey:     GenerateApiKey(),
		MobileKey:  GenerateMobileKey(),
		DefaultTtl: ttl,
		ProjectId:  project.Id,
	}
	err = accounts(db).Update(bson.M{"_id": acct.Id}, bson.M{"$push": bson.M{"environments": bson.D{{"$each", []Environment{env}}, {"$position", 0}}}})
	return
}

// Create a new project with the default environments
func AddProject(db *mgo.Database, acct Account, name, key string, includeInSnippetByDefault bool) error {
	proj := Project{
		Id:   bson.NewObjectId(),
		Name: name,
		Key:  key,
		IncludeInSnippetByDefault: includeInSnippetByDefault,
		Integrations: Integrations{
			Optimizely: nil,
		},
	}

	envs := createDefaultEnvironments(proj.Id)
	return accounts(db).Update(bson.M{"_id": acct.Id},
		bson.M{
			"$push": bson.D{
				{"projects", bson.D{{"$each", []Project{proj}}, {"$position", 0}}},
				{"environments", bson.D{{"$each", envs}, {"$position", 0}}},
			},
		})
}

// Remove a project and all environments associated with the project
func RemoveProject(db *mgo.Database, acct Account, project Project) error {
	return accounts(db).UpdateId(acct.Id,
		bson.M{
			"$pull": bson.D{
				{"environments", bson.M{"projectId": project.Id}},
				{"projects", bson.M{"id": project.Id}},
			},
		})
}

func EncryptPassword(password string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
}

func CheckPassword(hash []byte, pw string) bool {
	err := bcrypt.CompareHashAndPassword(hash, []byte(pw))
	return err == nil
}

// Update a project
func UpdateProject(db *mgo.Database, acct Account, p Project) error {
	return accounts(db).Update(bson.M{"projects.id": p.Id}, bson.M{"$set": bson.M{"projects.$": p}})
}

func GenerateAccountToken() string {
	return fmt.Sprintf("api-%s", uuid.NewRandom().String())
}

func GenerateApiKey() string {
	return fmt.Sprintf("sdk-%s", uuid.NewRandom().String())
}

func GenerateMobileKey() string {
	return fmt.Sprintf("mob-%s", uuid.NewRandom().String())
}

func GenerateInviteToken() string {
	return uuid.New()
}

func GenerateRecoveryCode() string {
	return uuid.New()
}

func getEmailsForMembers(members []Member) []string {
	emails := make([]string, len(members))
	for i, member := range members {
		emails[i] = member.Email
	}
	return emails
}

func findDuplicateStrings(strs []string) []string {
	counts := map[string]int{}
	duplicates := []string{}
	for _, str := range strs {
		if count, exists := counts[str]; exists {
			if count == 1 {
				duplicates = append(duplicates, str)
			}
		}
		counts[str] += 1
	}
	return duplicates
}

func uniqueStrings(strs []string) []string {
	visited := map[string]bool{}
	uniques := []string{}
	for _, str := range strs {
		if _, exists := visited[str]; !exists {
			uniques = append(uniques, str)
		}
		visited[str] = true
	}
	return uniques
}
