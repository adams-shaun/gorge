package dirac

import "fmt"

// LayerSrcStats is what one LayerSrcCheck walked.
type LayerSrcStats struct {
	States   int  // states compared
	Sources  int  // layer sources the list held, summed over the states
	Slots    int  // slot collections compared
	Panicked bool // the game stopped on a kernel panic (a refused interaction): the states before it were compared
}

// LayerSrcCheck plays one game of tbl under the random policy and, every
// eighth decision up to steps, compares the layer walk's table-precomputed
// source records with the live per-ability scan for every occupied slot (the
// equivalence the precomputation rests on). It exists so the oracle's tests,
// the only readers of the synthetic pools, can run the comparison on real
// pools without reaching into dirac's internals.
func LayerSrcCheck(tbl *Table, decks [MaxSeats][]CardID, n uint8, seed uint64, steps int) (st LayerSrcStats, err error) {
	defer func() {
		if r := recover(); r != nil {
			st.Panicked = true
		}
	}()
	// a pointer, not a local State: passing a local's address to the
	// compare helper moves it to the heap (TestNoKernelLocalEscapes).
	s := new(State)
	s.InitN(tbl, decks, n, seed, Config{Mulligans: 2})
	pol := NewRandomPolicy(seed)
	var d Decision
	for step := 0; step < steps; step++ {
		if step%8 == 0 {
			src, nsl, cerr := layerSrcCompare(s, s.k())
			if cerr != nil {
				return st, fmt.Errorf("step %d: %w", step, cerr)
			}
			st.States++
			st.Sources += src
			st.Slots += nsl
		}
		s.StepInto(&d)
		if d.Kind == DkNone {
			break
		}
		s.Apply(pol.Pick(&d))
	}
	return st, nil
}

// layerSrcCompare checks the table-precomputed records against the live
// statM scan for every occupied slot.
func layerSrcCompare(s *State, k *prep) (sources, slots int, err error) {
	if k.srcs != nil {
		for m := s.statM; m.any(); m = m.rest() {
			if e := &k.srcs[s.cardAt(m.first())]; e.flags != 0 {
				sources++
			}
		}
	}
	slots, err = layerSrcCompareSlots(s, k, "masked")
	return sources, slots, err
}

// layerSrcCompareSlots is the per-slot half: lfxCollect (the production
// path) against the live per-ability scan.
func layerSrcCompareSlots(s *State, k *prep, what string) (slots int, err error) {
	for sl := Slot(0); sl < s.permHi; sl++ {
		if !s.occ.has(sl) {
			continue
		}
		var got, want [maxLfx]lfx
		ng := s.lfxCollect(k, sl, &got)
		nw := s.lfxCollectRecords(k, sl, &want)
		nw = s.lfxCollectStaticsDirect(k, sl, &want, nw)
		nw = s.lfxCollectEmblems(k, sl, &want, nw)
		if ng != nw {
			return slots, fmt.Errorf("slot %d: %s collected %d effects, direct %d", sl, what, ng, nw)
		}
		for i := 0; i < ng; i++ {
			if got[i] != want[i] {
				return slots, fmt.Errorf("slot %d effect %d: %s %+v, direct %+v", sl, i, what, got[i], want[i])
			}
		}
		slots++
	}
	return slots, nil
}
