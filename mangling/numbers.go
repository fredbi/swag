package mangling

func NumberWords[T Numerical](n T) string { // TODO: larger class of type constraint covering all numerics -- see swag/conv formatting functions}
	return ""
}

// 31 -> 31st
func NumberOrdinal[T Numerical](n T) string { // TODO: larger class of type constraint covering all numerics -- see swag/conv formatting functions}
	return ""
}

// 4 -> iv
func NumberRoman[T Integer](n T) string { // TODO: larger class of type constraint covering all numerics -- see swag/conv formatting functions}
	return ""
}

type (
	// these type constraints are redefined after golang.org/x/exp/constraints,
	// because importing that package causes an undesired go upgrade.

	// Signed integer types, cf. [golang.org/x/exp/constraints.Signed]
	Signed interface {
		~int | ~int8 | ~int16 | ~int32 | ~int64
	}

	// Unsigned integer types, cf. [golang.org/x/exp/constraints.Unsigned]
	Unsigned interface {
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
	}

	Integer interface {
		Signed | Unsigned
	}
	// Float numerical types, cf. [golang.org/x/exp/constraints.Float]
	Float interface {
		~float32 | ~float64
	}

	// Numerical types
	Numerical interface {
		Signed | Unsigned | Float
	}
)
