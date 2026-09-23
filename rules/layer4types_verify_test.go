package rules

// The rules test binary runs anyLayer4Active's fast path in verify mode:
// every time it answers without rebuilding active() (a live registered
// layer-4 effect, or the static precheck finding no possible layer-4
// static), active() is rebuilt and a disagreement panics, so the whole suite
// (repo-deck games, replay goldens, the acceptance ratchet) doubles as the
// empirical check that the precheck is conservative.
func init() { layer4PrecheckVerify = true }
