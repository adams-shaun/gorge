package dirac

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

func TestDiscardCostIsChosen(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	sc.lands(0, v.Mountain, 2)
	th := sc.hand(v.ThrillOfPossibility, 0)
	a := sc.hand(v.GrizzlyBears, 0)
	b := sc.hand(v.HillGiant, 0)
	sc.do(t, AkCast, uint8(th), 0xff)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.N != 2 {
		t.Fatalf("discard-cost decision kind %d n %d", d.Kind, d.N)
	}
	sc.s.Apply(MakeAction(AkChoose, TCode(b), 0, 0))
	sc.passBoth(t)
	sc.s.Step()
	if sc.s.objs[b].zone != kop.ZoneGraveyard || sc.s.objs[a].zone != kop.ZoneHand {
		t.Fatalf("wrong card discarded")
	}
	if sc.s.pl[0].handN != 3 { // bears + 2 drawn
		t.Fatalf("hand %d", sc.s.pl[0].handN)
	}
}

func TestSacrificeCostIsChosen(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	sc.lands(0, v.Swamp, 1)
	g := sc.perm(v.HungryGhoul, 0)
	x := sc.perm(v.GrizzlyBears, 0)
	y := sc.perm(v.SavannahLions, 0)
	sc.do(t, AkActivate, uint8(g), 0xff)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.N != 2 { // the ghoul itself is not "another creature"
		t.Fatalf("sac-cost decision kind %d n %d", d.Kind, d.N)
	}
	sc.s.Apply(MakeAction(AkChoose, TCode(y), 0, 0))
	sc.passBoth(t)
	sc.s.Step()
	if sc.onBF(y) || !sc.onBF(x) || sc.s.perms[sc.slot(g)].ctrLow(int(kop.CtrP1P1)) != 1 {
		t.Fatalf("sacrifice cost wrong")
	}
}

func TestEitherOrAdditionalCost(t *testing.T) {
	sc := newScen(t, Config{NoElide: true, GorgeAltAdd: true})
	sc.lands(0, v.Swamp, 5)
	ea := sc.hand(v.EatenAlive, 0)
	tg := sc.perm(v.GrizzlyBears, 1)
	sc.do(t, AkCast, uint8(ea), 0xff)
	d := sc.s.Step()
	// no creature of ours to sacrifice: the {3}{B} part first, then the
	// unpayable sacrifice part
	if d.Kind != DkChoose || d.Sub != ChAltAdd || d.N != 2 || d.Options[0].Target() != 1 || d.Options[1].Target() != 0 {
		t.Fatalf("altadd decision %v", d.Options[:d.N])
	}
	sc.s.Apply(d.Options[0])
	sc.do(t, AkTarget, 0xff, uint8(tg))
	sc.s.Step()
	n := 0
	for m := sc.s.ctrlM[0]; m.any(); m = m.rest() {
		if sc.s.perms[m.first()].flags&pfTapped != 0 {
			n++
		}
	}
	if n != 5 {
		t.Fatalf("%d lands tapped, want 5 ({B} + {3}{B})", n)
	}
	// choosing the unpayable part undoes the cast
	sc2 := newScen(t, Config{NoElide: true, GorgeAltAdd: true})
	sc2.lands(0, v.Swamp, 1)
	ea2 := sc2.hand(v.EatenAlive, 0)
	sc2.perm(v.GrizzlyBears, 1)
	sc2.perm(v.SavannahLions, 0)
	sc2.do(t, AkCast, uint8(ea2), 0xff)
	d = sc2.s.Step()
	if d.N != 2 || d.Options[0].Target() != 0 {
		t.Fatalf("altadd decision %v", d.Options[:d.N])
	}
	sc2.s.Apply(d.Options[1]) // {3}{B}: unpayable
	if d := sc2.s.Step(); d.Kind != DkPriority || sc2.s.objs[ea2].zone != kop.ZoneHand {
		t.Fatalf("unpayable part did not undo the cast")
	}
	// a targeted cast never reaches gorge's mana window: the target menu
	// withholds every candidate of an unpayable cast and the cast aborts at
	// the target ask, so nothing is tapped and no mana floats (FDN pair 1228)
	if tapped, floating := sc2.tappedAndFloating(0); tapped != 0 || floating != 0 {
		t.Fatalf("the undone targeted cast tapped %d permanents and left %d mana floating", tapped, floating)
	}
}

