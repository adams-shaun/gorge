package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// outOfTheTombsTable builds the draw-repl pool's Out of the Tombs matchup and
// returns the table and the ids of Out of the Tombs (and Grizzly Bears when
// withCreature is set).
func outOfTheTombsTable(t *testing.T, withCreature bool) (*kop.Table, CardID, CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Out of the Tombs", Count: 1}, {Name: "Swamp", Count: 39}}
	if withCreature {
		deck = []cardset.Entry{{Name: "Out of the Tombs", Count: 1}, {Name: "Grizzly Bears", Count: 1}, {Name: "Swamp", Count: 38}}
	}
	tbl, _, _, err := cardsettest.Set(t, "draw-repl").Matchup(deck, []cardset.Entry{{Name: "Swamp", Count: 40}})
	if err != nil {
		t.Fatal(err)
	}
	tombs, bear := CardID(255), CardID(255)
	for i := range tbl.Cards {
		switch tbl.Cards[i].Name {
		case "Out of the Tombs":
			tombs = CardID(i)
		case "Grizzly Bears":
			bear = CardID(i)
		}
	}
	if tombs == 255 {
		t.Fatal("precondition: the draw-repl table lacks Out of the Tombs")
	}
	if withCreature && bear == 255 {
		t.Fatal("precondition: the draw-repl table lacks Grizzly Bears")
	}
	return tbl, tombs, bear
}

