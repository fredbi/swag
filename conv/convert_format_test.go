// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package conv

import (
	"math"
	"math/big"
	"strconv"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"
)

var evaluatesAsTrue = map[string]struct{}{
	"true":     {},
	"1":        {},
	"yes":      {},
	"ok":       {},
	"y":        {},
	"on":       {},
	"selected": {},
	"checked":  {},
	"t":        {},
	"enabled":  {},
}

func TestConvertBool(t *testing.T) {
	for k := range evaluatesAsTrue {
		r, err := ConvertBool(k)
		require.NoError(t, err)
		assert.True(t, r)
	}
	for _, k := range []string{"a", "", "0", "false", "unchecked", "anythingElse"} {
		r, err := ConvertBool(k)
		require.NoError(t, err)
		assert.False(t, r)
	}
}

func TestFormatBool(t *testing.T) {
	assert.Equal(t, "true", FormatBool(true))
	assert.Equal(t, "false", FormatBool(false))
}

func TestConvertFloat(t *testing.T) {
	t.Run("with float32", func(t *testing.T) {
		validFloats := []float32{1.0, -1, math.MaxFloat32, math.SmallestNonzeroFloat32, 0, 5.494430303}
		invalidFloats := []string{"a", strconv.FormatFloat(math.MaxFloat64, 'f', -1, 64), "true", float64OverflowStr()}

		for _, f := range validFloats {
			str := FormatFloat(f)
			c1, err := ConvertFloat32(str)
			require.NoError(t, err)
			assert.InDelta(t, f, c1, 1e-6)

			c2, err := ConvertFloat[float32](str)
			require.NoError(t, err)
			assert.InDelta(t, c1, c2, 1e-6)
		}

		for _, f := range invalidFloats {
			_, err := ConvertFloat32(f)
			require.Error(t, err, testErrMsg(f))

			_, err = ConvertFloat[float32](f)
			require.Error(t, err, testErrMsg(f))
		}
	})

	t.Run("with float64", func(t *testing.T) {
		validFloats := []float64{1.0, -1, float64(math.MaxFloat32), float64(math.SmallestNonzeroFloat32), math.MaxFloat64, math.SmallestNonzeroFloat64, 0, 5.494430303}
		invalidFloats := []string{"a", "true", float64OverflowStr()}

		for _, f := range validFloats {
			str := FormatFloat(f)
			c1, err := ConvertFloat64(str)
			require.NoError(t, err)
			assert.InDelta(t, f, c1, 1e-6)

			c2, err := ConvertFloat64(str)
			require.NoError(t, err)
			assert.InDelta(t, c1, c2, 1e-6)
		}

		for _, f := range invalidFloats {
			_, err := ConvertFloat64(f)
			require.Error(t, err, testErrMsg(f))

			_, err = ConvertFloat[float64](f)
			require.Error(t, err, testErrMsg(f))
		}
	})
}

