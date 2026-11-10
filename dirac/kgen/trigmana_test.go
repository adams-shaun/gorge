package kgen

import (
	"strings"
	"testing"
)

// The triggered artifact mana census: the only two
// accepted cards whose triggered ability made fixed mana from an artifact
// source -- Myr Moonvessel's dies trigger and The Vision and Scarlet Witch's
// spell-cast trigger -- diverged on both lockstep surfaces. gorge records an
// artifact source's mana as typed (effects.ManaProducerTag) and
// pay.PlanPoolOK refuses a pool holding typed mana; the kernel's pool keeps
// no typed bit. The whole class is refused at the Mana lowering.
var trigmanaForms = []formCase{
	{card: "Myr Moonvessel", form: "an artifact's triggered fixed mana is typed: refused", refused: "typed mana"},
	{card: "The Vision and Scarlet Witch", form: "an artifact's triggered fixed mana is typed: refused", refused: "typed mana"},
	{card: "Burning-Tree Emissary", form: "a nonartifact source's triggered fixed mana still lowers", lir: "AddMana obj=You A=1 CS=R"},
}

func TestTrigmanaForms(t *testing.T) { checkForms(t, trigmanaForms) }

// a spell's DB$ Mana resolves like a trigger's (gorge
// effMana is the one resolver; a spell is not a mana ability, CR 605.1a), so
// the three Standard carriers lower to AddMana. An AB$ whose sub-ability adds
// mana lowers only when it is no mana ability (loyalty, targeted or
// library-moving) and stays refused otherwise (CR 605.1a), a
// Defined$ that names a seat other than You stays refused (gorge
// ManaRecipients), and a spend restriction on the produced mana is a
// separate shape, lowered when every alternative is a Spell.
// one (Desolation of Smaug).
var dbmanaForms = []formCase{
	{card: "Konstrari Charm", form: "a charm mode's fixed DB$ Mana (Produced$ C Amount$ 3) lowers", lir: "AddMana obj=You A=3"},
	{card: "Rapturous Moment", form: "a spell chain's fixed DB$ Mana (Produced$ U U R R R) lowers", lir: "AddMana obj=You A=1"},
	{card: "Narset's Rebuke", form: "a spell chain's defined-You DB$ Mana lowers", lir: "AddMana obj=You A=1"},
	{card: "Soulbright Seeker", form: "a targeted AB$ Pump with a DB$ Mana sub is admitted, and its Count$ResolvedThisTurn X counts the activated ability's resolutions", lir: "AddMana obj=You A=4"},
	{card: "Sacellum Godspeaker", form: "gorge finding: a CR 605.1a mana ability gorge resolves on the stack stays refused (cards/ability_class.go L48)", refused: "DB$ Mana under a CR 605.1a mana ability gorge resolves on the stack (gorge cards/ability_class.go L48)"},
	{card: "Priest of Forgotten Gods", form: "an any-number-of-target-players chain lowers now that the multi-target Sacrifice asks each player", lir: "AddMana obj=You A=2"},
	{card: "Red Death, Shipwrecker", form: "a targeted AB$ Goad with a library-moving Draw and a DB$ Mana sub lowers", lir: "AddMana obj=You A=1"},
	{card: "Hulking Raptor", form: "Defined$ TriggeredPlayer under a your-Phase trigger is the controller", lir: "AddMana obj=You A=2"},
	{card: "Quintorius Kand", form: "a loyalty ability's DB$ Mana sub resolves on the stack", lir: "AddMana"},
	{card: "Desolation of Smaug", form: "a restricted colour-choice DB$ Mana (RestrictValid$ Spell.Dragon) lowers", lir: "ManaChoice"},
	{card: "Abstract Paintmage", form: "a restricted fixed-colour DB$ Mana (RestrictValid$ Spell.Instant,Spell.Sorcery) lowers", lir: "AddMana obj=You A=1"},
	{card: "Smoky Lounge // Misty Salon", form: "a Room's DB$ Mana (Defined$ TriggeredPlayer) lowers its Static.Unlock and Spell.Room restriction", lir: "AddMana obj=You A=2 CS=R F={types=0x0 sub=Room | types=0x0 special=0x10}"},
}

func TestDbmanaForms(t *testing.T) { checkForms(t, dbmanaForms) }

// TestManaEffectDefinedOpponentRefused: only Defined$ You names the same seat
// the no-Defined$ case does (gorge ManaRecipients' activating player); every
// other selector stays refused. The corpus has no literal Defined$ Opponent
// carrier, so the refusal is exercised directly.
func TestManaEffectDefinedOpponentRefused(t *testing.T) {
	lc := &lowerCtx{ab: &lAbility{Kind: akSpell}, out: &lCard{}}
	ps := newParams(map[string]string{"Produced": "U", "Defined": "Opponent"})
	err := lc.manaEffect(ps)
	if err == nil || !strings.Contains(err.Error(), "DB$ Mana Defined$") {
		t.Fatalf("DB$ Mana Defined$ Opponent: got %v, want DB$ Mana Defined$", err)
	}
}

// a loyalty ability's fixed-colour AB$ Mana with
// RestrictValid$ Spell.<spec> adds its mana on resolution (CR 605.1a) as
// restricted mana (CR 106.6): the AddMana carries the filter, which the
// kernel floats in State.rb (mana_resolverestriction.go).
var chandraLoyaltyForms = []formCase{
	{card: "Chandra, Chill of Compliance", form: "a loyalty AB$ Mana with RestrictValid$ Spell.nonCreature lowers to a filtered AddMana", lir: "AddMana obj=You A=1 CS=U F={types=0x0 not=0x2}"},
}

func TestChandraLoyaltyManaRestr(t *testing.T) { checkForms(t, chandraLoyaltyForms) }

// TestManaEffectRestrictedAnyRefused: Ashling, Rimebound's restricted
// Produced$ Any (Amount$ 2, Spell.cmcGE4) stays refused: its restricted units
// float beside the flip trigger's pay cost and paid abilities, where gorge's
// offers differ from the kernel's (report ## Gorge finding).
// A restricted Combo Any lowers (Desolation of Smaug, dbmanaForms).
func TestManaEffectRestrictedAnyRefused(t *testing.T) {
	lc := &lowerCtx{ab: &lAbility{Kind: akTriggered}, out: &lCard{}}
	ps := newParams(map[string]string{"Produced": "Any", "Amount": "2", "RestrictValid": "Spell.cmcGE4"})
	err := lc.manaEffect(ps)
	if err == nil || !strings.Contains(err.Error(), "DB$ Mana RestrictValid$ over Produced$ Any") {
		t.Fatalf("restricted Produced$ Any: got %v, want the named refusal", err)
	}
}