// Without Config.GorgeAltAdd the ask lists the payable parts only, and a
// spell no part of which can be paid in full is not offered.
func TestEitherOrPayableOnly(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	sc.lands(0, v.Swamp, 1)
	ea := sc.hand(v.EatenAlive, 0)
	sc.perm(v.GrizzlyBears, 1)
	sc.perm(v.SavannahLions, 0)
	sc.do(t, AkCast, uint8(ea), 0xff)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChAltAdd || d.N != 1 || d.Options[0].Target() != 0 {
		t.Fatalf("altadd decision %v, want the sacrifice part alone", d.Options[:d.N])
	}
	// one Swamp and no creature to sacrifice: neither part can be paid
	sc2 := newScen(t, Config{NoElide: true})
	sc2.lands(0, v.Swamp, 1)
	ea2 := sc2.hand(v.EatenAlive, 0)
	sc2.perm(v.GrizzlyBears, 1)
	d = sc2.s.Step()
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == AkCast && d.Options[i].Source() == TCode(uint8(ea2)) {
			t.Fatalf("an unpayable either-or spell is offered")
		}
	}
}

// TestFreeCastPaysAltAddMana: a spell cast without paying its mana cost
// (Etali, Primal Storm) still pays the {3}{B} part of Eaten Alive's
// either-or additional cost (CR 118.9d: only the mana cost is skipped;
// the pair 9954). Before, buildCostGen returned an empty cost.
func TestFreeCastPaysAltAddMana(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Eaten Alive", Count: 1}, {Name: "Swamp", Count: 8}})
	ea := sc.hand(id["Eaten Alive"], 0)
	*sc.s.bld() = build{e: stackEntry{kind: seSpell, obj: ea, card: id["Eaten Alive"], ctrl: 0, flags: sfFree, trigObj: NoObj}}
	sc.s.bld().altAdd = 1
	if c := sc.s.buildCost(sc.k); c.Generic != 3 || c.Pips != [6]uint8{0, 0, 1} {
		t.Fatalf("free Eaten Alive with the {3}{B} part costs %+v, want {3}{B}", c)
	}
	sc.s.bld().altAdd = 0
	if c := sc.s.buildCost(sc.k); c != (kop.ManaCost{}) {
		t.Fatalf("free Eaten Alive with the sacrifice part costs %+v, want nothing", c)
	}
}

// TestEitherOrUndoFromGraveyard: choosing the unpayable part of Eaten
// Alive's either-or cost on a flashback cast reverses the cast, and gorge
// moves the card from the stack back to the END of its owner's graveyard
// (the pair 2359), as a hand cast goes back to the end of the hand.
func TestEitherOrUndoFromGraveyard(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Eaten Alive", Count: 1}, {Name: "Sphinx of Forgotten Lore", Count: 1}, {Name: "Burglar Rat", Count: 2}, {Name: "Swamp", Count: 8}})
	sc.s.cfg |= cfgGorgeAltAdd // gorge lists the unpayable part after the payable one
	sc.perm(id["Swamp"], 0)
	sc.perm(id["Burglar Rat"], 0) // part 0 (the sacrifice) payable: with neither part payable gorge aborts with no ask
	sc.perm(id["Burglar Rat"], 1)
	ea := sc.obj(id["Eaten Alive"], 0)
	sc.s.place(ea, kop.ZoneGraveyard, false)
	rat := sc.obj(id["Burglar Rat"], 0)
	sc.s.place(rat, kop.ZoneGraveyard, false)
	sc.s.grantFB(sc.k, ea)
	pl := &sc.s.pl[0]
	if pl.graveN != 2 || pl.grave[0] != ea {
		t.Fatalf("graveyard %v", pl.grave[:pl.graveN])
	}
	sc.stepTo(t, 1, func() bool { return false }, func(d *Decision) (Action, bool) {
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(ea)) {
				return a, true
			}
		}
		t.Fatalf("flashback Eaten Alive not offered: %v", d.Options[:d.N])
		return 0, false
	})
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChAltAdd || d.N != 2 {
		t.Fatalf("altadd decision kind %d %v, want both parts", d.Kind, d.Options[:d.N])
	}
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Target() == 1 {
			sc.s.Apply(d.Options[i])
		}
	}
	if d := sc.s.Step(); d.Kind != DkPriority || sc.s.objs[ea].zone != kop.ZoneGraveyard {
		t.Fatalf("the unpayable part did not reverse the cast: kind %d zone %d", d.Kind, sc.s.objs[ea].zone)
	}
	if pl.graveN != 2 || pl.grave[1] != ea {
		t.Fatalf("graveyard %v, want Eaten Alive moved to the end", pl.grave[:pl.graveN])
	}
	// the granted flashback survives the reversal: gorge offers it again;
	// after a second reversal in a row it holds the cast out (castHeldOut)
	offered := func() bool {
		d := sc.s.Step()
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(ea)) {
				sc.s.Apply(a)
				return true
			}
		}
		return false
	}
	if !offered() {
		t.Fatal("flashback Eaten Alive not offered again")
	}
	d = sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Target() == 1 {
			sc.s.Apply(d.Options[i])
		}
	}
	if offered() {
		t.Fatal("flashback Eaten Alive offered after two reversals in a row")
	}
}

