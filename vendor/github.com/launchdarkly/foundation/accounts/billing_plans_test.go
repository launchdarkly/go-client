package accounts

import (
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	stripe "github.com/stripe/stripe-go"
)

var testPlanId = "beta+jko.test@example.com"
var goldPlan = Plan{
	PlanRef: PlanRef{
		Name:              "Gold",
		Version:           42,
		StripePlanId:      &testPlanId,
		PubliclyAvailable: true,
	},
	ActiveDate:   time.Now().AddDate(0, 0, -1),
	InactiveDate: nil,
	CreatedDate:  time.Now(),
}
var now = time.Now()
var oneHourAgo = now.Add(-1 * time.Hour)
var yesterday = now.AddDate(0, 0, -1)
var tomorrow = now.AddDate(0, 0, 1)
var twoDaysFromNow = now.AddDate(0, 0, 2)
var twoDaysAgo = now.Add(-48 * time.Hour)
var thirtyDaysFromNow = now.Add(30 * 24 * time.Hour)
var thirtyOneDaysAgo = now.Add(-31 * 24 * time.Hour)
var thirtyOneDaysFromNow = now.Add(31 * 24 * time.Hour)
var sixtyOneDaysAgo = now.Add(61 * 24 * time.Hour)

func TestDateRangeInRange(t *testing.T) {
	startDate := yesterday
	endDate := tomorrow
	if !nowInRange(now, startDate, &endDate) {
		t.Errorf("now not in range, but should be. now: %s, start date: %s, end date: %s", now, startDate, endDate)
	}
}

func TestDateRangeBeforeRange(t *testing.T) {
	startDate := tomorrow
	endDate := twoDaysFromNow
	if nowInRange(now, tomorrow, &twoDaysFromNow) {
		t.Errorf("now in range, but should not be. now: %s, start date: %s, end date: %s", now, startDate, endDate)
	}
}

func TestDateRangeAfterRange(t *testing.T) {
	startDate := twoDaysAgo
	endDate := yesterday
	if nowInRange(now, startDate, &endDate) {
		t.Errorf("now in range, but should not be. now: %s, start date: %s, end date: %s", now, startDate, endDate)
	}
}

func TestDateRangeInRangeAtStartDate(t *testing.T) {
	startDate := now
	endDate := tomorrow
	if !nowInRange(now, startDate, &endDate) {
		t.Errorf("now not in range, but should be. now: %s, start date: %s, end date: %s", now, startDate, endDate)
	}
}

func TestDateRangeInRangeAtEndDate(t *testing.T) {
	startDate := yesterday
	endDate := now
	if !nowInRange(now, startDate, &endDate) {
		t.Errorf("now not in range, but should be. now: %s, start date: %s, end date: %s", now, startDate, endDate)
	}
}

func TestDateRangeInOpenRange(t *testing.T) {
	startDate := yesterday
	if !nowInRange(now, startDate, nil) {
		t.Errorf("now not in range, but should be. now: %s, start date: %s, end date: %v", now, startDate, nil)
	}
}

func TestPlanIsActiveInDateRange(t *testing.T) {
	plan := Plan{
		PlanRef: PlanRef{
			PubliclyAvailable: true,
		},
		ActiveDate:   yesterday,
		InactiveDate: &tomorrow,
	}
	assertIsActive(plan, t)
}

func TestPlanIsActiveInOpenDateRange(t *testing.T) {
	plan := Plan{
		PlanRef: PlanRef{
			PubliclyAvailable: true,
		},
		ActiveDate: yesterday,
	}
	assertIsActive(plan, t)
}

func TestNonPublicPlanIsNotActiveInOpenDateRange(t *testing.T) {
	plan := Plan{
		PlanRef: PlanRef{
			PubliclyAvailable: false,
		},
		ActiveDate: yesterday,
	}
	assertIsNotActive(plan, t)
}

func TestBetaAccountSubscription(t *testing.T) {
	plan := Subscription{}
	assertIsBetaAccount(plan, t)
}

func TestNonBetaAccountSubscription(t *testing.T) {
	plan := Subscription{
		TrialStartDate: now,
		TrialEndDate:   thirtyDaysFromNow,
	}
	assertIsNotBetaAccount(plan, t)
}

