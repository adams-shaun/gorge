package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// sacSourceCensusRepro is a four-seat hosted-cmdr pod (kfuzz -kernel
// hosted-cmdr -seats 4 -policy heuristic, pod 57): Temporal Cleansing convoked
// with one creature leaves a cost no mana source covers, beside Kykar, Wind's
// Fury ("Sacrifice a Spirit: Add {R}") with no Spirit. gorge's window census
// cannot price a sacrifice cost, whether or not it can be paid, so it keeps the
// target candidates and asks for the target; the cast reverses at payment
// (CR 601.2c, 601.2h). The kernel counted the source only with a Spirit to
// sacrifice and reversed the cast at the target ask.
const sacSourceCensusRepro = "convoke_target_ask_beside_sacrifice_mana_source_four_seat_pod"

// sacSourceCensusAt is the decision the recording diverged at.
const sacSourceCensusAt = 1087

func TestHostedCmdrSacrificeSourceCensusLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	res := hostedCmdrPodReplay(t, sacSourceCensusRepro, func(*GorgeSide) {})
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) || res.Decisions <= sacSourceCensusAt {
		t.Fatalf("stall %q after %d decisions, want a clean game past %d", res.Stall, res.Decisions, sacSourceCensusAt)
	}
	// The precondition: the replay reaches an ask that turns on the gap.
	dirac.DebugNoSacCensusGap = true
	defer func() { dirac.DebugNoSacCensusGap = false }()
	res = hostedCmdrPodReplay(t, sacSourceCensusRepro, func(*GorgeSide) {})
	if res.Div == nil || res.Div.At != sacSourceCensusAt {
		t.Fatalf("a census without the sacrifice gap did not diverge at %d: %+v", sacSourceCensusAt, res.Div)
	}
}
