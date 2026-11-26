package kgen

// kw:Read ahead (CR 702.155, 714.3b; gorge engine/rules/readahead.go): "As
// this Saga enters, choose a number between one and this Saga's final chapter
// number. This Saga enters with that many lore counters on it", and its
// chapter abilities cannot trigger the turn it enters unless it has exactly
// their chapter's lore count (702.155a).
//
// gorge poses the choice at the entry boundary as a "readahead" KChoose of
// one option per chapter, ascending (cast_etbchoice.go faceETBChoice). The
// kernel asks it as the as-enters number choice: the Saga's enters program
// opens with OpExt ExtChooseNumber over 1..the final chapter
// (lower_choosenumber.go), and the answer is the Saga's entry lore count
// instead of one (dirac/kw_readahead.go reads the shape; its chapter gate is
// the 702.155a half).
//
// The final chapter is K:Chapter's count, so the keyword is lowered in the
// card-level tail, after every line is read. A Saga face with another enters
// choice of its own refuses: the kernel's one number ask is the chapter's.
func (lc *lowerCtx) lowerReadAhead() error {
	if lc.out.SagaN <= 0 {
		return refuse("read ahead (CR 702.155b) on a face without chapters")
	}
	if lc.out.SagaN > numberKernelCeiling {
		return refuse("read ahead over %d chapters", lc.out.SagaN)
	}
	for i := range lc.out.Repls {
		switch lc.out.Repls[i].Kind {
		case "EntersWith", "EntersTapped", "EntersWithCounters":
			return refuse("read ahead beside another enters replacement")
		}
	}
	lc.out.Repls = append(lc.out.Repls, lRepl{Kind: "EntersWith", Prog: []lInstr{{
		Op: "ChooseNumber", Obj: lRef{Kind: rYou}, B: cst(1), C: lc.out.SagaN,
		Note: "read ahead: the chapter this Saga enters on",
	}}})
	return nil
}
