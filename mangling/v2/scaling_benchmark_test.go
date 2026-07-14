// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package mangling

import (
	"fmt"
	"io"
	"strings"
	"testing"
)

// BenchmarkGoIdentUnexportedScaling measures how IdentUnexported scales with the number of input tokens.
//
// The reported "ns/token" metric stays roughly constant when scaling is linear (the goal); a rising ns/token would flag
// super-linear behaviour.
func BenchmarkGoIdentUnexportedScaling(b *testing.B) {
	g := MakeGoMangler()

	for _, n := range []int{1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024} {
		in := makeTokenInput(n)

		b.Run(fmt.Sprintf("tokens=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			var res string
			for range b.N {
				res = g.IdentUnexported(in)
			}

			fmt.Fprintln(io.Discard, res)
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(n), "ns/token")
		})
	}
}

// scalingTokenPool mixes plain words with default initialisms (http, id, json, api, uuid) so a swept input exercises
// the trie, folding and assembly together — IdentUnexported is the richest default path.
var scalingTokenPool = []string{
	"http", "server", "id", "json", "payload", "config", "handler",
	"request", "response", "uuid", "api", "gateway", "service",
	"worker", "queue", "cache", "index", "buffer", "stream", "token",
}

// makeTokenInput builds a space-separated input of exactly n tokens, cycling the pool.
func makeTokenInput(n int) string {
	var b strings.Builder
	for i := range n {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(scalingTokenPool[i%len(scalingTokenPool)])
	}

	return b.String()
}
