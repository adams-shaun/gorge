package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The lockstep: the three Auras whose activated ability
// is ChangeZone Defined$ Enchanted | Origin$ Battlefield | Destination$
// Library | Shuffle$ True -- Stay Hidden, Stay Silent; Dramatic Accusation;
// Watery Grasp. gorge's definedSpecEquipped names the source's AttachedTo,
// or its LastBearer once the source is off the battlefield
// (engine/effects/context.go:1306-1328); objectPathShuffleOwners then
// shuffles the bearer's owner even when the bearer already left
// (engine/effects/zone_library.go:83-97, CR 701.24c). The pool reaches the
// bearer-present, token-bearer and bearer-already-left cases; Swords to
// Plowshares removes a bearer with the activation on the stack.
//
// Only deck A carries the three carriers: each has "other creatures don't
// untap", and untapCands holds four replacement candidates, so one copy of
// each (three permanents) keeps a single creature's untap check inside the
// cap.

var kequipshufCarriers = [3]string{
	"Stay Hidden, Stay Silent",
	"Dramatic Accusation",
	"Watery Grasp",
}

var kequipshufDecks = [2][]cardset.Entry{
	{
		{Name: "Stay Hidden, Stay Silent", Count: 1},
		{Name: "Dramatic Accusation", Count: 1},
		{Name: "Watery Grasp", Count: 1},
		{Name: "Raise the Alarm", Count: 2},
		{Name: "Swords to Plowshares", Count: 2},
		{Name: "Elite Vanguard", Count: 3},
		{Name: "Silvercoat Lion", Count: 3},
		{Name: "Ornithopter", Count: 2},
		{Name: "Memnite", Count: 4},
		{Name: "Island", Count: 11},
		{Name: "Plains", Count: 10},
	},
	{
		{Name: "Raise the Alarm", Count: 2},
		{Name: "Swords to Plowshares", Count: 3},
		{Name: "Elite Vanguard", Count: 5},
		{Name: "Silvercoat Lion", Count: 5},
		{Name: "Ornithopter", Count: 2},
		{Name: "Memnite", Count: 2},
		{Name: "Island", Count: 10},
		{Name: "Plains", Count: 11},
	},
}