// TestOutOfTheTombsDrawReplBranches drives the real lowered card in the kernel:
// the mandatory hidden pick returns a creature card from an empty library's
// replacement and leaves no empty-library mark; with no creature card the
// Remembered-gated LosesGame branch loses the player before any state-based
// check (so the loss is the replacement's, not CR 704.5b's).
func TestOutOfTheTombsDrawReplBranches(t *testing.T) {
	t.Run("a creature returns", func(t *testing.T) {
		tbl, tombs, bear := outOfTheTombsTable(t, true)
		var land CardID
		for i := range tbl.Cards {
			if tbl.Cards[i].Name == "Swamp" {
				land = CardID(i)
			}
		}
		sc := newScenTableLib(t, tbl, land, Config{NoElide: true})
		sc.perm(tombs, 0)
		grave := sc.grave(bear, 0, 1)[0]
		sc.s.pl[0].libLo = sc.s.pl[0].libHi
		// The draw step's own draw runs outside a resolution, so the kernel
		// takes gorge's no-host fallback: the only eligible card moves. The
		// in-resolution pick is the two subtests below, with the same shape
		// compared against gorge by the pool pair tombs_divination
		// (oracle/repl_drawrepl_lockstep_test.go).
		sc.s.draw(0)
		if got := sc.s.objs[grave].zone; got != kop.ZoneBattlefield {
			t.Fatalf("returned creature zone = %d, want battlefield", got)
		}
		if sc.s.pl[0].lost || sc.s.pl[0].mark&pmDrewEmpty != 0 {
			t.Fatalf("lost=%v mark=%d, want neither: the replacement applied", sc.s.pl[0].lost, sc.s.pl[0].mark)
		}
		if sc.s.g.nrem != 0 {
			t.Fatalf("remembered set size = %d after Cleanup, want 0", sc.s.g.nrem)
		}
	})
	t.Run("the pick is asked inside a resolution", func(t *testing.T) {
		tbl, tombs, bear := outOfTheTombsTable(t, true)
		var land CardID
		for i := range tbl.Cards {
			if tbl.Cards[i].Name == "Swamp" {
				land = CardID(i)
			}
		}
		drawOne := CardID(len(tbl.Cards))
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Draw One", Types: kop.TInstant,
			Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{{Op: kop.OpDraw, Obj: kop.RefTriggerPlayer, A: kop.Const(1)}}}}})
		sc := newScenTableLib(t, tbl, land, Config{NoElide: true})
		tombsObj := sc.perm(tombs, 0)
		grave := sc.grave(bear, 0, 1)[0]
		src := sc.s.perms[sc.s.objs[tombsObj].slot]
		sc.s.res = resCtx{e: stackEntry{kind: seAbility, obj: src.obj, srcVer: sc.s.objs[src.obj].ver, card: drawOne, ab: 0, ctrl: src.ctrl, trigObj: NoObj}}
		sc.s.res.e.tgt = noTargets
		sc.s.res.active = true
		sc.s.res.pc = 0
		sc.s.pl[0].libLo = sc.s.pl[0].libHi
		sc.s.runProg(sc.k)
		d := sc.s.Step()
		if d.Kind != DkChoose || d.Sub != ChGenHiddenPick {
			t.Fatalf("decision = %+v, want the graveyard hidden pick inside the resolution", d)
		}
		sc.do(t, AkChoose, TCode(grave), 0)
		if got := sc.s.objs[grave].zone; got != kop.ZoneBattlefield {
			t.Fatalf("returned creature zone = %d, want battlefield", got)
		}
		if sc.s.pl[0].lost {
			t.Fatal("seat 0 lost although the in-resolution pick returned a creature")
		}
	})
	t.Run("the outer Draw continues after the pick", func(t *testing.T) {
		tbl, tombs, bear := outOfTheTombsTable(t, true)
		var land CardID
		for i := range tbl.Cards {
			if tbl.Cards[i].Name == "Swamp" {
				land = CardID(i)
			}
		}
		drawTwo := CardID(len(tbl.Cards))
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Draw Two", Types: kop.TInstant,
			Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{{Op: kop.OpDraw, Obj: kop.RefTriggerPlayer, A: kop.Const(2)}}}}})
		sc := newScenTableLib(t, tbl, land, Config{NoElide: true})
		tombsObj := sc.perm(tombs, 0)
		graves := sc.grave(bear, 0, 2)
		if len(graves) != 2 {
			t.Fatalf("precondition: graveyard creatures = %d, want 2", len(graves))
		}
		src := sc.s.perms[sc.s.objs[tombsObj].slot]
		sc.s.res = resCtx{e: stackEntry{kind: seAbility, obj: src.obj, srcVer: sc.s.objs[src.obj].ver, card: drawTwo, ab: 0, ctrl: src.ctrl, trigObj: NoObj}}
		sc.s.res.e.tgt = noTargets
		sc.s.res.active = true
		sc.s.res.pc = 0
		sc.s.pl[0].libLo = sc.s.pl[0].libHi
		sc.s.runProg(sc.k)
		d := sc.s.Step()
		if d.Kind != DkChoose || d.Sub != ChGenHiddenPick {
			t.Fatalf("decision = %+v, want the graveyard hidden pick inside the resolution", d)
		}
		// gorge poses the pick with the resolving entry still on the stack
		// (engine/rules/repl_draw_empty.go replaces each draw event on its
		// own), so the first replaced draw suspends its instruction: the
		// resolution is still running and nothing has moved yet.
		if !sc.s.res.active {
			t.Fatal("the Draw Two resolution ended while its first replaced draw was on the pick")
		}
		for i, g := range graves {
			if z := sc.s.objs[g].zone; z != kop.ZoneGraveyard {
				t.Fatalf("creature %d zone = %d at the open pick, want graveyard", i, z)
			}
		}
		sc.do(t, AkChoose, TCode(graves[0]), 0)
		d = sc.s.Step()
		if d.Kind != DkChoose || d.Sub != ChGenHiddenPick {
			t.Fatalf("decision = %+v, want the second draw's pick: the outer Draw must continue past the answer", d)
		}
		sc.do(t, AkChoose, TCode(graves[1]), 0)
		for i, g := range graves {
			if z := sc.s.objs[g].zone; z != kop.ZoneBattlefield {
				t.Fatalf("creature %d zone = %d, want battlefield", i, z)
			}
		}
		if sc.s.res.active {
			t.Fatal("the Draw Two resolution never finished after both draws were replaced")
		}
		if sc.s.pl[0].lost || sc.s.pl[0].mark&pmDrewEmpty != 0 {
			t.Fatalf("lost=%v mark=%d, want neither: both draws were replaced", sc.s.pl[0].lost, sc.s.pl[0].mark)
		}
	})
	t.Run("nothing to return loses", func(t *testing.T) {
		tbl, tombs, _ := outOfTheTombsTable(t, false)
		var land CardID
		for i := range tbl.Cards {
			if tbl.Cards[i].Name == "Swamp" {
				land = CardID(i)
			}
		}
		sc := newScenTableLib(t, tbl, land, Config{NoElide: true})
		sc.perm(tombs, 0)
		// The scenario library is empty and the only graveyard candidates are
		// lands once the library's cards are gone, so the mandatory pick moves
		// nothing and the Remembered gate runs the LosesGame.
		sc.s.pl[0].libLo = sc.s.pl[0].libHi
		sc.s.draw(0)
		if !sc.s.pl[0].lost {
			t.Fatal("seat 0 did not lose through Out of the Tombs' LosesGame branch")
		}
		if sc.s.pl[0].mark&pmDrewEmpty != 0 {
			t.Fatal("the loss was recorded as an empty-library mark, not the replacement's LosesGame")
		}
	})
}
