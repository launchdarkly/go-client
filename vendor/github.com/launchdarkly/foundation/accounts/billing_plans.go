package accounts

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/launchdarkly/foundation/ferror"
	"github.com/launchdarkly/foundation/logger"
	stripe "github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/card"
	"github.com/stripe/stripe-go/coupon"
	"github.com/stripe/stripe-go/customer"
	"github.com/stripe/stripe-go/discount"
	"github.com/stripe/stripe-go/plan"
	"github.com/stripe/stripe-go/sub"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

const (
	couponSource            = "source"
	couponBlurb             = "blurb"
	CouponDurationForever   = "forever"
	CouponDurationRepeating = "repeating"
	CouponDurationOnce      = "once"
)

type PlanRef struct {
	Id                bson.ObjectId `bson:"_id,omitempty"`
	Name              string
	Version           int
	StripePlanId      *string `bson:"stripePlanId,omitempty"`
	PubliclyAvailable bool    `bson:"publiclyAvailable"`
}

type Plan struct {
	PlanRef      `bson:",inline"`
	ActiveDate   time.Time  `bson:"activeDate"`
	InactiveDate *time.Time `bson:"inactiveDate,omitempty"`
	CreatedDate  time.Time  `bson:"createdDate"`
}

func (p Plan) IsActive() bool {
	return p.PubliclyAvailable && nowInRange(time.Now(), p.ActiveDate, p.InactiveDate)
}

func (p Plan) BilledByStripe() bool {
	return p.StripePlanId != nil
}

func (p Plan) MonthlyPrice() (uint64, error) {
	if p.PubliclyAvailable && !p.BilledByStripe() { // free plan
		return 0, nil
	} else if !p.BilledByStripe() {
		return 0, errors.New("Cannot determine monthly price of private plan not billed by stripe")
	}
	splan, err := plan.Get(*p.StripePlanId, nil)
	if err != nil {
		return 0, err
	}
	return splan.Amount, nil
}

type Subscription struct {
	Plan                 *PlanRef
	StripeSubscriptionId *string    `bson:"stripeSubscriptionId,omitempty"`
	TrialStartDate       time.Time  `bson:"trialStartDate"`
	TrialEndDate         time.Time  `bson:"trialEndDate"`
	GraceStartDate       *time.Time `bson:"graceStartDate,omitempty"`
	GraceEndDate         *time.Time `bson:"graceEndDate,omitempty"`
	CanceledDate         *time.Time `bson:"canceledDate,omitempty"`
	Coupon               *string    `bson:"coupon,omitempty"`
	SeatCount            *int       `bson:"seatCount,omitempty"`
}

type CouponDetails struct {
	Code            string  `json:"code"`
	FlatDiscount    *uint64 `json:"flatDiscount,omitempty"`
	PercentDiscount *uint64 `json:"percentDiscount,omitempty"`
	Active          bool    `json:"active"`
	Source          string  `json:"source"` // indicate where the code was distributed
	Blurb           string  `json:"blurb"`
	Duration        string  `json:"duration"`                 // forever, once, repeating
	DurationMonths  *uint64 `json:"durationMonths,omitempty"` // when duration is repeating
}

func GetCouponDetails(couponCode string) (*CouponDetails, error) {
	if c, stripeErr := coupon.Get(couponCode, nil); stripeErr == nil {
		ret := CouponDetails{
			Code:     c.ID,
			Active:   c.Valid,
			Source:   c.Meta[couponSource],
			Blurb:    c.Meta[couponBlurb],
			Duration: string(c.Duration),
		}
		if c.DurationPeriod != 0 {
			ret.DurationMonths = &c.DurationPeriod
		}
		if c.Amount != 0 {
			ret.FlatDiscount = &c.Amount
		}
		if c.Percent != 0 {
			ret.PercentDiscount = &c.Percent
		}
		return &ret, nil
	} else {
		if stripeErr, ok := stripeErr.(*stripe.Error); ok && stripeErr.HTTPStatusCode == http.StatusNotFound {
			return nil, nil
		}
		return nil, stripeErr
	}
}

