package assertions_test

import (
	"errors"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/internal/testharness/assertions"
)

func TestAssertValue(t *testing.T) {
	t.Run("Equal", func(t *testing.T) {
		// Pass cases
		if r := assertions.Equal(42, 42); !r.Passed {
			t.Error("Equal(42, 42) should pass")
		}
		if r := assertions.Equal("hello", "hello"); !r.Passed {
			t.Error("Equal strings should pass")
		}
		if r := assertions.Equal([]int{1, 2, 3}, []int{1, 2, 3}); !r.Passed {
			t.Error("Equal slices should pass")
		}

		// Fail cases
		if r := assertions.Equal(42, 43); r.Passed {
			t.Error("Equal(42, 43) should fail")
		}
		if r := assertions.Equal("hello", "world"); r.Passed {
			t.Error("Different strings should fail")
		}
	})

	t.Run("NotEqual", func(t *testing.T) {
		if r := assertions.NotEqual(42, 43); !r.Passed {
			t.Error("NotEqual(42, 43) should pass")
		}
		if r := assertions.NotEqual(42, 42); r.Passed {
			t.Error("NotEqual(42, 42) should fail")
		}
	})

	t.Run("True/False", func(t *testing.T) {
		if r := assertions.True(true); !r.Passed {
			t.Error("True(true) should pass")
		}
		if r := assertions.True(false); r.Passed {
			t.Error("True(false) should fail")
		}
		if r := assertions.False(false); !r.Passed {
			t.Error("False(false) should pass")
		}
		if r := assertions.False(true); r.Passed {
			t.Error("False(true) should fail")
		}
	})

	t.Run("Nil/NotNil", func(t *testing.T) {
		if r := assertions.Nil(nil); !r.Passed {
			t.Error("Nil(nil) should pass")
		}
		if r := assertions.Nil(42); r.Passed {
			t.Error("Nil(42) should fail")
		}
		if r := assertions.NotNil(42); !r.Passed {
			t.Error("NotNil(42) should pass")
		}
		if r := assertions.NotNil(nil); r.Passed {
			t.Error("NotNil(nil) should fail")
		}
	})
}

func TestAssertState(t *testing.T) {
	t.Run("HasState", func(t *testing.T) {
		if r := assertions.HasState("CONTROLLED", "CONTROLLED"); !r.Passed {
			t.Error("HasState matching should pass")
		}
		if r := assertions.HasState("CONTROLLED", "AUTONOMOUS"); r.Passed {
			t.Error("HasState not matching should fail")
		}
	})
}

func TestAssertTiming(t *testing.T) {
	t.Run("WithinDuration", func(t *testing.T) {
		// Within tolerance
		r := assertions.WithinDuration(100*time.Millisecond, 100*time.Millisecond, 10*time.Millisecond)
		if !r.Passed {
			t.Error("Exact match should pass")
		}

		r = assertions.WithinDuration(105*time.Millisecond, 100*time.Millisecond, 10*time.Millisecond)
		if !r.Passed {
			t.Error("Within tolerance should pass")
		}

		// Outside tolerance
		r = assertions.WithinDuration(200*time.Millisecond, 100*time.Millisecond, 10*time.Millisecond)
		if r.Passed {
			t.Error("Outside tolerance should fail")
		}
	})
}

func TestAssertError(t *testing.T) {
	t.Run("NoError", func(t *testing.T) {
		if r := assertions.NoError(nil); !r.Passed {
			t.Error("NoError(nil) should pass")
		}
		if r := assertions.NoError(errors.New("test")); r.Passed {
			t.Error("NoError(err) should fail")
		}
	})

	t.Run("Error", func(t *testing.T) {
		if r := assertions.Error(errors.New("test")); !r.Passed {
			t.Error("Error(err) should pass")
		}
		if r := assertions.Error(nil); r.Passed {
			t.Error("Error(nil) should fail")
		}
	})

	t.Run("ErrorContains", func(t *testing.T) {
		err := errors.New("connection timeout")
		if r := assertions.ErrorContains(err, "timeout"); !r.Passed {
			t.Error("ErrorContains with matching substring should pass")
		}
		if r := assertions.ErrorContains(err, "refused"); r.Passed {
			t.Error("ErrorContains with non-matching substring should fail")
		}
		if r := assertions.ErrorContains(nil, "any"); r.Passed {
			t.Error("ErrorContains(nil, ...) should fail")
		}
	})

	t.Run("HasErrorCode", func(t *testing.T) {
		if r := assertions.HasErrorCode(2, 2); !r.Passed {
			t.Error("HasErrorCode matching should pass")
		}
		if r := assertions.HasErrorCode(2, 3); r.Passed {
			t.Error("HasErrorCode not matching should fail")
		}
	})
}

