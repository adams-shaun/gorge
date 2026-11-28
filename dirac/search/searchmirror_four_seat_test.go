package search_test

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/oracle"
)

// TestSearchMirrorsAzmctsFourSeat plays one four-seat cmd5 pod in lockstep
// and, at every searched decision of its first 10 turns, asks both searches
// the same position: bots/azmcts on the gorge side (an honest-root redeal
// from that seat's feed) and dirac/search on the kernel side. The candidate
// lists, mapped to canonical actions, must agree at every decision the two
// engines pose with the same ordinal.
//
// The chosen candidate and the visits are recorded too, but are not asserted:
// the two engines' world streams differ by design (dirac/search/doc.go,
// "Where this does not match azmcts exactly"), so at sims 8 a visit count can
// part without a candidate difference. The harness reports that first gap so
// it can be measured rather than guessed.
//
// Pod 11 is chosen because its first ten turns pose no decision beneath the
// canonical surface (no lowered X cast, manual payment window or mana-colour
// ask), which is the precondition for the two per-decision search seeds to
// fold the same ordinal.
func TestSearchMirrorsAzmctsFourSeat(t *testing.T) {
	cardsDir, err := cardset.CorpusDir()
	if err != nil {
		t.Skipf("no gorge corpus: %v", err)
	}
	dir, remove, err := oracle.ExtractSlice("cmd5")
	if err != nil {
		t.Skipf("no cmd5 slice: %v", err)
	}
	defer remove()
	c, err := oracle.OpenCorpus(cardsDir, filepath.Join(dir, "decks"))
	if err != nil {
		t.Fatal(err)
	}
	pool, err := oracle.ReadPool(filepath.Join(dir, "pool.tsv"), "all")
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(pool))
	for i, p := range pool {
		names[i] = p.Name
	}
	minis, err := oracle.MiniCommanderDecks(c, names, 40)
	if err != nil {
		t.Fatal(err)
	}
	c.AddDecks(minis)
	set, err := cardset.ByName("cmd5")
	if err != nil {
		t.Skipf("no cmd5 table (make dirac-tables): %v", err)
	}
	set = cardset.AllowUnverified(set)
	decks := make([]oracle.PoolDeck, len(names))
	for i, n := range names {
		decks[i] = oracle.PoolDeck{Name: n}
	}

	const pod = 11
	p := oracle.PodAt(decks, pod, 1, 4)
	m, tbl, err := oracle.SetMatchupN(set, c, p.Decks(), 0)
	if err != nil {
		t.Fatal(err)
	}
	res, err := oracle.SearchMirrorPod(m, tbl, set, p.Seed, [4]uint64{11, 22, 33, 44}, 10, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Res.Div != nil {
		t.Fatalf("pod %d lockstep diverged: %v", pod, res.Res.Div)
	}
	if res.Compared == 0 {
		t.Fatal("no decision was searched on both engines: the compared precondition did not hold")
	}
	if res.Beneath != 0 {
		t.Fatalf("pod %d poses %d decisions beneath the canonical surface in its first 10 turns; the seeds part there", pod, res.Beneath)
	}
	if res.Refused != 0 {
		t.Fatalf("pod %d's honest root refused at %d decisions", pod, res.Refused)
	}
	if res.Mismatch != nil {
		t.Fatalf("pod %d candidate lists diverged at decision %d (turn %d, %s p%d): %s\n  gorge:  %v\n  kernel: %v",
			pod, res.Mismatch.At, res.Mismatch.Turn, res.Mismatch.Kind, res.Mismatch.Player,
			res.Mismatch.Detail, res.Mismatch.Gorge, res.Mismatch.Kernel)
	}
	t.Logf("pod %d mirrored candidate lists at %d searched decisions over 10 turns (%v)", pod, res.Compared, res.Kinds)
	if res.Choice != nil {
		t.Logf("first chosen-candidate difference (world streams): %s", res.Choice)
	}
	if res.Visits != nil {
		t.Logf("first visit difference (world streams): %s", res.Visits)
	}
}
