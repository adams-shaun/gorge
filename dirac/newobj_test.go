package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

func newObjScen(t *testing.T, deck []cardset.Entry) (*scen, map[string]CardID) {
	t.Helper()
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	return newScenTable(t, tbl, Config{NoElide: true}), id
}

// stepTo applies, at each decision, the first option matching pick (or
// the first option) until done reports true or n decisions pass.
func (sc *scen) stepTo(t *testing.T, n int, done func() bool, pick func(d *Decision) (Action, bool)) {
	t.Helper()
	for i := 0; i < n && !done(); i++ {
		d := sc.s.Step()
		if d.Kind == DkNone {
			return
		}
		a := d.Options[0]
		if pick != nil {
			if b, ok := pick(&d); ok {
				a = b
			}
		}
		sc.s.Apply(a)
	}
}

// TestTriggerExileFindsNoNewObject: Tinybones' "exile it from their
// graveyard" names the discarded card; once that card has returned itself
// to the battlefield (Reassembling Skeleton, in response) it is a new
// object and stays there (CR 400.7). Before, the kernel exiled the
// Skeleton from the battlefield (pair 2469).
func TestTriggerExileFindsNoNewObject(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Tinybones, Bauble Burglar", Count: 1}, {Name: "Reassembling Skeleton", Count: 1}, {Name: "Swamp", Count: 8}})
	sc.perm(id["Tinybones, Bauble Burglar"], 0)
	for i := 0; i < 4; i++ {
		sc.perm(id["Swamp"], 0)
	}
	sc.perm(id["Swamp"], 1)
	sc.perm(id["Swamp"], 1)
	sk := sc.hand(id["Reassembling Skeleton"], 1)
	pass := func(d *Decision) (Action, bool) {
		for i := 0; i < int(d.N); i++ {
			if d.Options[i].Kind() == AkPass {
				return d.Options[i], true
			}
		}
		return 0, false
	}
	// p0 activates Tinybones' discard ability and both pass: p1 discards
	// the Skeleton, which triggers Tinybones
	sc.stepTo(t, 1, func() bool { return false }, func(d *Decision) (Action, bool) {
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkActivate {
				return a, true
			}
		}
		t.Fatalf("Tinybones' ability not offered: kind %d %v", d.Kind, d.Options[:d.N])
		return 0, false
	})
	sc.stepTo(t, 10, func() bool { return sc.s.objs[sk].zone == kop.ZoneGraveyard }, pass)
	if sc.s.objs[sk].zone != kop.ZoneGraveyard {
		t.Fatalf("Skeleton zone %d, want it discarded", sc.s.objs[sk].zone)
	}
	// p1 answers the trigger by returning the Skeleton
	sc.stepTo(t, 10, func() bool { return sc.s.objs[sk].zone == kop.ZoneBattlefield || sc.s.objs[sk].zone == kop.ZoneExile }, func(d *Decision) (Action, bool) {
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkActivate && a.Source() == TCode(uint8(sk)) {
				return a, true
			}
		}
		return pass(d)
	})
	sc.stepTo(t, 10, func() bool { return sc.s.sp == 0 && sc.s.ctl == ctlPriority }, pass)
	if z := sc.s.objs[sk].zone; z != kop.ZoneBattlefield {
		t.Fatalf("Skeleton zone %d, want the battlefield", z)
	}
}

