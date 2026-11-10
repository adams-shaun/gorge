package kgen

// RememberedLKI$CardManaCost (gorge count_ref_property.go): the mana values
// summed over the resolution's remembered objects. gorge applies no
// zone-change snapshot to the RememberedLKI spelling, so it reads each
// object's mana value where it is now (state.ManaValue), which is the
// kernel's rememberedMV. The read is admitted only over the set one targeted
// Return with RememberLKI$ wrote in the same program (Reanimate): any other
// writer, a ClearRemembered$ before the read, or a second writer anywhere
// on the card could leave gorge's source list (which it unions into the
// RememberedLKI group) holding objects the kernel's per-resolution set does
// not, so those cards refuse by name.

const remLKIMVShape = "RememberedLKI$CardManaCost over a set not written by one targeted Return RememberLKI$"

// remLKIManaValue lowers the read and counts it for refuseRemLKIMV.
func (lc *lowerCtx) remLKIManaValue() (lValue, error) {
	if lc.ab == nil {
		return lValue{}, refuse("%s (a read outside an ability program)", remLKIMVShape)
	}
	writers := 0
	for _, in := range lc.ab.Prog {
		switch {
		case in.Op == "ClearRemembered":
			return lValue{}, refuse("%s (ClearRemembered$ before the read)", remLKIMVShape)
		case kremWrites(in):
			writers++
			if in.Op != "Return" || in.Obj.Kind != rTarget {
				return lValue{}, refuse("%s (the writer is a %s over %v)", remLKIMVShape, in.Op, in.Obj.Kind)
			}
		}
	}
	if writers != 1 {
		return lValue{}, refuse("%s (%d earlier writers)", remLKIMVShape, writers)
	}
	lc.remLKMVN++
	return lValue{Kind: vMVOf, Ref: lRef{Kind: rRemembered}}, nil
}

// refuseRemLKIMV refuses a card that reads RememberedLKI$CardManaCost and
// has a remember writer other than the one the read paired with.
func (lc *lowerCtx) refuseRemLKIMV() {
	if lc.remLKMVN == 0 {
		return
	}
	n := 0
	for _, a := range lc.out.Abilities {
		n += countWriters(a.Prog)
	}
	for _, r := range lc.out.Repls {
		n += countWriters(r.Prog)
	}
	if n != 1 {
		lc.refuseNode("card", "%s (%d remember writers on the card)", remLKIMVShape, n)
	}
}

func countWriters(prog []lInstr) int {
	n := 0
	for _, in := range prog {
		if kremWrites(in) {
			n++
		}
	}
	return n
}
