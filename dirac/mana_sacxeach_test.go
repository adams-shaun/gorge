package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// CheckSacXEachMana is the kernel-side half of TestSacXEachManaGap (dirac_test).
// Bolg's Company's "{T}, Sacrifice another Goblin: Add {B}{R}" is a census gap
// gorge's prefix search activates (pay.PaymentPlanShapeTierOf defers it; its
// relaxed alternative is the whole {B}{R}). With no land, a second Bolg's
// Company ({B}{R}) in hand is offered exactly when another Goblin can be
// sacrificed: the source's {T}, the sacrifice (the first candidate in
// battlefield order, gorge's scriptAnswers order), then the mana.
func CheckSacXEachMana(t *testing.T, tbl *kop.Table, id map[string]CardID) {
	t.Helper()
	board := func(cfg Config, others ...string) (*scen, Obj, []Obj, Obj) {
		sc := newScenTableLib(t, tbl, id["Swamp"], cfg)
		bolg := sc.perm(id["Bolg's Company"], 0)
		var os []Obj
		for _, n := range others {
			os = append(os, sc.perm(id[n], 0))
		}
		spell := sc.hand(id["Bolg's Company"], 0)
		sc.hand(id["Swamp"], 0) // a card for Exiled Boggart's dies trigger to discard
		return sc, bolg, os, spell
	}
	castOf := func(s *State, o Obj) (Action, bool) {
		d := s.Step()
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkCast && Obj(a.Source()) == o {
				return a, true
			}
		}
		return 0, false
	}
	for _, cfg := range []Config{{NoElide: true}, {GorgeLowering: true, NoElide: true}} {
		// (a) Bolg's Company alone: no other Goblin, nothing to sacrifice ("another"
		// leaves the source out), so no activation and no offer
		sc, bolg, _, spell := board(cfg)
		if !sc.k.sacXOf(sc.s.objs[bolg].card) {
			t.Fatal("Bolg's Company is no sacrifice-another source in the table")
		}
		var cs [MaxPerm]Slot
		if n := sc.s.tapXCands(sc.k, 0, sc.slot(bolg), &cs); n != 0 {
			t.Errorf("gorgeLowering=%v: Bolg's Company alone has %d sacrifice candidates, want 0 (never itself)", cfg.GorgeLowering, n)
		}
		if offersCast(sc.s, spell) {
			t.Errorf("gorgeLowering=%v: the {B}{R} spell offered with no other Goblin", cfg.GorgeLowering)
		}

		// (d) beside a non-Goblin only: not offered
		sc, bolg, os, spell := board(cfg, "Grizzly Bears")
		if !sc.onBF(os[0]) {
			t.Fatal("Grizzly Bears not on the battlefield")
		}
		if n := sc.s.tapXCands(sc.k, 0, sc.slot(bolg), &cs); n != 0 {
			t.Errorf("gorgeLowering=%v: Bolg's Company beside Grizzly Bears has %d candidates, want 0", cfg.GorgeLowering, n)
		}
		if offersCast(sc.s, spell) {
			t.Errorf("gorgeLowering=%v: the {B}{R} spell offered beside a non-Goblin only", cfg.GorgeLowering)
		}

		// (b) one Exiled Boggart: offered; casting it taps Bolg's Company,
		// sacrifices the Boggart (its dies trigger fires) and pays {B}{R} from
		// the pool
		sc, bolg, os, spell = board(cfg, "Exiled Boggart")
		boggart := os[0]
		if sc.s.pl[0].pool != ([kop.NumManaKinds]uint8{}) || sc.tapped(bolg) {
			t.Fatal("the board starts with mana floating or Bolg's Company tapped")
		}
		cast, ok := castOf(sc.s, spell)
		if !ok {
			t.Fatalf("gorgeLowering=%v: the {B}{R} spell not offered beside Exiled Boggart", cfg.GorgeLowering)
		}
		sc.s.Apply(cast) // the prefix: Bolg's {T}, the sacrifice, {B}{R} floating
		if sc.s.pl[0].pool[kop.B] != 1 || sc.s.pl[0].pool[kop.R] != 1 {
			t.Errorf("gorgeLowering=%v: pool %v after the prefix, want {B}{R} floating", cfg.GorgeLowering, sc.s.pl[0].pool)
		}
		// the sorcery-speed cast waits for the dies trigger (gorge waitsForStack:
		// the path passes), so the prefix ends with it resolved: the Swamp
		// discarded beside the dead Boggart, the stack empty, the spell in hand
		if sc.s.pl[0].graveN != 2 || sc.s.sp != 0 || sc.s.objs[spell].zone != kop.ZoneHand {
			t.Errorf("gorgeLowering=%v: after the prefix graveyard %d, stack %d, spell zone %d: want the Boggart and the discarded Swamp, an empty stack, the spell in hand", cfg.GorgeLowering, sc.s.pl[0].graveN, sc.s.sp, sc.s.objs[spell].zone)
		}
		if !driveToCast(sc.s, spell) {
			t.Fatalf("gorgeLowering=%v: the spell was never cast after the prefix", cfg.GorgeLowering)
		}
		if !sc.tapped(bolg) {
			t.Errorf("gorgeLowering=%v: Bolg's Company untapped after the cast", cfg.GorgeLowering)
		}
		if z := sc.s.objs[boggart].zone; z != kop.ZoneGraveyard {
			t.Errorf("gorgeLowering=%v: Exiled Boggart in zone %d, want the graveyard", cfg.GorgeLowering, z)
		}
		if z := sc.s.objs[spell].zone; z != kop.ZoneStack {
			t.Errorf("gorgeLowering=%v: the spell in zone %d, want the stack", cfg.GorgeLowering, z)
		}
		if sc.s.pl[0].pool != ([kop.NumManaKinds]uint8{}) {
			t.Errorf("gorgeLowering=%v: pool %v after the cast, want empty", cfg.GorgeLowering, sc.s.pl[0].pool)
		}
		if sc.s.pl[0].graveN != 2 || sc.s.pl[0].handN != 0 {
			t.Errorf("gorgeLowering=%v: graveyard %d hand %d, want the Boggart and the Swamp its dies trigger discarded", cfg.GorgeLowering, sc.s.pl[0].graveN, sc.s.pl[0].handN)
		}

		// (c) two other Goblins: the search's first answer sacrifices the first
		// in battlefield order (Raging Goblin entered first)
		sc, bolg, os, spell = board(cfg, "Raging Goblin", "Exiled Boggart")
		if n := sc.s.tapXCands(sc.k, 0, sc.slot(bolg), &cs); n != 2 || cs[0] != sc.slot(os[0]) {
			t.Fatalf("gorgeLowering=%v: candidates %v (%d), want Raging Goblin then Exiled Boggart", cfg.GorgeLowering, cs[:n], n)
		}
		cast, ok = castOf(sc.s, spell)
		if !ok {
			t.Fatalf("gorgeLowering=%v: the {B}{R} spell not offered beside two Goblins", cfg.GorgeLowering)
		}
		sc.s.Apply(cast)
		if !driveToCast(sc.s, spell) {
			t.Fatalf("gorgeLowering=%v: the spell was never cast beside two Goblins", cfg.GorgeLowering)
		}
		if z := sc.s.objs[os[0]].zone; z != kop.ZoneGraveyard {
			t.Errorf("gorgeLowering=%v: Raging Goblin (first in battlefield order) in zone %d, want the graveyard", cfg.GorgeLowering, z)
		}
		if !sc.onBF(os[1]) {
			t.Errorf("gorgeLowering=%v: Exiled Boggart left the battlefield, want only the first candidate sacrificed", cfg.GorgeLowering)
		}
	}
}

// driveToCast runs s on until o is cast: the other decisions take a pass or
// the first answer (the Boggart's discard). False when o is never cast.
func driveToCast(s *State, o Obj) bool {
	for guard := 0; guard < 16; guard++ {
		if s.objs[o].zone == kop.ZoneStack {
			return true
		}
		d := s.Step()
		if d.N == 0 {
			return false
		}
		pick := d.Options[0]
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkCast && Obj(a.Source()) == o {
				pick = a
				break
			} else if a.Kind() == AkPass {
				pick = a
			}
		}
		s.Apply(pick)
	}
	return s.objs[o].zone == kop.ZoneStack
}
