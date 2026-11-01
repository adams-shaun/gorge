package feat

// FNV-1a 64 over the exact token bytes, masked to the shared table: policynet
// hashID / hashstream.go, streamed so no token string is ever built.

const (
	off   uint64 = 14695981039346656037
	prime uint64 = 1099511628211

	// TableRows is policynet.TableRows.
	TableRows = 1 << 14
	rowMask   = TableRows - 1
)

func add(h uint64, s string) uint64 {
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= prime
	}
	return h
}

func addB(h uint64, s []byte) uint64 {
	for _, c := range s {
		h ^= uint64(c)
		h *= prime
	}
	return h
}

func addByte(h uint64, c byte) uint64 {
	h ^= uint64(c)
	return h * prime
}

// addInt streams v's decimal digits (policynet fnvAddInt).
func addInt(h uint64, v int64) uint64 {
	if v == 0 {
		return addByte(h, '0')
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var digits [20]byte
	i := len(digits)
	for v > 0 {
		i--
		digits[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		digits[i] = '-'
	}
	for ; i < len(digits); i++ {
		h = addByte(h, digits[i])
	}
	return h
}

func row(h uint64) uint16 { return uint16(h & rowMask) }

// clampI is policynet mzClamp.
func clampI(v, hi int32) int64 {
	if v < 0 {
		v = 0
	}
	if v > hi {
		v = hi
	}
	return int64(v)
}
