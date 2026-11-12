package oracle

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// commanderPrecons are two committed foundations Commander precons.
var commanderPrecons = [2]string{"foundations-calling-all-angels", "foundations-keen-engineering"}

// commanderCorpus is the test corpus's registry reading the committed
// Commander precons.
func commanderCorpus(t *testing.T) *Corpus {
	t.Helper()
	c, _ := testCorpus(t)
	return &Corpus{Reg: c.Reg, DecksDir: filepath.Join("..", "..", "testkit", "testutil", "decks"), decks: map[string][]*cards.Card{}}
}

func commanderMatchup(t *testing.T) *Matchup {
	t.Helper()
	m, err := commanderCorpus(t).Matchup(commanderPrecons[0], commanderPrecons[1], 0)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestCommanderMatchupConfig(t *testing.T) {
	c := commanderCorpus(t)
	m := commanderMatchup(t)
	if !m.Commander {
		t.Fatal("a matchup of two commander deck files is not a Commander matchup")
	}
	for s, name := range commanderPrecons {
		f, err := c.deckFile(name)
		if err != nil {
			t.Fatal(err)
		}
		want := f.CommanderIndices()
		if len(want) != 1 {
			t.Fatalf("%s: commander indices %v, want one", name, want)
		}
		if m.Commanders[s] != want[0] {
			t.Errorf("seat %d: commander index %d, want %d", s, m.Commanders[s], want[0])
		}
		if got := cardName(m.Decks[s][m.Commanders[s]]); !strings.EqualFold(got, f.CommanderNames()[0]) {
			t.Errorf("seat %d: deck card %d is %q, the deck's commander is %q", s, m.Commanders[s], got, f.CommanderNames()[0])
		}
	}
	cfg := m.Config(1)
	if cfg.Format != rules.FormatCommander || cfg.StartingLife != 40 {
		t.Errorf("config format %v life %d, want Commander and 40", cfg.Format, cfg.StartingLife)
	}
	if len(cfg.Commanders) != 2 || !slices.Equal(cfg.Commanders[0], []int{m.Commanders[0]}) || !slices.Equal(cfg.Commanders[1], []int{m.Commanders[1]}) {
		t.Errorf("config commanders %v, want %v", cfg.Commanders, m.Commanders)
	}
	on, ix := m.kernelCommanders()
	if !on || ix[0][0] != int16(m.Commanders[0]) || ix[1][0] != int16(m.Commanders[1]) {
		t.Errorf("kernel commanders %v %v", on, ix)
	}

	// a Constructed deck pair stays Constructed
	k := commanderCorpus(t)
	k.AddDecks(map[string]deck.File{"plain": {Format: "standard", Cards: []deck.Entry{{Name: "Forest", Count: 40}}}})
	pm, err := k.Matchup("plain", "plain", 0)
	if err != nil {
		t.Fatal(err)
	}
	if pc := pm.Config(1); pm.Commander || pc.Format != rules.FormatConstructed || pc.StartingLife != 0 || pc.Commanders != nil {
		t.Errorf("constructed matchup: commander %v, config %v %d %v", pm.Commander, pc.Format, pc.StartingLife, pc.Commanders)
	}
}

func TestCommanderReproRoundTrip(t *testing.T) {
	m := commanderMatchup(t)
	const seed, n = 7, 50
	rc := RunConfig{Config: Config{PassElision: true}, MaxDecisions: n}
	res := Lockstep(m, seed, mustSide(t, GorgeKernel(rc.Config), m, seed), RandomPolicy(seed), rc)
	if res.Div != nil {
		t.Fatalf("self-check diverged: %+v", res.Div)
	}
	if len(res.Actions) < n {
		t.Fatalf("game ended after %d actions, want %d", len(res.Actions), n)
	}
	path := filepath.Join(t.TempDir(), "cmd.repro.json")
	if err := NewRepro(m, seed, rc, &res).Write(path); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"commander":true`) || !strings.Contains(string(raw), `"commanders"`) {
		t.Fatalf("repro does not record the commander:\n%.400s", raw)
	}
	r, err := ReadRepro(path)
	if err != nil {
		t.Fatal(err)
	}
	m2, err := r.Matchup(m.reg)
	if err != nil {
		t.Fatal(err)
	}
	if !m2.Commander || m2.Commanders != m.Commanders {
		t.Fatalf("read back commander %v %v, want %v", m2.Commander, m2.Commanders, m.Commanders)
	}
	g, err := NewGorgeSide(m2, seed, r.RunConfig().Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	for s := 0; s < 2; s++ {
		pl := &g.E.G.Players[s]
		if pl.Life != 40 || len(pl.Commanders) != 1 || g.E.G.Obj(pl.Commanders[0]).Zone != state.ZCommand {
			t.Fatalf("seat %d: life %d, commanders %v: not a Commander game", s, pl.Life, pl.Commanders)
		}
	}
	re := r.Rerun(m2, GorgeKernel(r.RunConfig().Config), 0)
	if re.Div != nil {
		t.Fatalf("rerun diverged: %+v", re.Div)
	}
	if !slices.Equal(re.Actions, res.Actions[:n]) {
		t.Fatalf("rerun played %d actions, differing from the recorded %d", len(re.Actions), n)
	}

	// a repro written before the fields existed reads as Constructed
	old, err := ReadRepro(filepath.Join("testdata", "repro", "uhura_commander_identity_mana_pays_nothing.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	if old.Commander || old.Commanders != nil {
		t.Fatalf("old repro reads commander %v %v", old.Commander, old.Commanders)
	}
	om, err := old.Matchup(m.reg)
	if err != nil {
		t.Fatal(err)
	}
	if oc := om.Config(old.Seed); om.Commander || om.Commanders != [2]int{-1, -1} || oc.Format != rules.FormatConstructed || oc.Commanders != nil {
		t.Fatalf("old repro matchup: commander %v %v, config %v %v", om.Commander, om.Commanders, oc.Format, oc.Commanders)
	}
}

func mustSide(t *testing.T, mk SideFactory, m *Matchup, seed uint64) Side {
	t.Helper()
	s, err := mk(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestProjectionOverflowIsCapacityStall(t *testing.T) {
	c, _ := testCorpus(t)
	look := func(n string) *cards.Card {
		cd, ok := c.Reg.Lookup(n)
		if !ok {
			t.Skipf("%s not in the corpus", n)
		}
		return cd
	}
	// MaxInst-1 deck cards: the first token is an instance past the projection
	var decks [2][]*cards.Card
	for s, n := range [2]int{(MaxInst - 1) / 2, MaxInst - 1 - (MaxInst-1)/2} {
		for i := 0; i < n; i++ {
			name := "Plains"
			if s == 0 && i%2 == 1 {
				name = "Raise the Alarm"
			}
			decks[s] = append(decks[s], look(name))
		}
	}
	m, err := NewMatchup(c.Reg, [2]string{"tokens", "plains"}, decks, 0)
	if err != nil {
		t.Fatal(err)
	}
	const seed = 3
	rc := RunConfig{Config: Config{PassElision: true}, MaxTurns: 30}
	lead, err := NewGorgeSide(m, seed, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(lead.id.obj) != MaxInst-1 || lead.id.err != "" {
		t.Fatalf("genesis bound %d instances (err %q), want %d", len(lead.id.obj), lead.id.err, MaxInst-1)
	}
	res := LockstepLed(m, seed, lead, mustSide(t, GorgeKernel(rc.Config), m, seed), RandomPolicy(seed), rc)
	if lead.id.err == "" {
		t.Fatalf("no instance overflow: %d instances, stall %q", len(lead.id.obj), res.Stall)
	}
	if res.Stall != ProjectionStall || res.Div != nil || res.StallUnmodelled {
		t.Fatalf("stall %q, unmodelled %v, divergence %+v; want the %q capacity stall", res.Stall, res.StallUnmodelled, res.Div, ProjectionStall)
	}
	if !strings.HasPrefix(res.Stall, KernelStallPrefix) || !res.Clean() {
		t.Fatalf("stall %q is not counted as a capacity stall", res.Stall)
	}
}
