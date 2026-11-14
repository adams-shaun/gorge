package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// the four api:Play free-cast populations and the
// ReplaceGraveyard rider on OpCastFree. The lockstep proves the cards; these
// tests pin the kernel branches (Amount$ 1, target population, graveyard
// seat order, ForgetPlayed, and the counter destination the State audit
// chose).

func m11Scen(t *testing.T, prog []kop.Instr) (*scen, CardID, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Island", Count: 20},
		{Name: "Opt", Count: 2},
		{Name: "Shock", Count: 2},
		{Name: "Refute", Count: 1},
	}
	set := cardsettest.Set(t, "freeplay")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	lib, ok := id["Island"]
	if !ok {
		t.Fatal("Island not in the freeplay matchup")
	}
	synth := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name:      "M11 Play Test",
		Types:     kop.TCreature,
		Power:     1,
		Toughness: 1,
		Abilities: []kop.Ability{{Kind: kop.AbTriggered, Prog: prog}},
	})
	return newScenTableLib(t, tbl, lib, Config{NoElide: true}), synth, id
}

// m11Start runs the synthetic ability as a resolving trigger. The entry is
// pushed so a program that runs to its end can finishResolve.
func m11Start(sc *scen, card CardID, set func(*stackEntry)) *stackEntry {
	e := stackEntry{kind: seTrigger, obj: NoObj, card: card, ab: 0, ctrl: 0, trigObj: NoObj}
	if set != nil {
		set(&e)
	}
	sc.s.sp = 1
	sc.s.stack[0] = e
	sc.s.res = resCtx{e: e, active: true, lastTok: NoObj, eachObj: NoObj}
	sc.s.runProg(sc.k)
	return &sc.s.res.e
}

// m11FreeAsk returns the free-cast ask, failing if it is not posed.
func m11FreeAsk(t *testing.T, sc *scen) Decision {
	t.Helper()
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenCastFree {
		t.Fatalf("decision kind %d sub %d, want the free-cast ask", d.Kind, d.Sub)
	}
	return d
}

// m11Choose applies the option casting o.
func m11Choose(t *testing.T, sc *scen, d Decision, o Obj) {
	t.Helper()
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == AkChoose && Obj(a.Source()) == o {
			sc.s.Apply(a)
			return
		}
	}
	t.Fatalf("card %d not offered (%d options)", o, d.N)
}

// m11Drive passes priority and takes the first option until done or the
// predicate holds.
func m11Drive(t *testing.T, sc *scen, done func() bool) {
	t.Helper()
	for i := 0; i < 200 && !done(); i++ {
		d := sc.s.Step()
		if d.Kind == DkNone {
			return
		}
		if d.Kind == DkPriority {
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
			continue
		}
		sc.s.Apply(d.Options[0])
	}
}

func TestFreePlayAmountOne(t *testing.T) {
	sc, synth, id := m11Scen(t, []kop.Instr{{Op: kop.OpCastFree, Obj: kop.RefRemembered, C: 13}})
	var rem [3]Obj
	for i := range rem {
		rem[i] = sc.obj(id["Opt"], 0)
		sc.s.place(rem[i], kop.ZoneExile, false)
		sc.s.g.rem[i] = rem[i]
	}
	sc.s.g.nrem = 3
	m11Start(sc, synth, nil)
	d := m11FreeAsk(t, sc)
	if d.N != 4 {
		t.Fatalf("%d options, want 3 remembered cards and done", d.N)
	}
	m11Choose(t, sc, d, rem[1])
	if sc.s.g.nfq != 1 || sc.s.g.freeQ[0] != rem[1] {
		t.Fatalf("freeQ %v, want [%d]", sc.s.g.freeQ[:sc.s.g.nfq], rem[1])
	}
	if sc.s.g.nfree != 0 {
		t.Fatalf("%d candidates left after Amount$ 1, want 0", sc.s.g.nfree)
	}
	// the ask closed: the next decision is not another free-cast pick
	if d2 := sc.s.Step(); d2.Kind == DkChoose && d2.Sub == ChGenCastFree {
		t.Fatalf("a second free-cast ask was posed after Amount$ 1")
	}
}