func CreateCoupon(code string, flatDiscount, percentDiscount *uint64, duration string, durationMonths *uint64, source, blurb string) (*CouponDetails, error) {
	params := stripe.CouponParams{
		ID:       code,
		Duration: stripe.CouponDuration(duration),
		Params: stripe.Params{
			Meta: make(map[string]string),
		},
	}
	if durationMonths != nil {
		params.DurationPeriod = *durationMonths
	}
	if flatDiscount != nil {
		params.Amount = *flatDiscount
		params.Currency = "usd"
	}
	if percentDiscount != nil {
		params.Percent = *percentDiscount
	}
	params.Meta[couponBlurb] = blurb
	params.Meta[couponSource] = source
	_, err := coupon.New(&params)
	if err != nil {
		return nil, err
	} else {
		return GetCouponDetails(code)
	}
}

// Gets the coupon code that is currently associated with the account. This includes coupons that are
// tagged on the account before a paid plan is selected, and coupons that are already associated with
// and account in Stripe.
func (s Subscription) CouponCode(account Account) (*string, error) {
	if s.Coupon != nil {
		return s.Coupon, nil
	}
	if s.StripeSubscriptionId != nil {
		stripeSub, subErr := getStripeSubscription(*s.StripeSubscriptionId, *account.StripeId)
		if subErr != nil {
			return nil, subErr
		}
		if stripeSub.Discount == nil || stripeSub.Discount.Coupon == nil {
			return nil, nil
		} else {
			return &stripeSub.Discount.Coupon.ID, nil
		}

	}
	return nil, nil
}

func (s Subscription) CouponDetails() (*CouponDetails, error) {
	if s.Coupon == nil {
		return nil, nil
	}
	return GetCouponDetails(*s.Coupon)
}

func (s Subscription) GetBillingPeriod(account Account) (periodStart time.Time, periodEnd time.Time, err error) {
	if s.StripeSubscriptionId == nil {
		return
	}

	if account.StripeId == nil {
		return
	}

	stripeSub, subErr := getStripeSubscription(*s.StripeSubscriptionId, *account.StripeId)
	if subErr != nil {
		err = subErr
		return
	}

	periodStart = time.Unix(stripeSub.PeriodStart, 0)
	periodEnd = time.Unix(stripeSub.PeriodEnd, 0)
	return
}

func (s Subscription) IsBetaAccountSubscription() bool {
	return s.TrialEndDate.IsZero() && s.TrialStartDate.IsZero()
}

func (s Subscription) IsPlanSelected() bool {
	return s.Plan != nil
}

func (s Subscription) TrialGracePeriodEnd() (trialGraceEnd time.Time) {
	if s.Plan == nil && !s.TrialEndDate.IsZero() {
		trialGraceEnd = add30Days(s.TrialEndDate)
	} else {
		// if they selected a plan, they don't need the grace period
		trialGraceEnd = s.TrialEndDate
	}
	return
}

func (s Subscription) InTrial() bool {
	trialGraceEnd := s.TrialGracePeriodEnd()
	return nowInRange(time.Now(), s.TrialStartDate, &trialGraceEnd)
}

func (s Subscription) InGracePeriod() bool {
	if s.InTrial() {
		// see if we are in the trial grace period
		trialGraceEnd := s.TrialGracePeriodEnd()
		return nowInRange(time.Now(), s.TrialEndDate, &trialGraceEnd)
	} else {
		return s.GraceStartDate != nil && nowInRange(time.Now(), *s.GraceStartDate, s.GraceEndDate)
	}
}

func (s Subscription) IsLapsedTrialWithNoPlan() bool {
	return s.Plan == nil && !s.InTrial()
}

func (s Subscription) IsLapsed() bool {
	return !s.IsBetaAccountSubscription() &&
		(s.IsLapsedTrialWithNoPlan() || (s.GraceEndDate != nil && nowInRange(time.Now(), *s.GraceEndDate, nil)))
}

func (s Subscription) IsCanceled() bool {
	return s.CanceledDate != nil
}

func (s *Subscription) updatePlan(p Plan, stripeSubscriptionId *string) {
	s.Plan = &p.PlanRef
	s.StripeSubscriptionId = stripeSubscriptionId
	s.CanceledDate = nil
}

