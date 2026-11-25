package oracle

import "testing"

// Phasing and the prepared designation (CR 722, 702.26) in four-seat edh pods
// (kfuzz -kernel edh -seats 4 -policy heuristic, the Lorehold Spirit (SOC)
// deck's Kirol, History Buff beside Guardian of Faith and Teferi's
// Protection-style phase-outs).
//
//   - A prepared source that phases out has not left the battlefield (CR
//     702.26d): gorge keeps Object.Prepared and the exile copy's
//     PreparedSource, as Forge keeps its command-zone prepared effect. The
//     kernel read a phased-out source (kop.ZonePhased) as gone, unlinked the
//     copy and ceased it (pod 73 of the 200-pod heuristic sweep, decision
//     1989: Cards[408].Zone ref exile, kernel ceased).
//   - A permanent that phased out before its "becomes prepared" trigger
//     resolved does not become prepared (CR 702.26b; Forge's
//     AlterAttributeEffect skips a phased-out card): gorge granted it and
//     minted a copy the kernel never minted (pod 72, decision 1767, NInst
//     ref 422 kernel 421).
const (
	preparedPhasedKeepsCopyRepro = "prepared_source_phased_out_keeps_exile_copy"
	preparedPhasedKeepsCopyAt    = 1989
	phasedOutNotPreparedRepro    = "phased_out_permanent_does_not_become_prepared"
	phasedOutNotPreparedAt       = 1767
)

// TestPreparedSourcePhasedOutKeepsExileCopyLockstep: no divergence past the
// recorded decision.
func TestPreparedSourcePhasedOutKeepsExileCopyLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	res := fourSeatEdhReplay(t, preparedPhasedKeepsCopyRepro)
	if res.Div != nil {
		t.Errorf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if res.Decisions <= preparedPhasedKeepsCopyAt {
		t.Errorf("%d decisions, the game must go past decision %d", res.Decisions, preparedPhasedKeepsCopyAt)
	}
}

// TestPhasedOutPermanentDoesNotBecomePreparedLockstep: no divergence past the
// recorded decision.
func TestPhasedOutPermanentDoesNotBecomePreparedLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	res := fourSeatEdhReplay(t, phasedOutNotPreparedRepro)
	if res.Div != nil {
		t.Errorf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if res.Decisions <= phasedOutNotPreparedAt {
		t.Errorf("%d decisions, the game must go past decision %d", res.Decisions, phasedOutNotPreparedAt)
	}
}
