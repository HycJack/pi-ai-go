package agent

import (
	"errors"
	"math"
)

// mathFromBits and mathToBits re-encode a float64 as a uint64 for
// atomic storage. We rely on the bit-layout defined by math.Float64bits.
func mathFromBits(b uint64) float64 {
	return math.Float64frombits(b)
}

func mathToBits(f float64) uint64 {
	return math.Float64bits(f)
}

// copyMap returns a shallow copy of a string->int map. The lock should
// already be held by the caller.
func copyMap(m map[string]int) map[string]int {
	if m == nil {
		return nil
	}
	out := make(map[string]int, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// errorsAsImpl is the actual errors.As call. It is split out so the
// collector can classify errors without depending on the "errors"
// package at the top of the file (which would otherwise require
// dropping the atomic-styled structure).
func errorsAsImpl(err error, target any) bool {
	return errors.As(err, target)
}