// TestTriggerExileStashNeedsGraveyard: Tinybones' "exile it from their
// graveyard with a stash counter" does nothing once Abyssal Harvester has
// exiled the discarded card in response: the card in exile is a new object
// and gets no stash counter (CR 400.7, 608.2b; the pairs 4177 4280).
func TestTriggerExileStashNeedsGraveyard(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Tinybones, Bauble Burglar", Count: 1}, {Name: "Abyssal Harvester", Count: 1}, {Name: "Burglar Rat", Count: 1}, {Name: "Swamp", Count: 8}})
	sc.perm(id["Tinybones, Bauble Burglar"], 0)
	hv := sc.perm(id["Abyssal Harvester"], 0)
	for i := 0; i < 4; i++ {
		sc.perm(id["Swamp"], 0)
	}
	rat := sc.hand(id["Burglar Rat"], 1)
	pass := func(d *Decision) (Action, bool) {
		for i := 0; i < int(d.N); i++ {
			if d.Options[i].Kind() == AkPass {
				return d.Options[i], true
			}
		}
		return 0, false
	}
	sc.stepTo(t, 1, func() bool { return false }, func(d *Decision) (Action, bool) {
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkActivate && a.Source() != TCode(uint8(hv)) {
				return a, true
			}
		}
		t.Fatalf("Tinybones' ability not offered: kind %d %v", d.Kind, d.Options[:d.N])
		return 0, false
	})
	sc.stepTo(t, 10, func() bool { return sc.s.objs[rat].zone == kop.ZoneGraveyard }, pass)
	if sc.s.objs[rat].zone != kop.ZoneGraveyard {
		t.Fatalf("Rat zone %d, want it discarded", sc.s.objs[rat].zone)
	}
	// the trigger is on the stack: p0 exiles the Rat with the Harvester
	sc.stepTo(t, 10, func() bool { return sc.s.objs[rat].zone == kop.ZoneExile }, func(d *Decision) (Action, bool) {
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkActivate && a.Source() == TCode(uint8(hv)) {
				return a, true
			}
		}
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; d.Kind == DkTarget && a.Target() == TCode(uint8(rat)) {
				return a, true
			}
		}
		return pass(d)
	})
	if sc.s.objs[rat].zone != kop.ZoneExile {
		t.Fatalf("Rat zone %d, want the Harvester to exile it", sc.s.objs[rat].zone)
	}
	sc.stepTo(t, 20, func() bool { return sc.s.sp == 0 && sc.s.ctl == ctlPriority }, pass)
	if sc.s.sp != 0 {
		t.Fatalf("stack not empty: %d", sc.s.sp)
	}
	if sc.s.mObj(sc.k, mechStash, rat) {
		t.Fatal("the Rat exiled by the Harvester got Tinybones' stash counter")
	}
}

// TestLegendRuleByName: the legend rule groups legendary permanents by
// NAME (CR 704.5j): a Tinybones wearing Witness Protection is named
// Legitimate Businessperson, so a second Tinybones is no pair (the pair
// 28, Kiora and its token copy). Before, the kernel compared card ids and
// asked which to keep.
func TestLegendRuleByName(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Tinybones, Bauble Burglar", Count: 2}, {Name: "Witness Protection", Count: 1}, {Name: "Swamp", Count: 8}})
	t1 := sc.perm(id["Tinybones, Bauble Burglar"], 0)
	wp := sc.perm(id["Witness Protection"], 0)
	sc.s.perms[sc.slot(wp)].attached = sc.slot(t1)
	sc.s.loseAllOnset(sc.k, sc.slot(wp), sc.slot(t1))
	sc.perm(id["Tinybones, Bauble Burglar"], 0)
	if d := sc.s.Step(); d.Kind == DkChoose && d.Sub == ChLegend {
		t.Fatalf("legend rule asked for differently named permanents: %v", d.Options[:d.N])
	}
	if nc, ok := sc.s.FeatNameCard(t1); !ok || nc == id["Tinybones, Bauble Burglar"] {
		t.Fatalf("the Witness Protection name did not apply (%d %v)", nc, ok)
	}
}

// TestControlEOTReturnsToPrevious: Involuntary Employment's "gain control
// until end of turn" of p0's own Burglar Rat that p1 controls (Rise of the
// Dark Realms) ends at cleanup with the Rat back under p1, its previous
// controller, not p0, its owner (CR 514.2, 613.1b; the pair 858).
func TestControlEOTReturnsToPrevious(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Involuntary Employment", Count: 1}, {Name: "Burglar Rat", Count: 1}, {Name: "Mountain", Count: 8}})
	for i := 0; i < 4; i++ {
		sc.perm(id["Mountain"], 0)
	}
	rat := sc.obj(id["Burglar Rat"], 0)
	sc.s.perms[sc.s.enter(sc.k, rat, 1, false, kop.ZoneNone)].flags &^= pfSick
	sc.s.npend = 0
	ie := sc.hand(id["Involuntary Employment"], 0)
	turn := sc.s.turn
	sc.stepTo(t, 40, func() bool { return sc.s.turn != turn }, func(d *Decision) (Action, bool) {
		for i := 0; i < int(d.N); i++ {
			a := d.Options[i]
			if a.Kind() == AkCast && a.Source() == TCode(uint8(ie)) || d.Kind == DkTarget && a.Target() == TCode(uint8(rat)) {
				return a, true
			}
		}
		for i := 0; i < int(d.N); i++ {
			if d.Options[i].Kind() == AkPass {
				return d.Options[i], true
			}
		}
		return 0, false
	})
	if sc.s.objs[ie].zone != kop.ZoneGraveyard {
		t.Fatalf("Involuntary Employment zone %d, want it resolved", sc.s.objs[ie].zone)
	}
	if c := sc.s.perms[sc.slot(rat)].ctrl; sc.s.turn == turn || c != 1 {
		t.Fatalf("turn %d (from %d): the Rat's controller is p%d, want p1", sc.s.turn, turn, c)
	}
}

