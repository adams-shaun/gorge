package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// uncounterScen is a table with a DelaySpellUncounter registrar (the shape
// Mistrise Village and Theorist's Proxy lower to) and an instant.
type uncounterScen struct {
	*scen
	source, spell CardID
	src           Obj
}

func newUncounterScen(t *testing.T) *uncounterScen {
	t.Helper()
	set := cardsettest.Set(t, "fdn")
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	fid := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{})
	source := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Uncounter Source", Types: kop.TArtifact,
			Abilities: []kop.Ability{
				{Kind: kop.AbActivated, Prog: []kop.Instr{{Op: kop.OpDelayedProgram, A: kop.Const(kop.DelaySpellUncounter), C: 1}}},
				{Kind: kop.AbTriggered, AFlags: kop.AfLatent, Trig: kop.Trigger{Event: kop.TrigSpellCast, Filter: fid}},
			}},
		kop.CardDef{Name: "Bolt", Types: kop.TInstant, Cost: kop.ManaCost{Generic: 1}},
	)
	sc := newScenTableLib(t, tbl, da[0], Config{NoElide: true})
	if !sc.k.gen.anyDelaySpell {
		t.Fatal("genPrep.anyDelaySpell is off: the cast hook never runs")
	}
	return &uncounterScen{scen: sc, source: source, spell: source + 1, src: sc.perm(source, 0)}
}

func (u *uncounterScen) register() {
	e := &u.s.res.e
	e.ctrl, e.obj, e.card, e.ab = 0, u.src, u.source, 0
	u.s.exec(u.k, &u.k.t.Cards[u.source].Abilities[0].Prog[0])
}

// cast puts a new Bolt of p on the stack and runs the cast hook; it returns
// the stack index.
func (u *uncounterScen) cast(p uint8) int {
	o := u.obj(u.spell, p)
	u.s.objs[o].zone = kop.ZoneStack
	e := stackEntry{kind: seSpell, card: u.spell, ctrl: p, obj: o}
	u.s.stack[u.s.sp] = e
	u.s.sp++
	u.s.onCast(u.k, &u.s.stack[u.s.sp-1])
	return int(u.s.sp) - 1
}

// TestDelaySpellUncounterKernel: the registration fires on its controller's
// next cast with no pending trigger, and a counter (a ward counter: the
// same uncounterable gate as OpCounter) leaves that spell on the stack; a
// second spell cast the same turn is countered; the record is gone once the
// first spell leaves the stack; an unfired registration does not carry past
// the turn change, a fired one whose spell is on the stack does.
func TestDelaySpellUncounterKernel(t *testing.T) {
	u := newUncounterScen(t)
	u.register()
	if u.s.g.ndp != 1 || u.s.g.dp[0].ext != dpExtUncounter || u.s.g.dp[0].n&dpSpell == 0 {
		t.Fatalf("registration: ndp %d rec %+v, want one unfired uncounter record", u.s.g.ndp, u.s.g.dp[0])
	}
	if u.s.dprogReferenced(u.src) != true {
		t.Fatal("an unfired registration does not name its source")
	}

	first := u.cast(0)
	fo := u.s.stack[first].obj
	if u.s.npend != 0 {
		t.Fatalf("the static fire queued %d triggers, want none", u.s.npend)
	}
	if u.s.g.ndp != 1 || !dpUncounterFired(&u.s.g.dp[0]) || u.s.g.dp[0].objs[0] != fo {
		t.Fatalf("after the cast: ndp %d rec %+v, want the fired record over the spell", u.s.g.ndp, u.s.g.dp[0])
	}
	if u.s.dprogReferenced(u.src) {
		t.Fatal("a fired record still pins the registration's source")
	}
	u.s.wardCounterAt(u.k, first)
	if u.s.sp != 1 || u.s.objs[fo].zone != kop.ZoneStack {
		t.Fatalf("the protected spell was countered: sp %d zone %d", u.s.sp, u.s.objs[fo].zone)
	}

	second := u.cast(0)
	so := u.s.stack[second].obj
	u.s.wardCounterAt(u.k, second)
	if u.s.sp != 1 || u.s.objs[so].zone == kop.ZoneStack {
		t.Fatalf("the second spell was not countered: sp %d zone %d", u.s.sp, u.s.objs[so].zone)
	}
	if u.s.g.ndp != 1 {
		t.Fatalf("the second cast changed the record: ndp %d", u.s.g.ndp)
	}

	// the turn changes with the protected spell still on the stack: its
	// record stays (the Effect has no turn bound)
	u.s.dropDelayedSpell()
	if u.s.g.ndp != 1 {
		t.Fatalf("the fired record was dropped at the turn change with its spell on the stack: ndp %d", u.s.g.ndp)
	}

	// the first spell resolves: it leaves the stack and the record goes
	u.s.sp--
	u.s.objs[fo].zone = kop.ZoneNone
	u.s.placeFromK(u.k, fo, kop.ZoneGraveyard, false, kop.ZoneStack, false)
	u.s.sba(u.k)
	if u.s.g.ndp != 0 {
		t.Fatalf("the record outlived its spell: ndp %d", u.s.g.ndp)
	}

	// an unfired registration ends with the turn
	u.register()
	u.s.dropDelayedSpell()
	if u.s.g.ndp != 0 {
		t.Fatalf("an unfired registration carried past the turn change: ndp %d", u.s.g.ndp)
	}
	u.cast(0)
	if u.s.g.ndp != 0 {
		t.Fatalf("a cast after the turn change fired a dropped registration: ndp %d", u.s.g.ndp)
	}
}

// TestDelaySpellUncounterTwoStall: two fired records over one spell are two
// matches, gorge's CR 616.1 order ask: the kernel stalls loudly.
func TestDelaySpellUncounterTwoStall(t *testing.T) {
	u := newUncounterScen(t)
	u.register()
	u.register()
	i := u.cast(0)
	if u.s.g.ndp != 2 || !dpUncounterFired(&u.s.g.dp[1]) {
		t.Fatalf("ndp %d, want two fired records", u.s.g.ndp)
	}
	if !u.s.uncounterable(u.k, i) {
		t.Fatal("the spell is counterable")
	}
	if u.s.g.stalled != StallUncounterOrder {
		t.Fatalf("stalled %d, want StallUncounterOrder", u.s.g.stalled)
	}
}

// TestDelaySpellUncounterOpponent: the opponent's cast neither fires the
// registration nor is protected by it.
func TestDelaySpellUncounterOpponent(t *testing.T) {
	u := newUncounterScen(t)
	u.register()
	i := u.cast(1)
	if u.s.g.ndp != 1 || dpUncounterFired(&u.s.g.dp[0]) {
		t.Fatalf("the opponent's cast fired the registration: rec %+v", u.s.g.dp[0])
	}
	if u.s.uncounterable(u.k, i) {
		t.Fatal("the opponent's spell is protected")
	}
}
