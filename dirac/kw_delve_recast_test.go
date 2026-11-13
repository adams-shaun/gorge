package dirac

// The granted delve's credit at the recast and hand-cast offers (kw_delve.go's
// tbDelveCredit through offerHarmonizeCasts, offerMayhemCasts, offerSneakHand,
// offerWebSlingHand, offerMayFlashHand, beamCastOffered) and at the Play
// CopyCard$ read (cast_exilegrant.go's copyCastUnpayable): Teval, Arbiter of
// Virtue ("Spells you cast have delve") on the battlefield credits the casts
// the same way gorge's single offer choke point does (CR 702.66a).

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// recastScen is a seat-0 main-phase-1 scenario over the delvegrant pool with
// Teval, Arbiter of Virtue (the granted delve) on the battlefield when
// withTeval (its object, for taps), and the named cards' ids.
func recastScen(t *testing.T, withTeval bool, names ...string) (*scen, Obj, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Teval, Arbiter of Virtue", Count: 2},
		{Name: "Tormenting Voice", Count: 4},
		{Name: "Cathartic Reunion", Count: 4},
		{Name: "Rocket-Powered Goblin Glider", Count: 2},
		{Name: "Unending Whisper", Count: 2},
		{Name: "Open Communications", Count: 2},
		{Name: "Spider-Man, Brooklyn Visionary", Count: 2},
		{Name: "Mystical Tether", Count: 2},
		{Name: "Shredder, Unrelenting", Count: 2},
		{Name: "Flame Javelin", Count: 2},
		{Name: "Grizzly Bears", Count: 4},
		{Name: "Mountain", Count: 4},
		{Name: "Island", Count: 4},
		{Name: "Swamp", Count: 4},
		{Name: "Forest", Count: 4},
		{Name: "Plains", Count: 4},
	}
	set := cardsettest.Set(t, "delvegrant")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Mountain"], Config{NoElide: true})
	var teval Obj
	if withTeval {
		teval = sc.perm(id["Teval, Arbiter of Virtue"], 0)
	}
	return sc, teval, id
}

// offered counts the current decision's options of kind kind from object o
// with the given target and mode (0xff: any).
func offered(sc *scen, kind ActionKind, o Obj, tgt, mode uint8) (n int) {
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == kind && a.Source() == TCode(uint8(o)) && (tgt == 0xff || a.Target() == TCode(tgt)) && (mode == 0xff || a.Mode() == mode) {
			n++
		}
	}
	return n
}

// applyCast applies the option of kind kind from object o with the given
// target and mode, failing the test when it is not offered.
func applyCast(t *testing.T, sc *scen, kind ActionKind, o Obj, tgt, mode uint8) {
	t.Helper()
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == kind && a.Source() == TCode(uint8(o)) && (tgt == 0xff || a.Target() == TCode(tgt)) && (mode == 0xff || a.Mode() == mode) {
			sc.s.Apply(a)
			return
		}
	}
	t.Fatalf("option kind %d src %d tgt %d mode %d not offered: %v", kind, o, tgt, mode, d.Options[:d.N])
}

// delvePaid walks the granted-delve ask (ChGenDelve), applying a posed
// Done-only ask in front of it (the harmonize announcement with no untapped
// creature), picking every pick it offers and asserting the cast is then paid
// at priority. picks are the objects the ask's AkChoose options must come from.
func delvePaid(t *testing.T, sc *scen, picks []Obj) {
	t.Helper()
	d := sc.s.Step()
	for i := 0; i < 4; i++ {
		if d.Kind == DkChoose && d.Sub == ChGenDelve {
			break
		}
		if d.Kind == DkChoose && d.N == 1 && d.Options[0].Kind() == AkDone {
			sc.s.Apply(d.Options[0])
			d = sc.s.Step()
			continue
		}
		t.Fatalf("delve ask: decision %v/%d, want %v/%d", d.Kind, d.Sub, DkChoose, ChGenDelve)
	}
	want := map[Obj]bool{}
	for _, o := range picks {
		want[o] = true
	}
	var opts []Action
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		switch a.Kind() {
		case AkChoose:
			if !want[Obj(a.Source())] {
				t.Fatalf("delve pick option %d is object %d, want one of %v", i, a.Source(), picks)
			}
			opts = append(opts, a)
		case AkDone:
		default:
			t.Fatalf("delve ask option %d kind %v, want a pick or Done: %v", i, a.Kind(), d.Options[:d.N])
		}
	}
	if len(opts) != len(picks) {
		t.Fatalf("delve ask offers %d picks, want %d: %v", len(opts), len(picks), d.Options[:d.N])
	}
	for i, a := range opts {
		d = sc.s.Step()
		if d.Kind != DkChoose || d.Sub != ChGenDelve {
			t.Fatalf("delve pick %d: decision %v/%d", i, d.Kind, d.Sub)
		}
		sc.s.Apply(a)
	}
	if d = sc.s.Step(); d.Kind != DkPriority {
		t.Fatalf("decision after the picks %v/%d, want priority", d.Kind, d.Sub)
	}
}

