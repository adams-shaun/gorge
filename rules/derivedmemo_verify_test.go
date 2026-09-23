package rules

// The rules test binary runs every memoized Derived hit in verify mode: each
// hit is recomputed from scratch and a difference panics, so the whole suite
// (repo-deck games, replay goldens, the acceptance ratchet) doubles as the
// empirical check of derivedmemo.go's invalidation argument.
func init() { derivedMemoVerify = true }
