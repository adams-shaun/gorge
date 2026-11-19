package oracle

import "testing"

// CR 601.2b with CR 601.2g and CR 605.3a: Chord of Calling is announced with
// convoke (two green picks pay two of the {G}{G}{G}, two generic picks pay
// into X), and Desolate Mire's "{1}, {T}: Add {W}{B}" is one of the sources
// CR 601.2g's window can still activate for the announced X -- the floating
// mana pays its {1} fee (CR 605.3a). gorge's xAsk prices each X through
// castWindowPays and offers X = 17; the kernel's per-X read priced only the
// planner census (a paid census gap: mana_filterfirst.go) and stopped one
// short (decision 1117, tail-75-c003 pair 7, autopay).
func TestMBCTimbermawLarvaReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "xask_convoke_filter_fee")
}
