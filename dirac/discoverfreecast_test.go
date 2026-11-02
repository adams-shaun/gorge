package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// discoverCastScen builds a resolving ability whose program is
// Discover 1, CastFree, Discover tail, CreateToken (a board-affecting tail:
// Hit the Mother Lode's Treasures), with Giant Growth on top of the library
// and Savannah Lions on the battlefield as its target.
func discoverCastScen(t *testing.T, ctrl uint8) (sc *scen, gg Obj, tokens func() int) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Giant Growth", Count: 2}, {Name: "Savannah Lions", Count: 2}, {Name: "Plains", Count: 16}}
	set := cardsettest.Set(t, "fdn")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	tok := CardID(len(tbl.Cards))
	src := tok + 1
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Token", Types: kop.TCreature | kop.TToken, Power: 1, Toughness: 1},
		kop.CardDef{Name: "Test Discover Cast", Types: kop.TCreature, Power: 1, Toughness: 1,
			Abilities: []kop.Ability{{Kind: kop.AbTriggered, Prog: []kop.Instr{
				{Op: kop.OpDiscover, Obj: kop.RefController, A: kop.Const(1)},
				{Op: kop.OpCastFree, Obj: kop.RefRemembered, C: 5}, // 5 = ZoneExile
				{Op: kop.OpDiscover, C: 1},
				{Op: kop.OpCreateToken, Obj: kop.RefController, A: kop.Const(1), C: int16(tok)},
				{Op: kop.OpClearRemembered},
			}}}})
	sc = newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	s := sc.s
	sc.perm(id["Savannah Lions"], ctrl)
	gg = sc.obj(id["Giant Growth"], ctrl)
	s.objs[gg].zone = kop.ZoneLibrary
	s.pl[ctrl].libTop(gg)
	srcObj := sc.perm(src, ctrl)
	e := stackEntry{kind: seTrigger, obj: srcObj, srcVer: s.objs[srcObj].ver, card: src, ab: 0, ctrl: ctrl, trigObj: NoObj}
	s.sp = 1
	s.stack[0] = e
	s.res = resCtx{e: e, active: true, lastTok: NoObj, eachObj: NoObj}
	s.runProg(sc.k)
	tokens = func() int {
		n := 0
		for o := 0; o < int(s.nobj); o++ {
			if s.objs[o].card == tok && s.objs[o].zone == kop.ZoneBattlefield {
				n++
			}
		}
		return n
	}
	return sc, gg, tokens
}

// TestDiscoverFreeCastTargetOrder: a Discover's free cast is announced, its
// target asked, before the tail after it runs (CR 701.57a, CR 608.2c; gorge
// effDiscover runs effDiscoverBottom only when the play is not suspended).
func TestDiscoverFreeCastTargetOrder(t *testing.T) {
	sc, gg, tokens := discoverCastScen(t, 0)
	s := sc.s
	d := m11FreeAsk(t, sc)
	m11Choose(t, sc, d, gg)
	if n := tokens(); n != 0 {
		t.Fatalf("tail ran before the cast was announced: %d tokens, want 0", n)
	}
	d = s.Step()
	if d.Kind != DkTarget {
		t.Fatalf("decision kind %d sub %d, want the cast's target ask", d.Kind, d.Sub)
	}
	if n := tokens(); n != 0 {
		t.Fatalf("at the target ask the tail had already run: %d tokens, want 0", n)
	}
	if z := s.objs[gg].zone; z != kop.ZoneStack {
		t.Fatalf("Giant Growth is in zone %d at its target ask, want the stack", z)
	}
	s.Apply(d.Options[0])
	d = s.Step()
	if n := tokens(); n != 1 {
		t.Fatalf("after the cast was announced: %d tokens, want 1", n)
	}
	if z := s.objs[gg].zone; z != kop.ZoneStack || s.sp != 1 || s.stack[0].obj != gg {
		t.Fatalf("Giant Growth zone %d, stack depth %d: want it alone on the stack", z, s.sp)
	}
	if d.Kind != DkPriority {
		t.Fatalf("decision kind %d after the tail, want priority", d.Kind)
	}
}

// TestDiscoverFreeCastNonActiveController: the controller is not the active
// player. The casts are made mid-resolution and the active player holds
// priority when the resolution is done (gorge), the hold left nothing
// behind to restore over a later pass (the Play carriers' lockstep).
func TestDiscoverFreeCastNonActiveController(t *testing.T) {
	sc, gg, tokens := discoverCastScen(t, 1)
	s := sc.s
	if s.active != 0 {
		t.Fatalf("active player %d, the scenario wants the controller to be the other", s.active)
	}
	d := m11FreeAsk(t, sc)
	m11Choose(t, sc, d, gg)
	d = s.Step()
	if d.Kind != DkTarget || d.Player != 1 {
		t.Fatalf("decision kind %d player %d, want the caster's (p1) target ask", d.Kind, d.Player)
	}
	if n := tokens(); n != 0 {
		t.Fatalf("at the target ask the tail had already run: %d tokens, want 0", n)
	}
	s.Apply(d.Options[0])
	d = s.Step()
	if n := tokens(); n != 1 {
		t.Fatalf("after the cast was announced: %d tokens, want 1", n)
	}
	if d.Kind != DkPriority || d.Player != s.active {
		t.Fatalf("decision kind %d player %d after the tail, want the active player's priority", d.Kind, d.Player)
	}
	if s.g.hold || s.g.priSave != 0 {
		t.Fatalf("hold %v priSave %#x left after the tail", s.g.hold, s.g.priSave)
	}
	// the active player passes: the other gets priority (a stale hold
	// restored over this pass handed it back to the active player)
	s.Apply(MakeAction(AkPass, 0, 0, 0))
	if d = s.Step(); d.Kind != DkPriority || d.Player != 1 {
		t.Fatalf("after the pass: decision kind %d player %d, want p1's priority", d.Kind, d.Player)
	}
}

// TestDiscoverFreeCastTailOldOrder: with the park off the tail runs first,
// as before (the pre-fix order the test above rejects).
func TestDiscoverFreeCastTailOldOrder(t *testing.T) {
	DebugNoFreeCastPark = true
	defer func() { DebugNoFreeCastPark = false }()
	sc, gg, tokens := discoverCastScen(t, 0)
	d := m11FreeAsk(t, sc)
	m11Choose(t, sc, d, gg)
	if n := tokens(); n != 1 {
		t.Fatalf("old order: %d tokens right after the answer, want 1", n)
	}
}
