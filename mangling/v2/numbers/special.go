// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package numbers

import (
	"math"
	"strconv"
)

// matchSpecial returns the name registered (via [WithSpecialNumbers]) for a special number.
//
// The numeric string s is matched within the detection tolerance, or ("", false).
//
// When several match, the closest one wins, so the result is deterministic regardless of map iteration order.
func matchSpecial(s string, o numberOptions) (string, bool) {
	if len(o.specials) == 0 {
		return "", false
	}

	x, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return "", false
	}

	tol := o.tolerance()
	best, name, found := math.Inf(1), "", false
	for key, special := range o.specials {
		v, err := strconv.ParseFloat(key, 64)
		if err != nil {
			continue
		}
		if d := math.Abs(x - v); d <= tol && d < best {
			best, name, found = d, special, true
		}
	}

	return name, found
}