// delveSpent asserts every card in cards left the graveyard to exile, the
// delve picks, and the pool's col mana spent when the cast needed it.
func delveSpent(t *testing.T, sc *scen, col int, need bool, cards []Obj) {
	t.Helper()
	if need && sc.s.pl[0].pool[col] != 0 {
		t.Fatalf("pool col %d holds %d, want the cast's payment", col, sc.s.pl[0].pool[col])
	}
	if sc.s.pl[0].graveN != 0 {
		t.Fatalf("graveyard holds %d cards, want 0", sc.s.pl[0].graveN)
	}
	for _, o := range cards {
		if z := sc.s.objs[o].zone; z != kop.ZoneExile {
			t.Fatalf("delved card %d in zone %v, want exile", o, z)
		}
	}
}

// TestDelveGrantRecastCredit covers the recast and hand-cast offers: with the
// grant each carrier is offered at its delve-credited cost where the pool alone
// cannot pay it, without the grant the offer is unchanged.
func TestDelveGrantRecastCredit(t *testing.T) {
	t.Run("mayhem", func(t *testing.T) {
		// Rocket-Powered Goblin Glider's mayhem cost {2}: two generic nothing
		// on the battlefield pays, the graveyard's two cards can.
		sc, _, id := recastScen(t, true, "Rocket-Powered Goblin Glider")
		rg := &sc.k.t.Cards[id["Rocket-Powered Goblin Glider"]]
		if rg.AltCost.Generic != 2 || rg.AltCost.Hyb[0] != 0 || rg.AltCost.Pips != ([kop.NumManaKinds]uint8{}) {
			t.Fatalf("mayhem cost %v, want {2}", rg.AltCost)
		}
		grave := sc.grave(id["Tormenting Voice"], 0, 2)
		rgO := sc.grave(id["Rocket-Powered Goblin Glider"], 0, 1)[0]
		DebugMayhemAnyGrave = true
		defer func() { DebugMayhemAnyGrave = false }()
		if n := offered(sc, AkCast, rgO, 0xff, 2); n != 1 {
			t.Fatalf("with Teval the mayhem cast is offered %d times, want 1", n)
		}
		applyCast(t, sc, AkCast, rgO, 0xff, 2)
		delvePaid(t, sc, grave)
		if z := sc.s.objs[rgO].zone; z != kop.ZoneStack {
			t.Fatalf("the paid mayhem cast in zone %v, want the stack", z)
		}
		delveSpent(t, sc, kop.B, false, grave)
	})
	t.Run("mayhemWithoutGrant", func(t *testing.T) {
		sc, _, id := recastScen(t, false, "Rocket-Powered Goblin Glider")
		rgO := sc.grave(id["Rocket-Powered Goblin Glider"], 0, 1)[0]
		sc.grave(id["Tormenting Voice"], 0, 2)
		DebugMayhemAnyGrave = true
		defer func() { DebugMayhemAnyGrave = false }()
		if n := offered(sc, AkCast, rgO, 0xff, 2); n != 0 {
			t.Fatalf("without Teval the mayhem cast is offered %d times with nothing to pay it", n)
		}
	})
	t.Run("harmonize", func(t *testing.T) {
		// Unending Whisper's harmonize cost {5}{U}: five generic the single
		// blue mana cannot pay, the graveyard's five cards can.
		sc, teval, id := recastScen(t, true, "Unending Whisper")
		wh := &sc.k.t.Cards[id["Unending Whisper"]]
		if wh.AltCost.Generic != 5 {
			t.Fatalf("harmonize cost %v, want {5}{U}-shaped", wh.AltCost)
		}
		grave := append(sc.grave(id["Tormenting Voice"], 0, 4), sc.grave(id["Cathartic Reunion"], 0, 1)...)
		whO := sc.grave(id["Unending Whisper"], 0, 1)[0]
		sc.s.pl[0].pool[kop.U] = 1
		// Teval (power 6) tapped: the harmonize announcement has no tap
		// candidate, so the credit is the only thing that pays the generic
		sc.s.perms[sc.slot(teval)].flags |= pfTapped
		if n := offered(sc, AkCast, whO, 0xff, 2); n != 1 {
			t.Fatalf("with Teval the harmonize cast is offered %d times, want 1", n)
		}
		applyCast(t, sc, AkCast, whO, 0xff, 2)
		delvePaid(t, sc, grave)
		if z := sc.s.objs[whO].zone; z != kop.ZoneStack {
			t.Fatalf("the paid harmonize cast in zone %v, want the stack", z)
		}
		delveSpent(t, sc, kop.U, true, grave)
	})
	t.Run("harmonizeWithoutGrant", func(t *testing.T) {
		sc, _, id := recastScen(t, false, "Unending Whisper")
		whO := sc.grave(id["Unending Whisper"], 0, 1)[0]
		sc.grave(id["Tormenting Voice"], 0, 5)
		sc.s.pl[0].pool[kop.U] = 1
		if n := offered(sc, AkCast, whO, 0xff, 2); n != 0 {
			t.Fatalf("without Teval the harmonize cast is offered %d times on one blue mana", n)
		}
	})
	t.Run("sneak", func(t *testing.T) {
		// Shredder, Unrelenting's sneak cost {3}{B}: three generic the single
		// black mana cannot pay, the graveyard's three cards can. The offer
		// waits for the declare-blockers window with an unblocked attacker.
		for _, withTeval := range []bool{true, false} {
			sc, _, id := recastScen(t, withTeval, "Shredder, Unrelenting")
			sd := &sc.k.t.Cards[id["Shredder, Unrelenting"]]
			if sd.SneakCost.Generic != 3 || sd.SneakCost.Pips[kop.B] != 1 || sd.SneakCost.Hyb[0] != 0 {
				t.Fatalf("sneak cost %v, want {3}{B}-shaped", sd.SneakCost)
			}
			sc.grave(id["Tormenting Voice"], 0, 3)
			ninja := sc.hand(id["Shredder, Unrelenting"], 0)
			bear := sc.perm(id["Grizzly Bears"], 0)
			sc.passUntil(t, kop.StepAttackers)
			sc.do(t, AkAttack, uint8(bear), 0xff)
			sc.do(t, AkDone, 0xff, 0xff)
			for i := 0; i < 16; i++ {
				d := sc.s.Step()
				if d.Kind == DkPriority && d.Player == 0 && sc.s.step == kop.StepBlockers {
					break
				}
				if d.Kind != DkPriority {
					t.Fatalf("sneak window: decision %v/%d, want priority", d.Kind, d.Sub)
				}
				sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
			}
			// the floating pool survives only inside a step: float it at the
			// window (CR 500.4)
			sc.s.pl[0].pool[kop.B] = 1
			if n := offered(sc, AkSneak, ninja, 0xff, 0); n != map[bool]int{true: 1, false: 0}[withTeval] {
				t.Fatalf("with Teval %v the sneak cast is offered %d times", withTeval, n)
			}
		}
	})
	t.Run("websling", func(t *testing.T) {
		// Spider-Man, Brooklyn Visionary's web-slinging cost {2}{G}: two
		// generic the single green mana cannot pay, the graveyard's two cards
		// can. The offer needs a tapped creature to return beside the
		// floating green mana.
		for _, withTeval := range []bool{true, false} {
			sc, _, id := recastScen(t, withTeval, "Spider-Man, Brooklyn Visionary")
			sm := &sc.k.t.Cards[id["Spider-Man, Brooklyn Visionary"]]
			if sm.WebSlingCost.Generic != 2 || sm.WebSlingCost.Pips[kop.G] != 1 || sm.WebSlingCost.Hyb[0] != 0 {
				t.Fatalf("web-slinging cost %v, want {2}{G}-shaped", sm.WebSlingCost)
			}
			sc.grave(id["Tormenting Voice"], 0, 2)
			spider := sc.hand(id["Spider-Man, Brooklyn Visionary"], 0)
			bear := sc.perm(id["Grizzly Bears"], 0)
			sc.s.perms[sc.slot(bear)].flags |= pfTapped
			sc.s.pl[0].pool[kop.G] = 1
			n := -1
			for i := 0; i < 200 && n < 0; i++ {
				d := sc.s.Step()
				if d.Kind != DkPriority {
					t.Fatalf("decision %v/%d, want priority", d.Kind, d.Sub)
				}
				if d.Player == 0 {
					n = offered(sc, AkWebSling, spider, 0xff, 0)
					continue
				}
				sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
			}
			if want := map[bool]int{true: 1, false: 0}[withTeval]; n != want {
				t.Fatalf("with Teval %v the web-slinging cast is offered %d times, want %d", withTeval, n, want)
			}
		}
	})
	t.Run("mayflash", func(t *testing.T) {
		// Mystical Tether's may-flash cost {4}{W}: four generic the single
		// white mana cannot pay, the graveyard's four cards can. The offer
		// waits for a window the plain cast's timing refuses (the opponent's
		// turn).
		for _, withTeval := range []bool{true, false} {
			sc, _, id := recastScen(t, withTeval, "Mystical Tether")
			mt := &sc.k.t.Cards[id["Mystical Tether"]]
			if mt.Cost.Generic != 2 || mt.MayFlashCost.Generic != 2 {
				t.Fatalf("may-flash cost %v + %v, want {2}{W} plus 2 generic", mt.Cost, mt.MayFlashCost)
			}
			sc.grave(id["Tormenting Voice"], 0, 4)
			tether := sc.hand(id["Mystical Tether"], 0)
			sc.perm(id["Grizzly Bears"], 0)
			// the ETB trigger's target is an opponent's creature: the cast
			// census needs one to offer (spellTargetsFeasible)
			sc.perm(id["Grizzly Bears"], 1)
			n := -1
			for i := 0; i < 200 && n < 0; i++ {
				d := sc.s.Step()
				switch d.Kind {
				case DkPriority:
				case DkAttackers, DkBlockers:
					sc.do(t, AkDone, 0xff, 0xff) // no attackers, no blocks
					continue
				default:
					t.Fatalf("opponent turn: decision %v/%d, want priority", d.Kind, d.Sub)
				}
				if d.Player != 0 || sc.s.active != 1 {
					// the may-flash window is a seat-0 priority on the
					// opponent's turn (the plain cast's own timing refused
					// only there): pass and wait
					sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
					continue
				}
				// the floating pool drains at every step end (CR 500.4), so
				// the mana is floated only now, before the scan's Step
				sc.s.pl[0].pool[kop.W] = 1
				n = offered(sc, AkMayFlash, tether, 0xff, 0)
			}
			if want := map[bool]int{true: 1, false: 0}[withTeval]; n != want {
				t.Fatalf("with Teval %v the may-flash cast is offered %d times, want %d", withTeval, n, want)
			}
		}
	})
	t.Run("beemmeup", func(t *testing.T) {
		// Open Communications' beam me up cost {2}{U}: two generic the single
		// blue mana cannot pay, the graveyard's two cards can.
		for _, withTeval := range []bool{true, false} {
			sc, _, id := recastScen(t, withTeval, "Open Communications")
			oc := &sc.k.t.Cards[id["Open Communications"]]
			if oc.AltCost.Generic != 2 {
				t.Fatalf("beam me up cost %v, want {2}{U}-shaped", oc.AltCost)
			}
			sc.grave(id["Tormenting Voice"], 0, 1)
			ocO := sc.grave(id["Open Communications"], 0, 1)[0]
			sc.perm(id["Grizzly Bears"], 0) // the return pick's candidate
			sc.s.pl[0].pool[kop.U] = 1
			if n := offered(sc, AkCast, ocO, 0xff, 2); n != map[bool]int{true: 1, false: 0}[withTeval] {
				t.Fatalf("with Teval %v the beam me up cast is offered %d times", withTeval, n)
			}
		}
	})
}