func (p *Subscription) PaymentDenied(deniedDate time.Time) {
	if !p.InGracePeriod() && !p.IsLapsed() {
		endDate := add30Days(deniedDate)
		p.GraceStartDate = &deniedDate
		p.GraceEndDate = &endDate
	}
}

func (p *Subscription) PaymentAccepted() {
	p.CanceledDate = nil
	p.GraceStartDate = nil
	p.GraceEndDate = nil
}

func (p *Subscription) cancelSubscription(canceledDate, periodEndDate time.Time) {
	p.CanceledDate = &canceledDate
	p.GraceStartDate = &canceledDate
	p.GraceEndDate = &periodEndDate
}

type PaymentCard struct {
	LastFour     string `json:"last4"`
	Brand        string `json:"brand"`
	ExpMonth     uint8  `json:"expMonth"`
	ExpYear      uint16 `json:"expYear"`
	stripeCardId string
}

func SetSubscription(db *mgo.Database, account Account, basePlan Plan, stripeToken *string, requestId string) (newAcct Account, subscription Subscription, ferr *ferror.Error) {
	logger.Info.Printf("Updating subscription for account %s: %s", account.Id.Hex(), spew.Sdump(basePlan.StripePlanId))
	if !account.HasPaymentCardOnFile(db) && stripeToken == nil && basePlan.BilledByStripe() {
		return newAcct, subscription, ferror.NewInvalidRequest("The account doesn't have a card on file, and a stripe token was not provided", http.StatusBadRequest, nil, requestId)
	} else if stripeToken != nil {
		logger.Info.Printf("Updating payment card for account %s", account.Id.Hex())
		// update the card on file:
		_, cardErr := SetPaymentCard(db, account, *stripeToken)
		if cardErr != nil {
			logger.Error.Printf("Error updating payment card for account %s: %+v", account.Id.Hex(), cardErr)
			return newAcct, subscription, errToFerr(cardErr, requestId)
		}
		// reload the account from the database-- the stripe id may have been updated
		var accErr error
		account, accErr = FindAccountById(db, account.Id)
		if accErr != nil {
			return newAcct, subscription, errToFerr(accErr, requestId)
		}
	}
	var subErr error
	if !account.HasSubscription() {
		newAcct, subscription, subErr = newSubscription(db, account, basePlan)
		return newAcct, subscription, errToFerr(subErr, requestId)
	} else {
		newAcct, subscription, subErr = changeSubscription(db, account, basePlan)
		return newAcct, subscription, errToFerr(subErr, requestId)
	}
}

func errToFerr(err error, requestId string) *ferror.Error {
	if err == nil {
		return nil
	}
	if e, ok := err.(*stripe.Error); ok {
		return ferror.NewInvalidRequest(e.Msg, http.StatusBadRequest, e, requestId)
	}
	return ferror.NewInternalError(err.Error(), err, requestId)
}

func GetPaymentCard(db *mgo.Database, account Account) (pc *PaymentCard, err error) {
	if account, err = CreateStripeCustomer(db, account); err != nil {
		logger.Error.Printf("Error creating customer record in stripe for account %s: %+v", account.Id.Hex(), err)
		return
	}
	cardIter := card.List(&stripe.CardListParams{Customer: *account.StripeId})
	if cardIter.Next() { // For simplicity, we are only supporting one card at a time
		existingCard := cardIter.Card()
		if existingCard != nil {
			pc = &PaymentCard{
				LastFour:     existingCard.LastFour,
				ExpMonth:     existingCard.Month,
				ExpYear:      existingCard.Year,
				Brand:        string(existingCard.Brand),
				stripeCardId: existingCard.ID,
			}
		}
	}
	return
}

