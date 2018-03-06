package deepcopy

import (
	"reflect"

	"fmt"

	dc "github.com/mohae/deepcopy"
)

func DeepCopy(dst interface{}, src interface{}) error {
	var (
		srcValue = reflect.ValueOf(src)
		dstValue = reflect.ValueOf(dst)
	)

	if src == nil {
		return fmt.Errorf("src is nil")
	}

	if srcValue.Type().Kind() != reflect.Ptr {
		return fmt.Errorf("src must be a pointer but got %s", srcValue.Type())
	}

	from := srcValue.Elem()
	if !from.IsValid() {
		return fmt.Errorf("src (%s) is not defined", srcValue)
	}

	if dst == nil {
		return fmt.Errorf("dst is nil")
	}

	if dstValue.Type().Kind() != reflect.Ptr {
		return fmt.Errorf("dst must be a pointer but got %s", dstValue.Type())
	}

	to := dstValue.Elem()

	if !to.CanAddr() {
		return fmt.Errorf("dst (%s) is not addressable", dstValue)
	}

	if from.Type() != to.Type() {
		return fmt.Errorf("type of dst (%s) does not match type of src (%s)", to.Type(), from.Type())
	}

	cpy := dc.Copy(from.Interface())

	to.Set(reflect.ValueOf(cpy))
	return nil
}

func MustDeepCopy(dst interface{}, src interface{}) {
	if err := DeepCopy(dst, src); err != nil {
		panic(err)
	}
}