// TestLoweredCastPaysAltAddThroughWindow: a cast gorge's planner does not
// price (a granted flashback Eaten Alive) is lowered: the oracle taps the
// witness for its printed {B} at priority and casts from the floating
// pool, so gorge's cast has no payment plan and the {3}{B} part of the
// either-or cost chosen after it is paid through the mana window, which
// activates the first untapped source in battlefield order while the pool
// cannot pay (pair 4512: the Swamps first in the list, not the planner's
// Island and later Swamps).
func TestLoweredCastPaysAltAddThroughWindow(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Eaten Alive", Count: 1}, {Name: "Sphinx of Forgotten Lore", Count: 1}, {Name: "Burglar Rat", Count: 2}, {Name: "Swamp", Count: 8}, {Name: "Island", Count: 8}})
	sc.s.cfg |= cfgGorgeAutopay
	// the Islands' objects are older than the Swamps' (pair 4512: gorge's
	// planner ranks by object, the window walks the battlefield list)
	var isl, sw []Obj
	for i := 0; i < 3; i++ {
		isl = append(isl, sc.obj(id["Island"], 0))
	}
	for i := 0; i < 5; i++ {
		sw = append(sw, sc.obj(id["Swamp"], 0))
	}
	lands := []Obj{sw[0], sw[1], sw[2], isl[0], sw[3], isl[1], sw[4], isl[2]}
	for _, o := range lands {
		sl := sc.s.enter(sc.k, o, 0, false, kop.ZoneNone)
		sc.s.perms[sl].flags &^= pfSick
		sc.s.npend = 0
	}
	sc.perm(id["Burglar Rat"], 1)
	ea := sc.obj(id["Eaten Alive"], 0)
	sc.s.place(ea, kop.ZoneGraveyard, false)
	sc.s.grantFB(sc.k, ea)
	sc.stepTo(t, 1, func() bool { return false }, func(d *Decision) (Action, bool) {
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(ea)) {
				return a, true
			}
		}
		t.Fatalf("flashback Eaten Alive not offered: %v", d.Options[:d.N])
		return 0, false
	})
	for i := 0; i < 4; i++ {
		d := sc.s.Step()
		if d.Kind == DkPriority {
			break
		}
		a := d.Options[0]
		if d.Kind == DkChoose && d.Sub == ChAltAdd {
			for j := 0; j < int(d.N); j++ {
				if d.Options[j].Target() == 1 {
					a = d.Options[j]
				}
			}
		}
		sc.s.Apply(a)
	}
	if sc.s.objs[ea].zone != kop.ZoneStack {
		t.Fatalf("Eaten Alive not cast: zone %d", sc.s.objs[ea].zone)
	}
	var tapped []int
	for i, o := range lands {
		if sc.s.perms[sc.s.objs[o].slot].flags&pfTapped != 0 {
			tapped = append(tapped, i)
		}
	}
	// the witness's Swamp, then the window: the next three Swamps (the
	// pips) and the first Island in the list (the generic)
	if fmt.Sprint(tapped) != "[0 1 2 3 4]" {
		t.Fatalf("tapped lands %v, want [0 1 2 3 4] (the window's battlefield order)", tapped)
	}
}

