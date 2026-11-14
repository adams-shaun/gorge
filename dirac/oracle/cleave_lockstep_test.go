package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The cleave lockstep (CR 702.148a): Fierce Retribution ("Destroy target
// [attacking] creature", cleave {5}{W}) is cast for its cleave cost, with the
// bracket-free text's target (any creature) and its destroy, identically in
// gorge and the kernel. The printed half only targets an attacking creature,
// so a cleaved cast that resolves against a creature that is not attacking
// is the text change observed, not the printed spell.
//
// gorge prices no cleaved cast in its planner, so the option exists only
// while the floating pool pays {5}{W}: Channel the Suns ({W}{U}{B}{R}{G})
// and then Dark Ritual ({B}{B}{B}, paid from that pool) float seven mana in
// the caster's main phase, and the policy casts the cleaved spell from it.

const (
	cleaveCard    = "Fierce Retribution"
	cleaveChannel = "Channel the Suns"
	cleaveRitual  = "Dark Ritual"
	cleaveBears   = "Grizzly Bears"
)

// cleavePolicy plays lands, casts two Grizzly Bears (a creature on the
// board to target), floats the ritual mana, and casts the case card for its
// cleave cost the moment that option is offered (never at its printed
// cost). Every other pick is the seeded random policy.
type cleavePolicy struct {
	rnd   Policy
	fired int
	bears int
}

func (p *cleavePolicy) Pick(c *PickCtx) Canon {
	switch c.D.Kind {
	case DMulligan:
		return pickKind(c, ActKeep)
	case DPriority:
		if c.G == nil {
			break
		}
		name := func(a Canon) string { return c.G.M.InstanceCard(int(a.Src())) }
		for _, a := range c.D.Options {
			if a.Kind() == ActCast && a.Mode()&0xf == castModeOther && name(a) == cleaveCard {
				p.fired++
				return a
			}
		}
		if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActPlayLand }); ok {
			return a
		}
		// Dark Ritual is offered only while the pool pays its {B}: the
		// deck has no Swamp, so it follows Channel the Suns
		for _, want := range []string{cleaveRitual, cleaveChannel} {
			if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActCast && name(a) == want }); ok {
				return a
			}
		}
		if p.bears < 2 {
			if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActCast && name(a) == cleaveBears }); ok {
				p.bears++
				return a
			}
		}
		if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActPass }); ok {
			return a
		}
	}
	return p.rnd.Pick(c)
}

// cleaveWitness observes one seed: a cleaved cast of the case card must
// resolve and destroy a battlefield creature.
type cleaveWitness struct {
	cleaved   map[state.ObjID]bool // CastInfo said this stack object was cleaved
	evAt      int
	armed     bool
	armedObj  state.ObjID
	resolved  bool
	destroyed bool
}

func (w *cleaveWitness) check(ref *GorgeSide, _ Side) *Divergence {
	evs := ref.E.L.Events
	if w.evAt > len(evs) {
		w.evAt, w.armed = 0, false
	}
	for i := w.evAt; i < len(evs); i++ {
		e := &evs[i]
		if !w.armed {
			if e.Kind == events.CastInfo && strings.Contains(e.Counter, "cleaved") {
				w.cleaved[e.Obj] = true
			}
			if e.Kind == events.Resolve && e.Text == cleaveCard && w.cleaved[e.Obj] {
				w.armed, w.armedObj, w.resolved = true, e.Obj, true
			}
			continue
		}
		if e.Kind == events.MoveZone && e.From == state.ZBattlefield && e.To == state.ZGraveyard {
			w.destroyed = true
		}
		if e.Kind == events.MoveZone && e.Obj == w.armedObj && e.From == state.ZStack {
			w.armed = false // the spell left the stack: its resolution is over
		}
	}
	w.evAt = len(evs)
	return nil
}

// cleaveDeck builds forty cards from (name, count) pairs, padded with land.
func cleaveDeck(t *testing.T, reg *cards.Registry, land string, spells map[string]int, order []string) []*cards.Card {
	t.Helper()
	var deck []*cards.Card
	add := func(name string, n int) {
		c, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("%s not in the corpus", name)
		}
		for i := 0; i < n; i++ {
			deck = append(deck, c)
		}
	}
	for _, n := range order {
		add(n, spells[n])
	}
	add(land, 40-len(deck))
	return deck
}

// TestCleaveLockstep plays Fierce Retribution identically for its cleave
// cost; a seed counts only when the policy cast it cleaved and that cast
// resolved and destroyed a creature.
func TestCleaveLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	order := []string{cleaveCard, cleaveChannel, cleaveRitual, cleaveBears}
	main := cleaveDeck(t, reg, "Forest", map[string]int{cleaveCard: 4, cleaveChannel: 4, cleaveRitual: 4, cleaveBears: 4}, order)
	opp := cleaveDeck(t, reg, "Plains", map[string]int{cleaveBears: 8, "Hill Giant": 8}, []string{cleaveBears, "Hill Giant"})
	set := printedDelveSet(t, [2][]string{{cleaveCard, cleaveChannel, cleaveRitual, cleaveBears, "Forest"}, {cleaveBears, "Hill Giant", "Plains"}})
	tbl, _, _, err := set.Matchup(overloadEntries(main), overloadEntries(opp))
	if err != nil {
		t.Fatal(err)
	}
	m, err := NewMatchup(reg, [2]string{"cleave caster", "opponent"}, [2][]*cards.Card{main, opp}, 0)
	if err != nil {
		t.Fatal(err)
	}
	fired := 0
	for seed := uint64(1); seed <= 60; seed++ {
		k, err := SetSide(set, tbl, Config{})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := &cleavePolicy{rnd: RandomPolicy(seed)}
		w := &cleaveWitness{cleaved: map[state.ObjID]bool{}}
		res := Lockstep(m, seed, k, pol, RunConfig{MaxDecisions: 8000, MaxTurns: 24, Check: w.check})
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v\nref %v\nker %v\n%v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail)
		}
		fired += pol.fired
		if w.resolved && w.destroyed {
			t.Logf("seed %d: %s cast cleaved %d time(s) and destroyed a creature", seed, cleaveCard, pol.fired)
			return
		}
	}
	t.Fatalf("no seed cast and resolved %s cleaved with a destroy observable (cleaved casts: %d)", cleaveCard, fired)
}
