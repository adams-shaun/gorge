package oracle

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The cards a four-seat pod still refuses at its start (StallTwoSeat), named
// here with the reading that keeps each two-seat only, and the cards the pod
// retirement lifted, which must not come back. The remainder shrinks only:
// lifting a card moves it from podRefusedCards to podLiftedCards, in the
// change that lifts it, with its pod lockstep (podlimited_cards_mp_test.go).
var podRefusedCards = map[string]string{
	// a LosesGame trigger on a granted Effect: in a pod the game goes on, so
	// the trigger resolves (copies of the loser's artifacts and creatures from
	// last-known information), which the kernel has no loss trigger for
	"Miss Highwater": "a LosesGame Effect trigger never resolves: a two-player game ends with the loss",
	// the as-enters ChoosePlayer is followed by a look at that player's hand
	// and a name choice: the kernel's pre-entry ask poses the look at the
	// next living opponent only
	"Sorcerous Spyglass": "ChoosePlayer among the opponents answered as the one opponent",
}

// podLiftedCards are the cards kgen once marked two-seat only that read the
// seat they name now: each has a four-seat lockstep in
// podlimited_cards_mp_test.go.
var podLiftedCards = []string{"Ravenloft Adventurer", "Etrata, the Silencer", "The Weaver King", "Vulcan Mind-Meld"}

// TestPodLimitedRemainderRefusedByName: each remaining pod-limited card is
// marked two-seat by its recorded reason, a four-seat table holding it is
// refused as two-seat only through the kernel's own preparation, and the
// stall is the declared StallTwoSeat at the game's start, not a divergence.
// The lifted cards carry no mark.
func TestPodLimitedRemainderRefusedByName(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	var names []string
	for n := range podRefusedCards {
		names = append(names, n)
	}
	slices.Sort(names)
	sup, err := kgen.SupportFor(cardsDir, append(slices.Clone(names), podLiftedCards...))
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range podLiftedCards {
		if sup[n].TwoSeat {
			t.Errorf("%s is two-seat only again: %v", n, sup[n].TwoSeatWhy)
		}
	}
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		why := podRefusedCards[n]
		if !sup[n].TwoSeat || !slices.ContainsFunc(sup[n].TwoSeatWhy, func(w string) bool { return strings.Contains(w, why) }) {
			t.Errorf("%s: TwoSeat %v why %v, want the reason %q (lift it: move it to podLiftedCards with a pod lockstep)", n, sup[n].TwoSeat, sup[n].TwoSeatWhy, why)
			continue
		}
		c, ok := reg.Lookup(n)
		if !ok {
			t.Fatalf("%s not in the corpus", n)
		}
		basic, ok := reg.Lookup("Swamp")
		if !ok {
			t.Fatal("no Swamp")
		}
		pd, err := kgen.TableFromCards(reg, []*cards.Card{c, basic}, "podrefused-oracle")
		if err != nil {
			t.Fatalf("%s table: %v", n, err)
		}
		set := cardset.Raw(cardset.NewDataSet(pd))
		deck := []cardset.Entry{{Name: n, Count: 4}, {Name: "Swamp", Count: 20}}
		tbl, ids, err := set.MatchupN([][]cardset.Entry{deck, deck, deck, deck})
		if err != nil {
			t.Fatal(err)
		}
		if !dirac.TwoSeatOnly(tbl) {
			t.Fatalf("%s: a four-seat table is not refused as two-seat only", n)
		}
		m, err := KernelMatchupN(reg, tbl, []string{"pod-a", "pod-b", "pod-c", "pod-d"}, ids, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		rc := RunConfig{MaxDecisions: 200, MaxTurns: 5}
		k, err := SetSide(set, tbl, rc.Config)(m, 1)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, 1, k, RandomPolicy(1), rc)
		CloseSide(k)
		if res.Div != nil || !strings.HasPrefix(res.Stall, KernelStallPrefix) || !strings.Contains(res.Stall, dirac.StallName(dirac.StallTwoSeat)) {
			t.Errorf("%s: a four-seat game ended %+v stall %q, want the kernel's %s stall at its start", n, res.Div, res.Stall, dirac.StallName(dirac.StallTwoSeat))
		}
	}
}
