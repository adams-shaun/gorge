package rules

// payment_plan_altface_restricted_pool_test.go pins ValidateCastPayment's
// settle for an alternate-spell-face cast whose witness spends restricted
// floating mana (Abstract Paintmage's "Spend this mana only to cast instant
// and sorcery spells"). The planner builds the spend pool with the object
// flipped to the Adventure face (a Sorcery), so the restriction admits it;
// the validator used to read the same pool at the live front face (an
// Artifact), so the pool came back empty and the witness the planner just
// built was rejected as "payment witness does not settle".

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

const altFaceRestrictedSrc = "Name:Rune Hammer\nManaCost:2\nTypes:Legendary Artifact Equipment\nK:Equip:2\n" +
	"AlternateMode:Adventure\nOracle:front\nALTERNATE\nName:Gleam Test\nManaCost:2 U\n" +
	"Types:Sorcery Adventure\nA:SP$ Draw | Num$ 1\nOracle:back\n"

func TestValidateCastPaymentAltFaceSpendsRestrictedPool(t *testing.T) {
	t.Parallel()
	e, _, id := newFixtureDeck(t, 7415, altFaceRestrictedSrc)
	onBoard(t, e, 0, ppMountain)
	toMain1(t, e)
	pl := &e.G.Players[0]
	pl.Pool[state.ManaIndex('U')]++
	pl.Pool[state.ManaIndex('R')]++
	pl.RestrictedMana = append(pl.RestrictedMana,
		state.ManaRestriction{Color: "U", Amount: 1, Valid: "Spell.Instant,Spell.Sorcery"},
		state.ManaRestriction{Color: "R", Amount: 1, Valid: "Spell.Instant,Spell.Sorcery"})
	ppPriority(t, e)
	alt := plannerPaymentAction(t, e.EnsurePaymentActions(), id, 1, "adventure_alt")
	if len(alt.Plans) == 0 {
		t.Fatalf("face action has no plan: %+v", alt)
	}
	// The precondition the assertion rests on: the witness really spends the
	// restricted floating mana (it names the Mountain for the rest).
	if spend := alt.Plans[0].PoolSpend; spend[state.ManaIndex('U')] != 1 || spend[state.ManaIndex('R')] != 1 {
		t.Fatalf("witness PoolSpend %v, want the restricted U and R", spend)
	}
	if err := e.ValidateCastPayment(0, alt.Cast, alt.Plans[0]); err != nil {
		t.Fatalf("ValidateCastPayment rejected the planner's own face witness: %v", err)
	}
}