func TestSubscriptionInNewTrial(t *testing.T) {
	plan := Subscription{
		TrialStartDate: now,
		TrialEndDate:   thirtyDaysFromNow,
	}
	assertInTrial(plan, t)
}

func TestTrialJustLapsedInGracePeriod(t *testing.T) {
	plan := Subscription{
		TrialStartDate: thirtyOneDaysAgo,
		TrialEndDate:   add30Days(thirtyOneDaysAgo),
	}
	assertInTrial(plan, t)
	assertInGracePeriod(plan, t)
	assertNotLapsed(plan, t)
}

func TestTrialLongAgoLapsedNotInGracePeriod(t *testing.T) {
	plan := Subscription{
		TrialStartDate: sixtyOneDaysAgo,
		TrialEndDate:   add30Days(sixtyOneDaysAgo),
	}
	assertNotInTrial(plan, t)
	assertNotInGracePeriod(plan, t)
	assertLapsed(plan, t)
}

func TestSubscriptionInNewGrace(t *testing.T) {
	plan := Subscription{
		GraceStartDate: &now,
		GraceEndDate:   &thirtyDaysFromNow,
	}
	assertInGracePeriod(plan, t)
}

func TestSubscriptionBeforeGrace(t *testing.T) {
	plan := Subscription{}
	assertNotInGracePeriod(plan, t)
}

func TestSubscriptionAfterGrace(t *testing.T) {
	plan := Subscription{
		GraceStartDate: &thirtyOneDaysAgo,
		GraceEndDate:   &yesterday,
	}
	assertNotInGracePeriod(plan, t)
}

func TestSubscriptionAfterGraceLapsed(t *testing.T) {
	plan := Subscription{
		TrialStartDate: thirtyOneDaysAgo,
		TrialEndDate:   thirtyOneDaysAgo,
		GraceStartDate: &thirtyOneDaysAgo,
		GraceEndDate:   &yesterday,
	}
	assertLapsed(plan, t)
}

func TestSubscriptionBeforeGraceNotLapsed(t *testing.T) {
	plan := Subscription{
		Plan:           &PlanRef{},
		TrialStartDate: thirtyOneDaysAgo,
		TrialEndDate:   yesterday,
		GraceStartDate: &yesterday,
		GraceEndDate:   &thirtyOneDaysFromNow,
	}
	assertNotLapsed(plan, t)
}

func TestRecentNewAccountInTrial(t *testing.T) {
	plan := Subscription{
		TrialStartDate: oneHourAgo,
		TrialEndDate:   add30Days(oneHourAgo),
	}
	stripeId := stripeSubWithTrialStart(oneHourAgo).ID
	plan.updatePlan(goldPlan, &stripeId)
	assertInTrial(plan, t)
}

func TestOldNewAccountNotInTrial(t *testing.T) {
	plan := Subscription{
		TrialStartDate: thirtyOneDaysAgo,
		TrialEndDate:   add30Days(thirtyOneDaysAgo),
	}
	stripeId := stripeSubWithTrialStart(thirtyOneDaysAgo).ID
	plan.updatePlan(goldPlan, &stripeId)
	assertNotInTrial(plan, t)
}

func TestPaymentDeniedInGracePeriod(t *testing.T) {
	plan := Subscription{
		TrialStartDate: thirtyOneDaysAgo,
		TrialEndDate:   add30Days(thirtyOneDaysAgo),
	}
	stripeId := stripeSubWithTrialStart(thirtyOneDaysAgo).ID
	plan.updatePlan(goldPlan, &stripeId)
	plan.PaymentDenied(oneHourAgo)
	assertInGracePeriod(plan, t)
}

func TestVeryOldPaymentDeniedLapsed(t *testing.T) {
	plan := Subscription{
		TrialStartDate: sixtyOneDaysAgo,
		TrialEndDate:   add30Days(sixtyOneDaysAgo),
	}
	stripeId := stripeSubWithTrialStart(sixtyOneDaysAgo).ID
	plan.updatePlan(goldPlan, &stripeId)
	plan.PaymentDenied(thirtyOneDaysAgo)
	assertLapsed(plan, t)
}

