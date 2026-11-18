package kgen

import (
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestLowerRadiation: every card of the rad-counter class (api:Radiation,
// gorge effRadiation) lowers its Radiation node to OpExt ExtRadiation. A card
// that stays refused is refused only for its other, measured reasons; the one
// Radiation reason left is Mariposa Military Base's, whose rad counters ride
// an optional enters-tapped replacement's "if you do" chain the replacement
// lowering does not run.
func TestLowerRadiation(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, c := range []struct {
		name string
		left []string // the measured reasons the card keeps (none: lowered)
	}{
		{"Feral Ghoul", nil},
		{"Contaminated Drink", nil},
		{"Nuclear Fallout", nil},
		{"Megaton's Fate", nil},
		// Their Radiation lowers; what refuses now is the node's other read,
		// which the api's old refusal hid.
		{"Struggle for Project Purity", []string{`DB$ Radiation: SVar X="TriggerObjectsAttackers$Amount/Times.2": TriggerObjectsAttackers$Amount outside a batch AttackersDeclared trigger`}},
		{"Acquired Mutation", []string{`DB$ Radiation: Defined$ "TriggeredDefendingPlayer" outside an attack trigger of the controller's own attack`}},
		{"The Wise Mothman", []string{"trigger mode MilledAll"}},
		{"Screeching Scorchbeast", []string{"trigger mode MilledAll"}},
		{"Mirelurk Queen", []string{"trigger mode MilledAll"}},
		{"Glowing One", []string{"trigger mode Milled"}},
		{"Infesting Radroach", []string{"TriggerZones$ Graveyard over Mode$ Milled: the kernel's walk for it reads no graveyard"}},
		{"Tato Farmer", []string{`AB$ ChangeZone: filter "Land.milledThisTurn": "Land.milledThisTurn": unknown property "milledThisTurn"`}},
		{"The Master, Transcendent", []string{"AB$ ChangeZone: StaticEffect unread [RemoveCreatureTypes SetColor]"}},
		{"Nightkin Ambusher", []string{"static unread params [ValidDefender]"}},
		{"Vault 12: The Necropolis", []string{`DB$ Token: SVar X="PlayerCountPlayers$Counters.RAD": PlayerCountPlayers$Counters.RAD`}},
		{"Mariposa Military Base", []string{"ETB replacement api Radiation", `SVar X="Count$YourCountersRAD": Count$YourCountersRAD`}},
	} {
		card, ok := reg.Lookup(c.name)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", c.name)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		lc := lowerCard(p, card.Faces[0].Name, card, false)
		var got []string
		for _, r := range lc.Refusals {
			got = append(got, r.Reason)
		}
		sort.Strings(got)
		want := append([]string(nil), c.left...)
		sort.Strings(want)
		if strings.Join(got, " | ") != strings.Join(want, " | ") {
			t.Errorf("%s: refusals %q, want %q", c.name, got, want)
		}
		if len(c.left) != 0 {
			continue
		}
		if n := radiationOps(lc); n == 0 {
			t.Errorf("%s: no OpExt ExtRadiation lowered", c.name)
		}
	}
}

// radiationOps counts the Radiation instructions a lowered card carries.
func radiationOps(lc *lCard) int {
	n := 0
	for _, ab := range lc.Abilities {
		for _, in := range ab.Prog {
			if in.Op == "Radiation" {
				n++
			}
		}
	}
	return n
}
