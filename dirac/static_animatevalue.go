package dirac

// DebugIgnoreAnimateValue makes a value-driven override use 0/0 instead of
// its Power$/Toughness$ values; the lockstep's mutation test flips it to
// prove the value path is what keeps parity.
var DebugIgnoreAnimateValue bool

// overridePT returns the P/T an override applies to a record: its literal
// SetP/SetT, or for a value-driven Animate its Power$/Toughness$ read once
// here, at resolution (gorge effAnimate computes them once per object).
func (s *State) overridePT(k *prep, idx int) (int16, int16) {
	o := &k.t.Overrides[idx]
	if !o.ValPT {
		return int16(o.SetP), int16(o.SetT)
	}
	if DebugIgnoreAnimateValue {
		return 0, 0
	}
	return int16(s.value(k, o.PA)), int16(s.value(k, o.TA))
}