func kequipshufSetup(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("equipshuf")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

// kequipshufWatch fills the kernel's observation window with the carrier
// CardIDs of tbl (the table the kernel side plays).
func kequipshufWatch(t *testing.T, tbl *kop.Table) {
	t.Helper()
	dirac.DebugEquipShuf = dirac.EquipShufDebug{On: true}
	for i := range dirac.DebugEquipShuf.Watch {
		dirac.DebugEquipShuf.Watch[i] = kop.NoCard
	}
	for i, name := range kequipshufCarriers {
		for j := range tbl.Cards {
			if tbl.Cards[j].Name == name {
				dirac.DebugEquipShuf.Watch[i] = kop.CardID(j)
				break
			}
		}
		if dirac.DebugEquipShuf.Watch[i] == kop.NoCard {
			t.Fatalf("carrier %q is not in the kernel table", name)
		}
	}
}

// kequipshufRun plays seeds 1..seeds of the matchup with the random policy;
// it returns the games played, whether any diverged (the first is logged
// when stopOnDiv) and the kernel stalls.
func kequipshufRun(t *testing.T, reg *cards.Registry, set cardset.Set, seeds uint64, stopOnDiv bool) (games int, div bool, stalls int) {
	t.Helper()
	decks := kcastDeck(t, reg, kequipshufDecks)
	tbl, _, _, err := set.Matchup(kequipshufDecks[0], kequipshufDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	kequipshufWatch(t, tbl)
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kequipshuf-a", "kequipshuf-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		games++
		if res.Stall != "" && len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			stalls++
			if stopOnDiv {
				t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			div = true
			if !stopOnDiv {
				return
			}
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	return
}

func TestEquipshufLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, set := kequipshufSetup(t)
	defer func() {
		dirac.DebugEquipShuf.On = false
		dirac.DebugEquipShuf = dirac.EquipShufDebug{}
	}()
	games, div, stalls := kequipshufRun(t, reg, set, 256, true)
	h := dirac.DebugEquipShuf
	t.Logf("%d games, diverged %v, stalls %d, Stay Hidden %d, Dramatic Accusation %d, Watery Grasp %d, bearer already gone %d, token bearer %d (of those gone %d)",
		games, div, stalls, h.Hits[0], h.Hits[1], h.Hits[2], h.Gone, h.Token, h.TokenGone)
	if div {
		return
	}
	if stalls != 0 {
		t.Errorf("%d kernel stalls", stalls)
	}
	total := h.Hits[0] + h.Hits[1] + h.Hits[2]
	if total < 20 {
		t.Errorf("resolved shuffle activations %d, want at least 20", total)
	}
	for i, name := range kequipshufCarriers {
		if h.Hits[i] < 3 {
			t.Errorf("%s resolved shuffle activations %d, want at least 3", name, h.Hits[i])
		}
	}
	if h.Gone < 1 {
		t.Errorf("no resolution used LastBearer (bearer already gone); want at least 1")
	}
	if h.Token < 1 {
		t.Errorf("no resolution named a token bearer; want at least 1")
	}
}

// TestEquipshufMutation: skipping the shuffle of a RefAttached referent
// (DebugEquipShufNoShuffle) must make the lockstep diverge on some seed.
func TestEquipshufMutation(t *testing.T) {
	reg, set := kequipshufSetup(t)
	dirac.DebugEquipShufNoShuffle = true
	defer func() {
		dirac.DebugEquipShufNoShuffle = false
		dirac.DebugEquipShuf.On = false
		dirac.DebugEquipShuf = dirac.EquipShufDebug{}
	}()
	if _, div, _ := kequipshufRun(t, reg, set, 128, false); !div {
		t.Fatalf("DebugEquipShufNoShuffle: no divergence in 128 seeds")
	}
}

// TestEquipshufWaterbendArtifactMana: a Watery Grasp waterbend activation
// whose credited offer cost the floating pool pays is an offered option, not
// a lowered play, even while artifact mana floats (Mind Stone / Sol Ring on
// the board). Seed 39 once panicked "autopay found no plan" at decision 106.
func TestEquipshufWaterbendArtifactMana(t *testing.T) {
	reg, set := kequipshufSetup(t)
	decksAB := [2][]cardset.Entry{
		{
			{Name: "Stay Hidden, Stay Silent", Count: 1}, {Name: "Dramatic Accusation", Count: 1}, {Name: "Watery Grasp", Count: 1},
			{Name: "Raise the Alarm", Count: 2}, {Name: "Swords to Plowshares", Count: 2},
			{Name: "Elite Vanguard", Count: 3}, {Name: "Silvercoat Lion", Count: 3},
			{Name: "Ornithopter", Count: 2}, {Name: "Memnite", Count: 2},
			{Name: "Mind Stone", Count: 2}, {Name: "Sol Ring", Count: 1}, {Name: "Island", Count: 10}, {Name: "Plains", Count: 10},
		},
		{
			{Name: "Raise the Alarm", Count: 2}, {Name: "Swords to Plowshares", Count: 3},
			{Name: "Elite Vanguard", Count: 4}, {Name: "Silvercoat Lion", Count: 4},
			{Name: "Ornithopter", Count: 2}, {Name: "Memnite", Count: 2},
			{Name: "Mind Stone", Count: 2}, {Name: "Sol Ring", Count: 1}, {Name: "Island", Count: 10}, {Name: "Plains", Count: 10},
		},
	}
	decks := kcastDeck(t, reg, decksAB)
	tbl, _, _, err := set.Matchup(decksAB[0], decksAB[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= 39; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kequipshuf-a", "kequipshuf-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
}
