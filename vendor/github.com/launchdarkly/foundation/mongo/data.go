package mongo

import (
	"fmt"

	"gopkg.in/mgo.v2/bson"
)

// Given any object, get a document that can be used in an upsert command to merge the fields in the given
// object with those existing
func CreateUpdateMergeDocument(doc interface{}) (map[string]interface{}, error) {
	// do bson (de)serialization dance to ensure we get keys that use the bson field names
	bsondata, err := bson.Marshal(doc)
	if err != nil {
		return nil, err
	}
	mapdata := make(map[string]interface{})
	err = bson.Unmarshal(bsondata, mapdata)
	if err != nil {
		return nil, err
	}

	fields := make(map[string]interface{})
	flatten(mapdata, &fields, "")
	ret := map[string]interface{}{
		"$set": fields,
	}
	return ret, nil
}

func flatten(input interface{}, output *map[string]interface{}, parent string) {
	switch input := input.(type) {
	default:
		(*output)[parent] = input
		return
	case map[string]interface{}:
		for k, v := range input {
			var newKey string
			if parent == "" {
				newKey = k
			} else {
				newKey = fmt.Sprintf("%s.%s", parent, k)
			}
			flatten(v, output, newKey)
		}
	}
	return
}
