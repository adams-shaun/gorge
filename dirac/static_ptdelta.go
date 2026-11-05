package dirac

// An Override with PTDelta is a +N/+M pump (PumpAll Duration$
// UntilYourNextTurn NumAtt$/NumDef$: V'Ger, the Intruder). It rides the
// ordinary override record: p and t hold the pump, evaluated once at
// resolution (CR 611.2c, the permanents present then); fl carries the
// next-turn boundary dropNextTurnOv ends. ptGen adds it with the counters
// (layer 7c, CR 613.4c), once per merged application (ovRec.reps). It adds no
// state: the record's p, t and fl already exist.

// DebugPTDeltaNoExpire is the mutation switch (dirac/oracle
// TestVgerTheIntruderMutation): a P/T pump record never expires at its
// controller's next turn, so the lockstep against gorge must diverge once
// the turn comes round.
var DebugPTDeltaNoExpire bool

// ptDeltaFl is the duration bits of override idx's record: fl, or with the
// mutation on and idx a P/T pump, a record that never ends.
func (s *State) ptDeltaFl(k *prep, idx int, fl uint8) uint8 {
	if DebugPTDeltaNoExpire && k.t.Overrides[idx].PTDelta {
		return 0
	}
	return fl
}

// ptDeltaRecs is the sum of the P/T pumps among the override records on sl.
//
//go:noinline
func (s *State) ptDeltaRecs(k *prep, sl Slot) (dp, dt int) {
	s.eachOvRec(sl, func(r ovRec) {
		if k.t.Overrides[r.idx-1].PTDelta {
			dp += int(r.p) * r.reps()
			dt += int(r.t) * r.reps()
		}
	})
	return
}