func TestRetriedPaymentDeniedDoesNotExtendGracePeriod(t *testing.T) {
	plan := Subscription{
		TrialStartDate: sixtyOneDaysAgo,
		TrialEndDate:   add30Days(sixtyOneDaysAgo),
	}
	stripeId := stripeSubWithTrialStart(sixtyOneDaysAgo).ID
	plan.updatePlan(goldPlan, &stripeId)
	plan.PaymentDenied(thirtyOneDaysAgo)
	plan.PaymentDenied(oneHourAgo)
	assertLapsed(plan, t)
}

func TestAccountInGracePeriodAcceptedPayment(t *testing.T) {
	plan := Subscription{
		TrialStartDate: thirtyOneDaysAgo,
		TrialEndDate:   add30Days(thirtyOneDaysAgo),
	}
	stripeId := stripeSubWithTrialStart(thirtyOneDaysAgo).ID
	plan.updatePlan(goldPlan, &stripeId)
	plan.PaymentDenied(oneHourAgo)
	plan.PaymentAccepted()
	assertNotInGracePeriod(plan, t)
}

func TestLapsedAccountAcceptedPaymentNotInGracePeriod(t *testing.T) {
	plan := Subscription{
		TrialStartDate: sixtyOneDaysAgo,
		TrialEndDate:   add30Days(sixtyOneDaysAgo),
	}
	stripeId := stripeSubWithTrialStart(sixtyOneDaysAgo).ID
	plan.updatePlan(goldPlan, &stripeId)
	plan.PaymentDenied(thirtyOneDaysAgo)
	plan.PaymentAccepted()
	assertNotInGracePeriod(plan, t)
}

func TestLapsedAccountAcceptedPaymentNotLapsed(t *testing.T) {
	plan := Subscription{
		TrialStartDate: sixtyOneDaysAgo,
		TrialEndDate:   add30Days(sixtyOneDaysAgo),
	}
	stripeId := stripeSubWithTrialStart(sixtyOneDaysAgo).ID
	plan.updatePlan(goldPlan, &stripeId)
	plan.PaymentDenied(thirtyOneDaysAgo)
	plan.PaymentAccepted()
	assertNotLapsed(plan, t)
}

func TestCanceledAccountInGracePeriodUntilBillingPeriodEnd(t *testing.T) {
	plan := Subscription{
		TrialStartDate: sixtyOneDaysAgo,
		TrialEndDate:   add30Days(sixtyOneDaysAgo),
	}
	stripeId := stripeSubWithTrialStart(sixtyOneDaysAgo).ID
	plan.updatePlan(goldPlan, &stripeId)
	plan.cancelSubscription(yesterday, tomorrow)
	assertInGracePeriod(plan, t)
}

func TestCanceledAccountNotLapsedUntilBillingPeriodEnd(t *testing.T) {
	plan := Subscription{
		TrialStartDate: sixtyOneDaysAgo,
		TrialEndDate:   add30Days(sixtyOneDaysAgo),
	}
	stripeId := stripeSubWithTrialStart(sixtyOneDaysAgo).ID
	plan.updatePlan(goldPlan, &stripeId)
	plan.cancelSubscription(yesterday, tomorrow)
	assertNotLapsed(plan, t)
}

func TestCanceledAccountLapsedAfterBillingPeriodEnd(t *testing.T) {
	plan := Subscription{
		TrialStartDate: sixtyOneDaysAgo,
		TrialEndDate:   add30Days(sixtyOneDaysAgo),
	}
	stripeId := stripeSubWithTrialStart(sixtyOneDaysAgo).ID
	plan.updatePlan(goldPlan, &stripeId)
	plan.cancelSubscription(thirtyOneDaysAgo, yesterday)
	assertLapsed(plan, t)
}

