package oracle

import (
	"math/rand/v2"
	"os"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestMorphPickLowers plays the morph pick turn-up costs kgen lowers, in
// lockstep with gorge over the morphpick fixture pool
// (dirac/cardset/pools/fixtures/morphpick):
//
//   - a turn-up cost of one Discard part: Gathan Raiders' Morph—Discard a
//     card, Putrid Raptor's Discard<1/Zombie> (CardDef.MorphPick,
//     dirac/face_morphpick.go). The kernel's window poses gorge
//     morph_turnup.go's turnUpDiscardAsk before the flip.
//   - a turn-up cost of one Return part: Fathom Seer's return two Islands,
//     Raven Guild Initiate's Return<1/Bird>.
//   - a turn-up cost of one Sac part: Skirk Volcanist's sacrifice two
//     Mountains, Gift of Doom's sacrifice another creature -- beside Gift of
//     Doom's ReplTurnUpAttach replacement (face_turnupattach.go): the attach
//     election parks the turn-up still face down, as gorge
//     applyTurnFaceUpReplacements parks the event for its body.
//
// The policy takes a watched card's special action, face-down cast or cast
// seven times in eight. Precondition: the watched cards' turn-ups happen
// (the mutation subtests diverge only if they do). Each mutation drops one
// window from the table and must diverge.
func TestMorphPickLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	t.Run("lockstep", func(t *testing.T) {
		run, n := runMorphPick(t, 120, true, nil)
		t.Logf("%d games, %d divergences, %d stalls, %d watched offers taken", run.games, run.divs, run.stalls, n.taken)
		if n.taken < 8 {
			t.Errorf("watched offers taken %d times, want at least 8", n.taken)
		}
		// precondition: every pick carrier's turn-up (its pick window) was played
		for _, name := range []string{"Gathan Raiders", "Putrid Raptor", "Fathom Seer", "Raven Guild Initiate", "Skirk Volcanist", "Gift of Doom"} {
			if run.casts[name+"#turn"] == 0 {
				t.Errorf("%s was never turned up: its pick window went unplayed", name)
			}
		}
	})
	t.Run("MorphPick/mutation", func(t *testing.T) {
		run, n := runMorphPick(t, 24, false, func(t *testing.T, tbl *kop.Table) {
			cd := morphPickCard(t, tbl, "Gathan Raiders")
			if cd.MorphPick.Flags != kop.CostDiscard || cd.MorphPick.N != 1 {
				t.Fatalf("Gathan Raiders: MorphPick %+v, want a discard of one", cd.MorphPick)
			}
			cd.MorphPick = kop.Cost{}
		})
		t.Logf("turn-ups: %d", n.turns)
		run.requireDivergence(t, "a Morph—Discard a card turned up for free")
	})
	t.Run("TurnUpAttach/mutation", func(t *testing.T) {
		run, n := runMorphPick(t, 24, false, func(t *testing.T, tbl *kop.Table) {
			cd := morphPickCard(t, tbl, "Gift of Doom")
			n := 0
			for i := range cd.Abilities {
				if cd.Abilities[i].Kind == kop.AbReplacement && cd.Abilities[i].Repl.Kind == kop.ReplTurnUpAttach {
					cd.Abilities[i].Repl.Kind = kop.ReplTurnUpProgram // an empty program: no ask, the fold proceeds
					n++
				}
			}
			if n != 1 {
				t.Fatalf("Gift of Doom: %d ReplTurnUpAttach replacements, want 1", n)
			}
		})
		t.Logf("turn-ups: %d", n.turns)
		run.requireDivergence(t, "a turn-up attach elected for free")
	})
}

// morphPickCard is the table's card named name.
func morphPickCard(t *testing.T, tbl *kop.Table, name string) *kop.CardDef {
	t.Helper()
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == name {
			return &tbl.Cards[i]
		}
	}
	t.Fatalf("%s not in the table", name)
	return nil
}

