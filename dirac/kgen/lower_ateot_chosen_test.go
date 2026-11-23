package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestAnimateAtEOTChosenCard: an Animate's AtEOT$ on Defined$ Remembered
// lowers when the sole rememberer is a one-card ChooseMove (a hand pick:
// Arms Race, Incandescent Soulstoke, Meek Attack, Sneak Attack) or a Dig of
// one card (Impromptu Raid, Killer Instinct) — the pick remembers at most
// one card, which is the one object the kernel's OpAtEOT takes. A Dig of
// more than one, a second rememberer beside the pick and a ClearRemembered
// before the pick still refuse (Animate AtEOT on a set).
func TestAnimateAtEOTChosenCard(t *testing.T) {
	t.Run("the six carriers lower", func(t *testing.T) {
		for _, c := range lowerNamed(t, []string{
			"Arms Race", "Impromptu Raid", "Incandescent Soulstoke",
			"Killer Instinct", "Meek Attack", "Sneak Attack",
		}) {
			for _, r := range c.Refusals {
				t.Errorf("%s: refused: %s: %s", c.Name, r.Node, r.Reason)
			}
			var sb strings.Builder
			dumpLIR(&sb, c)
			if !strings.Contains(sb.String(), "AtEOT obj=Remembered") {
				t.Errorf("%s: lowered IR lacks AtEOT obj=Remembered:\n%s", c.Name, sb.String())
			}
		}
	})
	animate := "DB$ Animate | Defined$ Remembered | AtEOT$ Sacrifice"
	pick := "DB$ ChangeZone | Origin$ Hand | Destination$ Battlefield | ChangeType$ Creature.YouCtrl | Optional$ True | RememberChanged$ True"
	t.Run("Dig of two refuses", func(t *testing.T) {
		got := kdefRefusals(t,
			"AB$ Dig | Cost$ 2 RG | DigNum$ 2 | Reveal$ True | ChangeNum$ All | ChangeValid$ Creature | DestinationZone$ Battlefield | RememberChanged$ True | SubAbility$ DBX",
			animate)
		kdefWant(t, got, "Animate AtEOT on a set")
	})
	t.Run("second rememberer refuses", func(t *testing.T) {
		got := ateotSynth(t,
			"AB$ ChangeZone | Cost$ R | Origin$ Hand | Destination$ Battlefield | ChangeType$ Creature.YouCtrl | Optional$ True | RememberChanged$ True | SubAbility$ DBM",
			map[string]string{"DBM": pick + " | SubAbility$ DBA", "DBA": animate})
		kdefWant(t, got, "Animate AtEOT on a set")
	})
	t.Run("ClearRemembered before the pick refuses", func(t *testing.T) {
		got := ateotSynth(t,
			"AB$ ChangeZone | Cost$ R | Origin$ Hand | Destination$ Battlefield | ChangeType$ Creature.YouCtrl | Optional$ True | RememberChanged$ True | SubAbility$ DBC",
			map[string]string{
				"DBC": "DB$ Cleanup | ClearRemembered$ True | SubAbility$ DBM",
				"DBM": pick + " | SubAbility$ DBA",
				"DBA": animate,
			})
		kdefWant(t, got, "Animate AtEOT on a set")
	})
}

// ateotSynth lowers a synthetic ability whose SubAbility$ chain lives in
// svars (ResolveSVar links it) and returns its refusal reasons.
func ateotSynth(t *testing.T, root string, svars map[string]string) []string {
	t.Helper()
	svars["R"] = root
	r := cards.ResolveSVar(svars, "R")
	if r == nil {
		t.Fatalf("root %q did not resolve", root)
	}
	face := &cards.Face{Name: "Synthetic", ManaCost: "B", Types: []string{"Sorcery"}, SVars: svars, Abilities: []*cards.SA{r}}
	card := &cards.Card{Faces: []*cards.Face{face}}
	p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	var out []string
	for _, rf := range lowerCard(p, "Synthetic", card, false).Refusals {
		out = append(out, rf.Reason)
	}
	return out
}
