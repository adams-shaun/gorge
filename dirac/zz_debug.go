package dirac

// Throwaway diagnostic (deleted before commit): exported for a debug test in
// dirac/oracle.

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

func DebugDumpBedlam(s *State) string {
	k := s.k()
	var out strings.Builder
	var br CardID
	for c := CardID(0); c < CardID(len(k.t.Cards)); c++ {
		if k.t.CardName(c) == "Bedlam Reveler" {
			br = c
			break
		}
	}
	if br < 0 {
		return "no Bedlam Reveler in table\n"
	}
	cd := &k.t.Cards[br]
	fmt.Fprintf(&out, "p0 turn=%d\n", s.turn)
	fmt.Fprintf(&out, "  cost=%v\n", cd.Cost)
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatReduceCost {
			fmt.Fprintf(&out, "  static kind=%v stflags=%x zone=%v A=%d VA=%+v cond2=%d F=%d F2=%d\n",
				a.Static.Kind, a.Static.StFlags, a.Static.Zone, a.Static.A, a.Static.VA, a.Static.Cond2, a.Static.Filter, a.Static.Filter2)
		}
	}
	for p := uint8(0); p < 2; p++ {
		pl := &s.pl[p]
		fmt.Fprintf(&out, "  grave p%d n=%d:", p, pl.graveN)
		for j := 0; j < int(pl.graveN); j++ {
			o := pl.grave[j]
			fmt.Fprintf(&out, " %s(o%d)", k.t.CardName(s.objs[o].card), o)
		}
		fmt.Fprintln(&out)
	}
	// the reveler's hand object and repricing
	var ho Obj = NoObj
	for i := 0; i < int(s.pl[0].handN); i++ {
		o := s.pl[0].hand[i]
		if s.objs[o].card == br {
			ho = o
		}
	}
	fmt.Fprintf(&out, "  handobj=%v ownerbits=%x zone=%d\n", ho, s.objs[ho].acts, s.objs[ho].zone)
	adj := s.adjustCostGen(k, 0, br, ho, cd.Cost)
	fmt.Fprintf(&out, "  adjCost=%v\n", adj)
	var av avail
	s.availFor(k, 0, &av)
	fmt.Fprintf(&out, "  avail=%+v\n", av)
	fit := s.adjustFit(k, &av, 0, br, ho, cd.Cost, false)
	fmt.Fprintf(&out, "  fit=%v\n", fit)
	fav := avail{}
	cav := s.castGrantAvail(k, 0, ho, &av, &fav)
	gate := s.castGateOrPoolPays(k, 0, br, ho, &fit, cav)
	fmt.Fprintf(&out, "  gate=%v\n", gate)
	tgt := s.spellTargetsFeasibleFrom(k, br, 0, ho)
	fmt.Fprintf(&out, "  targets=%v\n", tgt)
	sc := s.spellCostsFeasible(k, br, 0, ho)
	fmt.Fprintf(&out, "  costsFeasible=%v\n", sc)
	aa := s.altAddCastableFrom(k, br, 0, ho, &adj, &cd.Cost, false, false, &av)
	fmt.Fprintf(&out, "  altAdd=%v\n", aa)
	fmt.Fprintf(&out, "  heldOut=%v gorgeOffers=%v\n", s.castHeldOut(ho), s.gorgeOffers(0, ho, 0))
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatReduceCost && a.Static.StFlags&kop.StDynamic != 0 {
			v := s.evalV(k, a.Static.VA, evalCtx{src: ho, srcSl: NoSlot, ctrl: 0})
			fmt.Fprintf(&out, "  evalV=%d\n", v)
		}
	}
	return out.String()
}

var debugBedlamTrace bool

// ZzSetTrace flips the throwaway bedlam trace (called from package oracle's
// throwaway dump test).
func ZzSetTrace(v bool) { debugBedlamTrace = v }