// TestProjectPowerPast127: a creature's projected power holds more than
// 127 (Mossborn Hydra doubling its +1/+1 counters, pair 5419: gorge 128,
// the kernel's int8 projection wrapped to -128).
func TestProjectPowerPast127(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Burglar Rat", Count: 1}, {Name: "Swamp", Count: 8}})
	rat := sc.perm(id["Burglar Rat"], 0)
	sc.s.perms[sc.slot(rat)].setCtrN(sc.s, int(kop.CtrP1P1), 130)
	var p Projection
	sc.s.Project(&p)
	if o := p.Objs[rat]; o.Power != 131 || o.Tough != 131 {
		t.Fatalf("projected %d/%d, want 131/131", o.Power, o.Tough)
	}
}

// TestAllCreatureTypesNotForest: an animated Soulstone Sanctuary has every
// creature type, but Forest is a land type (CR 205.3m), so it counts for
// no "Forest you control" (Blanchwood Armor: the pair 4682, kernel 7
// against gorge 6).
func TestAllCreatureTypesNotForest(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Soulstone Sanctuary", Count: 1}, {Name: "Blanchwood Armor", Count: 1}, {Name: "Forest", Count: 8}})
	ss := sc.perm(id["Soulstone Sanctuary"], 0)
	for i := 0; i < 4; i++ {
		sc.perm(id["Forest"], 0)
	}
	sc.stepTo(t, 12, func() bool { return sc.s.isCreature(sc.k, sc.slot(ss)) }, func(d *Decision) (Action, bool) {
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkActivate && a.Source() == TCode(uint8(ss)) && a.Mode() == 1 {
				return a, true
			}
		}
		for i := 0; i < int(d.N); i++ {
			if d.Options[i].Kind() == AkPass {
				return d.Options[i], true
			}
		}
		return 0, false
	})
	if !sc.s.isCreature(sc.k, sc.slot(ss)) {
		t.Skip("Soulstone Sanctuary did not animate in the scenario")
	}
	forest, elf := -1, -1
	for i, n := range sc.k.t.Subtypes {
		switch n {
		case "Forest":
			forest = i
		case "Elf":
			elf = i
		}
	}
	if forest < 0 {
		t.Skip("no Forest subtype in the table")
	}
	if sc.s.hasSubGen(sc.k, sc.slot(ss), uint8(forest)) {
		t.Fatal("the animated Sanctuary is a Forest")
	}
	if elf >= 0 && !sc.s.hasSubGen(sc.k, sc.slot(ss), uint8(elf)) {
		t.Fatal("the animated Sanctuary lost its creature types")
	}
}

// TestControlEOTEndsInOrder: two "until end of turn" control changes end
// at cleanup in the order they were made (gorge's continuous effects
// order), so the permanents rejoin their controller's battlefield list in
// the order they were taken, not in slot order (two Involuntary
// Employments: pair 8824, Erudite Wizard before Strix Lookout).
func TestControlEOTEndsInOrder(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Burglar Rat", Count: 2}, {Name: "Swamp", Count: 8}})
	r1 := sc.perm(id["Burglar Rat"], 0)
	r2 := sc.perm(id["Burglar Rat"], 0)
	steal := func(o Obj) { // OpGainControlEOT's control record (CR 613.1b)
		sc.s.gainControl(sc.slot(o), 1, 1, kop.DurEOT, NoObj, 0, false)
	}
	steal(r2)
	steal(r1)
	sc.s.finishCleanup(sc.k)
	var order [MaxPerm]Slot
	n := sc.s.battlefieldOrder(0, sc.s.ctrlM[0], order[:], 0)
	var got []Obj
	for _, sl := range order[:n] {
		if o := sc.s.perms[sl].obj; o == r1 || o == r2 {
			got = append(got, o)
		}
	}
	if len(got) != 2 || got[0] != r2 {
		t.Fatalf("p0's Rats in battlefield order %v, want %d (taken first) then %d", got, r2, r1)
	}
}