func SetPaymentCard(db *mgo.Database, account Account, stripeToken string) (pc PaymentCard, err error) {
	if account, err = CreateStripeCustomer(db, account); err != nil {
		logger.Error.Printf("Error creating customer record in stripe for account %s: %+v", account.Id.Hex(), err)
		return
	}
	existingCard, cardErr := GetPaymentCard(db, account)
	if cardErr != nil {
		err = cardErr
		return
	}
	params := stripe.CardParams{
		Customer: *account.StripeId,
		Token:    stripeToken,
	}
	var newCard *stripe.Card
	newCard, err = card.New(&params)
	if err == nil && existingCard != nil {
		// the new card has added sucessfully, and we need to remove the old card
		err = card.Del(existingCard.stripeCardId, &params)
	}
	if err == nil {
		pc = PaymentCard{
			LastFour:     newCard.LastFour,
			ExpMonth:     newCard.Month,
			ExpYear:      newCard.Year,
			Brand:        string(newCard.Brand),
			stripeCardId: newCard.ID,
		}
	}
	return
}

func newSubscription(db *mgo.Database, account Account, basePlan Plan) (newAcct Account, subscription Subscription, err error) {
	if account, err = CreateStripeCustomer(db, account); err != nil {
		logger.Error.Printf("Error creating customer record in stripe for account %s: %+v", account.Id.Hex(), err)
		return
	}
	if account.HasSubscription() {
		err = errors.New("Account already has a subscription. You should be changing the subscription.")
		return
	}
	if account.Subscription.IsBetaAccountSubscription() {
		account.Subscription.TrialStartDate = time.Now().UTC()
		account.Subscription.TrialEndDate = add30Days(account.Subscription.TrialStartDate)
	}
	logger.Info.Printf("Creating new subscription for account %s: %s", account.Id.Hex(), spew.Sdump(basePlan.StripePlanId))
	var stripeSubId *string
	if basePlan.BilledByStripe() {
		stripeSub, subErr := createStripeSubscription(*account.StripeId, basePlan, account.Subscription.TrialEndDate, account.Subscription.Coupon)
		if subErr != nil {
			err = subErr
			return
		}
		if stripeSub != nil {
			stripeSubId = &stripeSub.ID
		}
	}
	var accountPtr = &account
	newAcct = *accountPtr
	newAcct.Subscription.updatePlan(basePlan, stripeSubId)
	newAcct.Subscription.Coupon = nil
	newAcct.Version += 1
	err = UpdateAccount(db, account, newAcct)
	subscription = newAcct.Subscription
	return
}

func changeSubscription(db *mgo.Database, account Account, newBasePlan Plan) (newAcct Account, sub Subscription, err error) {
	if account, err = CreateStripeCustomer(db, account); err != nil {
		logger.Error.Printf("Error creating customer record in stripe for account %s: %+v", account.Id.Hex(), err)
		return
	}
	if !account.HasSubscription() {
		err = errors.New("Account doesn't already have a subscription. You should be adding the subscription.")
		return
	}
	logger.Info.Printf("Changing subscription for account %s: from %s to %s", account.Id.Hex(), spew.Sdump(account.Subscription.Plan.StripePlanId), spew.Sdump(newBasePlan.StripePlanId))
	stripeSub, subErr := updateStripeSubscriptionPlan(*account.Subscription.StripeSubscriptionId, *account.StripeId, newBasePlan, account.Subscription.TrialEndDate)
	if subErr != nil {
		err = subErr
		return
	}
	var stripeSubId *string
	if stripeSub != nil {
		stripeSubId = &stripeSub.ID
	}

	var accountPtr = &account
	newAcct = *accountPtr
	newAcct.Subscription.updatePlan(newBasePlan, stripeSubId)
	newAcct.Version += 1
	err = UpdateAccount(db, account, newAcct)
	sub = newAcct.Subscription
	return
}

func CancelSubscription(db *mgo.Database, account Account) (newAcct Account, err error) {
	if account, err = CreateStripeCustomer(db, account); err != nil {
		return
	}
	if !account.HasSubscription() {
		return newAcct, errors.New("Account doesn't already have a subscription. There is nothing to cancel.")
	}
	logger.Info.Printf("Canceling subscription for account %s: %s", account.Id.Hex(), spew.Sdump(account.Subscription.Plan.StripePlanId))
	stripeSub, subErr := getStripeSubscription(*account.Subscription.StripeSubscriptionId, *account.StripeId)
	if subErr != nil {
		err = subErr
		return
	}
	err = cancelStripeSubscription(*account.Subscription.StripeSubscriptionId, *account.StripeId)
	if err != nil {
		return
	}
	var accountPtr = &account
	newAcct = *accountPtr
	newAcct.Subscription.cancelSubscription(time.Unix(stripeSub.Canceled, 0), time.Unix(stripeSub.PeriodEnd, 0))
	newAcct.Version += 1
	err = UpdateAccount(db, account, newAcct)
	return
}