// TestLoweredEitherOrManaPart: with no creature to sacrifice, Eaten Alive from the
// hand is offered once five lands pay {B} and the {3}{B} part (CR 601.2f), and the
// witness taps all of them; four lands do not pay it.
func TestLoweredEitherOrManaPart(t *testing.T) {
	for _, lands := range []int{4, 5} {
		sc, id := newObjScen(t, []cardset.Entry{{Name: "Eaten Alive", Count: 1}, {Name: "Swamp", Count: 8}, {Name: "Burglar Rat", Count: 1}})
		sc.s.cfg |= cfgGorgeLowering
		var swamps []Obj
		for i := 0; i < lands; i++ {
			swamps = append(swamps, sc.perm(id["Swamp"], 0))
		}
		sc.perm(id["Burglar Rat"], 1)
		ea := sc.obj(id["Eaten Alive"], 0)
		sc.s.place(ea, kop.ZoneHand, false)
		d := sc.s.Step()
		var cast Action
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(ea)) {
				cast = a
			}
		}
		if (cast != 0) != (lands == 5) {
			t.Fatalf("%d lands: offered %v: %v", lands, cast != 0, d.Options[:d.N])
		}
		if cast == 0 {
			continue
		}
		sc.s.Apply(cast)
		for _, o := range swamps {
			if sc.s.perms[sc.s.objs[o].slot].flags&pfTapped == 0 {
				t.Fatalf("Swamp %d is untapped after the cast", o)
			}
		}
	}
}

// TestLoweredStashCastPaysThroughWindow: the same for a Tinybones stash
// cast of the opponent's Eaten Alive (mana of any type): the lowered cast
// taps no printed-symbol plan at payment (tapPrinted is for a planned
// cast); the window activates the first untapped sources in battlefield
// order, Plains included (pair 7247).
func TestLoweredStashCastPaysThroughWindow(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Tinybones, Bauble Burglar", Count: 1}, {Name: "Eaten Alive", Count: 1}, {Name: "Burglar Rat", Count: 2}, {Name: "Swamp", Count: 8}, {Name: "Plains", Count: 8}})
	sc.s.cfg |= cfgGorgeAutopay | cfgGorgeUnpriced
	var pl, sw []Obj
	for i := 0; i < 4; i++ {
		pl = append(pl, sc.obj(id["Plains"], 0))
	}
	for i := 0; i < 3; i++ {
		sw = append(sw, sc.obj(id["Swamp"], 0))
	}
	lands := []Obj{pl[3], sw[0], pl[2], sw[1], pl[1], pl[0], sw[2]}
	for _, o := range lands {
		sl := sc.s.enter(sc.k, o, 0, false, kop.ZoneNone)
		sc.s.perms[sl].flags &^= pfSick
		sc.s.npend = 0
	}
	sc.perm(id["Tinybones, Bauble Burglar"], 0)
	sc.perm(id["Burglar Rat"], 1)
	ea := sc.obj(id["Eaten Alive"], 1)
	sc.s.place(ea, kop.ZoneExile, false)
	sc.s.stashSet(sc.k, ea)
	sc.stepTo(t, 1, func() bool { return false }, func(d *Decision) (Action, bool) {
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(ea)) {
				return a, true
			}
		}
		t.Fatalf("stashed Eaten Alive not offered: %v", d.Options[:d.N])
		return 0, false
	})
	for i := 0; i < 4; i++ {
		d := sc.s.Step()
		if d.Kind == DkPriority {
			break
		}
		a := d.Options[0]
		if d.Kind == DkChoose && d.Sub == ChAltAdd {
			for j := 0; j < int(d.N); j++ {
				if d.Options[j].Target() == 1 {
					a = d.Options[j]
				}
			}
		}
		sc.s.Apply(a)
	}
	if sc.s.objs[ea].zone != kop.ZoneStack {
		t.Fatalf("Eaten Alive not cast: zone %d", sc.s.objs[ea].zone)
	}
	var tapped []int
	for i, o := range lands {
		if sc.s.perms[sc.s.objs[o].slot].flags&pfTapped != 0 {
			tapped = append(tapped, i)
		}
	}
	if fmt.Sprint(tapped) != "[0 1 2 3 4]" {
		t.Fatalf("tapped lands %v, want [0 1 2 3 4] (the window's battlefield order)", tapped)
	}
}