func TestConvertInteger(t *testing.T) {
	t.Run("with int8", func(t *testing.T) {
		validInts := []int8{0, 1, -1, math.MaxInt8, math.MinInt8}
		invalidInts := []string{"1.233", "a", "false", strconv.FormatInt(int64(math.MaxInt64), 10)}

		for _, f := range validInts {
			str := FormatInteger(f)
			c1, err := ConvertInt8(str)
			require.NoError(t, err)
			assert.Equal(t, f, c1)

			c2, err := ConvertInteger[int8](str)
			require.NoError(t, err)
			assert.Equal(t, c1, c2)
		}

		for _, f := range invalidInts {
			_, err := ConvertInt8(f)
			require.Error(t, err, testErrMsg(f))

			_, err = ConvertInteger[int8](f)
			require.Error(t, err, testErrMsg(f))
		}
	})

	t.Run("with int16", func(t *testing.T) {
		validInts := []int16{0, 1, -1, math.MaxInt8, math.MinInt8, math.MaxInt16, math.MinInt16}
		invalidInts := []string{"1.233", "a", "false", strconv.FormatInt(int64(math.MaxInt64), 10)}

		for _, f := range validInts {
			str := FormatInteger(f)
			c1, err := ConvertInt16(str)
			require.NoError(t, err)
			assert.Equal(t, f, c1)

			c2, err := ConvertInteger[int16](str)
			require.NoError(t, err)
			assert.Equal(t, c1, c2)
		}

		for _, f := range invalidInts {
			_, err := ConvertInt16(f)
			require.Error(t, err, testErrMsg(f))

			_, err = ConvertInteger[int16](f)
			require.Error(t, err, testErrMsg(f))
		}
	})

	t.Run("with int32", func(t *testing.T) {
		validInts := []int32{0, 1, -1, math.MaxInt8, math.MinInt8, math.MaxInt16, math.MinInt16, math.MinInt32, math.MaxInt32}
		invalidInts := []string{"1.233", "a", "false", strconv.FormatInt(int64(math.MaxInt64), 10)}

		for _, f := range validInts {
			str := FormatInteger(f)
			c1, err := ConvertInt32(str)
			require.NoError(t, err)
			assert.Equal(t, f, c1)

			c2, err := ConvertInteger[int32](str)
			require.NoError(t, err)
			assert.Equal(t, c1, c2)
		}

		for _, f := range invalidInts {
			_, err := ConvertInt32(f)
			require.Error(t, err, testErrMsg(f))

			_, err = ConvertInteger[int32](f)
			require.Error(t, err, testErrMsg(f))
		}
	})

	t.Run("with int64", func(t *testing.T) {
		validInts := []int64{0, 1, -1, math.MaxInt8, math.MinInt8, math.MaxInt16, math.MinInt16, math.MinInt32, math.MaxInt32, math.MaxInt64, math.MinInt64}
		invalidInts := []string{"1.233", "a", "false"}

		for _, f := range validInts {
			str := FormatInteger(f)
			c1, err := ConvertInt64(str)
			require.NoError(t, err)
			assert.Equal(t, f, c1)

			c2, err := ConvertInt64(str)
			require.NoError(t, err)
			assert.Equal(t, c1, c2)
		}

		for _, f := range invalidInts {
			_, err := ConvertInt64(f)
			require.Error(t, err, testErrMsg(f))

			_, err = ConvertInteger[int64](f)
			require.Error(t, err, testErrMsg(f))
		}
	})
}

func TestConvertUinteger(t *testing.T) {
	t.Run("with uint8", func(t *testing.T) {
		validInts := []uint8{0, 1, math.MaxUint8}
		invalidInts := []string{"1.233", "a", "false", strconv.FormatUint(math.MaxUint64, 10), "-1"}

		for _, f := range validInts {
			str := FormatUinteger(f)
			c1, err := ConvertUint8(str)
			require.NoError(t, err)
			assert.Equal(t, f, c1)

			c2, err := ConvertUinteger[uint8](str)
			require.NoError(t, err)
			assert.Equal(t, c1, c2)
		}

		for _, f := range invalidInts {
			_, err := ConvertUint8(f)
			require.Error(t, err, testErrMsg(f))

			_, err = ConvertUinteger[uint8](f)
			require.Error(t, err, testErrMsg(f))
		}
	})

	t.Run("with uint16", func(t *testing.T) {
		validUints := []uint16{0, 1, math.MaxUint8, math.MaxUint16}
		invalidUints := []string{"1.233", "a", "false", strconv.FormatUint(math.MaxUint64, 10), strconv.FormatInt(-1, 10)}

		for _, f := range validUints {
			str := FormatUinteger(f)
			c1, err := ConvertUint16(str)
			require.NoError(t, err)
			assert.Equal(t, f, c1)

			c2, err := ConvertUinteger[uint16](str)
			require.NoError(t, err)
			assert.Equal(t, c1, c2)
		}

		for _, f := range invalidUints {
			_, err := ConvertUint16(f)
			require.Error(t, err, testErrMsg(f))

			_, err = ConvertUinteger[uint16](f)
			require.Error(t, err, testErrMsg(f))
		}
	})

	t.Run("with uint32", func(t *testing.T) {
		validUints := []uint32{0, 1, math.MaxUint8, math.MaxUint16, math.MaxUint32}
		invalidUints := []string{"1.233", "a", "false", strconv.FormatUint(math.MaxUint64, 10), strconv.FormatInt(-1, 10)}

		for _, f := range validUints {
			str := FormatUinteger(f)
			c1, err := ConvertUint32(str)
			require.NoError(t, err)
			assert.Equal(t, f, c1)

			c2, err := ConvertUint32(str)
			require.NoError(t, err)
			assert.Equal(t, c1, c2)
		}

		for _, f := range invalidUints {
			_, err := ConvertUint32(f)
			require.Error(t, err, testErrMsg(f))

			_, err = ConvertUinteger[uint32](f)
			require.Error(t, err, testErrMsg(f))
		}
	})

	t.Run("with uint64", func(t *testing.T) {
		validUints := []uint64{0, 1, math.MaxUint8, math.MaxUint16, math.MaxUint32, math.MaxUint64}
		invalidUints := []string{"1.233", "a", "false", strconv.FormatInt(-1, 10), uint64OverflowStr()}

		for _, f := range validUints {
			str := FormatUinteger(f)
			c1, err := ConvertUint64(str)
			require.NoError(t, err)
			assert.Equal(t, f, c1)

			c2, err := ConvertUinteger[uint64](str)
			require.NoError(t, err)
			assert.Equal(t, c1, c2)
		}
		for _, f := range invalidUints {
			_, err := ConvertUint64(f)
			require.Error(t, err, testErrMsg(f))

			_, err = ConvertUinteger[uint64](f)
			require.Error(t, err, testErrMsg(f))
		}
	})
}

