package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// squadReplicateCards is the ticket's class: every corpus card blocked only
// by K:Squad / K:Replicate and the Count$SquadPaid / Count$ReplicatePaid
// reads the keyword expansions add.
var squadReplicateCards = []string{
	"Arco-Flagellant", "Changing Loyalty", "Exterminate!", "Galadhrim Brigade",
	"Gary Clone", "Gigadrowse", "Imperial Hullbreachers", "Leap of Flame",
	"Lose Focus", "Powder Ganger", "Pyromatics", "Reiterating Bolt",
	"Roadkill Rodney", "Ruthless Radrat", "Securitron Squadron", "Shattering Spree",
	"Sicarian Infiltrator", "Siege of Towers", "Space Marine Devastator", "Super Combo",
	"Thrill-Kill Disciple", "Thunderheads", "Train of Thought", "Ultramarines Honour Guard",
	"Vacuumelt", "Vanguard Suppressor", "Wasteland Raider", "Zephyrim",
}

// TestSquadReplicateLowers: every card of the class lowers its keyword's
// per-payment cost onto the CardDef's multikicker slot with the matching
// kind tag, including the four non-mana costs among these 28
// (Exterminate!'s tapXType, Reiterating Bolt's PayEnergy, Ruthless Radrat's
// ExileFromGrave, Thrill-Kill Disciple's Discard; Psionic Ritual is the
// fifth carrier outside this class). A card carrying two of the three
// repeated additional costs refuses by name.
func TestSquadReplicateLowers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, name := range squadReplicateCards {
		t.Run(name, func(t *testing.T) {
			lc := lowerRepeatedCard(t, reg, name)
			for _, rf := range lc.Refusals {
				t.Errorf("refused: %s", rf.Reason)
			}
			if lc.RepeatedKind != kop.RepeatSquad && lc.RepeatedKind != kop.RepeatReplicate {
				t.Fatalf("RepeatedKind = %d, want Squad or Replicate", lc.RepeatedKind)
			}
			if lc.Multikicker == nil {
				t.Fatal("lowered without the per-payment mana slot")
			}
			_, all, _, err := buildPool(&lowered{cards: []*lCard{lc}})
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			d := &all[0].def
			if !d.HasMultikicker || d.RepeatedKind != lc.RepeatedKind {
				t.Fatalf("CardDef HasMultikicker=%v RepeatedKind=%d, want true/%d", d.HasMultikicker, d.RepeatedKind, lc.RepeatedKind)
			}
		})
	}

	const two = "Name:Two Repeated Probe\nManaCost:2 U\nTypes:Creature\nK:Squad:2\nK:Replicate:1 U\n"
	card, diags := cards.ParseBytes("two_repeated_probe.txt", []byte(two))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	const name = "Two Repeated Probe"
	p := &pool{names: []string{name}, byName: map[string]*cards.Card{name: card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lc := lowerPool(p).cards[0]
	found := false
	for _, rf := range lc.Refusals {
		if strings.Contains(rf.Reason, "two repeated additional costs") {
			found = true
		}
	}
	if !found {
		t.Fatalf("two repeated costs: refusals %v, want the two-repeated-costs reason", lc.Refusals)
	}
}

func lowerRepeatedCard(t *testing.T, reg *cards.Registry, name string) *lCard {
	t.Helper()
	card, ok := reg.Lookup(name)
	if !ok || card == nil {
		t.Fatalf("%s not in the corpus", name)
	}
	p := &pool{names: []string{name}, byName: map[string]*cards.Card{name: card}, nDecks: map[string]int{}, reg: reg}
	return lowerPool(p).cards[0]
}
