package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// kcrimeLower lowers a synthetic card body (lines after PT:) and returns its
// refusals.
func kcrimeLower(t *testing.T, body string) []refusal {
	t.Helper()
	src := "Name:Crime Probe\nManaCost:1 R\nTypes:Creature Human\nPT:1/1\n" + body + "\n"
	card, diags := cards.ParseBytes("crime_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{"Crime Probe"}, byName: map[string]*cards.Card{"Crime Probe": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lw := lowerPool(p)
	if _, _, _, err := buildPool(lw); err != nil {
		t.Fatal(err)
	}
	return lw.cards[0].Refusals
}

const kcrimeTrig = "T:Mode$ CommitCrime | ValidPlayer$ You | TriggerZones$ Battlefield | Execute$ ProbeDraw\nSVar:ProbeDraw:DB$ Draw | NumCards$ 1"

// TestCrimeRefusals: trig:CommitCrime, Count$CommittedCrimeThisTurn and
// CheckDefinedPlayer$ You.committedCrimeThisTurn lower in their gorge-read
// shapes only; every other shape is refused with a named reason.
func TestCrimeRefusals(t *testing.T) {
	for _, ok := range []string{
		kcrimeTrig,
		strings.Replace(kcrimeTrig, "ValidPlayer$ You", "ValidPlayer$ Opponent", 1),
		strings.Replace(kcrimeTrig, "TriggerZones$ Battlefield", "TriggerZones$ Graveyard", 1),
		strings.Replace(kcrimeTrig, "Execute$ ProbeDraw", "Execute$ ProbeDraw | CheckDefinedPlayer$ You.committedCrimeThisTurn", 1),
		"S:Mode$ ReduceCost | ValidCard$ Card.Self | Type$ Spell | Amount$ 1 | EffectZone$ All | CheckSVar$ Count$CommittedCrimeThisTurn.1.0",
	} {
		if r := kcrimeLower(t, ok); len(r) != 0 {
			t.Fatalf("an accepted shape is refused: %v\n%s", r, ok)
		}
	}
	cases := []struct{ name, body, want string }{
		{"ValidPlayer", strings.Replace(kcrimeTrig, "ValidPlayer$ You", "ValidPlayer$ Player.Foo", 1), "CommitCrime ValidPlayer"},
		{"TriggerZones Exile", strings.Replace(kcrimeTrig, "TriggerZones$ Battlefield", "TriggerZones$ Exile", 1), "CommitCrime TriggerZones"},
		{"TriggerZones Hand", strings.Replace(kcrimeTrig, "TriggerZones$ Battlefield", "TriggerZones$ Hand", 1), "CommitCrime TriggerZones"},
		{"TriggerZones Command", strings.Replace(kcrimeTrig, "TriggerZones$ Battlefield", "TriggerZones$ Command", 1), "CommitCrime TriggerZones"},
		{"Effect trigger", "A:AB$ Effect | Cost$ 1 | Triggers$ CrimeT | Duration$ Permanent | Name$ Crime Probe Effect\nSVar:CrimeT:Mode$ CommitCrime | ValidPlayer$ You | Execute$ ProbeDraw\nSVar:ProbeDraw:DB$ Draw | NumCards$ 1", "CommitCrime trigger on an Effect"},
		{"unread ValidCard", strings.Replace(kcrimeTrig, "ValidPlayer$ You", "ValidPlayer$ You | ValidCard$ Creature", 1), "unread params"},
		{"arms one", "S:Mode$ ReduceCost | ValidCard$ Card.Self | Type$ Spell | Amount$ 1 | EffectZone$ All | CheckSVar$ Count$CommittedCrimeThisTurn.1", "Count$CommittedCrimeThisTurn arms"},
		{"arms svar", "S:Mode$ ReduceCost | ValidCard$ Card.Self | Type$ Spell | Amount$ 1 | EffectZone$ All | CheckSVar$ Count$CommittedCrimeThisTurn.1.Z\nSVar:Z:Count$Morbid.1.0", "Count$CommittedCrimeThisTurn arms"},
		{"CheckDefinedPlayer monarch", strings.Replace(kcrimeTrig, "Execute$ ProbeDraw", "Execute$ ProbeDraw | CheckDefinedPlayer$ Player.isMonarch", 1), `CheckDefinedPlayer "Player.isMonarch"`},
		{"CheckDefinedPlayer opponent", strings.Replace(kcrimeTrig, "Execute$ ProbeDraw", "Execute$ ProbeDraw | CheckDefinedPlayer$ Opponent.committedCrimeThisTurn", 1), `CheckDefinedPlayer "Opponent.committedCrimeThisTurn"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rs := kcrimeLower(t, c.body)
			if len(rs) == 0 {
				t.Fatal("accepted, want a refusal")
			}
			for _, r := range rs {
				if strings.Contains(r.Reason, c.want) {
					return
				}
			}
			t.Fatalf("refusals %v lack %q", rs, c.want)
		})
	}
}
