package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// villainousDeck is one 40-card villainouschoice deck: the carriers it leans
// on (Genesis, Plan, Eggman, Emperor, Damocles) beside burn and red/black/blue lands.
func villainousDeck(genesis, plan, eggman, emperor, damocles int) []cardset.Entry {
	spells := genesis + plan + eggman + emperor + damocles + 4
	lands := 40 - spells
	return []cardset.Entry{
		{Name: "Genesis of the Daleks", Count: genesis},
		{Name: "Great Intelligence's Plan", Count: plan}, {Name: "Dr. Eggman", Count: eggman},
		{Name: "The Dalek Emperor", Count: emperor}, {Name: "Damocles Base, Sword of Kang", Count: damocles},
		{Name: "Lightning Bolt", Count: 4},
		{Name: "Mountain", Count: lands / 3}, {Name: "Swamp", Count: lands / 3},
		{Name: "Island", Count: lands - 2*(lands/3)},
	}
}

func newVillainousChoicePod(t *testing.T) voteChoicesPod {
	t.Helper()
	reg := synthReg(t) // skips without the corpus, before any table load
	set, err := cardset.RawByName("villainouschoice")
	if err != nil {
		t.Fatal(err)
	}
	decks := [4][]cardset.Entry{
		villainousDeck(3, 4, 3, 3, 3),
		villainousDeck(2, 4, 4, 3, 3),
		villainousDeck(4, 3, 3, 3, 2),
		villainousDeck(3, 3, 4, 3, 3),
	}
	tbl, _, err := set.MatchupN([][]cardset.Entry{decks[0], decks[1], decks[2], decks[3]})
	if err != nil {
		t.Fatal(err)
	}
	return voteChoicesPod{reg: reg, set: set, tbl: tbl, decks: decks}
}

// countModeAsks counts the mode picks a game's applied actions carry. The
// fixture has no charm, so every one is a villainous victim's pick of a
// branch.
func countModeAsks(res Result) int {
	n := 0
	for _, a := range res.Actions {
		if strings.HasPrefix(a.String(), "mode(") {
			n++
		}
	}
	return n
}

// TestVillainousChoicePod plays four-seat pods of the villainouschoice
// fixture: each opponent of the controller, in gorge's Defined$ order, picks
// a branch of the villainous choice and the chosen body runs for them. Over
// the seeds at least one victim ask resolves, and gorge and the kernel agree
// on every decision.
func TestVillainousChoicePod(t *testing.T) {
	p := newVillainousChoicePod(t)
	asks := 0
	for _, seed := range []uint64{1, 2, 3, 4, 5, 6, 7, 8} {
		res, _ := p.playFour(t, seed, 40)
		if res.Div != nil {
			voteChoicesFail(t, seed, res)
		}
		voteChoicesStop(t, seed, res)
		n := countModeAsks(res)
		asks += n
		t.Logf("seed %d: %d decisions, over %v %s; %d mode ask(s)", seed, res.Decisions, res.Over, res.Stall, n)
	}
	if asks == 0 {
		t.Fatal("no villainous choice was asked in any seed")
	}
}

// TestVillainousChoiceTwoSeat is the duel form: the sole opponent is the one
// victim.
func TestVillainousChoiceTwoSeat(t *testing.T) {
	p := newVillainousChoicePod(t)
	asks := 0
	for _, seed := range []uint64{1, 2, 3, 4} {
		res, _ := p.playTwo(t, seed, 40)
		if res.Div != nil {
			voteChoicesFail(t, seed, res)
		}
		voteChoicesStop(t, seed, res)
		asks += countModeAsks(res)
	}
	if asks == 0 {
		t.Fatal("no villainous choice was asked in any seed")
	}
}
