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

// Day of Black Sun (PumpAll RememberPumped$), Break the Spell and
// Gloomwidow's Feast (Destroy RememberLKI$) are lowered mass remember
// writers whose set a later link of the same resolution reads.
// TestStdSetReprosLockstep replays one recorded game of each;
// rememberMassHits counts the kind-events of card's first resolution: from its
// Resolve to the spell's own move off the stack.
func rememberMassHits(evs []events.Event, g *state.Game, card string, kind events.Kind) (hits int) {
	for i := range evs {
		e := &evs[i]
		if e.Kind != events.Resolve {
			continue
		}
		if o := g.Obj(e.Obj); o == nil || o.Face() == nil || o.Face().Name != card {
			continue
		}
		for j := i + 1; j < len(evs); j++ {
			if evs[j].Kind == events.MoveZone && evs[j].Obj == e.Obj {
				break
			}
			if evs[j].Kind == kind {
				hits++
			}
		}
		return hits
	}
	return 0
}

func rememberMassPlay(t *testing.T, reg *cards.Registry, c rememberMassCase, r *Repro, set cardset.Set, pol Policy) (Result, int) {
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
		rc.MaxDecisions = 3000 // recording
	}
	if pol == nil {
		pol = &ScriptPolicy{Actions: r.Actions}
	}
	hits := 0
	rc.Finish = func(g *GorgeSide) {
		hits = rememberMassHits(g.E.L.Events, g.E.G, c.cardName, c.kind)
	}
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	return Lockstep(m, r.Seed, k, pol, rc), hits
}

// TestRememberWritersMassReprosResolve requires the recorded game to resolve
// the writer and the effect that reads the set.
//
// The games are recorded by TestRememberWritersMassRecord
// (REMEMBER_MASS_RECORD=1), on the scripted policy of charmRepeatPolicy.

// rememberMassCase is a recorded game's case; full: recorded on the full
// surface (gorge's own payment asks), since the autopay planner prices no {X}
// cast of Day of Black Sun and never offers it.
type rememberMassCase struct {
	rememberChoiceCase
	full bool
}

func rememberMassCases() []rememberMassCase {
	return []rememberMassCase{
		{rememberChoiceCase: rememberChoiceCase{
			// the opponent's mana value 0 creatures are pumped, so remembered:
			// they lose their abilities and are destroyed (two moves at least)
			charmRepeatCase: charmRepeatCase{
				name: "pump_all_remembers_the_pumped_day_of_black_sun", card: "Day of Black Sun", prefer: []string{"Memnite", "Ornithopter"},
				decks: [2][]string{
					append(deckOf(6, "Day of Black Sun"), deckOf(18, "Swamp")...),
					append(deckOf(8, "Memnite"), append(deckOf(8, "Ornithopter"), deckOf(8, "Plains")...)...),
				},
				kind: events.MoveZone, n: 2,
			},
			cardName: "Day of Black Sun",
		}, full: true},
		{rememberChoiceCase: rememberChoiceCase{
			// the caster's own enchantment is the only target: it is destroyed,
			// so remembered, and was a permanent its caster controlled: a draw
			charmRepeatCase: charmRepeatCase{
				name: "destroy_remembers_lki_break_the_spell", card: "Break the Spell",
				decks: [2][]string{
					append(deckOf(6, "Break the Spell"), append(deckOf(6, "Glorious Anthem"), deckOf(12, "Plains")...)...),
					deckOf(24, "Forest"),
				},
				kind: events.Draw, n: 1,
			},
			cardName: "Break the Spell",
		}},
		{rememberChoiceCase: rememberChoiceCase{
			// a blue flyer destroyed, so remembered: the Spider token is made
			charmRepeatCase: charmRepeatCase{
				name: "destroy_remembers_lki_gloomwidows_feast", card: "Gloomwidow's Feast", prefer: []string{"Wind Drake"},
				decks: [2][]string{
					append(deckOf(6, "Gloomwidow's Feast"), deckOf(18, "Forest")...),
					append(deckOf(8, "Wind Drake"), deckOf(16, "Island")...),
				},
				kind: events.TokenCreate, n: 1,
			},
			cardName: "Gloomwidow's Feast",
		}},
	}
}

// TestRememberWritersMassReprosResolve replays the recorded games: no
// divergence, every scripted action offered, the writer resolved and the
// effect reading its set happened.
func TestRememberWritersMassReprosResolve(t *testing.T) {
	reg := synthReg(t)
	for _, c := range rememberMassCases() {
		t.Run(c.cardName, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			script := &ScriptPolicy{Actions: r.Actions}
			res, hits := rememberMassPlay(t, reg, c, r, charmRepeatSet(t, r.Cards), script)
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

// TestRememberWritersMassRecord plays the cases' scripted policies over seeds
// until one resolves the card with its effect, and writes the repro. It runs
// only with REMEMBER_MASS_RECORD=1; the games it writes are the committed
// repros.
func TestRememberWritersMassRecord(t *testing.T) {
	if os.Getenv("REMEMBER_MASS_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips, since a skip here must name a missing pool (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	for _, c := range rememberMassCases() {
		t.Run(c.cardName, func(t *testing.T) {
			set := charmRepeatSet(t, c.decks)
			for seed := uint64(1); seed <= 400; seed++ {
				surface := "autopay"
				if c.full {
					surface = "full"
				}
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: surface, Kernel: "custom"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, hits := rememberMassPlay(t, reg, c, r, set, charmRepeatPolicy(m, c.charmRepeatCase))
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
