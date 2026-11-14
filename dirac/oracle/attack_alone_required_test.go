package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/decision"
)

// CR 508.1d with CR 506.5: Toby, Beastie Befriender's Beast token can't attack
// alone and wears the opponent's Bloodthirsty Blade ("attacks each combat if
// able"), and its controller's only other creatures are Patagia Viper's
// summoning-sick Snakes. Declaring the Beast alone breaks the restriction, so
// the requirement doesn't bind and the empty declaration is the legal answer.
// The gorge side once demanded the Beast in every answer and, with {Beast}
// refused as an attack alone, posed an attackers decision with no options
// (decision 415 of 615; dr-mbc-aa pair 811 before MBC was split).
func TestMBCLoneRequiredAttackerReprosLockstep(t *testing.T) {
	t.Parallel()
	replayRepro(t, synthReg(t), cardsettest.Set(t, "mbc"), "lone_required_attacker_cant_attack_alone", true, false)
}

// requiredCovered reads the same quota gorge's validateAttackers does: a lone
// Required attacker that can't attack alone binds nothing, while the same
// option without the restriction still has to be declared.
func TestRequiredCoveredLoneAloneRestricted(t *testing.T) {
	t.Parallel()
	frameFor := func(alone bool) *frame {
		d := &decision.Decision{Kind: decision.KAttackers, Min: 0, Max: 1, Options: []decision.Option{
			{Index: 0, Kind: "attacker", Obj: 7, Player: 1, Required: true, AloneRestricted: alone},
		}}
		return &frame{d: d, valid: []bool{true}}
	}
	if requiredCovered(frameFor(false), nil) {
		t.Fatal("precondition: an unrestricted Required attacker left out must not be covered")
	}
	if q := frameFor(true).d.RequiredQuota(); q != 0 {
		t.Fatalf("precondition: gorge's RequiredQuota for a lone alone-restricted attacker = %d, want 0", q)
	}
	if !requiredCovered(frameFor(true), nil) {
		t.Fatal("the empty declaration must be complete when the lone required attacker can't attack alone")
	}
}
