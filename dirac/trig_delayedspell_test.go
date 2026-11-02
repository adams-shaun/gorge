package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// newDelaySpellScen is a scenario over the delayspell pool; it returns the
// deck cards' ids by name.
func newDelaySpellScen(t *testing.T) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Teach by Example", Count: 1}, {Name: "Shock", Count: 1},
		{Name: "Grizzly Bears", Count: 1}, {Name: "Rimefire Torque", Count: 1}, {Name: "Mountain", Count: 20}}
	set := cardsettest.Set(t, "delayspell")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Mountain"], Config{NoElide: true})
	if !sc.k.gen.anyDelaySpell || !sc.k.gen.anyDelayUp {
		t.Fatalf("genPrep.anyDelaySpell %v anyDelayUp %v, want both set", sc.k.gen.anyDelaySpell, sc.k.gen.anyDelayUp)
	}
	return sc, id
}

// register resolves Teach by Example for p: one delayed SpellCast record.
func (sc *scen) delaySpellRegister(t *testing.T, teach CardID, p uint8) {
	t.Helper()
	o := sc.obj(teach, p)
	sc.s.res.e.ctrl, sc.s.res.e.obj, sc.s.res.e.card = p, o, teach
	prog := sc.k.t.Cards[teach].Abilities[0].Prog
	for i := range prog {
		sc.s.exec(sc.k, &prog[i])
	}
}

// delaySpellCast runs the cast hook for a spell of card c cast by p.
func (sc *scen) delaySpellCast(c CardID, p uint8) Obj {
	o := sc.obj(c, p)
	sc.s.objs[o].zone = kop.ZoneStack // the cast filter reads the stack
	e := stackEntry{card: c, ctrl: p, obj: o}
	sc.s.onCast(sc.k, &e)
	return o
}

// TestDelayspellFiresOnce: a registration fires on its controller's next
// instant or sorcery cast, with that spell as the triggering object, and is
// gone after; a creature spell and the opponent's instant leave it live;
// two registrations both fire on one cast.
func TestDelayspellFiresOnce(t *testing.T) {
	sc, id := newDelaySpellScen(t)
	teach, shock, bears := id["Teach by Example"], id["Shock"], id["Grizzly Bears"]
	sc.delaySpellRegister(t, teach, 0)
	if sc.s.g.ndp != 1 || sc.s.g.dp[0].n&dpSpell == 0 || sc.s.g.dp[0].n&dpWhenMask != 0 {
		t.Fatalf("registered %d records, n=%#x; want 1 flagged dpSpell with no timing bits", sc.s.g.ndp, sc.s.g.dp[0].n)
	}
	sc.delaySpellCast(bears, 0)
	if sc.s.npend != 0 || sc.s.g.ndp != 1 {
		t.Fatalf("creature cast: %d pending, %d registered; want 0 and 1", sc.s.npend, sc.s.g.ndp)
	}
	sc.delaySpellCast(shock, 1)
	if sc.s.npend != 0 || sc.s.g.ndp != 1 {
		t.Fatalf("opponent's instant: %d pending, %d registered; want 0 and 1", sc.s.npend, sc.s.g.ndp)
	}
	o := sc.delaySpellCast(shock, 0)
	if sc.s.npend != 1 || sc.s.g.ndp != 0 {
		t.Fatalf("own instant: %d pending, %d registered; want 1 and 0", sc.s.npend, sc.s.g.ndp)
	}
	if p := sc.s.pend[0]; p.trigObj != o || p.card != teach || p.ab != 1 || p.ctrl != 0 {
		t.Fatalf("pending %+v: want Teach by Example's latent ab 1 over the cast spell %d", p, o)
	}
	sc.s.npend = 0
	sc.delaySpellCast(shock, 0)
	if sc.s.npend != 0 {
		t.Fatalf("second instant: %d pending, want 0 (one-shot)", sc.s.npend)
	}
	sc.delaySpellRegister(t, teach, 0)
	sc.delaySpellRegister(t, teach, 0)
	sc.delaySpellCast(shock, 0)
	if sc.s.npend != 2 || sc.s.g.ndp != 0 {
		t.Fatalf("two registrations: %d pending, %d registered; want 2 and 0", sc.s.npend, sc.s.g.ndp)
	}
}

