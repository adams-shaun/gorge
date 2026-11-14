package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestScriptWalkGraveyardOrder: gorge's potential walk lists the graveyard
// casts in graveyard order, a flashback cast among the permission casts
// (Muldrotha's creature, then Think Twice's flashback below it), so the
// four searched plays are the first four there. Before, the kernel keyed
// them by its own option order (flashback walk apart), searched Think
// Twice and left out Gutless Plunderer (pair 45914).
func TestScriptWalkGraveyardOrder(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Gutless Plunderer", Count: 1}, {Name: "Think Twice", Count: 1}, {Name: "Swamp", Count: 8}})
	gp := sc.obj(id["Gutless Plunderer"], 0)
	sc.s.place(gp, kop.ZoneGraveyard, false)
	tt := sc.obj(id["Think Twice"], 0)
	sc.s.place(tt, kop.ZoneGraveyard, false)
	fb := MakeAction(AkCast, TCode(tt), 0, 2)
	may := MakeAction(AkCast, TCode(gp), 0, castMayPlay)
	if kf, km := sc.s.walkKey(fb, 0, 0), sc.s.walkKey(may, 0, 1); km > kf {
		t.Fatalf("walk keys: Plunderer (graveyard 0) %#x after Think Twice's flashback (graveyard 1) %#x", km, kf)
	}
}

// TestMayPlayRecordAndStaticOnce: a graveyard creature card both a
// permission record (Zul Ashur, Lich Lord's "you may cast target Zombie
// creature card from your graveyard this turn") and a static (Muldrotha,
// the Gravetide's creature permission) let p0 cast is offered once, as
// gorge lists one may-play cast per card (pair 91243). Before, the kernel
// listed it twice.
func TestMayPlayRecordAndStaticOnce(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Muldrotha, the Gravetide", Count: 1}, {Name: "Soul-Shackled Zombie", Count: 1}, {Name: "Swamp", Count: 8}})
	sc.perm(id["Muldrotha, the Gravetide"], 0)
	for i := 0; i < 6; i++ {
		sc.perm(id["Swamp"], 0)
	}
	z := sc.obj(id["Soul-Shackled Zombie"], 0)
	sc.s.place(z, kop.ZoneGraveyard, false)
	sc.s.g.may[0] = mayPlay{obj: z, ver: sc.s.objs[z].ver, ctrl: 0, turn: mayWord(sc.s.turn)}
	sc.s.g.nmay = 1
	d := sc.s.Step()
	n := 0
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(z)) {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("Soul-Shackled Zombie offered %d times, want once (decision kind %d: %v)", n, d.Kind, d.Options[:d.N])
	}
}

