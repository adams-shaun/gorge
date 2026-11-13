package oracle

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
)

// TestMiniCommanderDecksDeterministic: a mini deck is the commander first,
// then an even spread of the rest of the list, the same every time, with the
// land share of a list that ends with its lands, and a partner pair keeps
// both commanders.
func TestMiniCommanderDecksDeterministic(t *testing.T) {
	f := deck.File{Name: "pod deck", Format: "commander", Commander: "Leader of the Pod"}
	f.Cards = append(f.Cards, deck.Entry{Name: "Leader of the Pod", Count: 1})
	for i := range 64 {
		f.Cards = append(f.Cards, deck.Entry{Name: "Spell " + string(rune('A'+i/26)) + string(rune('a'+i%26)), Count: 1})
	}
	f.Cards = append(f.Cards, deck.Entry{Name: "Dual Land", Count: 1}, deck.Entry{Name: "Swamp", Count: 34})
	a, err := MiniCommanderDeck(f, 40)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := MiniCommanderDeck(f, 40)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("two cuts of one list differ")
	}
	n, lands := 0, 0
	for i, e := range a.Cards {
		n += e.Count
		if e.Name == "Swamp" || e.Name == "Dual Land" {
			lands += e.Count
		}
		if i > 0 && e.Name == "Leader of the Pod" {
			t.Fatal("the commander is listed twice")
		}
	}
	if n != 40 || a.Cards[0] != (deck.Entry{Name: "Leader of the Pod", Count: 1}) {
		t.Fatalf("%d cards, first %v: want 40 led by the commander", n, a.Cards[0])
	}
	if ix := a.CommanderIndices(); len(ix) != 1 || ix[0] != 0 {
		t.Fatalf("commander indices %v, want [0]", ix)
	}
	// 35 of the 99 others are lands: 39 even picks hold 13 or 14 of them
	if lands < 13 || lands > 14 {
		t.Fatalf("%d lands in the cut, want 13 or 14", lands)
	}
	f.Commander, f.Commanders = "", []string{"Leader of the Pod", "Spell Ab"}
	p, err := MiniCommanderDeck(f, 40)
	if err != nil {
		t.Fatal(err)
	}
	if ix := p.CommanderIndices(); len(ix) != 2 || ix[0] != 0 || ix[1] != 1 {
		t.Fatalf("partner pair indices %v, want [0 1]", ix)
	}
	if _, err := MiniCommanderDeck(deck.File{Name: "constructed", Format: "standard", Cards: f.Cards}, 40); err == nil {
		t.Fatal("a constructed deck was cut")
	}
}

// cmd5MiniCorpus is the cmd5 pool's committed slice with every deck cut to
// n cards, and its deck names.
func cmd5MiniCorpus(t *testing.T, n int) (*Corpus, []string) {
	t.Helper()
	base, _ := testCorpus(t)
	dir, remove, err := ExtractSlice("cmd5")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(remove)
	pool, err := ReadPool(filepath.Join(dir, "pool.tsv"), "all")
	if err != nil {
		t.Fatal(err)
	}
	c := &Corpus{Reg: base.Reg, DecksDir: filepath.Join(dir, "decks"), decks: map[string][]*cards.Card{}}
	var names []string
	for _, p := range pool {
		names = append(names, p.Name)
	}
	minis, err := MiniCommanderDecks(c, names, n)
	if err != nil {
		t.Fatal(err)
	}
	c.AddDecks(minis)
	return c, names
}

// TestMatchupNCommanders: a four-seat pod of commander decks is a Commander
// matchup with every seat's commander, and gorge deals it with each
// commander in the command zone and every seat at 40 life.
func TestMatchupNCommanders(t *testing.T) {
	c, names := cmd5MiniCorpus(t, 40)
	m, err := c.MatchupN(names[:4], 0)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Commander || m.NSeat() != 4 {
		t.Fatalf("commander %v, %d seats", m.Commander, m.NSeat())
	}
	cfg := m.Config(7)
	if len(cfg.Commanders) != 4 {
		t.Fatalf("%d seats of commanders, want 4", len(cfg.Commanders))
	}
	for s := range 4 {
		if ix, p := m.SeatCommanders(s); ix != 0 || p != -1 || !reflect.DeepEqual(cfg.Commanders[s], []int{0}) {
			t.Fatalf("seat %d: commander %d partner %d, config %v", s, ix, p, cfg.Commanders[s])
		}
	}
	on, kc := m.kernelCommanders()
	for s := range min(4, len(kc)) {
		if !on || kc[s] != [2]int16{0, -1} {
			t.Fatalf("seat %d: kernel commanders %v", s, kc[s])
		}
	}
	gs, err := NewGorgeSide(m, 7, Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	gs.Step()
	var p Projection
	gs.Project(&p)
	for s := range 4 {
		// the commander left the 40-card library before the opening hand
		if sp := p.seat(s); *sp.life != 40 || *sp.hand != 7 || *sp.lib != 32 {
			t.Fatalf("seat %d: life %d hand %d library %d, want 40 7 32", s, *sp.life, *sp.hand, *sp.lib)
		}
	}
}

// TestCmdProjectionTwoSeatStable: a recorded two-seat Commander repro keeps
// its commander fields, and a two-seat record writes two of each, as before
// the further seats carried commanders.
func TestCmdProjectionTwoSeatStable(t *testing.T) {
	r, err := ReadRepro(filepath.Join("testdata", "repro", "cmd5_commander_bounce_waits_for_the_command_zone_choice.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := r.Matchup(synthReg(t))
	if err != nil {
		t.Fatal(err)
	}
	on, ix, partners := m.commanderFields()
	if !on || !reflect.DeepEqual(ix, r.Commanders) || !reflect.DeepEqual(partners, r.Partners) {
		t.Fatalf("fields %v %v %v, recorded %v %v %v", on, ix, partners, r.Commander, r.Commanders, r.Partners)
	}
	if len(ix) != 2 || partners != nil {
		t.Fatalf("a two-seat record writes %v and %v, want two commanders and no partners", ix, partners)
	}
}
