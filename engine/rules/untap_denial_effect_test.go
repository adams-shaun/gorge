package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// krakenShapeSrc is a freely-authored sorcery with Icebreaker Kraken's Effect
// body (artifacts and creatures), never a corpus .txt.
const krakenShapeSrc = "Name:Kraken Shape\nManaCost:U\nTypes:Sorcery\nOracle:x\n" +
	"A:SP$ Effect | ValidTgts$ Opponent | StaticAbilities$ DontUntap | Triggers$ EndAtUntap | Duration$ Permanent | RememberObjects$ Targeted | SpellDescription$ x\n" +
	"SVar:DontUntap:Mode$ Continuous | AffectedZone$ Battlefield | Affected$ Artifact.RememberedPlayerCtrl,Creature.RememberedPlayerCtrl | AddHiddenKeyword$ This card doesn't untap during your next untap step.\n" +
	"SVar:EndAtUntap:Mode$ Phase | Phase$ Untap | ValidPlayer$ Player.IsRemembered | TriggerZones$ Command | Static$ True | Execute$ EndEffect\n" +
	"SVar:EndEffect:DB$ ChangeZone | Defined$ Self | Origin$ Command | Destination$ Exile\n"

const (
	udBear  = "Name:Ud Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	udRock  = "Name:Ud Rock\nManaCost:1\nTypes:Artifact\nOracle:x\n"
	udSwamp = "Name:Ud Marsh\nTypes:Land\nOracle:x\n"
)

// TestUntapDenialEffectsHoldTheNamedPermanents covers the Effect family whose
// StaticAbilities$ body is `Affected$ <types>.RememberedPlayerCtrl |
// AddHiddenKeyword$ This card doesn't untap during your next untap step.`
// (Misstep, Exhaustion, Mana Vapors, Icebreaker Kraken): the named
// permanents the targeted player controls stay tapped through that player's
// next untap step, everything else untaps, and the step after untaps all.
func TestUntapDenialEffectsHoldTheNamedPermanents(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	cases := []struct {
		name, spell            string
		heldCreature, heldLand bool
		heldArtifact           bool
	}{
		{"Misstep", "Misstep", true, false, false},
		{"Exhaustion", "Exhaustion", true, true, false},
		{"Mana Vapors", "Mana Vapors", false, true, false},
		{"Kraken shape", "Kraken Shape", true, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var spell *cards.Card
			if tc.spell == "Kraken Shape" {
				spell = card(t, krakenShapeSrc)
			} else {
				var ok bool
				if spell, ok = reg.Lookup(tc.spell); !ok {
					t.Fatalf("corpus fixture: %s missing", tc.spell)
				}
			}
			bear, rock, marsh := card(t, udBear), card(t, udRock), card(t, udSwamp)
			cfg := seatZeroStart(Config{Seed: 4243, Names: []string{"a", "b"},
				Decks: [][]*cards.Card{
					append([]*cards.Card{spell}, mountainDeck(t, 39)...),
					append([]*cards.Card{bear, rock, marsh}, mountainDeck(t, 37)...),
				},
				Tokens: map[string]*cards.Card{},
			})
			e := New(cfg)
			e.Advance()
			spellID := moveByName(t, e, 0, tc.spell, state.ZHand)
			bearID := moveByName(t, e, 1, "Ud Bear", state.ZBattlefield)
			rockID := moveByName(t, e, 1, "Ud Rock", state.ZBattlefield)
			marshID := moveByName(t, e, 1, "Ud Marsh", state.ZBattlefield)
			ids := []state.ObjID{bearID, rockID, marshID}
			addMana(t, e, 0, "CCCU")
			submitChoices(t, e, castOptionFor(t, e, spellID).Index)
			if d := e.Pending(); d != nil && d.Kind == decision.KTarget {
				pick := -1
				for _, o := range d.Options {
					if o.Player == 1 && o.Obj == 0 {
						pick = o.Index
					}
				}
				if pick < 0 {
					t.Fatalf("no player-1 target offered: %+v", d.Options)
				}
				submitChoices(t, e, pick)
			}
			passUntilStackEmpty(t, e, 40)
			for _, id := range ids {
				e.emit(events.Event{Kind: events.Tap, Obj: id})
			}
			want := []bool{tc.heldCreature, tc.heldArtifact, tc.heldLand}
			driveToStepAny(t, e, 2, 1, state.StepUpkeep)
			for i, id := range ids {
				if got := e.G.Obj(id).Tapped; got != want[i] {
					t.Errorf("%s: permanent %d tapped after the first untap step = %v, want %v", tc.name, i, got, want[i])
				}
			}
			driveToStepAny(t, e, 4, 1, state.StepUpkeep)
			for i, id := range ids {
				if e.G.Obj(id).Tapped {
					t.Errorf("%s: permanent %d never untapped at the following untap step", tc.name, i)
				}
			}
		})
	}
}