// TestDamageTriggerGoneAbilitySource: Niv-Mizzet, Visionary's "a source you
// control deals noncombat damage to an opponent" sees the ping of a
// sacrificed Fanatical Firebrand: the ability's source is in the
// graveyard, and gorge reads its last known information (pair 3334).
func TestDamageTriggerGoneAbilitySource(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Niv-Mizzet, Visionary", Count: 1}, {Name: "Fanatical Firebrand", Count: 1}, {Name: "Mountain", Count: 8}})
	sc.perm(id["Niv-Mizzet, Visionary"], 0)
	ff := sc.perm(id["Fanatical Firebrand"], 0)
	hand := sc.s.pl[0].handN
	life := sc.s.pl[1].life
	sc.stepTo(t, 40, func() bool { return sc.s.pl[0].handN != hand }, func(d *Decision) (Action, bool) {
		for i := 0; i < int(d.N); i++ {
			a := d.Options[i]
			if a.Kind() == AkActivate && a.Source() == TCode(uint8(ff)) || d.Kind == DkTarget && a.Target() == PlayerTarget0+1 {
				return a, true
			}
		}
		for i := 0; i < int(d.N); i++ {
			if d.Options[i].Kind() == AkPass {
				return d.Options[i], true
			}
		}
		return 0, false
	})
	if sc.s.pl[1].life != life-1 || sc.s.pl[0].handN != hand+1 {
		t.Fatalf("life %d (from %d), hand %d (from %d): want the ping and Niv-Mizzet's draw", sc.s.pl[1].life, life, sc.s.pl[0].handN, hand)
	}
}

// TestDamageTriggerOwnControllerNotOpponent: the same ping aimed at the
// Firebrand's own controller is no damage to an opponent: Niv-Mizzet does
// not trigger (pair 63811; fireDamageSrc takes the players dealt damage).
func TestDamageTriggerOwnControllerNotOpponent(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Niv-Mizzet, Visionary", Count: 1}, {Name: "Fanatical Firebrand", Count: 1}, {Name: "Mountain", Count: 8}})
	sc.perm(id["Niv-Mizzet, Visionary"], 0)
	ff := sc.perm(id["Fanatical Firebrand"], 0)
	life := sc.s.pl[0].life
	sc.stepTo(t, 40, func() bool { return sc.s.pl[0].life != life && sc.s.sp == 0 }, func(d *Decision) (Action, bool) {
		for i := 0; i < int(d.N); i++ {
			a := d.Options[i]
			if a.Kind() == AkActivate && a.Source() == TCode(uint8(ff)) || d.Kind == DkTarget && a.Target() == PlayerTarget0 {
				return a, true
			}
		}
		for i := 0; i < int(d.N); i++ {
			if d.Options[i].Kind() == AkPass {
				return d.Options[i], true
			}
		}
		return 0, false
	})
	sc.s.Step()
	if sc.s.pl[0].life != life-1 || sc.s.sp != 0 || sc.s.npend != 0 {
		t.Fatalf("life %d (from %d), stack %d, pending %d: want the ping and no trigger", sc.s.pl[0].life, life, sc.s.sp, sc.s.npend)
	}
}

// TestControlEOTUntapStun: Involuntary Employment's "untap that creature"
// on a tapped creature with a stun counter removes the counter instead
// and the creature stays tapped (CR 122.1d; gorge's GainControl Untap$
// untaps through TryUntap since be95bda94: pair 7034, Diregraf Ghoul).
func TestControlEOTUntapStun(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Involuntary Employment", Count: 1}, {Name: "Burglar Rat", Count: 1}, {Name: "Mountain", Count: 8}})
	for i := 0; i < 4; i++ {
		sc.perm(id["Mountain"], 0)
	}
	rat := sc.obj(id["Burglar Rat"], 0)
	sl := sc.s.enter(sc.k, rat, 1, false, kop.ZoneNone)
	sc.s.perms[sl].flags |= pfTapped
	sc.s.perms[sl].setCtrN(sc.s, int(kop.CtrStun), 1)
	sc.s.npend = 0
	ie := sc.hand(id["Involuntary Employment"], 0)
	sc.stepTo(t, 40, func() bool { return sc.s.objs[ie].zone == kop.ZoneGraveyard }, func(d *Decision) (Action, bool) {
		for i := 0; i < int(d.N); i++ {
			a := d.Options[i]
			if a.Kind() == AkCast && a.Source() == TCode(uint8(ie)) || d.Kind == DkTarget && a.Target() == TCode(uint8(rat)) {
				return a, true
			}
		}
		for i := 0; i < int(d.N); i++ {
			if d.Options[i].Kind() == AkPass {
				return d.Options[i], true
			}
		}
		return 0, false
	})
	p := &sc.s.perms[sc.slot(rat)]
	if sc.s.objs[ie].zone != kop.ZoneGraveyard || p.ctrl != 0 {
		t.Fatalf("Involuntary Employment zone %d, Rat controller p%d: want it resolved", sc.s.objs[ie].zone, p.ctrl)
	}
	if p.flags&pfTapped == 0 || p.ctrLow(int(kop.CtrStun)) != 0 {
		t.Fatalf("Rat tapped=%v stun=%d, want tapped with the stun counter removed", p.flags&pfTapped != 0, p.ctrLow(int(kop.CtrStun)))
	}
}
