# Performance & scalability

## Summary

`GoMangler.IdentUnexported` (the richest default path: asciify → segment → ASCII fold → initialism
overlay → assemble → reserved-word repair) scales **linearly in the number of input tokens** and does a
**constant single allocation per call, independent of input size**.

- **Time:** O(n) — flat at ~380 ns/token from a handful of tokens up to a thousand.
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

| tokens | ns/op   | ns/token | B/op  | allocs/op |
|-------:|--------:|---------:|------:|----------:|
| 1      | 507     | 507      | 8     | 1         |
| 2      | 878     | 439      | 16    | 1         |
| 4      | 1,366   | 342      | 24    | 1         |
| 8      | 3,033   | 379      | 64    | 1         |
| 16     | 6,182   | 386      | 112   | 1         |
| 32     | 12,611  | 394      | 208   | 1         |
| 64     | 23,988  | 375      | 417   | 1         |
| 128    | 46,971  | 367      | 901   | 1         |
| 256    | 100,330 | 392      | 1,812 | 1         |
| 512    | 194,237 | 379      | 3,497 | 1         |
| 1024   | 357,549 | 349      | 6,933 | 1         |

## Analysis

**Time is linear.** `ns/token` holds at ~340–395 ns across a 1000× range in input size — no super-linear
knee. Segmentation is a single left-to-right rune scan; the initialism overlay is a trie walk bounded by
token length; assembly is a single pass writing into one builder. Every stage is O(n) in tokens, so the
whole pipeline is.

The `n=1` point (507 ns/token) is higher only because per-call fixed overhead — the value-receiver
mangler copy, the pool borrow/redeem, builder setup — is spread over a single token. From `n=4` upward
that overhead is amortized and the per-token cost settles at its steady-state ~380 ns.

**Allocations are constant.** 1 alloc/op at every size is the headline. The tokenizer materializes the
input into one shared `[]rune` (borrowed from `sync.Pool`, returned on `redeem`), and all transforms
operate as views over that slice — no per-token strings, no intermediate slices. The one allocation is
the output string built at assembly. A 1024-token identifier costs the same allocation count as a
1-token one.

**Memory is the output.** `B/op` tracks the output length (~6.8 B/token), not internal churn — there is
no hidden per-token garbage. This is what keeps GC pressure flat under codegen workloads that mangle many
names.

## Related single-call figures

For reference, the common-case single-call costs on the representative sample mix:

| Operation | ns/op | allocs/op |
|---|---|---|
| `Camelize` | ~1.0 µs | 1 |
| `IdentExported` / `IdentUnexported` | ~1.1 µs | 1 |
| `ConstName` | ~1.0 µs | 1 |

`numbers.NumberWords` is 1 alloc/op (0 for non-numeric input); `numbers.AppendWords` is 0 alloc/op when
the caller pools the destination buffer.