func TestAssertCollections(t *testing.T) {
	t.Run("Contains", func(t *testing.T) {
		// String
		if r := assertions.Contains("hello world", "world"); !r.Passed {
			t.Error("String contains should pass")
		}
		if r := assertions.Contains("hello world", "foo"); r.Passed {
			t.Error("String not contains should fail")
		}

		// Slice
		if r := assertions.Contains([]int{1, 2, 3}, 2); !r.Passed {
			t.Error("Slice contains should pass")
		}
		if r := assertions.Contains([]int{1, 2, 3}, 4); r.Passed {
			t.Error("Slice not contains should fail")
		}

		// Map
		m := map[string]int{"a": 1, "b": 2}
		if r := assertions.Contains(m, "a"); !r.Passed {
			t.Error("Map contains key should pass")
		}
		if r := assertions.Contains(m, "c"); r.Passed {
			t.Error("Map not contains key should fail")
		}
	})

	t.Run("Len", func(t *testing.T) {
		if r := assertions.Len([]int{1, 2, 3}, 3); !r.Passed {
			t.Error("Len matching should pass")
		}
		if r := assertions.Len([]int{1, 2, 3}, 2); r.Passed {
			t.Error("Len not matching should fail")
		}
		if r := assertions.Len("hello", 5); !r.Passed {
			t.Error("String len should pass")
		}
	})

	t.Run("Empty/NotEmpty", func(t *testing.T) {
		if r := assertions.Empty([]int{}); !r.Passed {
			t.Error("Empty(empty slice) should pass")
		}
		if r := assertions.Empty([]int{1}); r.Passed {
			t.Error("Empty(non-empty slice) should fail")
		}
		if r := assertions.NotEmpty([]int{1}); !r.Passed {
			t.Error("NotEmpty(non-empty slice) should pass")
		}
		if r := assertions.NotEmpty([]int{}); r.Passed {
			t.Error("NotEmpty(empty slice) should fail")
		}
	})
}

func TestAssertNumeric(t *testing.T) {
	t.Run("InRange", func(t *testing.T) {
		if r := assertions.InRange(5, 0, 10); !r.Passed {
			t.Error("InRange(5, 0, 10) should pass")
		}
		if r := assertions.InRange(0, 0, 10); !r.Passed {
			t.Error("InRange(0, 0, 10) should pass (inclusive)")
		}
		if r := assertions.InRange(10, 0, 10); !r.Passed {
			t.Error("InRange(10, 0, 10) should pass (inclusive)")
		}
		if r := assertions.InRange(15, 0, 10); r.Passed {
			t.Error("InRange(15, 0, 10) should fail")
		}
		if r := assertions.InRange(-5, 0, 10); r.Passed {
			t.Error("InRange(-5, 0, 10) should fail")
		}
	})

	t.Run("GreaterThan", func(t *testing.T) {
		if r := assertions.GreaterThan(10, 5); !r.Passed {
			t.Error("GreaterThan(10, 5) should pass")
		}
		if r := assertions.GreaterThan(5, 10); r.Passed {
			t.Error("GreaterThan(5, 10) should fail")
		}
		if r := assertions.GreaterThan(5, 5); r.Passed {
			t.Error("GreaterThan(5, 5) should fail (not strictly greater)")
		}
	})

	t.Run("LessThan", func(t *testing.T) {
		if r := assertions.LessThan(5, 10); !r.Passed {
			t.Error("LessThan(5, 10) should pass")
		}
		if r := assertions.LessThan(10, 5); r.Passed {
			t.Error("LessThan(10, 5) should fail")
		}
		if r := assertions.LessThan(5, 5); r.Passed {
			t.Error("LessThan(5, 5) should fail (not strictly less)")
		}
	})
}
