package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// unearthOffered reports whether the next decision offers p0's granted
// unearth of the graveyard card o.
func (sc *scen) unearthOffered(o Obj) bool {
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkActivate && a.Source() == TCode(o) && a.Mode() == unearthGrantMode {
			return true
		}
	}
	return false
}

// unearthBoard is Sedris, the Traitor King, the mana sources mana and a
// creature card in p0's graveyard: the state a granted unearth is offered in.
// Sedris's grant is unearth {2}{B}, so an offer test needs three mana with a
// black source in it; the card's own mana cost ({1}{G} for Grizzly Bears) is
// what a wrong price would charge instead, and the negative boards are picked
// to keep that payable.
func unearthBoard(t *testing.T, tbl *kop.Table, id map[string]CardID, mana ...CardID) (sc *scen, bears Obj) {
	t.Helper()
	sc = newScenTableLib(t, tbl, id["Swamp"], Config{NoElide: true})
	if !sc.k.anyOffKwGrant {
		t.Fatal("anyOffKwGrant unset: the table lowers no StatUnearthGrant static")
	}
	sc.perm(id["Sedris, the Traitor King"], 0)
	for _, m := range mana {
		sc.perm(m, 0)
	}
	bears = sc.grave(id["Grizzly Bears"], 0, 1)[0]
	return sc, bears
}

// CheckUnearthGrant (CR 702.84a, Sedris, the Traitor King): each creature
// card in its controller's graveyard is offered unearth {2}{B} at sorcery
// speed, the activation returns it to the battlefield with haste, and it is
// exiled at the beginning of the next end step or if it would leave the
// battlefield. The static's cost is the grant's {2}{B}, not the affected
// card's own mana cost; a card printing Unearth offers once, through its
// printed activation.
func CheckUnearthGrant(t *testing.T, tbl *kop.Table, id map[string]CardID) {
	t.Helper()

	t.Run("offered at sorcery speed for the grant cost", func(t *testing.T) {
		sc, bears := unearthBoard(t, tbl, id, id["Swamp"], id["Swamp"], id["Forest"])
		if !sc.unearthOffered(bears) {
			t.Fatal("granted unearth not offered with {2}{B} available at sorcery speed")
		}
		sc.s.step = kop.StepBeginCombat
		if sc.unearthOffered(bears) {
			t.Fatal("granted unearth offered outside a main phase")
		}
	})

	t.Run("one mana short", func(t *testing.T) {
		// {B}{G} is two mana: not enough for the grant's {2}{B}, but the
		// affected card's own {1}{G} would be payable -- the price is the
		// grant's.
		sc, bears := unearthBoard(t, tbl, id, id["Swamp"], id["Forest"])
		if sc.unearthOffered(bears) {
			t.Fatal("granted unearth offered two mana short of {2}{B}")
		}
	})

	t.Run("no black source", func(t *testing.T) {
		// three mana with no black: the grant's {B} is unpaid, while the
		// affected card's own {1}{G} would be payable.
		sc, bears := unearthBoard(t, tbl, id, id["Forest"], id["Forest"], id["Forest"])
		if sc.unearthOffered(bears) {
			t.Fatal("granted unearth offered without a black source: the cost is {2}{B}")
		}
	})

	t.Run("a non-creature card is not offered", func(t *testing.T) {
		sc, _ := unearthBoard(t, tbl, id, id["Swamp"], id["Swamp"], id["Forest"])
		f := sc.grave(id["Forest"], 0, 1)[0]
		if sc.unearthOffered(f) {
			t.Fatal("granted unearth offered for a Forest: the static's Affected$ Creature must filter it out")
		}
	})

	t.Run("another player's graveyard card is not offered", func(t *testing.T) {
		sc, _ := unearthBoard(t, tbl, id, id["Swamp"], id["Swamp"], id["Forest"])
		b := sc.grave(id["Grizzly Bears"], 1, 1)[0]
		if sc.unearthOffered(b) {
			t.Fatal("granted unearth offered for the other player's graveyard card")
		}
	})

	t.Run("activation returns the card with haste and exiles it at the end step", func(t *testing.T) {
		sc, bears := unearthBoard(t, tbl, id, id["Swamp"], id["Swamp"], id["Forest"])
		sc.do(t, AkActivate, TCode(bears), scAny)
		sc.do(t, AkPass, scAny, scAny)
		sc.do(t, AkPass, scAny, scAny)
		if sc.s.objs[bears].zone != kop.ZoneBattlefield {
			t.Fatalf("the unearthed card is in zone %d, want the battlefield", sc.s.objs[bears].zone)
		}
		if sc.s.perms[sc.slot(bears)].eotKw&kop.KwHaste == 0 {
			t.Fatal("the unearthed card has no haste")
		}
		if sc.s.sp != 0 {
			t.Fatalf("the unearth ability is still on the stack (%d entries)", sc.s.sp)
		}
		// cross combat: passUntil cannot cross the attackers and blockers
		// picks (they are not priority decisions), so declare none
		sc.passUntil(t, kop.StepAttackers)
		sc.do(t, AkDone, scAny, scAny)
		sc.passUntil(t, kop.StepBlockers)
		if sc.s.ctl == ctlBlockers {
			sc.do(t, AkDone, scAny, scAny)
		}
		sc.passUntil(t, kop.StepEnd)
		// the end-step exile is a delayed trigger on the stack from the end
		// step's beginning: pass priority until it resolves
		for i := 0; i < 50 && sc.s.sp != 0; i++ {
			d := sc.s.Step()
			if d.Kind != DkPriority {
				break
			}
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		}
		if z := sc.s.objs[bears].zone; z != kop.ZoneExile {
			t.Fatalf("the unearthed card is in zone %d, want exile at the end step", z)
		}
	})

	t.Run("a card printing Unearth offers once, through its printed activation", func(t *testing.T) {
		sc, _ := unearthBoard(t, tbl, id, id["Swamp"], id["Swamp"], id["Forest"])
		ks, ok := id["Chronomancer"]
		if !ok {
			t.Skip("no printed-unearth card in the table")
		}
		ksObj := sc.grave(ks, 0, 1)[0]
		d := sc.s.Step()
		granted, printed := 0, 0
		for i := 0; i < int(d.N); i++ {
			a := d.Options[i]
			if a.Kind() == AkActivate && a.Source() == TCode(ksObj) {
				if a.Mode() == unearthGrantMode {
					granted++
				} else {
					printed++
				}
			}
		}
		if granted != 0 {
			t.Fatal("a card printing Unearth is offered the granted activation again (gorge printedKeywordCovered)")
		}
		if printed != 1 {
			t.Fatalf("%d printed unearth activations, want one", printed)
		}
	})
}
