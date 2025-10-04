package conv

import (
	"fmt"
	"io"
	"math"
	"math/big"
	"math/bits"
	"slices"
	"strings"
	"testing"
)

// benchmarks
func BenchmarkConvertBool(b *testing.B) {
	inputs := []string{
		"a", "t", "ok", "false", "true", "TRUE", "no", "n", "y",
	}
	var isTrue bool

	b.ReportAllocs()
	b.ResetTimer()

	b.Run("use switch", func(b *testing.B) {
		var i int
		for b.Loop() {
			isTrue, _ = ConvertBool(inputs[i%len(inputs)])
			i++
		}
		fmt.Fprintln(io.Discard, isTrue)
	})

	b.Run("use map (previous version)", func(b *testing.B) {
		previousConvertBool := func(str string) (bool, error) {
			_, ok := evaluatesAsTrue[strings.ToLower(str)]
			return ok, nil
		}

		var i int
		for b.Loop() {
			isTrue, _ = previousConvertBool(inputs[i%len(inputs)])
			i++
		}
		fmt.Fprintln(io.Discard, isTrue)
	})

	b.Run("use slice.Contains", func(b *testing.B) {
		sliceContainsConvertBool := func(str string) (bool, error) {
			return slices.Contains(
				[]string{"true", "1", "yes", "ok", "y", "on", "selected", "checked", "t", "enabled"},
				strings.ToLower(str),
			), nil
		}

		var i int
		for b.Loop() {
			isTrue, _ = sliceContainsConvertBool(inputs[i%len(inputs)])
			i++
		}
		fmt.Fprintln(io.Discard, isTrue)
	})
}

func BenchmarkIsFloat64JSONInteger(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	b.SetBytes(0)

	b.Run("new float vs integer comparison", benchmarkIsFloat64JSONInteger(IsFloat64AJSONInteger))
	b.Run("previous float vs integer comparison", benchmarkIsFloat64JSONInteger(previousIsFloat64JSONInteger))
	b.Run("bitwise float vs integer comparison", benchmarkIsFloat64JSONInteger(bitwiseIsFloat64JSONInteger))
	b.Run("bitwise float vs integer comparison (2)", benchmarkIsFloat64JSONInteger(bitwiseIsFloat64JSONInteger2))
	b.Run("stdlib float vs integer comparison (2)", benchmarkIsFloat64JSONInteger(stdlibIsFloat64JSONInteger))
}

func BenchmarkBitwise(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	b.SetBytes(0)

	b.Run("bitwise float vs integer comparison (2)", benchmarkIsFloat64JSONInteger(bitwiseIsFloat64JSONInteger2))
}

func previousIsFloat64JSONInteger(f float64) bool {
	if math.IsNaN(f) || math.IsInf(f, 0) || f < minJSONFloat || f > maxJSONFloat {
		return false
	}
	fa := math.Abs(f)
	g := float64(uint64(f))
	ga := math.Abs(g)

	diff := math.Abs(f - g)

	// more info: https://floating-point-gui.de/errors/comparison/#look-out-for-edge-cases
	switch {
	case f == g: // best case
		return true
	case f == float64(int64(f)) || f == float64(uint64(f)): // optimistic case
		return true
	case f == 0 || g == 0 || diff < math.SmallestNonzeroFloat64: // very close to 0 values
		return diff < (epsilon * math.SmallestNonzeroFloat64)
	}
	// check the relative error
	return diff/math.Min(fa+ga, math.MaxFloat64) < epsilon
}

func stdlibIsFloat64JSONInteger(f float64) bool {
	if f < minJSONFloat || f > maxJSONFloat {
		return false
	}
	var bf big.Float
	bf.SetFloat64(f)

	return bf.IsInt()
}

func bitwiseIsFloat64JSONInteger(f float64) bool {
	if math.IsNaN(f) || math.IsInf(f, 0) || f < minJSONFloat || f > maxJSONFloat {
		return false
	}

	mant, exp := math.Frexp(f) // get normalized mantissa
	if exp == 0 && mant == 0 {
		return true
	}
	if exp <= 0 {
		return false
	}

	zeros := bits.TrailingZeros64(uint64(mant))

	return bits.UintSize-zeros <= exp
}

