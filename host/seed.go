package host

// MatchSeed derives match k's engine seed from its table's seed with
// splitmix64's finaliser over tableSeed XOR k·φ, so a table's whole history
// is a pure function of its configuration (spec D14) and consecutive
// matches share nothing but the table.
func MatchSeed(tableSeed uint64, k int) uint64 {
	z := tableSeed ^ (uint64(k) * 0x9E3779B97F4A7C15)
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}
