package oracle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMBCFilters11Lowers plays one lowered card of each spelling the MBC
// filters-11 family lowers, in lockstep against gorge on a table built for
// exactly its decks (kgen refused both before):
//
//   - EnchantedPlayerCtrl (Trespasser's Curse's ChangesZone ValidCard$
//     Creature.EnchantedPlayerCtrl, a Curse on a player): the opponent's
//     Grizzly Bears enter under the enchanted player's control and each drains
//     one life. Nothing else in those decks changes life by exactly one, so a
//     LifeChange of -1 is the trigger. Gorge did not know the word before this
//     change (Forge's CardProperty "EnchantedPlayer" + Ctrl): the Curse
//     matched nothing there.
//   - Card.!wasCastFromTheirHand as a Counter slot's ValidTgts$ (Wash Away's
//     front half, inline: the card's cleave half stays refused for its cost):
//     the counter is offered a spell only when it was not cast from a hand, so
//     only Think Twice's flashback is ever countered; a spell cast from a hand
//     is never a target.
//
// prop:kicked N stays refused (an and/or Kicker has no kernel model) and
// ControlledBy ParentTarget(edController) stays refused (gorge reads it as an
// unknown predicate and offers the second target nothing); see the report.
func TestMBCFilters11Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	bears := []cardset.Entry{{Name: "Grizzly Bears", Count: 16}, {Name: "Forest", Count: 24}}
	t.Run("EnchantedPlayerCtrl/Trespasser's_Curse", func(t *testing.T) {
		decks := [2][]cardset.Entry{{{Name: "Trespasser's Curse", Count: 14}, {Name: "Swamp", Count: 26}}, bears}
		reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
		if err != nil {
			t.Fatal(err)
		}
		cs, names := filters11Decks(t, reg, decks)
		set := printedDelveSet(t, names)
		reached := filters11Run(t, reg, set, decks, cs, func(ref *GorgeSide) bool {
			for _, e := range ref.E.L.Events {
				if e.Kind == events.LifeChange && e.Amount == -1 && e.Player == 1 {
					return true
				}
			}
			return false
		})
		t.Logf("the Curse drained in %d of 16 games", reached)
		if reached < 3 {
			t.Errorf("the Curse drained in %d of 16 games, want at least 3", reached)
		}
	})
	t.Run("wasCastFromTheirHand/Counter_target", func(t *testing.T) {
		dir := filters11Corpus(t, map[string]string{
			"Counter Probe": "Name:Counter Probe\nManaCost:U\nTypes:Instant\n" +
				"A:SP$ Counter | TargetType$ Spell | ValidTgts$ Card.!wasCastFromTheirHand | TgtPrompt$ Select target spell that wasn't cast from its owner's hand | SpellDescription$ Counter target spell that wasn't cast from its owner's hand.\n" +
				"Oracle:Counter target spell that wasn't cast from its owner's hand.\n",
		}, "Think Twice", "Island")
		reg, err := cards.SharedCorpus(dir)
		if err != nil {
			t.Fatal(err)
		}
		decks := [2][]cardset.Entry{
			{{Name: "Counter Probe", Count: 16}, {Name: "Island", Count: 24}},
			{{Name: "Think Twice", Count: 16}, {Name: "Island", Count: 24}},
		}
		cs, names := filters11Decks(t, reg, decks)
		set := printedDelveSetIn(t, dir, names)
		var fromHand, fromGrave int
		reached := filters11Run(t, reg, set, decks, cs, func(ref *GorgeSide) bool {
			ok := false
			for i, e := range ref.E.L.Events {
				if e.Kind != events.TargetsChosen || len(e.IDs) == 0 {
					continue
				}
				if o := ref.E.G.Obj(e.Obj); o == nil || o.Face() == nil || o.Face().Name != "Counter Probe" {
					continue
				}
				for j := i - 1; j >= 0; j-- {
					if p := ref.E.L.Events[j]; p.Kind == events.PutOnStack && p.Obj == e.IDs[0] {
						if p.From == state.ZHand {
							fromHand++
						} else {
							fromGrave++
							ok = true
						}
						break
					}
				}
			}
			return ok
		})
		t.Logf("a flashback spell was countered in %d of 16 games (targets cast from a hand %d, from elsewhere %d)", reached, fromHand, fromGrave)
		if fromHand != 0 {
			t.Errorf("Counter Probe targeted %d spells cast from a hand", fromHand)
		}
		if reached < 3 {
			t.Errorf("the Counter targeted a flashback spell in %d of 16 games, want at least 3", reached)
		}
	})
}

