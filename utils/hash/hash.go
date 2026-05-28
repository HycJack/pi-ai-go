// Package hash provides a fast, deterministic hash function.
package hash

// Short returns a deterministic base36 hash of the input string.
// Uses a dual 32-bit state for better distribution.
func Short(s string) string {
	h1 := uint32(0x811c9dc5)
	h2 := uint32(0x01000193)
	for i := 0; i < len(s); i++ {
		b := uint32(s[i])
		h1 ^= b
		h1 *= 0x01000193
		h2 ^= b
		h2 *= 0x811c9dc5
	}
	h := uint64(h1)<<32 | uint64(h2)
	if h == 0 {
		return "0"
	}
	const chars = "0123456789abcdefghijklmnopqrstuvwxyz"
	var buf [13]byte // max 13 chars for uint64 in base36
	i := len(buf)
	for h > 0 {
		i--
		buf[i] = chars[h%36]
		h /= 36
	}
	return string(buf[i:])
}
