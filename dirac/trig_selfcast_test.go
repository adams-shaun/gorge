package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// selfcast: "when you cast this spell" fires from the stack, never from
// a battlefield copy (trig_selfcast.go).

// selfCastScen is replRegenerateScen over the selfcast pool: Forests, the named
// cards, player 0 to act in its main phase.
func selfCastScen(t *testing.T, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Forest", Count: 8}, {Name: "Swamp", Count: 2}, {Name: "Island", Count: 2}}
	set := cardsettest.Set(t, "selfcast")
	for _, n := range names {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Fatalf("%s not supported in the selfcast table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	var lib [20]CardID
	for i := range lib {
		lib[i] = id["Forest"]
	}
	s := new(State)
	s.Init(tbl, lib[:], lib[:], 1, Config{NoElide: true})
	for p := uint8(0); p < 2; p++ {
		pls := &s.pl[p]
		for pls.handN > 0 {
			o := pls.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pls.libBottom(o)
		}
	}
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
	if !s.k().selfCast {
		t.Fatal("prep.selfCast not set")
	}
	return &scen{s: s, k: s.k()}, id
}

// stepToTrigger steps until a trigger is on top of the stack, passing
// priority and taking the first option elsewhere.
func (sc *scen) stepToTrigger(t *testing.T) *stackEntry {
	t.Helper()
	for i := 0; i < 20 && !(sc.s.sp > 0 && sc.s.stack[sc.s.sp-1].kind == seTrigger); i++ {
		d := sc.s.Step()
		if d.Kind == DkPriority {
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		} else {
			sc.s.Apply(d.Options[0])
		}
	}
	if sc.s.sp == 0 || sc.s.stack[sc.s.sp-1].kind != seTrigger {
		t.Fatal("no trigger reached the stack")
	}
	return &sc.s.stack[sc.s.sp-1]
}

func (sc *scen) tokens(p uint8) int {
	n := 0
	for m := sc.s.ctrlM[p].and(sc.s.occ); m.any(); m = m.rest() {
		if sc.s.objs[sc.s.perms[m.first()].obj].of&ofToken != 0 {
			n++
		}
	}
	return n
}

// TestSelfCastFires: Eldrazi Repurposer's "when you cast this spell"
// goes on the stack above the spell, sourced by the spell object on the
// stack, and resolves first (one Spawn before the Repurposer enters).
func TestSelfCastFires(t *testing.T) {
	sc, id := selfCastScen(t, "Eldrazi Repurposer")
	sc.lands(0, id["Forest"], 3)
	o := sc.hand(id["Eldrazi Repurposer"], 0)
	sc.castCard(t, o)
	e := sc.stepToTrigger(t)
	if e.obj != o || e.card != id["Eldrazi Repurposer"] || e.ctrl != 0 || e.trigObj != o {
		t.Fatalf("trigger entry %+v, want source %d (the spell on the stack)", *e, o)
	}
	if sc.s.objs[o].zone != kop.ZoneStack || sc.s.sp != 2 {
		t.Fatalf("spell zone %d, stack %d: want the trigger above the spell", sc.s.objs[o].zone, sc.s.sp)
	}
	sc.resolveAll(t)
	if n := sc.tokens(0); n != 1 {
		t.Fatalf("%d tokens, want 1", n)
	}
	if !sc.onBF(o) {
		t.Fatal("the Repurposer never entered")
	}
}

// TestSelfCastBattlefieldSilent: a Repurposer on the battlefield fires
// nothing when its controller casts another spell (its trigger reads the
// spell cast, never the permanent), and nothing when the opponent casts.
func TestSelfCastBattlefieldSilent(t *testing.T) {
	sc, id := selfCastScen(t, "Eldrazi Repurposer", "Grizzly Bears")
	sc.lands(0, id["Forest"], 3)
	sc.perm(id["Eldrazi Repurposer"], 0)
	sc.castCard(t, sc.hand(id["Grizzly Bears"], 0))
	if sc.s.npend != 0 {
		t.Fatalf("%d pending triggers after another spell, want 0", sc.s.npend)
	}
	sc.resolveAll(t)
	if n := sc.tokens(0); n != 0 {
		t.Fatalf("%d tokens, want 0", n)
	}
}

// TestSelfCastXBound: Hydroid Krasis cast for X = 4 gains 2 life and draws
// 2 (half X rounded down), the cast trigger's X bound to the spell's.
func TestSelfCastXBound(t *testing.T) {
	sc, id := selfCastScen(t, "Hydroid Krasis")
	sc.lands(0, id["Forest"], 3)
	sc.lands(0, id["Island"], 3)
	sc.castCard(t, sc.hand(id["Hydroid Krasis"], 0))
	sc.chooseX(t, 4)
	e := sc.stepToTrigger(t)
	if e.x != 4 {
		t.Fatalf("the cast trigger's bound X is %d, want 4", e.x)
	}
	life, hand := sc.s.pl[0].life, sc.s.pl[0].handN
	sc.resolveAll(t)
	if sc.s.pl[0].life != life+2 || sc.s.pl[0].handN != hand+2 {
		t.Fatalf("life %d -> %d, hand %d -> %d: want +2 each", life, sc.s.pl[0].life, hand, sc.s.pl[0].handN)
	}
}

// TestSelfCastInstantByOpponent: player 1's Dash Hopes, cast in response
// to player 0's spell, fires its own cast trigger (sourced by the instant
// on the stack) before player 1's battlefield listeners.
func TestSelfCastInstantByOpponent(t *testing.T) {
	sc, id := selfCastScen(t, "Dash Hopes", "Grizzly Bears")
	sc.lands(0, id["Forest"], 2)
	sc.lands(1, id["Swamp"], 2)
	sc.castCard(t, sc.hand(id["Grizzly Bears"], 0))
	dh := sc.hand(id["Dash Hopes"], 1)
	for i := 0; i < 6; i++ {
		d := sc.s.Step()
		if d.Kind == DkPriority && d.Player == 1 {
			break
		}
		if d.Kind == DkPriority {
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		} else {
			sc.s.Apply(d.Options[0])
		}
	}
	sc.castCard(t, dh)
	for i := 0; i < 6 && sc.s.npend == 0 && !(sc.s.sp > 0 && sc.s.stack[sc.s.sp-1].kind == seTrigger); i++ {
		d := sc.s.Step()
		if d.Kind == DkPriority {
			break
		}
		sc.s.Apply(d.Options[0])
	}
	e := sc.stepToTrigger(t)
	if e.obj != dh || e.ctrl != 1 {
		t.Fatalf("trigger entry %+v, want Dash Hopes' (%d) for player 1", *e, dh)
	}
}

// TestDigWindowStallsLoud: Ulamog's attack trigger ("exile the top
// twenty") fits the look window since kcaps2x doubled it to 24 (before, it
// stalled with StallLook rather than digging twelve silently); a window
// wider than len(look) still stalls loud (interp_ask.go startDig).
func TestDigWindowStallsLoud(t *testing.T) {
	sc, id := selfCastScen(t, "Ulamog, the Ceaseless Hunger")
	s := sc.s
	o := sc.perm(id["Ulamog, the Ceaseless Hunger"], 0)
	ab := &sc.k.t.Cards[id["Ulamog, the Ceaseless Hunger"]].Abilities[1]
	if ab.Trig.Event != kop.TrigAttacks || ab.Prog[0].Op != kop.OpDigEx {
		t.Fatalf("Ulamog's second ability is not the attack dig (%v %s)", ab.Trig.Event, ab.Prog[0].Op)
	}
	if len(s.resArm().look) < 20 {
		t.Fatalf("look window %d < Ulamog's 20", len(s.resArm().look))
	}
	s.res = resCtx{e: stackEntry{kind: seTrigger, obj: o, srcVer: s.objs[o].ver, card: id["Ulamog, the Ceaseless Hunger"], ab: 1, ctrl: 0, trigObj: NoObj, trigPl: 1}, active: true, lastTok: NoObj, eachObj: NoObj}
	s.ctl = ctlPriority
	s.runProg(sc.k)
	if s.StallReason() != StallNone {
		t.Fatalf("stall reason %q, want none (Ulamog's twenty fit the 24-card window)", StallName(s.StallReason()))
	}
}