// TestFreeCastEitherOrNoPayablePart: a free cast of Eaten Alive (Etali's,
// Hurl into History's Discover) with neither either-or part payable is
// reversed at the ask with no decision posed and nothing tapped (gorge
// altAddAsk, CR 733.1), the card back at the end of the hand. Four untapped
// Swamps pay the {3}{B} part at the window, so with them the ask is posed;
// three don't.
func TestFreeCastEitherOrNoPayablePart(t *testing.T) {
	for _, tc := range []struct {
		name   string
		swamps int
		ask    bool
	}{{"three", 3, false}, {"four", 4, true}} {
		t.Run(tc.name, func(t *testing.T) {
			sc, id := newObjScen(t, []cardset.Entry{{Name: "Eaten Alive", Count: 1}, {Name: "Swamp", Count: 8}})
			swamps := make([]Obj, tc.swamps)
			for i := range swamps {
				swamps[i] = sc.perm(id["Swamp"], 0)
			}
			ea := sc.hand(id["Eaten Alive"], 0)
			sc.s.cfg |= cfgGorgeLowering
			*sc.s.bld() = build{e: stackEntry{kind: seSpell, obj: ea, card: id["Eaten Alive"], ctrl: 0, flags: sfFree, trigObj: NoObj}}
			sc.s.ctl, sc.s.bld().stage = ctlBuild, bsAltAdd
			d := sc.s.Step()
			if tc.ask {
				if d.Kind != DkChoose || d.Sub != ChAltAdd {
					t.Fatalf("decision kind %d sub %d, want the altadd ask", d.Kind, d.Sub)
				}
				return
			}
			if d.Kind != DkPriority || sc.s.ctl != ctlPriority {
				t.Fatalf("decision kind %d ctl %d, want priority with the cast reversed", d.Kind, sc.s.ctl)
			}
			if pl := &sc.s.pl[0]; sc.s.objs[ea].zone != kop.ZoneHand || pl.hand[pl.handN-1] != ea {
				t.Fatalf("Eaten Alive zone %d, want the end of the hand", sc.s.objs[ea].zone)
			}
			for _, sw := range swamps {
				if sc.tapped(sw) {
					t.Fatalf("a Swamp was tapped on the aborted cast")
				}
			}
		})
	}
}

// TestFreeCastEitherOrFloatingPoolPays: the floating pool is what gorge's
// altAddAsk prices the {3}{B} part against, so a free cast of Eaten Alive
// with {3}{B} floating still poses the ask under the gorge configuration
// (the counterpart of TestFreeCastEitherOrNoPayablePart).
func TestFreeCastEitherOrFloatingPoolPays(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Eaten Alive", Count: 1}, {Name: "Swamp", Count: 8}})
	for i := 0; i < 4; i++ {
		sc.perm(id["Swamp"], 0)
	}
	ea := sc.hand(id["Eaten Alive"], 0)
	sc.s.cfg |= cfgGorgeLowering
	sc.s.pl[0].pool[kop.B] = 1
	sc.s.pl[0].pool[kop.C] = 3
	*sc.s.bld() = build{e: stackEntry{kind: seSpell, obj: ea, card: id["Eaten Alive"], ctrl: 0, flags: sfFree, trigObj: NoObj}}
	sc.s.ctl, sc.s.bld().stage = ctlBuild, bsAltAdd
	if d := sc.s.Step(); d.Kind != DkChoose || d.Sub != ChAltAdd {
		t.Fatalf("decision kind %d sub %d, want the altadd ask (the pool pays the mana part)", d.Kind, d.Sub)
	}
}