// TestFreePlayTailBounce: a Play's "if you don't cast it, return it to hand"
// tail (Solstice Revelations) runs once the cast is announced (the parked
// program, act_discoverfreecast.go); gorge has begun the cast by then,
// so the plain bounce must leave the card alone, in exile while it waits and
// on the stack once cast.
func TestFreePlayTailBounce(t *testing.T) {
	sc, synth, id := m11Scen(t, []kop.Instr{
		{Op: kop.OpCastFree, Obj: kop.RefRemembered, C: 13},
		{Op: kop.OpBounce, Obj: kop.RefRemembered},
	})
	o := sc.obj(id["Opt"], 0)
	sc.s.place(o, kop.ZoneExile, false)
	sc.s.g.rem[0] = o
	sc.s.g.nrem = 1
	src := sc.perm(synth, 0) // the tail's Ref resolves through the source's context
	m11Start(sc, synth, func(e *stackEntry) { e.obj = src })
	d := m11FreeAsk(t, sc)
	m11Choose(t, sc, d, o)
	if z := sc.s.objs[o].zone; z != kop.ZoneExile {
		t.Fatalf("queued free cast is in zone %d after the tail, want exile", z)
	}
	cast := false
	m11Drive(t, sc, func() bool {
		cast = cast || sc.s.objs[o].zone == kop.ZoneStack
		return cast
	})
	if !cast {
		t.Fatalf("the picked card was never cast (zone %d)", sc.s.objs[o].zone)
	}
	m11Drive(t, sc, func() bool { return sc.s.res.ask != raGenFreeParked })
	if z := sc.s.objs[o].zone; z != kop.ZoneStack {
		t.Fatalf("the tail moved the cast spell: zone %d, want the stack", z)
	}
}

func TestFreePlayTargetedGraveExiles(t *testing.T) {
	sc, synth, id := m11Scen(t, []kop.Instr{{Op: kop.OpCastFree, Obj: kop.RefTarget0, C: 36}})
	o := sc.obj(id["Opt"], 0)
	sc.s.place(o, kop.ZoneGraveyard, false)
	m11Start(sc, synth, func(e *stackEntry) {
		e.tgt[0] = TCode(o)
		e.nt = 1
	})
	d := m11FreeAsk(t, sc)
	if d.N != 2 {
		t.Fatalf("%d options, want the targeted card and done", d.N)
	}
	m11Choose(t, sc, d, o)
	cast := false
	m11Drive(t, sc, func() bool {
		z := sc.s.objs[o].zone
		if z == kop.ZoneStack {
			cast = true
		}
		return cast && z != kop.ZoneStack
	})
	if z := sc.s.objs[o].zone; z != kop.ZoneExile {
		t.Fatalf("resolved free cast is in zone %d, want exile", z)
	}

	// the fizzle exit uses the same ReplaceGraveyard rider
	for _, fizzled := range []bool{false, true} {
		sc, _, id := m11Scen(t, nil)
		o := sc.obj(id["Opt"], 0)
		sc.s.place(o, kop.ZoneStack, false)
		e := stackEntry{kind: seSpell, obj: o, ctrl: 0, flags: sfFree | sfFreeGY}
		sc.s.sp = 1
		sc.s.stack[0] = e
		sc.s.res = resCtx{e: e, active: true, lastTok: NoObj, eachObj: NoObj}
		sc.s.finishResolve(sc.k, fizzled)
		if z := sc.s.objs[o].zone; z != kop.ZoneExile {
			t.Fatalf("fizzled=%v: free cast is in zone %d, want exile", fizzled, z)
		}
	}
}

