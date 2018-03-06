package freflect

import (
	"reflect"
)

func IsPointer(i interface{}) bool {
	return reflect.TypeOf(i).Kind() == reflect.Ptr
}

func MaybeDeref(i interface{}) interface{} {
	if IsPointer(i) {
		return reflect.ValueOf(i).Elem().Interface()
	} else {
		return i
	}
}

// A utility wrapper for checking deep object equality across two objects
// whose static types are interfaces. The issue is that we may want to treat
// pointers to values and values as equivalent when we're dealing with structs:
//
// http://play.golang.org/p/ciRRWaxgTx
//
// Note : this utility will not recursively dereference interface types
// So structs with nested interface types may be structurally equal, but
// DeepEqualIface may fail
func DeepEqualIface(a1, a2 interface{}) bool {
	return reflect.DeepEqual(MaybeDeref(a1), MaybeDeref(a2))
}