// filters11Decks resolves each deck's entries against reg.
func filters11Decks(t *testing.T, reg *cards.Registry, decks [2][]cardset.Entry) (cs [2][]*cards.Card, names [2][]string) {
	t.Helper()
	for s, d := range decks {
		for _, e := range d {
			names[s] = append(names[s], e.Name)
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for range e.Count {
				cs[s] = append(cs[s], c)
			}
		}
	}
	return cs, names
}

// filters11Run plays 16 lockstep games of decks on set and returns how many
// reached(ref) held at the end. A divergence or an unmodelled stall fails.
func filters11Run(t *testing.T, reg *cards.Registry, set cardset.Set, decks [2][]cardset.Entry, cs [2][]*cards.Card, reached func(ref *GorgeSide) bool) int {
	t.Helper()
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for seed := uint64(1); seed <= 16; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"filters11-a", "filters11-b"}, cs, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		hit := false
		rc := RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40}
		rc.Finish = func(ref *GorgeSide) { hit = reached(ref) }
		res := Lockstep(m, seed, k, RandomPolicy(seed), rc)
		CloseSide(k)
		if err := unmodelledStallError(t.Name(), &res); err != nil {
			t.Errorf("seed %d: %v", seed, err)
		}
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nrecent %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, res.RecentSources(m, 6))
		}
		if hit {
			n++
		}
	}
	return n
}

// filters11Corpus writes a one-off corpus directory holding the inline
// fixture scripts and the real scripts of the named corpus cards, and returns
// it.
func filters11Corpus(t *testing.T, fixtures map[string]string, real ...string) string {
	t.Helper()
	src := corpusDirOrSkip(t)
	dir := t.TempDir()
	lock, err := os.ReadFile(filepath.Join(src, "cards.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cards.lock"), lock, 0o644); err != nil {
		t.Fatal(err)
	}
	put := func(name, body string) {
		file := strings.ToLower(strings.NewReplacer(" ", "_", "'", "", ",", "", "-", "_").Replace(name)) + ".txt"
		d := filepath.Join(dir, "cardsfolder", file[:1])
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, file), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range fixtures {
		put(name, body)
	}
	for _, name := range real {
		file := strings.ToLower(strings.NewReplacer(" ", "_", "'", "", ",", "", "-", "_").Replace(name)) + ".txt"
		body, err := os.ReadFile(filepath.Join(src, "cardsfolder", file[:1], file))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		put(name, string(body))
	}
	return dir
}

// printedDelveSetIn is printedDelveSet over the corpus directory dir.
func printedDelveSetIn(t *testing.T, dir string, cardNames [2][]string) cardset.Set {
	t.Helper()
	seen := map[string]bool{}
	var list []string
	for _, d := range cardNames {
		for _, n := range d {
			if !seen[n] {
				seen[n] = true
				list = append(list, n)
			}
		}
	}
	tmp := t.TempDir()
	listFile := filepath.Join(tmp, "cards.txt")
	if err := os.WriteFile(listFile, []byte(strings.Join(list, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "out")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if code := kgen.Main([]string{"-cards", dir, "-card-list", listFile, "-pool", "printed-delve", "-out", out}); code != 0 {
		t.Fatalf("kgen: exit %d", code)
	}
	pd, err := cardset.LoadFile(filepath.Join(out, "printed-delve.ktab"))
	if err != nil {
		t.Fatal(err)
	}
	return cardset.AllowUnverified(cardset.NewDataSet(pd))
}
