package main

import (
	"fmt"
	"math/rand/v2"
	"os"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
)

func TestSignatureCollapsesSeedNumbers(t *testing.T) {
	a := signature("livelock", "livelock detected (repeating cycle): object 12, kind priority, cycle: [seq 40 priority player=1]")
	b := signature("livelock", "livelock detected (repeating cycle): object 99, kind priority, cycle: [seq 7123 priority player=0]")
	if a != b {
		t.Fatalf("same cycle on different seeds must share a signature:\n%s\n%s", a, b)
	}
}

func TestGenerateIsDeterministicMonoColourSixty(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	p, err := buildPool(reg)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := loadCov(t.TempDir() + "/none.json")
	gen := func() genDeck { return generate(rand.New(rand.NewPCG(7, 9)), p, c) }
	d1, d2 := gen(), gen()
	if len(d1.Cards) != 60 {
		t.Fatalf("deck has %d cards, want 60", len(d1.Cards))
	}
	for i := range d1.Cards {
		if d1.Cards[i] != d2.Cards[i] {
			t.Fatalf("generation not deterministic at %d: %s vs %s", i, d1.Cards[i], d2.Cards[i])
		}
	}
	cards, err := resolveDeck(reg, d1)
	if err != nil {
		t.Fatal(err)
	}
	lands := 0
	ci := -1
	for i, n := range colourNames {
		if n == d1.Colour {
			ci = i
		}
	}
	for _, cd := range cards {
		if cd.Faces[0].IsLand() {
			lands++
		}
		if id := identity(cd); id != 0 && id != 1<<ci {
			t.Fatalf("%s has identity %b outside mono-%s", cardName(cd), id, d1.Colour)
		}
	}
	if lands < 20 {
		t.Fatalf("deck has %d lands, want at least 20", lands)
	}
}

// TestBoardGuardRecordsBigboard pins the harness-side board-size watchdog:
// a game whose live object count exceeds -max-objects ends as its own
// "bigboard" kind (never "hang" or "livelock", so triage can tell a runaway
// token engine from an engine bug), and the cap is inert below the limit
// and when disabled. Two 60-card decks put 120 live objects in the arena at
// genesis, so a cap of 100 fires at the first decision and one of 100000
// never does.
func TestBoardGuardRecordsBigboard(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	p, err := buildPool(reg)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := loadCov(t.TempDir() + "/none.json")
	r := rand.New(rand.NewPCG(3, 5))
	decks := []genDeck{generate(r, p, c), generate(r, p, c)}

	f, _ := playOne(reg, decks, 11, 3, 20000, 100, false)
	if f == nil || f.Kind != "bigboard" {
		t.Fatalf("failure = %+v, want kind bigboard", f)
	}
	if !strings.Contains(f.Diag, "exceeds -max-objects 100") || !strings.HasPrefix(f.Sig, "bigboard: ") {
		t.Fatalf("bigboard record lacks its diagnostic: sig %q diag %q", f.Sig, f.Diag)
	}
	if boardGuard(0) != nil {
		t.Fatalf("-max-objects 0 must disable the guard")
	}
	// Under the cap the guard is inert: the 3-turn cap ends the game as a
	// plain stall (not a failure record).
	if f, _ := playOne(reg, decks, 11, 3, 20000, 100000, false); f != nil {
		t.Fatalf("game under the object cap recorded %+v", f)
	}
}

// TestAbilityInventory pins the usable-ability inventory on real corpus
// cards: Gilded Goose carries an activated ability, a mana ability and an
// ETB trigger; a spell's own SP ability and statics are excluded, and a
// keyword-expanded activation (Cycling) counts.
func TestAbilityInventory(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	look := func(n string) []abilitySlot {
		c, ok := reg.Lookup(n)
		if !ok {
			t.Fatalf("%s not in corpus", n)
		}
		return abilityInventory(c)
	}
	goose := look("Gilded Goose")
	var got []string
	for _, s := range goose {
		got = append(got, fmt.Sprintf("%s:%s:%v", s.key, s.desc, s.mana))
	}
	want := []string{"f0/a0:Token:false", "f0/a1:Mana:true", "f0/t0:ChangesZone:false"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("Gilded Goose inventory = %v, want %v", got, want)
	}
	if goose[1].produced != "Any" {
		t.Fatalf("Gilded Goose mana Produced = %q, want Any", goose[1].produced)
	}
	if inv := look("Lightning Bolt"); len(inv) != 0 {
		t.Fatalf("Lightning Bolt's spell ability must not be in the inventory: %v", inv)
	}
	if inv := look("Street Wraith"); len(inv) != 1 || inv[0].desc != "Draw/Cycling" {
		t.Fatalf("Street Wraith inventory = %+v, want its expanded Cycling activation", inv)
	}
	if inv := look("Delver of Secrets"); len(inv) != 1 || inv[0].key != "f0/t0" {
		t.Fatalf("Delver of Secrets inventory = %+v, want its front-face upkeep trigger", inv)
	}
}

