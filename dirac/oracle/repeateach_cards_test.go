package oracle

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// RepeatEach over cards, one Role token attached to each (Asinine Antics, Become
// Brutes, Twisted Sewer-Witch): the kernel's OpRepeatEach lists the cards once
// (a battlefield filter in seat order from the controller, or the target
// slot's objects in pick order) and mints a Role attached to each, inside one
// zone batch. Each case is a constructed matchup played by a policy that
// plays lands and casts its costliest spell, recorded once
// (REPEATEACH_RECORD=1) as a repro whose gorge log holds two Role mints
// of one resolution. Replayed, both engines agree through the whole
// game. Each repro is also a row of TestStdSetReprosLockstep.

type repeatEachCase struct {
	name, card string
	decks      [2][]string
}

func repeatEachCases() []repeatEachCase {
	opp := slices.Concat(deckOf(24, "Grizzly Bears"), deckOf(16, "Forest"))
	return []repeatEachCase{
		{name: "repeateach_antics_roles_each_opposing_creature", card: "Asinine Antics", decks: [2][]string{
			slices.Concat(deckOf(8, "Asinine Antics"), deckOf(10, "Grizzly Bears"), deckOf(22, "Island")), opp}},
		{name: "repeateach_witch_roles_each_rat", card: "Twisted Sewer-Witch", decks: [2][]string{
			slices.Concat(deckOf(8, "Twisted Sewer-Witch"), deckOf(16, "Muck Rats"), deckOf(16, "Swamp")), opp}},
		{name: "repeateach_brutes_roles_each_target", card: "Become Brutes", decks: [2][]string{
			slices.Concat(deckOf(8, "Become Brutes"), deckOf(14, "Grizzly Bears"), deckOf(18, "Mountain")), opp}},
	}
}

// repeatEachPolicy plays a land, else casts the costliest spell, else plays
// randomly; a target ask takes a target before it takes Done, so Become
// Brutes aims at two creatures when it can.
func repeatEachPolicy(seed uint64) Policy {
	base := pipFilterPolicy(seed)
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DTarget {
			var picks []Canon
			for _, a := range c.D.Options {
				if a.Kind() == ActTarget {
					picks = append(picks, a)
				}
			}
			if len(picks) > 0 {
				d := *c.D
				d.Options = picks
				return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			}
		}
		return base.Pick(c)
	})
}

// repeatEachRoles is a Check hook that scans gorge's log as it grows for one
// resolution's Role mints: each Role token enters attached, so an Attach event
// whose object is a Role is one mint, and the loop's mints follow one another
// with only the token's own creation between (a gap of at most two events).
// at is the number of the decision (the Check call) that first saw a second
// mint in such a run, 0 for none.
type repeatEachRoles struct {
	seen  int
	last  int // the log index of the last Role attach
	run   int // Role attaches in the current run
	roles int
	at    int
	n     int
}

func (f *repeatEachRoles) check(ref *GorgeSide, ker Side) *Divergence {
	f.n++
	evs := ref.E.L.Events
	for ; f.seen < len(evs); f.seen++ {
		e := &evs[f.seen]
		if e.Kind != events.Attach {
			continue
		}
		if o := ref.E.G.Obj(e.Obj); o != nil && kroleIsRole(o) {
			f.roles++
			if f.run > 0 && f.seen-f.last <= 2 {
				f.run++
			} else {
				f.run = 1
			}
			f.last = f.seen
			if f.run == 2 && f.at == 0 {
				f.at = f.n
			}
		}
	}
	return nil
}

func repeatEachPlay(t *testing.T, reg *cards.Registry, r *Repro, pol Policy) (Result, *repeatEachRoles) {
	t.Helper()
	set := printedDelveSet(t, r.Cards)
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	f := &repeatEachRoles{}
	rc := r.RunConfig()
	rc.Check = f.check
	rc.MaxDecisions = 4000
	if pol == nil {
		pol = &ScriptPolicy{Actions: r.Actions}
		rc.MaxDecisions = len(r.Actions) + 200
	}
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	return Lockstep(m, r.Seed, k, pol, rc), f
}

// TestRepeatEachCardsRecord plays the cases over seeds until one makes two
// Roles with the engines in agreement, and writes the repro. It runs only with
// REPEATEACH_RECORD=1; the games it writes are the committed repros.
func TestRepeatEachCardsRecord(t *testing.T) {
	if os.Getenv("REPEATEACH_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	for _, c := range repeatEachCases() {
		t.Run(c.card, func(t *testing.T) {
			for seed := uint64(1); seed <= 600; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: "full", Kernel: "custom", MaxTurns: 40}
				res, f := repeatEachPlay(t, reg, r, repeatEachPolicy(seed))
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
				}
				if f.at == 0 {
					continue
				}
				r.setActions(res.Actions)
				if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
					t.Fatal(err)
				}
				t.Logf("seed %d: %d Roles, the second at decision %d of %d", seed, f.roles, f.at, len(res.Actions))
				return
			}
			t.Fatalf("no seed made two Roles with %s", c.card)
		})
	}
}

// TestRepeatEachCardsReprosLockstep replays the recorded games: no divergence,
// every scripted action offered, and two or more Roles made -- the lockstep
// alone would accept a script that never cast the carrier.
func TestRepeatEachCardsReprosLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	for _, c := range repeatEachCases() {
		t.Run(c.card, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			res, f := repeatEachPlay(t, reg, r, nil)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if f.at == 0 {
				t.Fatalf("the recorded game never made two Roles with %s", c.card)
			}
			t.Logf("%s: %d Roles, the second at decision %d of %d", c.card, f.roles, f.at, len(r.Actions))
		})
	}
}
