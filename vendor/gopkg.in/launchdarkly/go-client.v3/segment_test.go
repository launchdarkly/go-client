package ldclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	max_weight = 100000
	min_weight = 0
)

func TestExplicitIncludeUser(t *testing.T) {
	segment := Segment{
		Key:      "test",
		Included: []string{"foo"},
		Excluded: nil,
		Salt:     "abcdef",
		Rules:    nil,
		Version:  1,
		Deleted:  false,
	}

	userKey := "foo"

	user := User{
		Key: &userKey,
	}

	containsUser, reason := segment.ContainsUser(user)

	assert.True(t, containsUser, "Segment %+v should contain user %+v", segment, user)
	assert.NotNil(t, reason, "Reason should not be nil")
	assert.Equal(t, "included", reason.Kind, "Reason should be 'included'")
}

func TestExplicitExcludeUser(t *testing.T) {
	segment := Segment{
		Key:      "test",
		Included: nil,
		Excluded: []string{"foo"},
		Salt:     "abcdef",
		Rules:    nil,
		Version:  1,
		Deleted:  false,
	}

	userKey := "foo"

	user := User{
		Key: &userKey,
	}

	containsUser, reason := segment.ContainsUser(user)

	assert.False(t, containsUser, "Segment %+v should not contain user %+v", segment, user)
	assert.NotNil(t, reason, "Reason should not be nil")
	assert.Equal(t, "excluded", reason.Kind, "Reason should be 'excluded'")
}

func TestExplicitIncludeHasPrecedence(t *testing.T) {
	segment := Segment{
		Key:      "test",
		Included: []string{"foo"},
		Excluded: []string{"foo"},
		Salt:     "abcdef",
		Rules:    nil,
		Version:  1,
		Deleted:  false,
	}

	userKey := "foo"

	user := User{
		Key: &userKey,
	}

	containsUser, reason := segment.ContainsUser(user)

	assert.True(t, containsUser, "Segment %+v should contain user %+v", segment, user)
	assert.NotNil(t, reason, "Reason should not be nil")
	assert.Equal(t, "included", reason.Kind, "Reason should be 'included'")
}

func TestMatchingRuleWithFullRollout(t *testing.T) {
	rules := []SegmentRule{
		SegmentRule{
			Clauses: []Clause{Clause{
				Attribute: "email",
				Op:        OperatorIn,
				Values:    []interface{}{"test@example.com"},
				Negate:    false,
			}},
			Weight:   &max_weight,
			BucketBy: nil,
		},
	}

	segment := Segment{
		Key:      "test",
		Included: nil,
		Excluded: nil,
		Salt:     "abcdef",
		Rules:    rules,
		Version:  1,
		Deleted:  false,
	}

	userKey := "foo"
	userEmail := "test@example.com"

	user := User{
		Key:   &userKey,
		Email: &userEmail,
	}

	containsUser, reason := segment.ContainsUser(user)
	assert.True(t, containsUser, "Segment %+v should contain user %+v", segment, user)
	assert.NotNil(t, reason, "Reason should not be nil")
	assert.Equal(t, "rule", reason.Kind, "Reason should be 'rule'")
	assert.NotNil(t, reason.MatchedRule, "Matched rule should not be nil")
	assert.True(t, assert.ObjectsAreEqual(rules[0], *reason.MatchedRule), "Reason rule should match but %+v and %+v are not equal", rules[0], *reason.MatchedRule)
}

func TestMatchingRuleWithZeroRollout(t *testing.T) {
	rules := []SegmentRule{
		SegmentRule{
			Clauses: []Clause{Clause{
				Attribute: "email",
				Op:        OperatorIn,
				Values:    []interface{}{"test@example.com"},
				Negate:    false,
			}},
			Weight:   &min_weight,
			BucketBy: nil,
		},
	}

	segment := Segment{
		Key:      "test",
		Included: nil,
		Excluded: nil,
		Salt:     "abcdef",
		Rules:    rules,
		Version:  1,
		Deleted:  false,
	}

	userKey := "foo"
	userEmail := "test@example.com"

	user := User{
		Key:   &userKey,
		Email: &userEmail,
	}

	containsUser, reason := segment.ContainsUser(user)
	assert.False(t, containsUser, "Segment %+v should not contain user %+v", segment, user)
	assert.Nil(t, reason, "Reason should be nil")
}

func TestMatchingRuleWithMultipleClauses(t *testing.T) {
	rules := []SegmentRule{
		SegmentRule{
			Clauses: []Clause{Clause{
				Attribute: "email",
				Op:        OperatorIn,
				Values:    []interface{}{"test@example.com"},
				Negate:    false,
			},
				Clause{
					Attribute: "name",
					Op:        OperatorIn,
					Values:    []interface{}{"bob"},
				},
			},
			Weight:   nil,
			BucketBy: nil,
		},
	}

	segment := Segment{
		Key:      "test",
		Included: nil,
		Excluded: nil,
		Salt:     "abcdef",
		Rules:    rules,
		Version:  1,
		Deleted:  false,
	}

	userKey := "foo"
	userEmail := "test@example.com"
	userName := "bob"

	user := User{
		Key:   &userKey,
		Email: &userEmail,
		Name:  &userName,
	}

	containsUser, reason := segment.ContainsUser(user)
	assert.True(t, containsUser, "Segment %+v should contain user %+v", segment, user)
	assert.NotNil(t, reason, "Reason should not be nil")
	assert.Equal(t, "rule", reason.Kind, "Reason should be 'rule'")
	assert.NotNil(t, reason.MatchedRule, "Matched rule should not be nil")
	assert.True(t, assert.ObjectsAreEqual(rules[0], *reason.MatchedRule), "Reason rule should match but %+v and %+v are not equal", rules[0], *reason.MatchedRule)
}

func TestNonMatchingRuleWithMultipleClauses(t *testing.T) {
	rules := []SegmentRule{
		SegmentRule{
			Clauses: []Clause{Clause{
				Attribute: "email",
				Op:        OperatorIn,
				Values:    []interface{}{"test@example.com"},
				Negate:    false,
			},
				Clause{
					Attribute: "name",
					Op:        OperatorIn,
					Values:    []interface{}{"bill"},
				},
			},
			Weight:   nil,
			BucketBy: nil,
		},
	}

	segment := Segment{
		Key:      "test",
		Included: nil,
		Excluded: nil,
		Salt:     "abcdef",
		Rules:    rules,
		Version:  1,
		Deleted:  false,
	}

	userKey := "foo"
	userEmail := "test@example.com"
	userName := "bob"

	user := User{
		Key:   &userKey,
		Email: &userEmail,
		Name:  &userName,
	}

	containsUser, reason := segment.ContainsUser(user)
	assert.False(t, containsUser, "Segment %+v should not contain user %+v", segment, user)
	assert.Nil(t, reason, "Reason should be nil")
}
