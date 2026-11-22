package dirac

import (
	"fmt"
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

var dbgF *os.File

func dbg(format string, args ...any) {
	if dbgF == nil {
		var err error
		dbgF, err = os.Create("/home/sadams/projects/gorge/.worktrees/cli-20261121T163523Z-ea41785b/.ds4/scratch/dbg.dirac")
		if err != nil {
			panic(err)
		}
	}
	fmt.Fprintf(dbgF, format, args...)
}

// zz_scratch_ifd_test.go is a throwaway diagnostic for the Iron Fist, Hero for
// Hire options divergence (tail-08-c000 pair 43, decision 637): the kernel
// offers the hand cast while gorge does not. Delete before commit.
func TestZZScratchIronFistOffer(t *testing.T) {
	sc, id := synthScen(t, "tail-08", "Iron Fist, Hero for Hire", "Dread Specter", "Heirloom Blade", "Rain of Filth", "Mountain", "Swamp",
		"Library", "Shrine of the Forsaken Gods", "Secluded Steppe", "Magmatic Force",
		"The World Tree")
	// The World Tree plus four more Swamps: seven lands so The World Tree's
	// "six or more lands" grant ("lands you control have {T}: Add one mana of
	// any color") is live on every land
	more := []Obj{sc.perm(id["The World Tree"], 0)}
	for i := 0; i < 4; i++ {
		more = append(more, sc.perm(id["Swamp"], 0))
	}
	s := sc.s
	lib := sc.perm(id["Library"], 0)
	shr := sc.perm(id["Shrine of the Forsaken Gods"], 0)
	steppe := sc.perm(id["Secluded Steppe"], 0)
	// Board: a tapped Mountain and a tapped Swamp under p0; everything else
	// stays in the library. Hand: Iron Fist, Dread Specter, Heirloom Blade.
	mtn := sc.perm(id["Mountain"], 0)
	swp := sc.perm(id["Swamp"], 0)
	iron := sc.hand(id["Iron Fist, Hero for Hire"], 0)
	ds := sc.hand(id["Dread Specter"], 0)
	hb := sc.hand(id["Heirloom Blade"], 0)
	rain := sc.hand(id["Rain of Filth"], 0)
	s.perms[sc.slot(mtn)].flags |= pfTapped
	s.perms[sc.slot(swp)].flags |= pfTapped
	s.perms[sc.slot(lib)].flags |= pfTapped
	s.perms[sc.slot(shr)].flags |= pfTapped
	s.perms[sc.slot(steppe)].flags |= pfTapped
	for _, o := range more {
		s.perms[sc.slot(o)].flags |= pfTapped
	}
	dbg("cards: lib=%v shr=%v steppe=%v iron=%v ds=%v hb=%v rain=%v mtn=%v swp=%v\n", lib, shr, steppe, iron, ds, hb, rain, mtn, swp)

	// Sub-test A: no Rain of Filth, floating pool 0, all lands tapped.
	d := s.Step()
	opts := dumpOpts(&d)
	dbg("A (no rof): %v\n", opts)
	offerA := hasCastOf(&d, iron)

	// Sub-test B: with Rain of Filth's grant live. Float {B} from the (still
	// tapped? we untap for the activation) swamp, cast Rain of Filth, let it
	// resolve, then re-tap both lands and report.
	s.perms[sc.slot(swp)].flags &^= pfTapped
	d = s.Step()
	castRain := false
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == AkCast && Obj(a.Source()) == rain {
			s.Apply(a)
			castRain = true
			break
		}
	}
	if !castRain {
		t.Fatalf("Rain of Filth cast not offered; opts %v", dumpOpts(&d))
	}
	// pass until it resolves (priority passes; the spell resolves when both
	// seats pass)
	for i := 0; i < 20; i++ {
		if sc.onBF(rain) || s.objs[rain].zone == kop.ZoneGraveyard {
			break
		}
		d = s.Step()
		if d.Kind != DkPriority {
			break
		}
		s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	dbg("rain zone=%v\n", s.objs[rain].zone)
	s.perms[sc.slot(mtn)].flags |= pfTapped
	s.perms[sc.slot(swp)].flags |= pfTapped
	for _, o := range more {
		s.perms[sc.slot(o)].flags |= pfTapped
	}
	// the divergence ask has one untapped land (the fresh land drop): untap
	// the last Swamp so its granted "{T}: any colour" is a live tap
	untap := more[len(more)-1]
	s.perms[sc.slot(untap)].flags &^= pfTapped
	// back to p0's priority in main phase
	s.prio = 0
	s.passes = 0
	s.ctl = ctlPriority
	d = s.Step()
	optsB := dumpOpts(&d)
	dbg("B (rof live): %v\n", optsB)
	offerB := hasCastOf(&d, iron)

	// the internals: the plain-cast gate's reads for Iron Fist
	k := s.k()
	// grant wiring on the untapped land
	var gi [maxGrants]int16
	ng := s.grantIdxs(k, sc.slot(untap), &gi)
	dbg("untap slot=%d grants=%d", sc.slot(untap), ng)
	for i := 0; i < ng; i++ {
		gv := s.grantView(k.gen, gi[i], sc.slot(untap))
		dbg(" grant%d col=%x n=%d one=%v tap=%v sac=%v chosen=%v", i, gv.manaCol, gv.manaN, gv.manaOne, gv.manaTap, gv.grantSac, gv.manaChosen)
	}
	dbg(" ownManaUsable=%v\n", s.ownManaUsable(k, sc.slot(untap)))
	// the World Tree's static: is it in the state's static walk?
	wtslot := sc.slot(more[0])
	wtc := s.cardAt(wtslot)
	dbg("wt slot=%d card=%d flags=%x\n", wtslot, wtc, s.perms[wtslot].flags)
	wcd := &k.t.Cards[wtc]
	dbg("wt card name=%q abilities=%d\n", wcd.Name, len(wcd.Abilities))
	for ai := range wcd.Abilities {
		a := &wcd.Abilities[ai]
		dbg(" wt ab%d kind=%d statKind=%d grant=%d cond2=%d filter=%d manaCol=%x manaN=%d latent=%v\n",
			ai, a.Kind, a.Static.Kind, a.Static.Grant, a.Static.Cond2, a.Static.Filter, a.ManaColors, a.ManaN, a.AFlags&kop.AfLatent != 0)
	}
	s.eachStatic(k, func(st statSrc) bool {
		if s.cardAt(st.sl) == wtc {
			dbg(" wt static slot=%d kind=%d grant=%d cond=%d ctrl=%d\n", st.sl, st.a.Static.Kind, st.a.Static.Grant, st.a.Static.Cond2, st.ctrl)
		}
		return true
	})
	c := s.objs[iron].card
	o := iron
	var av avail
	s.availFor(k, 0, &av)
	dbg("census avail: total=%d n=%d masks=%v cnt=%v\n", av.total, av.n, av.mask[:8], av.cnt[:8])
	cost := s.featCost(k, 0, c, o, k.cards[c].cost)
	dbg("featCost=%v (printed %v)\n", cost, k.cards[c].cost)
	adj := s.adjustCost(k, 0, c, o, k.cards[c].cost)
	dbg("adjustCost=%v\n", adj)
	dbg("castGateOrPoolPays=%v gorgeOffers=%v plainHandPlayLowers=%v dhWithholds=%v\n",
		s.castGateOrPoolPays(k, 0, c, o, &adj, s.castGrantAvail(k, 0, o, &av, new(avail))),
		s.gorgeOffers(0, o, 0), s.plainHandPlayLowers(k, 0, c, o), s.dhWithholdsCast(k, 0, o, &cost))
	dbg("tgtOK=%v spellCostsFeasible=%v castHeldOut=%v altAddCastable=%v xBound=%v spellTgtDep=%v\n",
		s.spellTargetsFeasibleFrom(k, c, 0, o), s.spellCostsFeasible(k, c, 0, o), s.castHeldOut(o),
		s.altAddCastableFrom(k, c, 0, o, &k.cards[c].cost, &k.cards[c].cost, false, false, &av),
		s.spellXBoundPayable(k, c, 0, &k.cards[c].cost), s.spellTgtDep(k, 0, c, o))
	if offerA || offerB {
		t.Fatalf("the kernel offered the hand cast (A=%v B=%v)", offerA, offerB)
	}
}

// dumpOpts renders a decision's options one a line.
func dumpOpts(d *Decision) string {
	out := ""
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		out += fmt.Sprintf("\n  %d src=%d tgt=%d mode=%d", a.Kind(), a.Source(), a.Target(), a.Mode())
	}
	return out
}

// hasCastOf reports whether d offers a plain hand cast of o.
func hasCastOf(d *Decision, o Obj) bool {
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == AkCast && Obj(a.Source()) == o && a.Mode() == 0 {
			return true
		}
	}
	return false
}

var _ = cardset.Entry{}