func bitwiseIsFloat64JSONInteger2(f float64) bool {
	if f == 0 {
		return true
	}

	if f < minJSONFloat || f > maxJSONFloat || f != f || f < -math.MaxFloat64 || f > math.MaxFloat64 {
		return false
	}

	// inlined
	var (
		mant uint64
		exp  int
	)
	{
		const smallestNormal = 2.2250738585072014e-308 // 2**-1022

		if math.Abs(f) < smallestNormal {
			f *= (1 << shift) // x 2^52
			exp = -shift
		}

		x := math.Float64bits(f)
		exp += int((x>>shift)&mask) - bias + 1 //nolint:gosec // x>>12 & 0x7FF - 1022 : extract exp, recentered from bias

		x &^= mask << shift       // x= x &^ 0x7FF << 12 (clear 11 exp bits then shift 12)
		x |= (-1 + bias) << shift // x = x | 1022 << 12 ==> or with 1022 as exp location
		mant = uint64(math.Float64frombits(x))
	}
	/*
		{
			x := math.Float64bits(f)
			exp = int(x>>shift) & mask

			if exp < bias {
			} else if exp < bias+shift { // 1023 + 12
				exp -= bias
			}
		}
	*/
	/*
		e := uint(bits>>shift) & mask
		if e < bias {
			// Round abs(x) < 1 including denormals.
			bits &= signMask // +-0
			if e == bias-1 {
				bits |= uvone // +-1
			}
		} else if e < bias+shift {
			// Round any abs(x) >= 1 containing a fractional component [0,1).
			//
			// Numbers with larger exponents are returned unchanged since they
			// must be either an integer, infinity, or NaN.
			const half = 1 << (shift - 1)
			e -= bias
			bits += half >> e
			bits &^= fracMask >> e
		}
	*/

	// It returns frac and exp satisfying f == frac × 2**exp,
	// with the absolute value of frac in the interval [½, 1).
	if exp <= 0 {
		return false
	}

	zeros := bits.TrailingZeros64(mant)

	return bits.UintSize-zeros <= exp
}

const (
	mask  = 0x7FF
	shift = 64 - 11 - 1
	// uvinf    = 0x7FF0000000000000
	// uvneginf = 0xFFF0000000000000
	bias     = 1023
	fracMask = 1<<shift - 1
)

/*
func isNaN(x uint64) bool { // f != f
	return uint32(x>>shift)&mask == mask // && x != uvinf && x != uvneginf
}

func isInf(x uint64) bool { // f < - math.MaxFloat || f > math.MaxFloat
	return x == uvinf || x == uvneginf
}
*/

/*
func frexp(f float64) (frac uint64, exp int) {
	const smallestNormal = 2.2250738585072014e-308 // 2**-1022
	g := f

	if math.Abs(f) < smallestNormal {
		g *= (1 << 52)
		exp = -52
	}

	x := math.Float64bits(g)
	exp += int((x>>shift)&mask) - bias + 1
	x &^= mask << shift
	x |= (-1 + bias) << shift
	frac = uint64(math.Float64frombits(x))

	return
}
*/

func benchmarkIsFloat64JSONInteger(fn func(float64) bool) func(*testing.B) {
	assertCode := func() {
		panic("unexpected result during benchmark")
	}

	return func(b *testing.B) {
		testFunc := func() {
			if fn(math.Inf(1)) {
				assertCode()
			}
			if fn(maxJSONFloat + 1) {
				assertCode()
			}
			if fn(minJSONFloat - 1) {
				assertCode()
			}
			if fn(math.SmallestNonzeroFloat64) {
				assertCode()
			}
			if fn(0.5) {
				assertCode()
			}

			if !fn(1.0) {
				assertCode()
			}
			if !fn(maxJSONFloat) {
				assertCode()
			}
			if !fn(minJSONFloat) {
				assertCode()
			}
			if !fn(1 / 0.01 * 67.15000001) {
				assertCode()
			}
			/* can't compare both versions on this test case
			if !fn(1 / func() float64 { return 0.01 }() * 4643.4) {
				assertCode()
			}
			*/
			if !fn(math.SmallestNonzeroFloat64 / 2) {
				assertCode()
			}
			if !fn(math.SmallestNonzeroFloat64 / 3) {
				assertCode()
			}
			if !fn(math.SmallestNonzeroFloat64 / 4) {
				assertCode()
			}
		}

		for b.Loop() {
			testFunc()
		}
	}
}
