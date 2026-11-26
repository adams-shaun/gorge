package rules

// madness_granted_test.go — a madness keyword GRANTED by a continuous static
// (Falkenrath Gorger's "Each Vampire creature card you own that isn't on the
// battlefield has madness. The madness cost is equal to its mana cost",
// AddKeyword$ Madness:ManaCost) drives the same CR 702.35a-b mandatory
// discard-to-exile replacement and cast-or-not ask a printed K:Madness line
// does, priced at the card's own mana cost. The reader is the derived keyword
// list, exactly the read the granted Miracle cost takes (miracle.go).

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMadnessGrantedByStatic: a Vampire creature card discarded from its
// owner's hand while Falkenrath Gorger is on the battlefield is exiled (CR
// 702.35a), the madness ask is offered with a yes option priced at the
// card's mana cost, and the yes answer casts it (CR 702.35b).
func TestMadnessGrantedByStatic(t *testing.T) {
	t.Parallel()
	t.Run("discard exiles and offers the cast", func(t *testing.T) {
		e, cfg, _ := altCostEngine(t, 941, []string{"Vampire Interloper", "Falkenrath Gorger"},
			[]string{swampSrc, swampSrc}, nil)
		moveToBattlefieldByName(t, e, 0, "Falkenrath Gorger")
		moveToBattlefieldByName(t, e, 0, "Swamp")
		moveToBattlefieldByName(t, e, 0, "Swamp")
		id := findCardObj(t, e, 0, "Vampire Interloper", state.ZHand)
		// Precondition: the grant reaches the card's derived keyword list in
		// the hand, spelled AddKeyword$ Madness:ManaCost.
		if raw, ok := e.derivedKeywordParam(id, "Madness"); !ok || raw != "ManaCost" {
			t.Fatalf("precondition: derived madness param = %q,%v, want ManaCost", raw, ok)
		}
		e.pending = nil
		e.emit(events.Discard(id, 0))
		e.priorityRound()
		passOnceP(t, e)
		passOnceP(t, e)
		if z := e.G.Obj(id).Zone; z != state.ZExile {
			t.Fatalf("Vampire Interloper in %s after the discard, want exile (CR 702.35a)", z)
		}
		d := e.Pending()
		if d == nil || d.Kind != decision.KTriggerOptional || d.Source != id || d.ResumeKind != "madness" {
			t.Fatalf("madness resolution choice: %+v", d)
		}
		yes, no := optionIndexForKind(d, "yes"), optionIndexForKind(d, "no")
		if yes < 0 || no < 0 || len(d.Options) != 2 {
			t.Fatalf("madness ask options = %+v, want yes and no", d.Options)
		}
		if lab := d.Options[yes].Label; !strings.Contains(lab, "Cast Vampire Interloper for 1 B") {
			t.Fatalf("yes option label %q, want the card's own mana cost 1 B", lab)
		}
		submitChoices(t, e, yes)
		// Answer the cast's own asks (the payment window) until priority
		// returns; the payment taps both Swamps for the {1}{B}.
		for i := 0; i < 8; i++ {
			d := e.Pending()
			if d == nil || d.Kind == decision.KPriority {
				break
			}
			submitChoices(t, e, 0)
		}
		if z := e.G.Obj(id).Zone; z != state.ZStack {
			t.Fatalf("Vampire Interloper in %s after choosing yes, want stack (pending %+v)", z, e.Pending())
		}
		tapped := 0
		for _, sid := range e.G.Zone(state.ZBattlefield, 0) {
			if e.G.Obj(sid).Tapped {
				tapped++
			}
		}
		if tapped != 2 {
			t.Fatalf("%d Swamps tapped, want exactly 2 for {1}{B}", tapped)
		}
		replayCheck(t, e, cfg)
	})
	// Declining puts the card into its owner's graveyard (CR 702.35b).
	t.Run("decline goes to the graveyard", func(t *testing.T) {
		e, cfg, _ := altCostEngine(t, 942, []string{"Vampire Interloper", "Falkenrath Gorger"},
			[]string{swampSrc}, nil)
		moveToBattlefieldByName(t, e, 0, "Falkenrath Gorger")
		id := findCardObj(t, e, 0, "Vampire Interloper", state.ZHand)
		e.pending = nil
		e.emit(events.Discard(id, 0))
		e.priorityRound()
		passOnceP(t, e)
		passOnceP(t, e)
		d := e.Pending()
		if d == nil || d.ResumeKind != "madness" {
			t.Fatalf("madness resolution choice: %+v", d)
		}
		// One untapped Swamp cannot pay {1}{B} ({B} + {B}), so only the no
		// option is offered (the ask's own payability gate).
		if len(d.Options) != 1 || d.Options[0].Kind != "no" {
			t.Fatalf("madness ask options = %+v, want only no from one Swamp", d.Options)
		}
		submitChoices(t, e, optionIndexForKind(d, "no"))
		if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
			t.Fatalf("Vampire Interloper in %s after declining, want graveyard", z)
		}
		replayCheck(t, e, cfg)
	})
}
