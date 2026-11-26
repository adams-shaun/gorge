package codeshape

// The shrink-only ratchets tolerate a small growth: a measurement up to its ceiling plus the slack
// passes with a logged warning, and only growth beyond it fails. A ceiling is never raised to absorb
// the slack: a constant is lowered to the measurement on main after merge, so the slack never
// ratchets the ceiling upward.

// Slack is the growth a count ratchet (TestCodeShapeOnlyShrinks' constants, TestStringsRatchet's
// per-scope counts) tolerates over ceiling: max(2, 1% of it). A ceiling of zero has no slack: a census
// that reached zero stays exact.
func Slack(ceiling int) int {
	if ceiling <= 0 {
		return 0
	}
	return max(2, ceiling/100)
}