func TestProducesColour(t *testing.T) {
	for _, tc := range []struct {
		produced    string
		colour      byte
		exact, wild bool
	}{
		{"G", 'G', true, false},
		{"G", 'U', false, false},
		{"Combo W U", 'U', true, false},
		{"Any", 'R', false, true},
		{"", 'C', false, true},
	} {
		ex, wd := producesColour(tc.produced, tc.colour)
		if ex != tc.exact || wd != tc.wild {
			t.Errorf("producesColour(%q, %c) = %v,%v want %v,%v", tc.produced, tc.colour, ex, wd, tc.exact, tc.wild)
		}
	}
}

// TestAbilitiesUsedFromPlayedGame plays a real bot game with Mind Stone (a
// {T}: Add {C} mana ability plus a draw activation) and Solemn Simulacrum (an
// ETB and a dies trigger), and checks the log walk credits Mind Stone's mana
// ability (the Tap+ManaAdd proxy) and a Solemn trigger (the minted
// TriggerPush object), and credits only keys in each card's inventory.
func TestAbilitiesUsedFromPlayedGame(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	var d genDeck
	d.Colour = "C"
	for i := 0; i < 60; i++ {
		switch {
		case i < 22:
			d.Cards = append(d.Cards, "Wastes")
		case i < 41:
			d.Cards = append(d.Cards, "Mind Stone")
		default:
			d.Cards = append(d.Cards, "Solemn Simulacrum")
		}
	}
	f, gc := playOne(reg, []genDeck{d, d}, 5, 14, 20000, 0, false)
	if f != nil {
		t.Fatalf("game failed: %s", f.Diag)
	}
	for card, keys := range gc.used {
		c, _ := reg.Lookup(card)
		inv := abilityDescs(c)
		for k := range keys {
			if _, ok := inv[k]; !ok {
				t.Errorf("%s credited key %s outside its inventory %v", card, k, inv)
			}
		}
	}
	if !gc.cast["Mind Stone"] || !gc.cast["Solemn Simulacrum"] {
		t.Fatalf("setup: expected both cards cast, cast=%v", gc.cast)
	}
	if !gc.used["Mind Stone"]["f0/a0"] {
		t.Errorf("Mind Stone's mana ability not credited: used=%v", gc.used)
	}
	if !gc.used["Solemn Simulacrum"]["f0/t0"] {
		t.Errorf("Solemn Simulacrum's ETB trigger not credited: used=%v", gc.used)
	}
	if !gc.used["Wastes"]["f0/a0"] {
		t.Errorf("Wastes' mana ability not credited: used=%v", gc.used)
	}
}

// TestCovUsedBackwardCompatible: a state file written before Used existed
// loads with an empty Used map, and full coverage needs a cast plus every key.
func TestCovUsedBackwardCompatible(t *testing.T) {
	path := t.TempDir() + "/old.json"
	if err := os.WriteFile(path, []byte(`{"games":3,"included":{"X":1},"cast":{"X":1},"ability":{},"fails":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := loadCov(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Used == nil || len(c.Used) != 0 {
		t.Fatalf("Used = %v, want empty non-nil", c.Used)
	}
	if !c.full("X", nil) {
		t.Fatalf("a cast card with no abilities is fully covered")
	}
	keys := []string{"f0/a0", "f0/t0"}
	if c.full("X", keys) {
		t.Fatalf("no ability used yet: not full")
	}
	c.Used["X"] = map[string]int64{"f0/a0": 1}
	if got := c.missing("X", keys); len(got) != 1 || got[0] != "f0/t0" {
		t.Fatalf("missing = %v", got)
	}
	w := c.weight(poolCard{name: "X", keys: keys})
	c.Used["X"]["f0/t0"] = 1
	if !c.full("X", keys) {
		t.Fatalf("every key used and cast: full")
	}
	if w2 := c.weight(poolCard{name: "X", keys: keys}); w != 4*w2 {
		t.Fatalf("weight with a missing ability %v, want 4x the full weight %v", w, w2)
	}
}
