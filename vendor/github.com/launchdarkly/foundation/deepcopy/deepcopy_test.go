package deepcopy

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeepCopy(t *testing.T) {
	t.Parallel()

	t.Run("integer", func(t *testing.T) {
		src := 1
		var dst int
		assert.NoError(t, DeepCopy(&dst, &src))
		assert.Exactly(t, 1, dst)
	})

	t.Run("integer pointer", func(t *testing.T) {
		one := 1
		src := &one
		var dst *int
		assert.NoError(t, DeepCopy(&dst, &src))
		//assert.Exactly(t, 1, *dst)
		//assert.Exactly(t, src, dst)
		//// Ensure we have a different copy
		//*src = 2
		//assert.NotEqual(t, src, dst)
	})

	t.Run("integer pointer to zero", func(t *testing.T) {
		zero := 0
		src := &zero
		var dst *int
		assert.NoError(t, DeepCopy(&dst, &src))
		assert.Exactly(t, 0, *dst)
		assert.Exactly(t, src, dst)
	})

	t.Run("nil pointer", func(t *testing.T) {
		var src *int
		var dst *int
		assert.NoError(t, DeepCopy(&dst, &src))
		assert.Exactly(t, src, dst)
	})

	t.Run("empty src pointer", func(t *testing.T) {
		assert.EqualError(t, DeepCopy(nil, nil), "src is nil")
		assert.EqualError(t, DeepCopy(nil, (*int)(nil)), "src (%!s(*int=<nil>)) is not defined")
	})

	t.Run("empty dst pointer", func(t *testing.T) {
		src := 1
		assert.EqualError(t, DeepCopy(nil, &src), "dst is nil")
		assert.EqualError(t, DeepCopy((*int)(nil), &src), "dst (%!s(*int=<nil>)) is not addressable")
	})

	t.Run("slice", func(t *testing.T) {
		src := []int{1, 2}
		var dst []int
		assert.NoError(t, DeepCopy(&dst, &src))
		assert.Exactly(t, src, dst)

		// Ensure new copy is not the original
		dst[0] = 2
		assert.NotEqual(t, src, dst)
	})

	t.Run("nil slice", func(t *testing.T) {
		var src []int
		var dst []int
		assert.NoError(t, DeepCopy(&dst, &src))
		assert.Exactly(t, src, dst)
		assert.EqualValues(t, &src, &dst)
	})

	t.Run("array", func(t *testing.T) {
		src := [2]int{1, 2}
		var dst [2]int
		assert.NoError(t, DeepCopy(&dst, &src))
		assert.Exactly(t, src, dst)

		// Ensure we have a different copy
		dst[0] = 2
		assert.NotEqual(t, src, dst)
	})

	t.Run("map", func(t *testing.T) {
		src := map[string]int{
			"A": 1,
			"b": 2,
		}
		var dst map[string]int
		assert.NoError(t, DeepCopy(&dst, &src))
		assert.Exactly(t, src, dst)

		// Ensure we have a different copy
		dst["A"] = 2
		assert.NotEqual(t, src, dst)
	})

	t.Run("nil map", func(t *testing.T) {
		var src map[string]int
		var dst map[string]int
		assert.NoError(t, DeepCopy(&dst, &src))
		assert.Exactly(t, src, dst)
	})

	t.Run("struct", func(t *testing.T) {
		type TestStruct struct {
			A int
		}

		src := TestStruct{A: 1}
		var dst TestStruct
		assert.NoError(t, DeepCopy(&dst, &src))
		assert.Exactly(t, src, dst)

		// Ensure we have a different copy
		dst.A = 2
		assert.NotEqual(t, src, dst)
	})

	t.Run("struct with pointer to zero", func(t *testing.T) {
		type TestStruct struct {
			A *int
		}

		zero := 0
		one := 1
		src := TestStruct{A: &zero}
		var dst TestStruct
		assert.NoError(t, DeepCopy(&dst, &src))
		assert.Exactly(t, src, dst) // This fails when we use gob because A gets (*int)(<nil>)

		src = TestStruct{A: &one}
		assert.NoError(t, DeepCopy(&dst, &src))
		assert.Exactly(t, src, dst) // This still passes on gob
	})

	t.Run("type mismatch creates error", func(t *testing.T) {
		var src string
		var dst int
		assert.EqualError(t, DeepCopy(&dst, &src), "type of dst (int) does not match type of src (string)")
	})

	t.Run("private struct attributes are not copied", func(t *testing.T) {
		type TestStruct struct {
			a int
		}

		var src, dst TestStruct
		src.a = 1
		assert.NoError(t, DeepCopy(&dst, &src))
		assert.NotEqual(t, dst, src)
	})

	t.Run("struct with some private attributes doesn't copy private attributes", func(t *testing.T) {
		type TestStruct struct {
			A, b int
		}

		src := TestStruct{
			A: 1,
			b: 2,
		}
		var dst TestStruct

		assert.NoError(t, DeepCopy(&dst, &src))
		assert.NotEqual(t, dst, src)
		assert.Exactly(t, 1, dst.A)
		assert.Exactly(t, 0, dst.b)
	})
}

func TestMustDeepCopy(t *testing.T) {
	src := 1
	var dst string
	defer func() {
		err := recover()
		if assert.NotNil(t, err) && assert.IsType(t, errors.New(""), err) {
			assert.Exactly(t, "type of dst (string) does not match type of src (int)", err.(error).Error())
		}
	}()
	MustDeepCopy(&dst, &src)
}
