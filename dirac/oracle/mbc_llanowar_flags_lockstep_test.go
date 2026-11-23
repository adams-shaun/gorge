package oracle

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/deck"
)

// CR 601.2g: the round-2 tail-15 fx-wh hunt's pair 99 (deal seed 158018, game
// seed 9205021827846595347) is the ticketed Llanowar Wastes game: it diverged
// at decisions 470 and then 874 (Cards[62].Flags beside a granted pool and a
// pain tap) on the previous round's intermediate kernel states. Measured at
// this head, neither divergence reproduces on any committable tree — the game
// is clean at 716a596 and stays clean with feeOrderOf reverted to the pre-fix
// kernel — so there is no fix to red-check here; this pins the ticket's own
// game whole: the full clean recording fails on any divergence or drift the
// kernel behaviour that diverged it (or anything else) reintroduces. The
// pool's remaining projection divergences (the hunt's pairs
// 36/69/74/123/152/177/191) are other classes; this game is not one of them.
const (
	llanowarFlagsPool      = "tail-15"
	llanowarFlagsDealSeed  = 158018
	llanowarFlagsPair      = 99
	llanowarFlagsClassAt   = 470
	llanowarFlagsGameSeed  = 9205021827846595347
	llanowarFlagsRecording = "llanowar_wastes_flags_full_game"
	llanowarFlagsCard      = "Llanowar Wastes"
)

// llanowarFlagsFocus is the hunt's -focus list (the scratch fx-wh.txt: one
// card a line, enablers after a tab).
const llanowarFlagsFocus = `Wild Hunger	Abundant Growth
Slagwurm Armor
Vine Trellis
`

// TestMBCLlanowarFlagsRecord re-deals the hunt's fx-wh focus pool over tail-15
// (the same deal dealSynth makes: FocusPoolWith at the hunt's base seed), takes
// pair 99 and records its clean game. It runs only with PINS_RECORD=1.
func TestMBCLlanowarFlagsRecord(t *testing.T) {
	if os.Getenv("PINS_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	set, err := cardset.SynthByName(llanowarFlagsPool)
	if err != nil {
		t.Fatal(err)
	}
	focus, err := ParseFocus([]byte(llanowarFlagsFocus))
	if err != nil {
		t.Fatal(err)
	}
	poolDecks, files, err := FocusPoolWith(reg, set, nil, "fx-wh", focus, FocusPerDeck, llanowarFlagsDealSeed, SynthDecks)
	if err != nil {
		t.Fatal(err)
	}
	p := PairingAt(poolDecks, llanowarFlagsPair, llanowarFlagsDealSeed)
	if p.Seed != llanowarFlagsGameSeed {
		t.Fatalf("pair %d deals seed %d, want the hunt's game seed %d: the deal has moved", llanowarFlagsPair, p.Seed, llanowarFlagsGameSeed)
	}
	// Precondition: this is the ticket's game — Llanowar Wastes, the divergence's
	// implicated card, is in one of the pair's decks.
	held := false
	for _, name := range p.Decks() {
		f, ok := files[name]
		if !ok {
			t.Fatalf("pair %d deck %s was not dealt", llanowarFlagsPair, name)
		}
		if slices.ContainsFunc(f.Cards, func(e deck.Entry) bool { return e.Name == llanowarFlagsCard }) {
			held = true
		}
	}
	if !held {
		t.Fatalf("pair %d (%s): neither deck holds %s", llanowarFlagsPair, p.Decks()[0]+" vs "+p.Decks()[1], llanowarFlagsCard)
	}
	c, err := OpenCorpus(corpusDirOrSkip(t), "")
	if err != nil {
		t.Fatal(err)
	}
	c.AddDecks(files)
	m, tbl, err := SetMatchupN(set, c, p.Decks(), 0)
	if err != nil {
		t.Fatalf("pair %d refused: %v", llanowarFlagsPair, err)
	}
	rc := RunConfig{Config: Config{GorgeAutopaySurface: true}, MaxDecisions: 20000, MaxTurns: 100}
	k, err := SetSide(set, tbl, rc.Config)(m, p.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	res := Lockstep(m, p.Seed, k, RandomPolicy(p.Seed), rc)
	if res.Div != nil {
		t.Fatalf("pair %d diverges at %d of %d: %s %s %v", llanowarFlagsPair, res.Div.At, res.Decisions, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if res.Decisions <= llanowarFlagsClassAt {
		t.Fatalf("the game ends at %d decisions, before the region the fee-order class fired in (decision %d)", res.Decisions, llanowarFlagsClassAt)
	}
	r := NewRepro(m, p.Seed, rc, &res)
	r.Pool, r.Kernel = llanowarFlagsPool, "synth "+llanowarFlagsPool
	if err := r.Write(filepath.Join("testdata", "repro", llanowarFlagsRecording+".repro.json")); err != nil {
		t.Fatal(err)
	}
	t.Logf("pair %d (%s vs %s): %d decisions, %d actions", llanowarFlagsPair, p.Decks()[0], p.Decks()[1], res.Decisions, len(r.Actions))
}

// TestMBCLlanowarFlagsReprosLockstep replays the recorded game: no divergence
// over the whole game, every recorded action offered, and the kernel never
// stops. A return of the fee-order gap (the kernel's plain walk withholding
// the flashback cast again) diverges or skips actions and fails here.
func TestMBCLlanowarFlagsReprosLockstep(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro(filepath.Join("testdata", "repro", llanowarFlagsRecording+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	// Precondition: the recording is the ticket's game.
	if !slices.Contains(r.Cards[0], llanowarFlagsCard) && !slices.Contains(r.Cards[1], llanowarFlagsCard) {
		t.Fatalf("%s is not the Llanowar Wastes game: neither deck holds %s", llanowarFlagsRecording, llanowarFlagsCard)
	}
	stdReproNamed(t, llanowarFlagsRecording)
}