// TestDelayspellExpires: the turn change drops the SpellCast records and
// keeps the others in order.
func TestDelayspellExpires(t *testing.T) {
	sc, id := newDelaySpellScen(t)
	teach := id["Teach by Example"]
	sc.delaySpellRegister(t, teach, 0)
	sc.s.exec(sc.k, &kop.Instr{Op: kop.OpDelayedProgram, C: 1, A: kop.Const(kop.DelayUpkeep)})
	sc.delaySpellRegister(t, teach, 1)
	if sc.s.g.ndp != 3 {
		t.Fatalf("%d registered, want 3", sc.s.g.ndp)
	}
	sc.s.newTurnGen()
	if sc.s.g.ndp != 1 || sc.s.g.dp[0].n&dpSpell != 0 || sc.s.g.dp[0].n&dpWhenMask>>dpWhenShift != kop.DelayUpkeep {
		t.Fatalf("after the turn change %d registered (n=%#x), want the upkeep record only", sc.s.g.ndp, sc.s.g.dp[0].n)
	}
}

// TestDelayspellNotAtEndStep: the end step fires its own records and keeps
// a SpellCast registration.
func TestDelayspellNotAtEndStep(t *testing.T) {
	sc, id := newDelaySpellScen(t)
	teach := id["Teach by Example"]
	sc.delaySpellRegister(t, teach, 0)
	sc.s.exec(sc.k, &kop.Instr{Op: kop.OpDelayedProgram, C: 1})
	sc.s.fireDelayedGen(sc.k)
	if sc.s.npend != 1 || sc.s.g.ndp != 1 || sc.s.g.dp[0].n&dpSpell == 0 {
		t.Fatalf("end step: %d pending, %d registered; want 1 and the SpellCast record", sc.s.npend, sc.s.g.ndp)
	}
	sc.s.npend = 0
	sc.s.fireDelayedUpkeep(sc.k)
	sc.s.fireDelayedWhen(sc.k, 2)
	if sc.s.npend != 0 || sc.s.g.ndp != 1 {
		t.Fatalf("upkeep/end of combat: %d pending, %d registered; want 0 and 1", sc.s.npend, sc.s.g.ndp)
	}
}

// TestDelayspellLatentNotSubscribed: a latent SpellCast ability (Rimefire
// Torque's) is never a cast listener.
func TestDelayspellLatentNotSubscribed(t *testing.T) {
	sc, id := newDelaySpellScen(t)
	torque := id["Rimefire Torque"]
	lat := -1
	for j, a := range sc.k.t.Cards[torque].Abilities {
		if a.AFlags&kop.AfLatent != 0 && a.Trig.Event == kop.TrigSpellCast {
			lat = j
		}
	}
	if lat < 0 {
		t.Fatal("Rimefire Torque has no latent SpellCast ability")
	}
	o := sc.perm(torque, 0)
	if sc.s.subs[kop.TrigSpellCast].has(sc.slot(o)) {
		t.Fatal("Rimefire Torque subscribes to SpellCast")
	}
	sc.delaySpellCast(id["Shock"], 0)
	if sc.s.npend != 0 {
		t.Fatalf("%d pending after an instant cast, want 0", sc.s.npend)
	}
}

// TestDelayspellTokenSourceKept: a delayed program's source is referenced
// (never recycled by newToken while the record names it).
func TestDelayspellTokenSourceKept(t *testing.T) {
	sc, id := newDelaySpellScen(t)
	sc.delaySpellRegister(t, id["Teach by Example"], 0)
	if !sc.s.objReferenced(sc.s.g.dp[0].src) {
		t.Fatal("the record's source is not referenced")
	}
}
