package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Behold<1/Type> lockstep: the either-or additional
// cost "behold a <type> or pay {N}" of Kinsbaile Aspirant, Mudbutton
// Cursetosser, Silvergill Mentor, Caustic Exhale and Countersculpt. The
// kernel's ask must come where gorge's beholdCostAsk does: after the
// either-or answer, over the battlefield candidates then the hand ones, a
// lone candidate taken unasked, and the part payable only with a candidate.

var kbeholdDecks = [][2][]cardset.Entry{{
	// A: the four carriers and their tribes
	{
		{Name: "Kinsbaile Aspirant", Count: 4}, {Name: "Kithkin Billyrider", Count: 1}, {Name: "Knight of Meadowgrain", Count: 1},
		{Name: "Mudbutton Cursetosser", Count: 4}, {Name: "Goblin Bully", Count: 1},
		{Name: "Silvergill Mentor", Count: 4}, {Name: "Coral Merfolk", Count: 2}, {Name: "Merfolk of the Pearl Trident", Count: 2},
		{Name: "Caustic Exhale", Count: 3}, {Name: "Dragon Moose", Count: 1}, {Name: "Goblin Cavaliers", Count: 1},
		{Name: "Plains", Count: 4}, {Name: "Swamp", Count: 5}, {Name: "Island", Count: 4}, {Name: "Mountain", Count: 3},
	},
	// B: a midrange shell
	{
		{Name: "Hulking Bugbear", Count: 4}, {Name: "Maritime Guard", Count: 4}, {Name: "Vodalian Soldiers", Count: 4},
		{Name: "Mons's Goblin Raiders", Count: 4}, {Name: "Knight of Meadowgrain", Count: 2}, {Name: "Shock", Count: 4},
		{Name: "Mountain", Count: 6}, {Name: "Island", Count: 5}, {Name: "Plains", Count: 4}, {Name: "Swamp", Count: 3},
	},
}, {
	// the mirror of A: the carriers meet their own tribes on both sides
	{
		{Name: "Kinsbaile Aspirant", Count: 4}, {Name: "Kithkin Billyrider", Count: 1}, {Name: "Knight of Meadowgrain", Count: 1},
		{Name: "Mudbutton Cursetosser", Count: 4}, {Name: "Goblin Bully", Count: 1},
		{Name: "Silvergill Mentor", Count: 4}, {Name: "Coral Merfolk", Count: 2}, {Name: "Merfolk of the Pearl Trident", Count: 2},
		{Name: "Caustic Exhale", Count: 3}, {Name: "Dragon Moose", Count: 1}, {Name: "Goblin Cavaliers", Count: 1},
		{Name: "Plains", Count: 4}, {Name: "Swamp", Count: 5}, {Name: "Island", Count: 4}, {Name: "Mountain", Count: 3},
	},
	{
		{Name: "Kinsbaile Aspirant", Count: 4}, {Name: "Kithkin Billyrider", Count: 1}, {Name: "Knight of Meadowgrain", Count: 1},
		{Name: "Mudbutton Cursetosser", Count: 4}, {Name: "Goblin Bully", Count: 1},
		{Name: "Silvergill Mentor", Count: 4}, {Name: "Coral Merfolk", Count: 2}, {Name: "Merfolk of the Pearl Trident", Count: 2},
		{Name: "Caustic Exhale", Count: 3}, {Name: "Dragon Moose", Count: 1}, {Name: "Goblin Cavaliers", Count: 1},
		{Name: "Plains", Count: 4}, {Name: "Swamp", Count: 5}, {Name: "Island", Count: 4}, {Name: "Mountain", Count: 3},
	},
}}

// kbeholdTribe is the type each carrier beholds.
var kbeholdTribe = map[string]string{
	"Kinsbaile Aspirant": "Kithkin", "Mudbutton Cursetosser": "Goblin", "Silvergill Mentor": "Merfolk",
	"Caustic Exhale": "Dragon",
}

// kbeholdTally counts what the gorge side posed.
type kbeholdTally struct {
	beholdCasts int // either-or answers choosing the behold part with a candidate
	asked       int // ... with two or more candidates (the ask posed)
	unasked     int // ... with exactly one (taken unasked)
	manaCasts   int // either-or answers choosing the {N} part (gorge's pool-only gate leaves it unpayable: the cast is undone)
}

// note tallies one either-or answer (part 0 the behold, 1 the {N}) posed
// by decision d over game g: the behold candidates are the player's
// battlefield permanents and hand cards of the carrier's type, the cast
// card excepted.
func (tl *kbeholdTally) note(g *state.Game, d *decision.Decision, part int) {
	if part != 0 {
		tl.manaCasts++
		return
	}
	ty := kbeholdTribe[g.Obj(d.Source).Face().Name]
	n := 0
	for _, id := range g.Zone(state.ZBattlefield, d.Player) {
		if objectHasType(g.Obj(id), ty) {
			n++
		}
	}
	for _, id := range g.Zone(state.ZHand, d.Player) {
		if id != d.Source && objectHasType(g.Obj(id), ty) {
			n++
		}
	}
	switch {
	case n >= 2:
		tl.beholdCasts++
		tl.asked++
	case n == 1:
		tl.beholdCasts++
		tl.unasked++
	}
}

// kbeholdSpent: player p has no untapped land left, so an undone cast
// leaves nothing for gorge's failed payment to float.
func kbeholdSpent(g *state.Game, p state.PlayerID) bool {
	for _, id := range g.Zone(state.ZBattlefield, p) {
		if o := g.Obj(id); o != nil && !o.Tapped && objectHasType(o, "Land") {
			return false
		}
	}
	return true
}

