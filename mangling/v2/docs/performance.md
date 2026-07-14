# Performance & scalability

## Summary

`GoMangler.IdentUnexported` (the richest default path: asciify → segment → ASCII fold → initialism
overlay → assemble → reserved-word repair) scales **linearly in the number of input tokens** and does a
**constant single allocation per call, independent of input size**.

- **Time:** O(n) — flat at ~160 ns/token from a handful of tokens up to a thousand.
- **Allocations:** O(1) — **1 alloc/op** whether the input is 1 token or 1024.
- **Memory:** O(n) — `B/op` grows linearly (~6.8 B/token), which is exactly the output string.

This is the design's zero-copy, pooled token model made measurable: the token slice is borrowed from a
`sync.Pool` and reused, so growing the token count adds no allocations; the single allocation per call is
the final output string materialized once at assembly.

## Method

`BenchmarkGoIdentUnexportedScaling` builds a space-separated input of exactly *n* tokens, cycling a pool
that mixes plain words with default initialisms (`http`, `id`, `json`, `api`, `uuid`) so the trie,
folding and assembly are all exercised, then benchmarks `IdentUnexported` over a sweep of *n*.

It reports a custom **`ns/token`** metric (`elapsed / b.N / n`). A flat `ns/token` across the sweep means
linear scaling; a rising `ns/token` would flag super-linear behaviour.

```sh
go test -run '^$' -bench BenchmarkGoIdentUnexportedScaling -benchmem ./mangling/v2
```

## Results

Representative run (go 1.26, linux/amd64, GOMAXPROCS=16, `-benchtime=300ms`). Absolute nanoseconds are
machine-dependent; the **shape** — flat `ns/token`, constant `allocs/op` — is the result that matters.

```
BenchmarkGoIdentUnexportedScaling/tokens=1-16      	 4159804	       288.4 ns/op	       288.4 ns/token	       8 B/op	       1 allocs/op
BenchmarkGoIdentUnexportedScaling/tokens=2-16      	 2662651	       473.5 ns/op	       236.8 ns/token	      16 B/op	       1 allocs/op
BenchmarkGoIdentUnexportedScaling/tokens=4-16      	 1844190	       658.6 ns/op	       164.6 ns/token	      24 B/op	       1 allocs/op
BenchmarkGoIdentUnexportedScaling/tokens=8-16      	  763206	      1408 ns/op	       176.0 ns/token	      64 B/op	       1 allocs/op
BenchmarkGoIdentUnexportedScaling/tokens=16-16     	  497030	      2634 ns/op	       164.6 ns/token	     112 B/op	       1 allocs/op
BenchmarkGoIdentUnexportedScaling/tokens=32-16     	  240738	      5101 ns/op	       159.4 ns/token	     208 B/op	       1 allocs/op
BenchmarkGoIdentUnexportedScaling/tokens=64-16     	  112435	     10251 ns/op	       160.2 ns/token	     417 B/op	       1 allocs/op
BenchmarkGoIdentUnexportedScaling/tokens=128-16    	   55212	     19970 ns/op	       156.0 ns/token	     899 B/op	       1 allocs/op
BenchmarkGoIdentUnexportedScaling/tokens=256-16    	   29811	     42043 ns/op	       164.2 ns/token	    1801 B/op	       1 allocs/op
BenchmarkGoIdentUnexportedScaling/tokens=512-16    	   15118	     82618 ns/op	       161.4 ns/token	    3475 B/op	       1 allocs/op
BenchmarkGoIdentUnexportedScaling/tokens=1024-16   	    8167	    158104 ns/op	       154.4 ns/token	    6809 B/op	       1 allocs/op
```

## Analysis

**Time is linear.** `ns/token` holds at ~160 ns across a 1000× range in input size — no super-linear
knee. Segmentation is a single left-to-right rune scan; the initialism overlay is a trie walk bounded by
token length; assembly is a single pass writing into one builder. Every stage is O(n) in tokens, so the
whole pipeline is.

The `n=1` point (288 ns/token) is higher only because per-call fixed overhead — the value-receiver
mangler copy, the pool borrow/redeem, builder setup — is spread over a single token. From `n=4` upward
that overhead is amortized and the per-token cost settles at its steady-state ~160 ns.

**Allocations are constant.** 1 alloc/op at every size is the headline. The tokenizer materializes the
input into one shared `[]rune` (borrowed from `sync.Pool`, returned on `redeem`), and all transforms
operate as views over that slice — no per-token strings, no intermediate slices. The one allocation is
the output string built at assembly. A 1024-token identifier costs the same allocation count as a
1-token one.

**Memory is the output.** `B/op` tracks the output length (~6.8 B/token), not internal churn — there is
no hidden per-token garbage. This is what keeps GC pressure flat under codegen workloads that mangle many
names.

## Fast vs slow paths

```
BenchmarkGoManglerPaths/fast-16         	 2229770	       556.7 ns/op	      16 B/op	       1 allocs/op
BenchmarkGoManglerPaths/slow/diacritics-16         	 1581030	       765.3 ns/op	      24 B/op	       2 allocs/op
BenchmarkGoManglerPaths/slow/cjk-elided-16         	 1452753	       860.9 ns/op	      32 B/op	       2 allocs/op
BenchmarkGoManglerPaths/slow/greek-named-16        	  540944	      2110 ns/op	     160 B/op	       2 allocs/op
BenchmarkGoManglerPaths/slow/numeral-rune-16       	 1004134	      1191 ns/op	      56 B/op	       3 allocs/op
BenchmarkGoManglerPaths/slow/leading-number-16     	 1294728	       973.5 ns/op	      72 B/op	       3 allocs/op
BenchmarkGoManglerPaths/slow/operators-16          	 1437498	       819.2 ns/op	      56 B/op	       2 allocs/op
BenchmarkGoManglerPaths/slow/emoji-16              	 1185285	      1125 ns/op	      64 B/op	       2 allocs/op
```

ASCII verbalisation of unicode transcripts (e.g greek, cyrillic, arabic, ...) is essentially more complex and induces
much more work than the fast pure ASCII case. There is a similar penalty when non-ASCII numeral runes need transliteration.

## Related single-call figures

For reference, the common-case single-call costs on the representative sample mix:

| Operation | ns/op | allocs/op |
|---|---|---|
| `Camelize` | ~1.0 µs | 1 |
| `IdentExported` / `IdentUnexported` | ~1.1 µs | 1 |
| `ConstName` | ~1.0 µs | 1 |

`numbers.NumberWords` is 1 alloc/op (0 for non-numeric input); `numbers.AppendWords` is 0 alloc/op when
the caller pools the destination buffer.