// morphPickDecks are the two matchups' decks: seat A turns up its morphs by
// discarding and by sacrificing Mountains (the Zombies pay Putrid Raptor's
// Discard<1/Zombie>), seat B by returning two Islands and by sacrificing
// another creature -- the Vexing Gulls, whose birds also pay Raven Guild
// Initiate's Return<1/Bird> and carry Gift of Doom's attach. Both seats keep
// Swamps (Priest of Gix costs {1}{B}) and Mountains/Islands (the pick
// candidates live on the battlefield): a cast Priest of Gix floats the
// {B}{B}{B} a face-down cast prices from (the shape the mbccosts9 pool
// plays; gorge prices that cast from the floating pool).
var morphPickDecks = []fixtureDecks{
	{
		{{Name: "Gathan Raiders", Count: 6}, {Name: "Skirk Volcanist", Count: 5}, {Name: "Putrid Raptor", Count: 5}, {Name: "Zombie Cutthroat", Count: 5}, {Name: "Priest of Gix", Count: 8}, {Name: "Swamp", Count: 5}, {Name: "Mountain", Count: 6}},
		{{Name: "Fathom Seer", Count: 8}, {Name: "Raven Guild Initiate", Count: 6}, {Name: "Gift of Doom", Count: 6}, {Name: "Vexing Gull", Count: 6}, {Name: "Priest of Gix", Count: 8}, {Name: "Swamp", Count: 2}, {Name: "Island", Count: 4}},
	},
}

// morphPickWatch is the watched cards; the policy favours their offers.
var morphPickWatch = map[string]bool{
	"Gathan Raiders": true, "Skirk Volcanist": true, "Putrid Raptor": true, "Zombie Cutthroat": true,
	"Fathom Seer": true, "Raven Guild Initiate": true, "Gift of Doom": true, "Vexing Gull": true,
	"Priest of Gix": true,
}

// morphPickCounts is what the games took: a watched card's special action
// (its turn-up), face-down cast or cast; turns counts the watched turn-ups
// among them.
type morphPickCounts struct{ taken, turns int }

// morphPickPolicy plays randomly but, seven times in eight, takes the first
// offered special action, face-down cast or cast of a watched card, in that
// order of preference: the turn-up is the special action (CR 116.2b). A
// face-down cast prices from the floating pool alone, so a cast of the pool's
// floater (Priest of Gix) comes before the other watched casts: without it
// the floats never exist and no face-down cast is ever offered.
func morphPickPolicy(seed uint64, casts map[string]int) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x6d706963)) // "mpic"
	take := func(c *PickCtx, k CanonKind, only string) Canon {
		for _, a := range c.D.Options {
			if a.Kind() != k {
				continue
			}
			n := refCardName(c.G, a.Src())
			if n == "" || !morphPickWatch[n] || (only != "" && n != only) {
				continue
			}
			casts[n]++
			switch k {
			case ActSpecial:
				casts[n+"#turn"]++
			case ActCastFace:
				casts[n+"#face"]++
			case ActCast:
				casts[n+"#cast"]++
			}
			return a
		}
		return 0
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil || c.D.Kind != DPriority {
			return rnd.Pick(c)
		}
		if prng.IntN(8) != 0 {
			for _, w := range [...]struct {
				k    CanonKind
				only string
			}{{ActSpecial, ""}, {ActCastFace, ""}, {ActCast, "Priest of Gix"}, {ActCast, ""}} {
				if a := take(c, w.k, w.only); a != 0 {
					return a
				}
			}
		}
		return rnd.Pick(c)
	})
}

// runMorphPick plays seeds 1..seeds of the morphpick matchup, alternating the
// option surface by seed. mutate, when set, rewrites the table first.
func runMorphPick(t *testing.T, seeds int, report bool, mutate func(*testing.T, *kop.Table)) (fixtureRun, morphPickCounts) {
	t.Helper()
	n := morphPickCounts{}
	var mut func(*kop.Table)
	if mutate != nil {
		mut = func(tbl *kop.Table) { mutate(t, tbl) }
	}
	run := runFixtureLockstepPolicy(t, "morphpick", morphPickDecks, seeds, report, mut, func(seed uint64, casts map[string]int) Policy {
		base := morphPickPolicy(seed, casts)
		if os.Getenv("MORPHPICK_TRACE") != "" {
			return PolicyFunc(func(c *PickCtx) Canon {
				a := base.Pick(c)
				t.Logf("s%d d%d %v p%d opts=%v pick=%v", seed, c.Seq, c.D.Kind, c.D.Player, actionStrings(c.D.Options), actionStrings([]Canon{a}))
				return a
			})
		}
		return base
	})
	for name, m := range run.casts {
		n.taken += m
		if strings.HasSuffix(name, "#turn") {
			n.turns += m
		}
	}
	if report {
		t.Logf("taken by kind: %v", run.casts)
	}
	return run, n
}