func TestIsFloat64AJSONInteger(t *testing.T) {
	t.Run("should not be integers", testNotIntegers(IsFloat64AJSONInteger, false))
	t.Run("should be integers", testIntegers(IsFloat64AJSONInteger, false))
}

func TestPreviousIsFloat64AJSONInteger(t *testing.T) {
	t.Run("should not be integers", testNotIntegers(previousIsFloat64JSONInteger, false))
	t.Run("should be integers", testIntegers(previousIsFloat64JSONInteger, true))
}

func TestBitWiseIsFloat64AJSONInteger(t *testing.T) {
	t.Run("should not be integers", testNotIntegers(bitwiseIsFloat64JSONInteger, false))
	t.Run("should be integers", testIntegers(bitwiseIsFloat64JSONInteger, false))
}

func TestBitWise2IsFloat64AJSONInteger(t *testing.T) {
	t.Run("should not be integers", testNotIntegers(bitwiseIsFloat64JSONInteger2, false))
	t.Run("should be integers", testIntegers(bitwiseIsFloat64JSONInteger2, false))
}

func TestStdlib2IsFloat64AJSONInteger(t *testing.T) {
	t.Run("should not be integers", testNotIntegers(stdlibIsFloat64JSONInteger, true))
	t.Run("should be integers", testIntegers(stdlibIsFloat64JSONInteger, true))
}

func testNotIntegers(fn func(float64) bool, skipKnownFailure bool) func(*testing.T) {
	_ = skipKnownFailure

	return func(t *testing.T) {
		assert.False(t, fn(math.Inf(1)))
		assert.False(t, fn(maxJSONFloat+1))
		assert.False(t, fn(minJSONFloat-1))
		assert.False(t, fn(math.SmallestNonzeroFloat64))
		assert.False(t, fn(0.5))
		assert.False(t, fn(0.25))
		assert.False(t, fn(1.00/func() float64 { return 2.00 }()))
		assert.False(t, fn(1.00/func() float64 { return 4.00 }()))
		assert.False(t, fn(epsilon))
	}
}

func testIntegers(fn func(float64) bool, skipKnownFailure bool) func(*testing.T) {
	// wrapping in a function forces non-constant evaluation to test float64 rounding behavior
	return func(t *testing.T) {
		assert.True(t, fn(0.0))
		assert.True(t, fn(1.0))
		assert.True(t, fn(maxJSONFloat))
		assert.True(t, fn(minJSONFloat))
		if !skipKnownFailure {
			assert.True(t, fn(1/0.01*67.15000001))
		}
		if !skipKnownFailure {
			assert.True(t, fn(1.00/func() float64 { return 0.01 }()*4643.4))
		}
		assert.True(t, fn(1.00/func() float64 { return 1.00 / 3.00 }()))
		assert.True(t, fn(math.SmallestNonzeroFloat64/2))
		assert.True(t, fn(math.SmallestNonzeroFloat64/3))
		assert.True(t, fn(math.SmallestNonzeroFloat64/4))
	}
}

// test utilities

func testErrMsg(f string) string {
	const (
		expectedQuote = "expected '"
		errSuffix     = "' to generate an error"
	)

	return expectedQuote + f + errSuffix
}

func uint64OverflowStr() string {
	var one, maxUint, overflow big.Int
	one.SetUint64(1)
	maxUint.SetUint64(math.MaxUint64)
	overflow.Add(&maxUint, &one)

	return overflow.String()
}

func float64OverflowStr() string {
	var one, maxFloat64, overflow big.Float
	one.SetFloat64(1.00)
	maxFloat64.SetFloat64(math.MaxFloat64)
	overflow.Add(&maxFloat64, &one)

	return overflow.String()
}