// TestExsanguinateGainsTheLifeLost: Exsanguinate's "you gain life equal to
// the life lost this way" reads the life its LoseLife took (ExLastLifeLost:
// gorge's AFLifeLost). Before, nothing recorded it and the caster gained 0
// (pair 10051).
func TestExsanguinateGainsTheLifeLost(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Exsanguinate", Count: 1}, {Name: "Swamp", Count: 8}})
	for i := 0; i < 4; i++ {
		sc.perm(id["Swamp"], 0)
	}
	ex := sc.hand(id["Exsanguinate"], 0)
	l0, l1 := sc.s.pl[0].life, sc.s.pl[1].life
	sc.stepTo(t, 20, func() bool { return sc.s.objs[ex].zone == kop.ZoneGraveyard }, func(d *Decision) (Action, bool) {
		for i := 0; i < int(d.N); i++ {
			a := d.Options[i]
			if a.Kind() == AkCast && a.Source() == TCode(uint8(ex)) || d.Kind == DkChoose && d.Sub == ChX && a.Mode() == 2 {
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
	if sc.s.pl[1].life != l1-2 || sc.s.pl[0].life != l0+2 {
		t.Fatalf("life p0 %d -> %d, p1 %d -> %d: want +2 and -2", l0, sc.s.pl[0].life, l1, sc.s.pl[1].life)
	}
}

// TestCardKykarReturnsUnderOwner: Kykar, Zephyr Awakener's blinked creature
// comes back "under its owner's control" (OpReturn): a stolen creature
// returns to its owner. Before, the core's OpReturn entered it under the
// ability's controller (pair 71481: Gorehorn Raider).
func TestCardKykarReturnsUnderOwner(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Kykar, Zephyr Awakener", Count: 1}, {Name: "Burglar Rat", Count: 1}, {Name: "Burst Lightning", Count: 1}, {Name: "Mountain", Count: 8}})
	sc.perm(id["Kykar, Zephyr Awakener"], 0)
	for i := 0; i < 2; i++ {
		sc.perm(id["Mountain"], 0)
	}
	rat := sc.obj(id["Burglar Rat"], 1)
	sc.s.perms[sc.s.enter(sc.k, rat, 0, false, kop.ZoneNone)].flags &^= pfSick // p1's Rat under p0's control
	sc.s.npend = 0
	shock := sc.hand(id["Burst Lightning"], 0)
	turn := sc.s.turn
	sc.stepTo(t, 60, func() bool { return sc.s.turn != turn }, func(d *Decision) (Action, bool) {
		for i := 0; i < int(d.N); i++ {
			a := d.Options[i]
			switch {
			case a.Kind() == AkCast && a.Source() == TCode(uint8(shock)):
				return a, true
			case d.Kind == DkTarget && (a.Target() == PlayerTarget0+1 || a.Target() == TCode(uint8(rat))):
				return a, true
			case d.Kind == DkModes && a.Mode() == 0:
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
	if sc.s.objs[rat].zone != kop.ZoneBattlefield || sc.s.objs[rat].ver < 2 {
		t.Fatalf("the Rat is in zone %d (ver %d): want it blinked back", sc.s.objs[rat].zone, sc.s.objs[rat].ver)
	}
	if c := sc.s.perms[sc.slot(rat)].ctrl; c != 1 {
		t.Fatalf("the returned Rat's controller is p%d, want its owner p1", c)
	}
}

// TestStrokeTokenToLastController: Stroke of Midnight's "its controller
// creates a 1/1 Human" reads the destroyed permanent's last known
// controller (CR 608.2h; gorge TokenOwner$ TargetedController): p0's
// creature that p1 controls gives p1 the token. Before, the kernel read the
// card's controller in the graveyard, its owner (pair 84472: Grappling
// Kraken).
func TestStrokeTokenToLastController(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Stroke of Midnight", Count: 1}, {Name: "Burglar Rat", Count: 1}, {Name: "Plains", Count: 8}})
	for i := 0; i < 3; i++ {
		sc.perm(id["Plains"], 0)
	}
	rat := sc.obj(id["Burglar Rat"], 0)
	sc.s.perms[sc.s.enter(sc.k, rat, 1, false, kop.ZoneNone)].flags &^= pfSick // p0's Rat under p1's control
	sc.s.npend = 0
	st := sc.hand(id["Stroke of Midnight"], 0)
	sc.stepTo(t, 30, func() bool { return sc.s.objs[st].zone == kop.ZoneGraveyard }, func(d *Decision) (Action, bool) {
		for i := 0; i < int(d.N); i++ {
			a := d.Options[i]
			if a.Kind() == AkCast && a.Source() == TCode(uint8(st)) || d.Kind == DkTarget && a.Target() == TCode(uint8(rat)) {
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
	if sc.s.objs[rat].zone != kop.ZoneGraveyard {
		t.Fatalf("the Rat is in zone %d, want destroyed", sc.s.objs[rat].zone)
	}
	var n [2]int
	for m := sc.s.occ; m.any(); m = m.rest() {
		if p := &sc.s.perms[m.first()]; sc.s.objs[p.obj].of&ofToken != 0 {
			n[p.ctrl]++
		}
	}
	if n != [2]int{0, 1} {
		t.Fatalf("tokens p0 %d p1 %d, want the Human for p1", n[0], n[1])
	}
}

// TestValkyriesCallCardLeftGraveyard: Valkyrie's Call returns "that card"
// from the graveyard (gorge Origin$ Graveyard): a card exiled from the
// graveyard before the trigger resolves (Abyssal Harvester) is a new object
// and stays in exile (CR 400.7). Before, the kernel returned it from exile
// (pairs 69834 30137 35683 68528).
func TestValkyriesCallCardLeftGraveyard(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Valkyrie's Call", Count: 1}, {Name: "Burglar Rat", Count: 1}, {Name: "Plains", Count: 8}})
	sc.perm(id["Valkyrie's Call"], 0)
	rat := sc.perm(id["Burglar Rat"], 0)
	sc.s.leave(sc.k, sc.slot(rat), kop.ZoneGraveyard, false)
	if sc.s.npend == 0 {
		t.Fatal("Valkyrie's Call did not trigger")
	}
	sc.s.detach(rat)
	sc.s.place(rat, kop.ZoneExile, false)
	sc.stepTo(t, 20, func() bool { return sc.s.npend == 0 && sc.s.sp == 0 && sc.s.ctl == ctlPriority }, func(d *Decision) (Action, bool) {
		for i := 0; i < int(d.N); i++ {
			if d.Options[i].Kind() == AkPass {
				return d.Options[i], true
			}
		}
		return 0, false
	})
	if z := sc.s.objs[rat].zone; z != kop.ZoneExile {
		t.Fatalf("the Rat is in zone %d, want it left in exile", z)
	}
}

// TestExileInsteadOfDyingOnSacrifice: Fiery Annihilation's "if that
// creature would die this turn, exile it instead" covers any death, a
// sacrifice too (gorge's replacement on the move), not only lethal damage:
// a creature it hit and that survived, sacrificed later that turn, is
// exiled (pair 18678: Treetop Snarespinner sacrificed for Arbiter of Woe).
// Before, only the state-based death read the mark.
func TestExileInsteadOfDyingOnSacrifice(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Burglar Rat", Count: 2}, {Name: "Fiery Annihilation", Count: 1}, {Name: "Swamp", Count: 7}}) // the Annihilation puts exileEx in the arena
	a := sc.perm(id["Burglar Rat"], 0)
	b := sc.perm(id["Burglar Rat"], 0)
	sc.s.mSlSet(sc.k, mechExileEx, sc.slot(a))
	sc.s.mSlSet(sc.k, mechExileEx, sc.slot(b))
	sc.s.leave(sc.k, sc.slot(a), kop.ZoneGraveyard, false) // a sacrifice or a destroy
	sc.s.addSac(sc.slot(b))                                // an effect's sacrifice
	sc.s.flushSac(sc.k)
	if za, zb := sc.s.objs[a].zone, sc.s.objs[b].zone; za != kop.ZoneExile || zb != kop.ZoneExile {
		t.Fatalf("zones %d %d, want both exiled", za, zb)
	}
}

// TestSelfPowerLKI: an ability whose source left the battlefield reads the
// source's last known power (CR 608.2h, 113.7a): Alesha, Who Laughs at
// Fate's end-step return of "a creature card with mana value less than or
// equal to Alesha's power" after Alesha (3/3 with her attack counter) was
// sacrificed still returns a mana value 3 card. Before, the kernel read the
// card's printed power, 2, and the target was illegal (pair 12267: Goblin
// Boarders; 52089).
func TestSelfPowerLKI(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Alesha, Who Laughs at Fate", Count: 1}, {Name: "Swamp", Count: 8}})
	al := sc.perm(id["Alesha, Who Laughs at Fate"], 0)
	sc.s.perms[sc.slot(al)].setCtrN(sc.s, int(kop.CtrP1P1), 1)
	e := stackEntry{kind: seTrigger, obj: al, srcVer: sc.s.objs[al].ver, card: sc.s.objs[al].card, ab: 1, ctrl: 0, trigObj: NoObj}
	sc.s.leave(sc.k, sc.slot(al), kop.ZoneGraveyard, false)
	if p := sc.s.evalV(sc.k, kop.ValueRef{K: kop.VSelfPower}, sc.s.ctxEntry(&e)); p != 3 {
		t.Fatalf("Alesha's power after she left: %d, want her last known 3", p)
	}
}

// TestDigUntilProgenitusShuffle: Consuming Aberration's "reveal until a
// land, put them into the graveyard" moves the revealed cards one at a
// time, the rest still in the library: Progenitus revealed first is
// shuffled into a library that still holds the later revealed cards
// (gorge's moves in reveal order; pair 45767). Before, the kernel took
// every revealed card out first and shuffled a shorter library.
func TestDigUntilProgenitusShuffle(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Consuming Aberration", Count: 1}, {Name: "Progenitus", Count: 1}, {Name: "Burglar Rat", Count: 1}, {Name: "Swamp", Count: 8}})
	pl := &sc.s.pl[1]
	var top [3]Obj
	for i, name := range []string{"Swamp", "Burglar Rat", "Progenitus"} {
		o := sc.obj(id[name], 1)
		sc.s.objs[o].zone = kop.ZoneLibrary
		pl.libTop(o)
		top[2-i] = o
	}
	ref := sc.s.pl[1]
	rs := sc.s.rng
	// gorge: Progenitus onto the top, the whole library shuffled, then the
	// Rat and the Swamp out
	{
		l := ref.lib[ref.libLo:ref.libHi]
		for i := len(l) - 1; i > 0; i-- {
			j := rs.IntN(i + 1)
			l[i], l[j] = l[j], l[i]
		}
		ref.libRemove(top[1])
		ref.libRemove(top[2])
	}
	ab := &sc.k.t.Cards[id["Consuming Aberration"]].Abilities[0]
	sc.s.res.e = stackEntry{kind: seTrigger, ctrl: 0, card: id["Consuming Aberration"], trigObj: NoObj}
	sc.s.execDigUntil(sc.k, &ab.Prog[0], false)
	got := sc.s.pl[1].lib[sc.s.pl[1].libLo:sc.s.pl[1].libHi]
	want := ref.lib[ref.libLo:ref.libHi]
	if len(got) != len(want) {
		t.Fatalf("library %d cards, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("library %v, want %v (gorge's shuffle over the unrevealed library)", got, want)
		}
	}
	if sc.s.objs[top[1]].zone != kop.ZoneGraveyard || sc.s.objs[top[2]].zone != kop.ZoneGraveyard || sc.s.objs[top[0]].zone != kop.ZoneLibrary {
		t.Fatal("want Progenitus shuffled in, the Rat and the Swamp in the graveyard")
	}
}
