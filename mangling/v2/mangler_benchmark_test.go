package mangling

import (
	"fmt"
	"io"
	"testing"
)

// benchmarkSamples mirrors the v1 BenchmarkToXXXName inputs so the two are comparable.
var benchmarkSamples = []string{
	"sample text",
	"sample-text",
	"sample_text",
	"sampleText",
	"sample 2 Text",
	"findThingById",
	"日本語sample 2 Text",
	"日本語findThingById",
	"findTHINGSbyID",
}

func BenchmarkMangler(b *testing.B) {
	m := MakeMangler()

	b.Run("Pascalize", benchmarkMangle(m.Pascalize))
	b.Run("Camelize", benchmarkMangle(m.Camelize))
	b.Run("Snakize", benchmarkMangle(m.Snakize))
	b.Run("Kebabize", benchmarkMangle(m.Kebabize))
	b.Run("Humanize", benchmarkMangle(m.Humanize))
	b.Run("Titleize", benchmarkMangle(m.Titleize))
}

func benchmarkMangle(fn func(string) string) func(*testing.B) {
	return func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()

		var res string
		for i := 0; i < b.N; i++ {
			res = fn(benchmarkSamples[i%len(benchmarkSamples)])
		}

		fmt.Fprintln(io.Discard, res)
	}
}
