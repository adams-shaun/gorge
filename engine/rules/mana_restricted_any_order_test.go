package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// White Lotus Hideout's shape (std-tla): a land whose first mana ability makes
// {C} and whose second makes one mana of any colour restricted to Lesson and
// Shrine spells. AnnouncedAbilityColours keeps a RestrictValid$ ability a
// single manual option, so owedAbilitiesFirst could not see the colours it
// makes. A {U} Lesson cast with only this land untapped therefore listed
// "Add C" first; the deterministic first-option answer produced colourless
// mana, the {U} pip stranded, and the cast reversed (CR 601.2g).
const restrictedAnyOrderLotusSrc = `Name:Order Lotus
ManaCost:no cost
Types:Land
A:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ One colourless for the Order.
A:AB$ Mana | Cost$ T | Produced$ Any | RestrictValid$ Spell.Lesson | SpellDescription$ Any colour, but only for an Order Lesson.
Oracle:x
`

const restrictedAnyOrderLessonSrc = `Name:Order Lesson
ManaCost:U
Types:Instant Lesson
A:SP$ Draw | NumCards$ 1
Oracle:x
`

// TestRestrictedAnyColourAbilitySortsBeforeUselessFixed pins the ordering: the
// restriction admits this Lesson, so its any-colour ability owes {U} and must
// be the first option, ahead of the {C} ability the payment cannot spend. It
// then plays the first option through the colour ask and proves the cast
// completes without the CR 601.2g abort.
func TestRestrictedAnyColourAbilitySortsBeforeUselessFixed(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, restrictedAnyOrderLessonSrc))
	spell := e.G.Zone(state.ZHand, 0)[0]
	lotus := battlePerm(t, e, 0, restrictedAnyOrderLotusSrc)
	e.cast = &pendingCast{player: 0, card: spell, from: state.ZHand, ability: -1,
		cost: ParseCost("U")}
	e.continueCast()

	d := e.Pending()
	if d == nil || !strings.HasPrefix(d.Prompt, "Activate mana abilities to pay") {
		t.Fatalf("want the payment window, got %+v", d)
	}
	// Precondition: the fixed {C} ability is on the source and cannot pay the
	// {U} pip, while the restricted ability can -- otherwise the ordering
	// assertion below proves nothing.
	mas := e.availableManaAbilities(0, lotus)
	if len(mas) != 2 {
		t.Fatalf("precondition: Order Lotus mana abilities = %d, want 2", len(mas))
	}
	if owed, _ := e.paymentUsefulColours(e.cast, 0, []string{"C"}); len(owed) != 0 {
		t.Fatalf("precondition: payment owes {C}: %v", owed)
	}
	if owed, _ := e.paymentUsefulColours(e.cast, 0, []string{"U"}); len(owed) == 0 {
		t.Fatal("precondition: payment does not owe {U}")
	}
	submitIdx(t, e, d, optionByLabel(d.Options, "Activate Order Lotus for mana"))

	d = e.Pending()
	if d == nil || !strings.HasPrefix(d.Prompt, "Choose a mana ability of Order Lotus") {
		t.Fatalf("want the mana ability choice, got %+v", d)
	}
	if cIdx := optionByLabel(d.Options, "Add C"); cIdx == 0 {
		t.Fatalf("option 0 is the useless {C} ability: %+v", d.Options)
	}
	if d.Options[0].Label != "Add any color" {
		t.Fatalf("option 0 = %q, want the restricted any-colour ability: %+v", d.Options[0].Label, d.Options)
	}
	submitIdx(t, e, d, 0)

	d = e.Pending()
	if d == nil || !strings.HasPrefix(d.Prompt, "Choose a colour of mana") {
		t.Fatalf("want the colour ask, got %+v", d)
	}
	if len(d.Options) != 1 || d.Options[0].Label != "Add U" {
		t.Fatalf("colour ask options = %+v, want [Add U]", d.Options)
	}
	submitIdx(t, e, d, 0)

	passUntilStackEmpty(t, e, 60)
	if hasNote(e, "aborted") {
		t.Fatal("the cast aborted although the first mana-ability option pays the pip")
	}
	if o := e.G.Obj(spell); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("Order Lesson zone = %v, want the graveyard after resolving", o.Zone)
	}
}
