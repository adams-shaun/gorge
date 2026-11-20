package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// CR 602.2b: the twin cut mirrors gorge's potential play kind "ability" for
// the two populations dirac can tell apart by encoding: a printed activation
// (no grant anchor) and a gained one (GainedSource, the 0xC0 pair). An SVar
// grant (Kind "ability" with SVar and GrantSource) and a max-speed or exile
// grant (Kind "granted") are both AkActivate{0x80|ability}, which is why
// gorge's potentialTwinCut narrows to the printed/gained option shapes: the
// option shapes below are the ones legal_walk_battlefield.go's offer loops
// append (printed at :485, gained at :720, SVar grant at :760 and the
// off-battlefield grant walk) and the max-speed grant at :862. The pin is
// two-sided: the gorge option's shape and dirac's encoding must classify the
// same way.
func TestIsAbilityPlayMirrorsGorgeKind(t *testing.T) {
	type shape struct {
		name string
		opt  decision.Option // the option gorge's offer walk appends
		act  Action          // dirac's encoding of the same play
		cut  bool
	}
	shapes := []shape{
		{"printed", decision.Option{Kind: "ability", Obj: 7, Ability: 0}, MakeAction(AkActivate, 1, 0, 0), true},
		{"printed-ability-3", decision.Option{Kind: "ability", Obj: 7, Ability: 3, AltCostIndex: 1}, MakeAction(AkActivate, 1, 0, 3), true},
		{"gained", decision.Option{Kind: "ability", Obj: 7, GainedSource: 13, GainedIdx: 1}, MakeAction(AkActivate, 1, 2, gainedMode|3), true},
		{"svar-grant", decision.Option{Kind: "ability", Obj: 7, SVar: "ABMana", GrantSource: 9}, MakeAction(AkActivate, 1, 2, 0x80|3), false},
		{"off-battlefield-grant", decision.Option{Kind: "ability", Obj: 7, SVar: "ABMana", GrantSource: 11}, MakeAction(AkActivate, 1, 2, 0x80|3), false},
		{"max-speed-grant", decision.Option{Kind: "granted", Obj: 7, SVar: "ABMana"}, MakeAction(AkActivate, 1, 2, 0x80|3), false},
		{"cast", decision.Option{Kind: "cast", Obj: 7}, MakeAction(AkCast, 1, 0, 0), false},
		{"pass", decision.Option{Kind: "pass"}, MakeAction(AkPass, 0, 0, 0), false},
	}
	// Preconditions: an SVar grant is Kind "ability" exactly like a printed
	// one and carries both anchors, while dirac gives it the same 0x80 mode
	// as a max-speed grant (which is a different Kind). That collision is the
	// reason the cut cannot key on Kind alone.
	svarGrant, maxSpeed := shapes[3], shapes[5]
	if svarGrant.opt.Kind != shapes[0].opt.Kind || svarGrant.opt.SVar == "" || svarGrant.opt.GrantSource == 0 {
		t.Fatalf("precondition: the SVar grant shape is a Kind %q option with non-empty anchors: %+v", shapes[0].opt.Kind, svarGrant.opt)
	}
	if maxSpeed.opt.Kind == svarGrant.opt.Kind {
		t.Fatalf("precondition: an SVar grant is Kind %q and a max-speed grant Kind %q; they must differ", svarGrant.opt.Kind, maxSpeed.opt.Kind)
	}
	if svarGrant.act.Mode() != maxSpeed.act.Mode() {
		t.Fatalf("precondition: dirac encodes the SVar grant %#x and the max-speed grant %#x; they must collide", svarGrant.act.Mode(), maxSpeed.act.Mode())
	}
	gained := shapes[2]
	if gained.act.Mode() == svarGrant.act.Mode() {
		t.Fatalf("precondition: gained %#x and granted %#x modes must differ", gained.act.Mode(), svarGrant.act.Mode())
	}
	for _, tc := range shapes {
		// Gorge's side, derived from the option shape potentialTwinCut reads.
		gorgeCut := tc.opt.Kind == "ability" && (tc.opt.GainedSource != 0 || (tc.opt.SVar == "" && tc.opt.GrantSource == 0))
		if gorgeCut != tc.cut {
			t.Errorf("%s: gorge option %+v is cut=%v, want %v", tc.name, tc.opt, gorgeCut, tc.cut)
		}
		if got := isAbilityPlay(tc.act); got != tc.cut {
			t.Errorf("%s: isAbilityPlay = %v, want %v", tc.name, got, tc.cut)
		}
	}
}