func PaymentAccepted(db *mgo.Database, account Account) (err error) {
	if !account.HasSubscription() {
		return errors.New(fmt.Sprintf("Account %s has no subscription, but payment was accepted", account.Id.Hex()))
	}
	var accountPtr = &account
	newAcct := *accountPtr
	newAcct.Subscription.PaymentAccepted()
	err = UpdateAccount(db, account, newAcct)
	return
}

func PaymentDenied(db *mgo.Database, account Account, deniedDate time.Time) (err error) {
	if !account.HasSubscription() {
		return errors.New(fmt.Sprintf("Account %s has no subscription, but payment was denied", account.Id.Hex()))
	}
	var accountPtr = &account
	newAcct := *accountPtr
	newAcct.Subscription.PaymentDenied(deniedDate)
	err = UpdateAccount(db, account, newAcct)
	return
}

func CreateStripeCustomer(db *mgo.Database, account Account) (newAcct Account, err error) {
	if account.StripeId != nil { // don't need to do anything
		return account, nil
	}
	var accountPtr = &account

	c, custErr := customer.New(&stripe.CustomerParams{
		Desc:  "Stripe account for " + account.Id.Hex(),
		Email: account.Owner().Email,
	})

	if custErr != nil {
		err = custErr
		return
	}

	newAcct = *accountPtr
	newAcct.StripeId = &c.ID

	updateErr := UpdateAccount(db, account, newAcct)

	if updateErr != nil {
		err = updateErr
		return
	}

	return
}

func createStripeSubscription(accountStripeId string, basePlan Plan, trialEndDate time.Time, coupon *string) (*stripe.Sub, error) {
	if !basePlan.BilledByStripe() {
		return nil, errors.New("Cannot create stripe subscription to a plan not billed by stripe")
	}
	params := stripe.SubParams{
		Customer: accountStripeId,
		Plan:     *basePlan.StripePlanId,
	}
	if coupon != nil {
		params.Coupon = *coupon
	}
	if trialEndDate.After(time.Now()) {
		params.TrialEnd = trialEndDate.Unix()
	} else {
		params.TrialEndNow = true
	}
	return sub.New(&params)
}

func getStripeSubscription(subscriptionStripeId, accountStripeId string) (*stripe.Sub, error) {
	return sub.Get(subscriptionStripeId, &stripe.SubParams{
		Customer: accountStripeId,
	})
}

func updateStripeSubscriptionPlan(subscriptionStripeId, accountStripeId string, newPlan Plan, trialEndDate time.Time) (*stripe.Sub, error) {
	if !newPlan.BilledByStripe() {
		logger.Info.Printf("account (%s) changing to plan (%s), but that plan is not billed by stripe. Canceling old stripe subscription (%s)", accountStripeId, newPlan.Id, subscriptionStripeId)
		stripeErr := cancelStripeSubscription(subscriptionStripeId, accountStripeId)
		return nil, stripeErr
	}
	// remove any discount:
	if stripeSub, subErr := getStripeSubscription(subscriptionStripeId, accountStripeId); subErr == nil {
		if stripeSub.Discount != nil && stripeSub.Discount.Coupon != nil {
			if stripeErr := discount.DelSub(accountStripeId, subscriptionStripeId); stripeErr != nil {
				logger.Error.Printf("Error removing discount while changing plans for account %s: %+v", accountStripeId, stripeErr)
				return nil, stripeErr
			}
		}
	}

	params := stripe.SubParams{
		Customer: accountStripeId,
		Plan:     *newPlan.StripePlanId,
	}
	if trialEndDate.After(time.Now()) {
		params.TrialEnd = trialEndDate.Unix()
	}
	return sub.Update(subscriptionStripeId, &params)
}

func cancelStripeSubscription(subscriptionStripeId, accountStripeId string) error {
	return sub.Cancel(subscriptionStripeId, &stripe.SubParams{
		Customer:  accountStripeId,
		EndCancel: true,
	})
}

