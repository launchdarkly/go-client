package accounts

import (
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"

	"github.com/launchdarkly/foundation/accounts/roles"
)

func TestAccountBillingContactWithFullContactDefined(t *testing.T) {
	acc := Account{
		Members: []Member{CreateMember("owner@example.com", "password", strPtr("Org"), strPtr("Owner"), roles.OwnerRole, nil)},
		BillingContact: BillingContact{
			Name:       strPtr("Billing Department"),
			Email:      strPtr("billing@example.com"),
			Address1:   strPtr("123 Sesame St"),
			City:       strPtr("NewYork"),
			State:      strPtr("NY"),
			PostalCode: strPtr("01111"),
			PoNumber:   strPtr("qweasd123"),
		},
	}

	expected := BillingContact{
		Name:       strPtr("Billing Department"),
		Email:      strPtr("billing@example.com"),
		Address1:   strPtr("123 Sesame St"),
		City:       strPtr("NewYork"),
		State:      strPtr("NY"),
		PostalCode: strPtr("01111"),
		PoNumber:   strPtr("qweasd123"),
	}

	actual := acc.GetBillingContact()
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Got unexpected billing contact. expected: %s, got %s", spew.Sdump(expected), spew.Sdump(actual))
	}
}

func TestAccountBillingContactWithPartialContactDefined(t *testing.T) {
	acc := Account{
		Members: []Member{CreateMember("owner@example.com", "password", strPtr("Org"), strPtr("Owner"), roles.OwnerRole, nil)},
		BillingContact: BillingContact{
			Name: strPtr("Billing Department"),
		},
	}

	expected := BillingContact{
		Name:  strPtr("Billing Department"),
		Email: strPtr("owner@example.com"),
	}

	actual := acc.GetBillingContact()
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Got unexpected billing contact. expected: %s, got %s", spew.Sdump(expected), spew.Sdump(actual))
	}
}

func TestAccountBillingContactWithOwnerContactDefined(t *testing.T) {
	acc := Account{
		Members: []Member{CreateMember("owner@example.com", "password", strPtr("Org"), strPtr("Owner"), roles.OwnerRole, nil)},
	}

	expected := BillingContact{
		Name:  strPtr("Org Owner"),
		Email: strPtr("owner@example.com"),
	}

	actual := acc.GetBillingContact()
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Got unexpected billing contact. expected: %s, got %s", spew.Sdump(expected), spew.Sdump(actual))
	}
}

func TestAccountBillingContactWithNoContactDefined(t *testing.T) {
	acc := Account{
		Members: []Member{CreateMember("owner@example.com", "password", nil, nil, roles.OwnerRole, nil)},
	}

	expected := BillingContact{
		Name:  strPtr("owner@example.com"),
		Email: strPtr("owner@example.com"),
	}

	actual := acc.GetBillingContact()
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Got unexpected billing contact. expected: %s, got %s", spew.Sdump(expected), spew.Sdump(actual))
	}
}

func TestMemberOnboardingWithNilValue(t *testing.T) {
	member := CreateMember("owner@example.com", "password", nil, nil, roles.OwnerRole, nil)
	member.Onboarding = nil
	isOnboarded := member.IsOnboarded()

	if !isOnboarded {
		t.Errorf("Expected isOnboarded() to be true when onboarding does not exist")
	}
}

func TestMemberOnboardingNilOnboardedValue(t *testing.T) {
	member := CreateMember("owner@example.com", "password", nil, nil, roles.OwnerRole, nil)
	member.Onboarding = &Onboarding{}
	isOnboarded := member.IsOnboarded()

	if !isOnboarded {
		t.Errorf("Expected isOnboarded() to be true when onboarding exists but no value is set")
	}
}

func TestMemberOnboardingWithFalse(t *testing.T) {
	member := CreateMember("owner@example.com", "password", nil, nil, roles.OwnerRole, nil)
	expected := false
	member.Onboarding = &Onboarding{Onboarded: &expected}
	isOnboarded := member.IsOnboarded()

	if isOnboarded {
		t.Errorf("Expected isOnboarded() to be false when onboarding exists with false value")
	}
}

func TestMemberOnboardingWithTrue(t *testing.T) {
	member := CreateMember("owner@example.com", "password", nil, nil, roles.OwnerRole, nil)
	expected := true
	member.Onboarding = &Onboarding{Onboarded: &expected}
	isOnboarded := member.IsOnboarded()

	if !isOnboarded {
		t.Errorf("Expected isOnboarded() to be true when onboarding exists with true value")
	}
}

func TestFindDuplicateStrings(t *testing.T) {
	assert.Equal(t, []string{"a", "c"}, findDuplicateStrings([]string{"a", "a", "a", "b", "c", "c"}))
}

func TestUniqueStrings(t *testing.T) {
	assert.Equal(t, []string{"a", "b"}, uniqueStrings([]string{"a", "a", "a", "b"}))
}

func strPtr(s string) *string {
	return &s
}
