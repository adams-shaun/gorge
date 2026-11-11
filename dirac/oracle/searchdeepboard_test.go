package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/search"
	"github.com/adams-shaun/gorge/dirac/search/hbot"
)

// TestSearchDeepBoardStallsNotPanics: the search rollout's heuristic bot
// (dirac/search/hbot) answers a target decision that offers a stack
// ability (Ertai Resurrected's counter mode: StackTarget0 + its stack
// index, 0xEC..0xFB) without indexing its per-object tables with the byte.
// It did: optPlayer read s.objs[224], "index out of range [224] with
// length 192" (MaxObj), the TRK kplay search games' ~1% panic, on a deck
// whose Ertai-like counter-an-ability card met an ability on the stack.
// (The name is the ticket's; the first hypothesis, a mint site past MaxObj,
// was wrong: the object table was not full.)
//
// Random play drives the Ertai matchup of the stacktgt lockstep and asks
// the bot every target decision that lists a stack-ability option; every
// answer is one of the options, and the decisions exist (the precondition).
func TestSearchDeepBoardStallsNotPanics(t *testing.T) {
	t.Parallel()

	corpusDirOrSkip(t)
	set, err := cardset.RawByName("stacktgt")
	if err != nil {
		t.Fatal(err)
	}
	tbl, d0, d1, err := set.Matchup(kstacktgtErtaiDecks[0], kstacktgtErtaiDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	bt := hbot.New(hbot.NewFacts(tbl))
	asked := 0
	for seed := uint64(1); seed <= 3000; seed++ {
		s := dirac.NewConfig(tbl, d0, d1, seed, dirac.Config{})
		pol := dirac.NewRandomPolicy(seed*7919 + 1)
		var d dirac.Decision
		for n := 0; n < 4000; n++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			if d.Kind == dirac.DkTarget {
				has := false
				for i := 0; i < int(d.N); i++ {
					if _, ok := dirac.StackTargetIndex(d.Options[i].Target()); ok && d.Options[i].Kind() == dirac.AkTarget {
						has = true
					}
				}
				if has {
					for i := 0; i < int(d.N); i++ {
						// the identity accessors name the stack entry, not s.objs[224]
						if o := dirac.Obj(d.Options[i].Target()); d.Options[i].Kind() == dirac.AkTarget && !dirac.IsObjTarget(d.Options[i].Target()) && d.Options[i].Target() != dirac.NoTarget && d.Options[i].Target() < dirac.PlayerTarget0 {
							if s.ObjZone(o) != kop.ZoneStack || s.ObjCard(o) == 0 || s.Controller(o) > 1 || s.ObjOwner(o) > 1 || s.ObjToken(o) {
								t.Fatalf("seed %d: stack-ability target %#x reads zone %v card %d", seed, o, s.ObjZone(o), s.ObjCard(o))
							}
						}
					}
					asked++
					if i := bt.Pick(s, &d); i < 0 || i >= int(d.N) {
						t.Fatalf("seed %d decision %d: pick %d of %d options", seed, n, i, d.N)
					}
				}
			}
			a := pol.Pick(&d)
			if d.Kind == dirac.DkPriority && s.StackLen() > 0 {
				// hold Ertai for a response: it is the card that targets an ability
				for i := 0; i < int(d.N); i++ {
					o := d.Options[i]
					if o.Kind() == dirac.AkCast && tbl.Cards[s.ObjCard(dirac.Obj(o.Source()))].Name == "Ertai Resurrected" {
						a = o
					}
				}
			}
			s.Apply(a)
		}
	}
	if asked == 0 {
		t.Fatal("no target decision listed a stack ability: the test never reached the bot's stack-target read")
	}
	t.Logf("%d target decisions with a stack-ability option answered by the bot", asked)
}

// TestSearchHeuristicStackTargetNotPanics: search.Heuristic, the searcher's
// default bot, answers a target decision that lists a stack ability
// (StackTarget0 + its stack index) without PowerToughness reading
// s.objs[236] ("index out of range [236] with length 236"); the
// accessors answer a stack-ability byte as no creature, no P/T, public.
func TestSearchHeuristicStackTargetNotPanics(t *testing.T) {
	t.Parallel()

	corpusDirOrSkip(t)
	set, err := cardset.RawByName("stacktgt")
	if err != nil {
		t.Fatal(err)
	}
	tbl, d0, d1, err := set.Matchup(kstacktgtErtaiDecks[0], kstacktgtErtaiDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	asked := 0
	checked := false
	for seed := uint64(1); seed <= 3000; seed++ {
		s := dirac.NewConfig(tbl, d0, d1, seed, dirac.Config{})
		pol := dirac.NewRandomPolicy(seed*7919 + 1)
		var d dirac.Decision
		for n := 0; n < 4000; n++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			if d.Kind == dirac.DkTarget {
				var first dirac.Obj
				has := false
				for i := 0; i < int(d.N); i++ {
					if _, ok := dirac.StackTargetIndex(d.Options[i].Target()); ok && d.Options[i].Kind() == dirac.AkTarget {
						if !has {
							first = dirac.Obj(d.Options[i].Target())
						}
						has = true
					}
				}
				if has {
					asked++
					if i := (search.Heuristic{}).Pick(s, &d); i < 0 || i >= int(d.N) {
						t.Fatalf("seed %d decision %d: pick %d of %d options", seed, n, i, d.N)
					}
					if !checked {
						checked = true
						if pw, th := s.PowerToughness(first); pw != 0 || th != 0 {
							t.Fatalf("PowerToughness(%#x) = %d/%d, want 0/0", first, pw, th)
						}
						if s.IsCreature(first) || s.Hidden(first, 0) || s.Hidden(first, 1) {
							t.Fatalf("stack-ability byte %#x reads as a creature or hidden", first)
						}
					}
				}
			}
			a := pol.Pick(&d)
			if d.Kind == dirac.DkPriority && s.StackLen() > 0 {
				for i := 0; i < int(d.N); i++ {
					o := d.Options[i]
					if o.Kind() == dirac.AkCast && tbl.Cards[s.ObjCard(dirac.Obj(o.Source()))].Name == "Ertai Resurrected" {
						a = o
					}
				}
			}
			s.Apply(a)
		}
	}
	if asked == 0 {
		t.Fatal("no target decision listed a stack ability: the test never reached the heuristic's stack-target read")
	}
	t.Logf("%d target decisions with a stack-ability option answered by search.Heuristic", asked)
}
