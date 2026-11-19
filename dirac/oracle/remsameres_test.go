package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Same-resolution Remembered reads (kgen lower_remembered_rest.go): a writer
// whose set is read only by a later node of its own chain, through an amount
// SVar (Pyretic Rebirth X Remembered$CardManaCost, Lie in Wait
// Remembered$CardPower) or a branch the chain runs (Gilt-Leaf Ambush's Clash
// WinSubAbility reading IsRemembered). gorge keeps the set on the source,
// the kernel per resolution; for a read inside the resolution they agree.
// Four-seat pods (two-seat for the Clash card) over the remsameres fixture pool.

var remSameResCards = []string{"Pyretic Rebirth", "Lie in Wait", "Gilt-Leaf Ambush"}

func remSameResDeck(spell string) []string {
	d := []string{}
	add := func(n string, c int) {
		for i := 0; i < c; i++ {
			d = append(d, n)
		}
	}
	add(spell, 6)
	add("Grizzly Bears", 8)
	add("Hill Giant", 6)
	add("Lightning Bolt", 4)
	add("Swamp", 5)
	add("Mountain", 4)
	add("Forest", 4)
	add("Island", 3)
	return d
}

func TestRemSameResolutionFourSeatLockstep(t *testing.T) {
	corpusDirOrSkip(t)
	reg := synthReg(t)
	set, err := cardset.RawByName("remsameres")
	if err != nil {
		t.Fatal(err)
	}
	sup := set.Supported()
	for _, n := range remSameResCards {
		if i, ok := set.Lookup(n); !ok || i >= len(sup) || !sup[i] {
			t.Fatalf("the remsameres pool does not play %s: %s", n, set.Reason(i))
		}
	}
	casts := map[string]int{}
	watch := map[string]bool{"Pyretic Rebirth": true, "Lie in Wait": true, "Gilt-Leaf Ambush": true, "Lightning Bolt": true}
	pods := [][][]string{
		{remSameResDeck("Pyretic Rebirth"), remSameResDeck("Lie in Wait"), remSameResDeck("Lie in Wait"), remSameResDeck("Pyretic Rebirth")},
		{remSameResDeck("Lie in Wait"), remSameResDeck("Pyretic Rebirth"), remSameResDeck("Pyretic Rebirth"), remSameResDeck("Lie in Wait")},
		// Gilt-Leaf Ambush's Clash is two-seat only (CardPodLimited)
		{remSameResDeck("Gilt-Leaf Ambush"), remSameResDeck("Gilt-Leaf Ambush")},
	}
	for pi, decks := range pods {
		var lists [][]cardset.Entry
		for _, d := range decks {
			cnt := map[string]int{}
			for _, n := range d {
				cnt[n]++
			}
			var es []cardset.Entry
			for n, c := range cnt {
				es = append(es, cardset.Entry{Name: n, Count: c})
			}
			lists = append(lists, es)
		}
		tbl, _, err := set.MatchupN(lists)
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= 25; seed++ {
			r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{decks[0], decks[1]}, Seed: seed, PassElision: true, Surface: "full"}
			for i := 2; i < len(decks); i++ {
				r.MoreDecks = append(r.MoreDecks, "x"+string(rune('0'+i)))
				r.MoreCards = append(r.MoreCards, decks[i])
			}
			m, err := r.Matchup(reg)
			if err != nil {
				t.Fatal(err)
			}
			rc := r.RunConfig()
			rc.MaxDecisions, rc.MaxTurns = 3000, 30
			k, err := SetSide(set, tbl, rc.Config)(m, seed)
			if err != nil {
				t.Fatalf("pod %d seed %d: %v", pi, seed, err)
			}
			res := Lockstep(m, seed, k, fixtureWatchPolicy(seed, watch, casts), rc)
			CloseSide(k)
			if res.Div != nil {
				t.Fatalf("pod %d seed %d: divergence at %d: %s %s %v", pi, seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Fatalf("pod %d seed %d: kernel stopped: %s", pi, seed, res.Stall)
			}
		}
	}
	t.Logf("casts %v", casts)
	for _, n := range remSameResCards {
		if casts[n] == 0 {
			t.Errorf("%s was never cast", n)
		}
	}
}
