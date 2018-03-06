package mongo

import (
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"gopkg.in/mgo.v2/bson"
)

func TestFlattenNumber(t *testing.T) {
	input := 42
	output := make(map[string]interface{})
	expected := map[string]interface{}{
		"foo": 42,
	}
	flatten(input, &output, "foo")
	assertEqual(t, expected, output)
}

func TestFlattenBool(t *testing.T) {
	input := true
	output := make(map[string]interface{})
	expected := map[string]interface{}{
		"foo": true,
	}
	flatten(input, &output, "foo")
	assertEqual(t, expected, output)
}

func TestFlattenString(t *testing.T) {
	input := "bar"
	output := make(map[string]interface{})
	expected := map[string]interface{}{
		"foo": "bar",
	}
	flatten(input, &output, "foo")
	assertEqual(t, expected, output)
}

func TestFlattenFlatMap(t *testing.T) {
	input := map[string]interface{}{
		"foo":  "bar",
		"num":  33,
		"bool": false,
	}
	output := make(map[string]interface{})
	flatten(input, &output, "")
	assertEqual(t, input, output)
}

func TestFlattenNestedMap(t *testing.T) {
	input := map[string]interface{}{
		"foo":  "bar",
		"num":  33,
		"bool": false,
		"map": map[string]interface{}{
			"biz": "baz",
			"hot": "dog",
		},
	}
	expected := map[string]interface{}{
		"foo":     "bar",
		"num":     33,
		"bool":    false,
		"map.biz": "baz",
		"map.hot": "dog",
	}
	output := make(map[string]interface{})
	flatten(input, &output, "")
	assertEqual(t, expected, output)
}

func TestFlattenNestedList(t *testing.T) {
	input := map[string]interface{}{
		"foo":  "bar",
		"num":  33,
		"bool": false,
		"list": []interface{}{
			"biz",
			"baz",
			"hot",
			"dog",
		},
	}
	expected := map[string]interface{}{
		"foo":  "bar",
		"num":  33,
		"bool": false,
		"list": []interface{}{
			"biz",
			"baz",
			"hot",
			"dog",
		},
	}
	output := make(map[string]interface{})
	flatten(input, &output, "")
	assertEqual(t, expected, output)
}

func TestCreateUpdateMergeDocument(t *testing.T) {
	type testInput struct {
		Foo          string
		Id           bson.ObjectId
		Number       int  `bson:"num"`
		Toggle       bool `bson:"bool"`
		List         []string
		Attributes   map[string]string `bson:"map"`
		MaybeMissing string            `bson:",omitempty"`
	}
	input := testInput{
		Foo:    "bar",
		Id:     bson.ObjectIdHex("569f514183f2164430000004"),
		Number: 33,
		Toggle: false,
		List: []string{
			"biz",
			"baz",
			"hot",
			"dog",
		},
		Attributes: map[string]string{
			"biz": "baz",
			"hot": "dog",
		},
	}

	expected := map[string]interface{}{
		"$set": map[string]interface{}{
			"foo":  "bar",
			"id":   bson.ObjectIdHex("569f514183f2164430000004"),
			"num":  33,
			"bool": false,
			"list": []interface{}{
				"biz",
				"baz",
				"hot",
				"dog",
			},
			"map.biz": "baz",
			"map.hot": "dog",
		},
	}

	output, _ := CreateUpdateMergeDocument(input)
	assertEqual(t, expected, output)
}

func assertEqual(t *testing.T, expected, actual interface{}) {
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected: %s, but got: %s", spew.Sdump(expected), spew.Sdump(actual))
	}
}