// date ranges are defined as: [startDate, endDate], unless endDate is nil, then [startDate, +∞)
func nowInRange(now, startDate time.Time, endDate *time.Time) bool {
	onOrAfterStart := startDate.Equal(now) || startDate.Before(now)
	onOrBeforeEnd := endDate == nil || now.Equal(*endDate) || now.Before(*endDate)
	return onOrAfterStart && onOrBeforeEnd
}

////
// Data Access
////

func publiclyAvailableQuery() bson.D {
	now := time.Now()
	return bson.D{
		{"publiclyAvailable", true},
		{"activeDate", bson.M{"$lt": now}},
		{"$or", []bson.M{
			{"inactiveDate": bson.M{"$gt": now}},
			{"inactiveDate": bson.M{"$exists": false}},
		}}}
}

func GetPubliclyAvailablePlans(db *mgo.Database) (ps []Plan, err error) {
	err = billingPlans(db).Find(publiclyAvailableQuery()).All(&ps)
	return
}

func GetAllPlans(db *mgo.Database) (ps []Plan, err error) {
	err = billingPlans(db).Find(bson.D{}).All(&ps)
	return
}

func GetPlanById(db *mgo.Database, id bson.ObjectId) (p Plan, err error) {
	err = billingPlans(db).Find(bson.D{
		{"_id", id},
	}).One(&p)
	return
}

func GetActivePlanById(db *mgo.Database, id bson.ObjectId) (p Plan, err error) {
	query := append(bson.D{{"_id", id}}, publiclyAvailableQuery()...)
	err = billingPlans(db).Find(query).One(&p)
	return
}

func CreatePlan(db *mgo.Database, name string, version int, monthlyPrice *uint64, addToStripe, publiclyAvailable bool, activeDate, inactiveDate *time.Time) (p Plan, err error) {
	var stripeId *string
	now := time.Now().UTC()
	if addToStripe {
		id := bson.NewObjectId()
		statementDescriptor := fmt.Sprintf("LaunchDarkly %s", name)
		if len(statementDescriptor) > 22 {
			statementDescriptor = statementDescriptor[:22]
		}
		stripePlan, stripeErr := plan.New(&stripe.PlanParams{
			ID:            id.Hex(),
			Name:          fmt.Sprintf("%s (v%d)", name, version),
			Currency:      "usd",
			Amount:        *monthlyPrice,
			Interval:      "month",
			IntervalCount: 1,
			TrialPeriod:   0,
			Statement:     statementDescriptor,
		})
		if stripeErr != nil {
			err = stripeErr
			return
		}
		stripeId = &stripePlan.ID
	}

	if activeDate == nil {
		activeDate = &now
	}
	p = Plan{
		PlanRef: PlanRef{
			Id:                bson.NewObjectId(),
			Name:              name,
			Version:           version,
			StripePlanId:      stripeId,
			PubliclyAvailable: publiclyAvailable,
		},
		ActiveDate:   *activeDate,
		InactiveDate: inactiveDate,
		CreatedDate:  now,
	}
	mgoErr := billingPlans(db).Insert(&p)
	if mgoErr != nil {
		err = mgoErr
		return
	}
	return
}

func billingPlans(db *mgo.Database) *mgo.Collection {
	return db.C("billingPlans")
}

func add30Days(d time.Time) time.Time {
	return d.AddDate(0, 0, 30)
}

func billingPlanIndices(db *mgo.Database) (err error) {
	err = billingPlans(db).EnsureIndex(mgo.Index{
		Key:        []string{"name", "version"},
		Unique:     true,
		DropDups:   false,
		Background: true,
		Sparse:     false,
	})

	if err != nil {
		return
	}
	err = billingPlans(db).EnsureIndex(mgo.Index{
		Key:        []string{"publiclyAvailable", "activeDate", "inactiveDate"},
		Unique:     false,
		DropDups:   false,
		Background: true,
		Sparse:     false,
	})

	if err != nil {
		return
	}

	return
}

func InitializeBilling(apiKey string) {
	stripe.Key = apiKey
	stripe.LogLevel = 1
}
