package kgen

import (
	"fmt"
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The hosted-Commander misc shapes in lockstep (ticket cli-20261127T065132Z-290de7f0).
// Each shape's card is built, with real corpus fillers, into an in-process
// table through the ordinary kgen tableOf path (cardset.Raw: lowered, no
// evidence needed) and played against gorge over seeds of ordinary play with
// a policy that casts, activates and attacks whenever it can. A shape's count
// of gorge-side resolutions is asserted, so a pool that never reaches the card
// cannot pass.

// miscShape is one card's lockstep: its matchups (two 40-card decks each) and
// the gorge-side tally that proves the shape ran.
type miscShape struct {
	card     string
	matchups [][2][]cardset.Entry
	seeds    uint64
	// reached reads one game's gorge events and counts the shape's resolutions.
	reached func(g *state.Game, evs []events.Event) int
	min     int // the least total reach over all games
}

// miscCardNames is every distinct card name the matchups name.
func miscCardNames(m [][2][]cardset.Entry) []string {
	seen := map[string]bool{}
	for _, md := range m {
		for _, d := range md {
			for _, e := range d {
				seen[e.Name] = true
			}
		}
	}
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// miscPolicy casts, activates and plays lands whenever it can; in combat it
// attacks with everything and blocks with everything; else random.
func miscPolicy(seed uint64) oracle.Policy {
	rnd := oracle.RandomPolicy(seed)
	return oracle.PolicyFunc(func(c *oracle.PickCtx) oracle.Canon {
		buf := make([]oracle.Canon, 0, 8)
		for _, a := range c.D.Options {
			switch c.D.Kind {
			case oracle.DAttackers:
				if a.Kind() == oracle.ActAttack || a.Kind() == oracle.ActDone || a.Kind() == oracle.ActPass {
					buf = append(buf, a)
				}
			case oracle.DBlockers:
				if a.Kind() == oracle.ActBlock || a.Kind() == oracle.ActDone || a.Kind() == oracle.ActPass {
					buf = append(buf, a)
				}
			default:
				switch a.Kind() {
				case oracle.ActCast, oracle.ActCastFace, oracle.ActActivate, oracle.ActPlayLand:
					buf = append(buf, a)
				}
			}
		}
		if len(buf) == 0 || len(buf) == len(c.D.Options) {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&oracle.PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// miscRun plays the shape's lockstep and returns its divergences and reach.
func miscRun(t *testing.T, sh miscShape) (div, reach int) {
	t.Helper()
	cardsDir, err := cardset.CorpusDir()
	if err != nil {
		t.Skipf("no corpus: %v", err)
	}
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	p, err := loadCardNames(miscCardNames(sh.matchups), cardsDir)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	pd, _, _, err := tableOf(p, "miscshapes-"+fmt.Sprint(len(sh.card)), nil)
	if err != nil {
		t.Fatalf("table: %v", err)
	}
	set := cardset.Raw(cardset.NewDataSet(pd))
	if _, ok := set.Lookup(sh.card); !ok {
		t.Fatalf("%s is not in the table", sh.card)
	}
	for mi, md := range sh.matchups {
		var gd [2][]*cards.Card
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					gd[s] = append(gd[s], c)
				}
			}
			if len(gd[s]) != 40 {
				t.Fatalf("%s matchup %d deck %d: %d cards", sh.card, mi, s, len(gd[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= sh.seeds; seed++ {
			m, err := oracle.NewMatchup(reg, [2]string{"misc-a", "misc-b"}, gd, 1)
			if err != nil {
				t.Fatal(err)
			}
			cfg := oracle.Config{GorgeAutopaySurface: seed%4 == 0}
			k, err := oracle.SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := miscPolicy(seed)
			if seed%8 == 0 {
				pol = oracle.RandomPolicy(seed)
			}
			var last *oracle.GorgeSide
			res := oracle.Lockstep(m, seed, k, pol, oracle.RunConfig{Config: cfg, MaxDecisions: 5000, MaxTurns: 40,
				Check: func(ref *oracle.GorgeSide, _ oracle.Side) *oracle.Divergence { last = ref; return nil }})
			if last != nil {
				reach += sh.reached(last.E.G, last.E.L.Events)
			}
			oracle.CloseSide(k)
			if d := res.Div; d != nil {
				div++
				lo := d.At - 16
				if lo < 0 {
					lo = 0
				}
				t.Errorf("%s matchup %d seed %d: diverged at decision %d: %s %s %v %s (gorge %v / dirac %v) last=%v", sh.card, mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, res.Actions[lo:d.At])
			} else if res.Stall != "" && res.Stall != "turns" {
				t.Errorf("%s matchup %d seed %d: stalled: %s", sh.card, mi, seed, res.Stall)
			}
		}
	}
	return div, reach
}

// miscSpellResolved counts the Resolve events of a spell whose face is the
// named card.
func miscSpellResolved(name string) func(*state.Game, []events.Event) int {
	return func(g *state.Game, evs []events.Event) int {
		n := 0
		for _, ev := range evs {
			if ev.Kind != events.Resolve {
				continue
			}
			if o := g.Obj(ev.Obj); o != nil && o.Face() != nil && o.Face().Name == name {
				n++
			}
		}
		return n
	}
}

// miscAbilityResolved counts the Resolve events of an ability whose source
// is the named card. A spell's stack object has no Ability; an ability's
// does, and its Source outlives the resolution (foldResolve reads the same
// pair), so this names the card's ability rather than its creature spell.
// Noxious Gearhulk's only ability is the enters trigger under test.
func miscAbilityResolved(name string) func(*state.Game, []events.Event) int {
	return func(g *state.Game, evs []events.Event) int {
		n := 0
		for _, ev := range evs {
			if ev.Kind != events.Resolve {
				continue
			}
			o := g.Obj(ev.Obj)
			if o == nil || o.Ability == nil || o.Source == 0 {
				continue
			}
			if src := g.Obj(o.Source); src != nil && src.Face() != nil && src.Face().Name == name {
				n++
			}
		}
		return n
	}
}

// miscCountersMovedFrom counts the counters a move removed from the named
// card (a negative CounterChange on its object), proving Forgotten Ancient's
// any-number move resolved rather than only its trigger being put on the stack.
func miscCountersMovedFrom(name string) func(*state.Game, []events.Event) int {
	return func(g *state.Game, evs []events.Event) int {
		n := 0
		for _, ev := range evs {
			if ev.Kind != events.CounterChange || ev.Amount >= 0 {
				continue
			}
			if o := g.Obj(ev.Obj); o != nil && o.Face() != nil && o.Face().Name == name {
				n += int(-ev.Amount)
			}
		}
		return n
	}
}

// miscLibToBattlefield counts the cards put onto the battlefield from a library
// (Genesis Hydra's dig; nothing else in its pool moves a card that way).
// miscDrawnBySeat0 counts the cards seat 0 drew (Elenda and Azor's attack
// trigger is the only draw in its matchup, so the count proves the paid
// {X}{W}{U}{B} trigger resolved with X > 0).
func miscDrawnBySeat0(_ *state.Game, evs []events.Event) int {
	n := 0
	for _, ev := range evs {
		if ev.Kind == events.Draw && ev.Player == 0 {
			n++
		}
	}
	return n
}

func miscLibToBattlefield(_ *state.Game, evs []events.Event) int {
	n := 0
	for _, ev := range evs {
		if ev.Kind == events.MoveZone && ev.From == state.ZLibrary && ev.To == state.ZBattlefield {
			n++
		}
	}
	return n
}

// miscBrainCounters counts the brain counters a card gets in exile (Rex,
// Cyber-Hound's activated ability's WithCountersType$ BRAIN rider); no other
// card in its pool places that kind.
func miscBrainCounters(_ *state.Game, evs []events.Event) int {
	n := 0
	for _, ev := range evs {
		if ev.Kind == events.CounterChange && ev.Counter == "BRAIN" && ev.Amount > 0 {
			n++
		}
	}
	return n
}

func miscShapes() []miscShape {
	e := func(n string, c int) cardset.Entry { return cardset.Entry{Name: n, Count: c} }
	burn := []cardset.Entry{e("Shock", 6), e("Hill Giant", 8), e("Grizzly Bears", 8), e("Mountain", 10), e("Forest", 8)}
	return []miscShape{
		{card: "Fell the Mighty", seeds: 60, min: 20, reached: miscSpellResolved("Fell the Mighty"),
			matchups: [][2][]cardset.Entry{{
				{e("Fell the Mighty", 4), e("Grizzly Bears", 8), e("Hill Giant", 8), e("Plains", 8), e("Mountain", 6), e("Forest", 6)},
				burn,
			}, {
				{e("Fell the Mighty", 4), e("Grizzly Bears", 8), e("Hill Giant", 8), e("Plains", 8), e("Mountain", 6), e("Forest", 6)},
				{e("Fell the Mighty", 4), e("Raise the Alarm", 4), e("Grizzly Bears", 8), e("Hill Giant", 6), e("Plains", 8), e("Mountain", 5), e("Forest", 5)},
			}}},
		{card: "Noxious Gearhulk", seeds: 60, min: 20, reached: miscAbilityResolved("Noxious Gearhulk"),
			matchups: [][2][]cardset.Entry{{
				{e("Noxious Gearhulk", 4), e("Grizzly Bears", 9), e("Walking Corpse", 8), e("Swamp", 12), e("Forest", 7)},
				burn,
			}, {
				{e("Noxious Gearhulk", 4), e("Grizzly Bears", 9), e("Walking Corpse", 8), e("Swamp", 12), e("Forest", 7)},
				{e("Raise the Alarm", 4), e("Hill Giant", 8), e("Grizzly Bears", 8), e("Plains", 8), e("Mountain", 6), e("Forest", 6)},
			}}},
		{card: "Forgotten Ancient", seeds: 20, min: 2, reached: miscCountersMovedFrom("Forgotten Ancient"),
			matchups: [][2][]cardset.Entry{{
				{e("Forgotten Ancient", 4), e("Shock", 4), e("Grizzly Bears", 8), e("Hill Giant", 8), e("Forest", 8), e("Mountain", 8)},
				{e("Mountain", 40)},
			}}},
		{card: "Plasma Caster", seeds: 60, min: 5, reached: miscAbilityResolved("Plasma Caster"),
			matchups: [][2][]cardset.Entry{{
				{e("Plasma Caster", 4), e("Grizzly Bears", 8), e("Hill Giant", 8), e("Mountain", 12), e("Forest", 8)},
				burn,
			}}},
		{card: "Elenda and Azor", seeds: 60, min: 1, reached: miscDrawnBySeat0,
			matchups: [][2][]cardset.Entry{{
				{e("Elenda and Azor", 4), e("Grizzly Bears", 8), e("Hill Giant", 6), e("Walking Corpse", 4), e("Plains", 6), e("Island", 4), e("Swamp", 8)},
				burn,
			}}},
		{card: "Genesis Hydra", seeds: 60, min: 10, reached: miscLibToBattlefield,
			matchups: [][2][]cardset.Entry{{
				{e("Genesis Hydra", 4), e("Grizzly Bears", 10), e("Hill Giant", 6), e("Forest", 12), e("Mountain", 8)},
				burn,
			}}},
		// Rex's deck has no blockers to connect with, and the opponent's
		// Rootwallas are uncastable on Mountains, so the combat-damage mill
		// puts creature cards in a graveyard for Rex's {E}{E} exile.
		{card: "Rex, Cyber-Hound", seeds: 40, min: 5, reached: miscBrainCounters,
			matchups: [][2][]cardset.Entry{{
				{e("Rex, Cyber-Hound", 4), e("Grizzly Bears", 8), e("Forest", 10), e("Plains", 8), e("Island", 10)},
				{e("Rootwalla", 20), e("Mountain", 20)},
			}}},
	}
}

// TestMiscShapesHostedLockstep plays each lowered misc shape against gorge.
func TestMiscShapesHostedLockstep(t *testing.T) {
	for _, sh := range miscShapes() {
		t.Run(sh.card, func(t *testing.T) {
			t.Parallel()
			div, reach := miscRun(t, sh)
			if div != 0 {
				t.Fatalf("%d games diverged", div)
			}
			if reach < sh.min {
				t.Fatalf("the pool reached the shape %d times, want at least %d: the lockstep proves nothing", reach, sh.min)
			}
			t.Logf("%s: %d resolutions over %d seeds", sh.card, reach, sh.seeds)
		})
	}
}