// TestDelveGrantCopyCredit: the Play CopyCard$ cast of a twobrid spell — the
// copy read's own affordability (copyCastUnpayable, cast_exilegrant.go) — is
// payable over a large graveyard with the grant and unpayable without it.
func TestDelveGrantCopyCredit(t *testing.T) {
	for _, withTeval := range []bool{true, false} {
		sc, _, id := recastScen(t, withTeval, "Flame Javelin")
		// precondition: the cost really is {2/R}{2/R}{2/R} -- three twobrid
		// pips, no plain generic
		fj := sc.k.t.Cards[id["Flame Javelin"]].Cost
		if fj.TbN() != 3 || fj.Generic != 0 || fj.Pips != ([kop.NumManaKinds]uint8{}) {
			t.Fatalf("Flame Javelin cost %v, want {2/R}{2/R}{2/R}", fj)
		}
		sc.grave(id["Tormenting Voice"], 0, 4)
		sc.s.pl[0].pool[kop.R] = 1
		// a minted copy in no zone, as beginCopyCast leaves it for its build
		cp := sc.s.newToken(id["Flame Javelin"], 0)
		sc.s.objs[cp].of = ofCopy
		// gorge's target menu withholds every candidate of the copy the
		// credit cannot pay, so the cast aborts before its window: unpayable
		// without the grant, payable with it (one face's {R} takes the red
		// mana, the two generic faces take the four exiles)
		if got := sc.s.copyCastUnpayable(sc.k, id["Flame Javelin"], 0, cp, restrOrigin{}, 0); got != !withTeval {
			t.Fatalf("with Teval %v: copyCastUnpayable %v, want %v", withTeval, got, !withTeval)
		}
	}
}
