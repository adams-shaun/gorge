package rules

// The rules test binary runs the layer-inert cache reuse (layercache.go) in
// verify mode: every reuse is recomputed from scratch and a difference
// panics, so the whole suite doubles as the empirical check that the three
// reused event kinds leave every layer-path input untouched.
func init() { layerInertVerify = true }
