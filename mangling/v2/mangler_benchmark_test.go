package mangling

import (
	"fmt"
	"io"
	"testing"
)

func BenchmarkMangler(b *testing.B) {
	m := MakeMangler()

	b.Run("Pascalize", benchmarkMangle(m.Pascalize, benchmarkSamples))
	b.Run("Camelize", benchmarkMangle(m.Camelize, benchmarkSamples))
	b.Run("Snakize", benchmarkMangle(m.Snakize, benchmarkSamples))
	b.Run("Kebabize", benchmarkMangle(m.Kebabize, benchmarkSamples))
	b.Run("Humanize", benchmarkMangle(m.Humanize, benchmarkSamples))
	b.Run("Titleize", benchmarkMangle(m.Titleize, benchmarkSamples))
}

func BenchmarkGoMangler(b *testing.B) {
	g := MakeGoMangler()

	b.Run("IdentExported", benchmarkMangle(g.IdentExported, benchmarkSamples))
	b.Run("IdentUnexported", benchmarkMangle(g.IdentUnexported, benchmarkSamples))
	b.Run("ConstName", benchmarkMangle(g.ConstName, constNameSamples))
}

func benchmarkMangle(fn func(string) string, samples []string) func(*testing.B) {
	return func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()

		var res string
		for i := 0; i < b.N; i++ {
			res = fn(samples[i%len(samples)])
		}

		fmt.Fprintln(io.Discard, res)
	}
}

// benchmarkSamples is a representative mix of codegen inputs: overwhelmingly ASCII (the common case),
// with a single CJK entry to still exercise the rune-name elision path without letting the slow path
// dominate the numbers. The ASCII core mirrors the v1 BenchmarkToXXXName inputs for comparability.
var benchmarkSamples = []string{
	"sample text",
	"sample-text",
	"sample_text",
	"sampleText",
	"sample 2 Text",
	"findThingById",
	"findTHINGSbyID",
	"user_id",
	"HTTPResponseWriter",
	"created at timestamp",
	"list of email addresses",
	"日本語findThingById", // one non-Latin entry: exercises the rune-name path (CJK elided)
}

// constNameSamples are value-like inputs (enum members): numbers, fractions, signs, decimals, a
// path-ish literal and a diacritic — exercising ConstName's number-verbalization and folding paths.
var constNameSamples = []string{
	"active",
	"read only",
	"in progress",
	"1",
	"200",
	"0.25",
	"-5",
	"3.14",
	"status 200",
	"application/json",
	"café",
}