func TestFreePlayCounterDestination(t *testing.T) {
	// gorge effCounter rewrites a graveyard destination with
	// ExilesInsteadOfGraveyard (pin e276bf2b3; ReplaceGraveyard$ joined the
	// flashback family): a ReplaceGraveyard spell countered without a
	// Destination$ is exiled, and a named hand destination is honoured.
	for _, tc := range []struct {
		name string
		dest int16
		want kop.Zone
	}{
		{"counter", 0, kop.ZoneExile},
		{"counter to hand", kop.CtrDestHand, kop.ZoneHand},
		{"counter to exile", kop.CtrDestExile, kop.ZoneExile},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc, _, id := m11Scen(t, nil)
			o := sc.obj(id["Opt"], 0)
			sc.s.place(o, kop.ZoneStack, false)
			e := stackEntry{kind: seSpell, obj: o, ctrl: 0, flags: sfFree | sfFreeGY}
			sc.s.sp = 1
			sc.s.stack[0] = e
			if tc.dest == 0 {
				sc.s.counterAt(sc.k, 0)
			} else {
				sc.s.counterTo(sc.k, 0, tc.dest)
			}
			if z := sc.s.objs[o].zone; z != tc.want {
				t.Fatalf("countered free cast is in zone %d, want %d", z, tc.want)
			}
		})
	}
}

func TestFreePlayGraveSeatOrder(t *testing.T) {
	sc, synth, id := m11Scen(t, nil)
	fid := kop.FilterID(len(sc.k.t.Filters))
	sc.k.t.Filters = append(sc.k.t.Filters, kop.Filter{Types: kop.TInstant})
	// the synthetic card's program is read from the table, so patch it after
	// the filter exists
	sc.k.t.Cards[synth].Abilities[0].Prog = []kop.Instr{{Op: kop.OpCastFree, C: 44, F: fid}}
	s0 := sc.obj(id["Opt"], 0)
	s1 := sc.obj(id["Opt"], 1)
	sc.s.place(s0, kop.ZoneGraveyard, false)
	sc.s.place(s1, kop.ZoneGraveyard, false)
	m11Start(sc, synth, nil)
	d := m11FreeAsk(t, sc)
	if d.N != 3 {
		t.Fatalf("%d options, want done and two graveyard cards", d.N)
	}
	if Obj(d.Options[1].Source()) != s0 || Obj(d.Options[2].Source()) != s1 {
		t.Fatalf("options %d,%d, want seat 0 then seat 1 (%d,%d)", d.Options[1].Source(), d.Options[2].Source(), s0, s1)
	}
}

func TestFreePlayForgetPlayed(t *testing.T) {
	sc, synth, id := m11Scen(t, []kop.Instr{{Op: kop.OpCastFree, Obj: kop.RefRemembered, C: 29}})
	var rem [2]Obj
	for i := range rem {
		rem[i] = sc.obj(id["Opt"], 0)
		sc.s.place(rem[i], kop.ZoneExile, false)
		sc.s.g.rem[i] = rem[i]
	}
	sc.s.g.nrem = 2
	m11Start(sc, synth, nil)
	d := m11FreeAsk(t, sc)
	m11Choose(t, sc, d, rem[0])
	// the pick only queues the card: no cast has begun yet, so nothing is
	// forgotten (gorge forgets on the PlayDone pass, after the cast commits)
	if sc.s.g.nrem != 2 || sc.s.g.rem[0] != rem[0] || sc.s.g.rem[1] != rem[1] {
		t.Fatalf("remembered set %v after the pick, want %d then %d (the cast has not begun)", sc.s.g.rem[:sc.s.g.nrem], rem[0], rem[1])
	}
	if sc.s.g.freeQ[0] != rem[0] {
		t.Fatalf("freeQ[0] = %d, want the queued card %d", sc.s.g.freeQ[0], rem[0])
	}
	// the freeQ drain begins the cast: the card reaches the stack, and the
	// commit drops it from the remembered set (forge's PlayDone timing)
	cast := false
	m11Drive(t, sc, func() bool {
		z := sc.s.objs[rem[0]].zone
		cast = cast || z == kop.ZoneStack
		return cast
	})
	if !cast {
		t.Fatalf("the picked card was never cast (zone %d)", sc.s.objs[rem[0]].zone)
	}
	if sc.s.g.nrem != 1 || sc.s.g.rem[0] != rem[1] {
		t.Fatalf("remembered set %v, want only %d (ForgetPlayed dropped the begun card)", sc.s.g.rem[:sc.s.g.nrem], rem[1])
	}
}