// kbeholdPool is the mana floating in either player's pool.
func kbeholdPool(g *state.Game) int {
	n := 0
	for _, pl := range g.Players {
		for _, v := range pl.Pool {
			n += int(v)
		}
	}
	return n
}

// kbeholdPolicy casts whenever it can (so the games reach the carriers),
// else random. A posed either-or ask (the games without
// GorgeAutopaySurface) takes the first, payable, part: gorge's cast
// planner floats the spell's own mana only, so the {N} part is unpayable
// there and choosing it undoes the cast, which gorge does only after tapping
// the remaining sources (a floating-pool difference that is no behold
// matter); under GorgeAutopaySurface gorge answers the ask itself, the {N}
// part included, and the tally reads it through OnInternal.
func kbeholdPolicy(seed uint64, tl *kbeholdTally) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	drain := false // an undone {N} cast left its lowered mana floating: pass until it empties
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		if drain && c.G != nil {
			if kbeholdPool(c.G.E.G) == 0 {
				drain = false
			} else {
				for _, x := range c.D.Options {
					if x.Kind() == ActPass {
						return x
					}
				}
			}
		}
		hasCast := false
		for _, a := range c.D.Options {
			if a.Kind() == ActCast {
				hasCast = true
			}
		}
		// seed%4 == 2 floats mana first (a mana ability, then the cast),
		// so the {N} part is payable from the pool; 1 and 3 cast at once
		for _, a := range c.D.Options {
			if a.Kind() == ActCast && seed%4 != 2 || a.Kind() == ActActivate && seed%4 == 2 && hasCast {
				buf = append(buf, a)
			}
		}
		var a Canon
		if len(buf) == 0 || seed%4 == 0 {
			a = rnd.Pick(c)
		} else {
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		if c.G == nil || c.G.fr == nil || c.G.fr.d == nil {
			return a
		}
		if o, ok := c.G.GorgeOption(a); ok && o.Kind == "altaddcost" {
			g, d := c.G.E.G, c.G.fr.d
			want := 0
			if kbeholdSpent(g, d.Player) {
				want = 1
			}
			for _, x := range c.D.Options {
				if ox, ok := c.G.GorgeOption(x); ok && ox.Amount == want {
					a = x
				}
			}
			if o, ok = c.G.GorgeOption(a); ok {
				tl.note(g, d, o.Amount)
				drain = drain || o.Amount != 0
			}
		}
		return a
	})
}

// runBehold plays the matchups for seeds 1..seeds and returns the games,
// the divergences and the tally. mutate offers the hand candidates before
// the battlefield ones (the mutation must diverge).
func runBehold(t *testing.T, seeds int, mutate bool) (games, divs int, tl kbeholdTally) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("behold")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugBeholdHandFirst
	dirac.DebugBeholdHandFirst = mutate
	defer func() { dirac.DebugBeholdHandFirst = old }()
	for mi, md := range kbeholdDecks {
		var decks [2][]*cards.Card
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[s] = append(decks[s], c)
				}
			}
			if len(decks[s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kbehold-a", "kbehold-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			g, err := NewGorgeSide(m, seed, cfg, nil)
			if err != nil {
				t.Fatal(err)
			}
			g.OnInternal = func(d *decision.Decision, idx int) {
				if d != nil && idx >= 0 && idx < len(d.Options) && d.Options[idx].Kind == "altaddcost" {
					tl.note(g.E.G, d, d.Options[idx].Amount)
				}
			}
			res := LockstepLed(m, seed, g, k, kbeholdPolicy(seed, &tl), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			games++
			if d := res.Div; d != nil {
				divs++
				if !mutate {
					t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
			CloseSide(k)
			if strings.HasPrefix(res.Stall, KernelStallPrefix) && !mutate {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
		}
	}
	return games, divs, tl
}

// NOTE: manaCasts counts {N}-part picks, which are all undone. The planned
// offer now lists a behold-or-mana cast from untapped lands whenever the
// behold part is payable (planCastAltAdd, pair 73), so the policy casts
// before it taps out and this harness no longer takes the {N} part: the
// count is 0, not a coverage loss. The completed {N}-part payment (and the
// altAdd != 0 skip in beholdOptions) is covered by the kernel's
// TestBeholdManaPartCompletes; the undone part's CR 601.2g window (sources
// tapped, mana floating) by TestBeholdUnpayableManaPart. The
// kbeholdSpent/drain steering stays for the boards that still reach it (a
// cast from floating mana) and costs nothing.
func TestBeholdLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, tl := runBehold(t, 256, false)
	t.Logf("%d games, %d divergences, behold-part casts %d (asked %d, unasked %d), {N}-part picks (undone) %d",
		games, divs, tl.beholdCasts, tl.asked, tl.unasked, tl.manaCasts)
	if divs != 0 {
		t.Fatalf("%d divergences", divs)
	}
	if tl.beholdCasts < 50 || tl.asked < 20 || tl.unasked < 10 {
		t.Errorf("thin coverage: behold %d (want 50), asked %d (want 20), unasked %d (want 10)",
			tl.beholdCasts, tl.asked, tl.unasked)
	}
}

// TestBeholdMutation offers the hand candidates first and the battlefield
// ones only when the hand has none: gorge offers both zones, so a posed
// behold ask over both must diverge. (A bare reordering would not: the
// canonical options are sorted.) A green mutation means no game ever posed
// such an ask.
func TestBeholdMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _ := runBehold(t, 64, true)
	if divs == 0 {
		t.Fatalf("mutation dropped battlefield behold candidates in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
