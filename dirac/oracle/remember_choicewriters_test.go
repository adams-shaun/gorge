package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Season of Gathering and Kayla's Command (PutCounter Choices$ with
// RememberCards$) and Prishe's Wanderings (ChangeZone RememberSearcher$) are lowered remembered-set
// writers. TestStdSetReprosLockstep replays one recorded game of each;
// TestRememberChoiceWritersReprosResolve requires the recorded game to resolve
// the writer and the effect that reads the set, since a lockstep entry alone
// would pass a script that never cast the card.
//
// The games are recorded by TestRememberChoiceWritersRecord
// (REMEMBER_CHOICE_RECORD=1, the kernel table built from the decks), on the
// scripted policy of charmRepeatPolicy.

type rememberChoiceCase struct {
	charmRepeatCase
	// cardName is the writer's card; the effect that reads its set is counted
	// as the events of the embedded case's kind during its resolution, at
	// least n of them. later is the number of further stack resolutions the
	// window spans: the reflexive trigger a spell puts on the stack resolves
	// after the spell does.
	cardName string
	later    int
}

func rememberChoiceCases() []rememberChoiceCase {
	return []rememberChoiceCase{
		{
			// three one-pawprint counter picks and the two-pawprint destroy mode:
			// each counter's recipient is remembered, takes vigilance and trample,
			// and the set is cleared before the next instance
			charmRepeatCase: charmRepeatCase{
				name: "put_counter_choice_remembers_the_recipient_season_of_gathering", card: "Season of Gathering",
				decks: [2][]string{
					append(deckOf(6, "Season of Gathering"), append(deckOf(4, "Grizzly Bears"), deckOf(14, "Forest")...)...),
					append(deckOf(8, "Glorious Anthem"), deckOf(16, "Plains")...),
				},
				prefer: []string{"Grizzly Bears"}, picks: []int{0, 0, 0, 1}, kind: events.CounterChange, n: 3,
			},
			cardName: "Season of Gathering",
		},
		{
			// the counter mode beside the token mode: the pick is remembered and
			// gains double strike, then the set is cleared
			charmRepeatCase: charmRepeatCase{
				name: "put_counter_choice_remembers_the_recipient_kaylas_command", card: "Kayla's Command",
				decks: [2][]string{
					append(deckOf(6, "Kayla's Command"), append(deckOf(4, "Savannah Lions"), deckOf(14, "Plains")...)...),
					deckOf(24, "Plains"),
				},
				prefer: []string{"Savannah Lions"}, picks: []int{1, 0}, kind: events.CounterChange, n: 1,
			},
			cardName: "Kayla's Command",
		},
		{
			// the search by the caster is remembered whatever it finds: the
			// +1/+1 counter trigger it gates is put on the creature
			charmRepeatCase: charmRepeatCase{
				name: "search_remembers_the_searcher_prishes_wanderings", card: "Grizzly Bears", prefer: []string{"Prishe's Wanderings"},
				decks: [2][]string{
					append(deckOf(5, "Prishe's Wanderings"), append(deckOf(4, "Grizzly Bears"), deckOf(15, "Forest")...)...),
					deckOf(24, "Plains"),
				},
				kind: events.CounterChange, n: 1,
			},
			cardName: "Prishe's Wanderings", later: 1,
		},
	}
}

// rememberChoiceHits counts the kind-events between card's Resolve and the
// later-th Resolve after it (the next one for 0 is the following resolution's).
func rememberChoiceHits(evs []events.Event, g *state.Game, card string, kind events.Kind, later int) (hits int) {
	for i := range evs {
		e := &evs[i]
		if e.Kind != events.Resolve {
			continue
		}
		if o := g.Obj(e.Obj); o == nil || o.Face() == nil || o.Face().Name != card {
			continue
		}
		left := later
		for j := i + 1; j < len(evs); j++ {
			if evs[j].Kind == events.Resolve {
				if left == 0 {
					break
				}
				left--
			}
			if evs[j].Kind == kind {
				hits++
			}
		}
		return hits
	}
	return 0
}

func rememberChoicePlay(t *testing.T, reg *cards.Registry, c rememberChoiceCase, r *Repro, set cardset.Set, pol Policy) (Result, int) {
	t.Helper()
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
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions) + 200
	if len(r.Actions) == 0 {
		rc.MaxDecisions = 3000 // recording: the card is cast in the game's middle turns
	}
	if pol == nil {
		pol = &ScriptPolicy{Actions: r.Actions}
	}
	hits := 0
	rc.Finish = func(g *GorgeSide) {
		hits = rememberChoiceHits(g.E.L.Events, g.E.G, c.cardName, c.kind, c.later)
	}
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	return Lockstep(m, r.Seed, k, pol, rc), hits
}

// TestRememberChoiceWritersReprosResolve replays the recorded games: no
// divergence, every scripted action offered, the writer's card resolved and
// the effect reading its set happened.
func TestRememberChoiceWritersReprosResolve(t *testing.T) {
	reg := synthReg(t)
	for _, c := range rememberChoiceCases() {
		t.Run(c.cardName, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			script := &ScriptPolicy{Actions: r.Actions}
			res, hits := rememberChoicePlay(t, reg, c, r, charmRepeatSet(t, r.Cards), script)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			if hits < c.n {
				t.Fatalf("%d %v events from the %s resolution, want at least %d", hits, c.kind, c.cardName, c.n)
			}
		})
	}
}

// TestRememberChoiceWritersRecord plays the cases' scripted policies over
// seeds until one resolves the card with its effect, and writes the repro. It
// runs only with REMEMBER_CHOICE_RECORD=1; the games it writes are the
// committed repros.
func TestRememberChoiceWritersRecord(t *testing.T) {
	if os.Getenv("REMEMBER_CHOICE_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips, since a skip here must name a missing pool (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	for _, c := range rememberChoiceCases() {
		t.Run(c.cardName, func(t *testing.T) {
			set := charmRepeatSet(t, c.decks)
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: "autopay", Kernel: "custom"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, hits := rememberChoicePlay(t, reg, c, r, set, charmRepeatPolicy(m, c.charmRepeatCase))
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
				}
				if hits < c.n {
					continue
				}
				r.setActions(res.Actions)
				if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
					t.Fatal(err)
				}
				t.Logf("seed %d: %d decisions, %d events", seed, len(res.Actions), hits)
				return
			}
			t.Fatalf("no seed resolved %s with its effect", c.cardName)
		})
	}
}