func TestResubscribeAfterCancelIsNotCanceled(t *testing.T) {
	plan := Subscription{
		TrialStartDate: sixtyOneDaysAgo,
		TrialEndDate:   add30Days(sixtyOneDaysAgo),
	}
	stripeId := stripeSubWithTrialStart(sixtyOneDaysAgo).ID
	plan.updatePlan(goldPlan, &stripeId)
	plan.cancelSubscription(thirtyOneDaysAgo, yesterday)
	plan.updatePlan(goldPlan, &stripeId)
	assertIsNotCanceled(plan, t)
}

func stripeSubWithTrialStart(trialStart time.Time) stripe.Sub {
	return stripe.Sub{
		ID:         "sub_64BAR6nCfQ7fqF",
		TrialStart: trialStart.Unix(),
		TrialEnd:   trialStart.AddDate(0, 0, 30).Unix(),
	}
}

////
// assertion helpers:
////
func assertIsBetaAccount(sub Subscription, t *testing.T) {
	if !sub.IsBetaAccountSubscription() {
		t.Errorf("plan is not considered a beta account, but it should be: trial start date: %s, trial end date: %s", sub.TrialStartDate, sub.TrialEndDate)
	}
}

func assertIsNotBetaAccount(sub Subscription, t *testing.T) {
	if sub.IsBetaAccountSubscription() {
		t.Errorf("plan is considered a beta account, but it not should be: trial start date: %s, trial end date: %s", sub.TrialStartDate, sub.TrialEndDate)
	}
}

func assertLapsed(sub Subscription, t *testing.T) {
	if !sub.IsLapsed() {
		t.Errorf("plan is in not lapsed, but it should be. now: %s, grace period start date: %s, grace period end date: %s", now, sub.GraceStartDate, sub.GraceEndDate)
	}
}

func assertLapsedTrial(sub Subscription, t *testing.T) {
	if !sub.IsLapsedTrialWithNoPlan() {
		t.Errorf("plan is in not lapsed trial, but it should be. now: %s, grace period start date: %s, grace period end date: %s", now, sub.GraceStartDate, sub.GraceEndDate)
	}
}

func assertNotLapsed(sub Subscription, t *testing.T) {
	if sub.IsLapsed() {
		t.Errorf("plan is lapsed, but it should not be. now: %s, grace period start date: %s, grace period end date: %s", now, sub.GraceStartDate, sub.GraceEndDate)
	}
}

func assertInGracePeriod(sub Subscription, t *testing.T) {
	if !sub.InGracePeriod() {
		t.Errorf("plan is not in grace period, but it should be. now: %s, grace period start date: %s, grace period end date: %s", now, sub.GraceStartDate, sub.GraceEndDate)
	}
}

func assertNotInGracePeriod(sub Subscription, t *testing.T) {
	if sub.InGracePeriod() {
		t.Errorf("plan is in grace period, but it should not be. now: %s, grace period start date: %s, grace period end date: %s", now, sub.GraceStartDate, sub.GraceEndDate)
	}
}

func assertInTrial(sub Subscription, t *testing.T) {
	if !sub.InTrial() {
		t.Errorf("plan is not in trial, but it should be. now: %s, trial start date: %s, trial end date: %s", now, sub.TrialStartDate, sub.TrialEndDate)
	}
}
func assertNotInTrial(sub Subscription, t *testing.T) {
	if sub.InTrial() {
		t.Errorf("plan is in trial, but it not should be. now: %s, trial start date: %s, trial end date: %s", now, sub.TrialStartDate, sub.TrialEndDate)
	}
}

func assertIsActive(plan Plan, t *testing.T) {
	if !plan.IsActive() {
		t.Errorf("plan is not active, but it should be. now: %s, active date: %s, inactive date: %s, publicly available: %t", now, plan.ActiveDate, plan.InactiveDate, plan.PubliclyAvailable)
	}
}

func assertIsNotActive(plan Plan, t *testing.T) {
	if plan.IsActive() {
		t.Errorf("plan is active, but it should not be. now: %s, active date: %s, inactive date: %s, publicly available: %t", now, plan.ActiveDate, plan.InactiveDate, plan.PubliclyAvailable)
	}
}

func assertIsNotCanceled(sub Subscription, t *testing.T) {
	if sub.IsCanceled() {
		t.Errorf("subscription is canceled, but it should not be: %s", spew.Sdump(sub))
	}
}
