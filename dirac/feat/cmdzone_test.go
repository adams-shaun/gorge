package feat

import (
	"testing"

	"github.com/adams-shaun/gorge/bots/policynet"
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/view"
)

// commanderZoneDecision is the owner's two-option CR 903.9a ask: mode 0 into
// the command zone, mode 1 leave it.
func commanderZoneDecision() dirac.Decision {
	d := dirac.Decision{Kind: dirac.DkChoose, Sub: dirac.ChCommanderZone, N: 2}
	d.Options[0] = dirac.MakeAction(dirac.AkChoose, 0, 0, 0)
	d.Options[1] = dirac.MakeAction(dirac.AkChoose, 0, 0, 1)
	return d
}

// TestCommanderZoneFeatMatchesPolicynet: gorge's KCommanderZone options are
// "command_zone" and "leave", neither in policynet's measured 23-name
// vocabulary. Policynet therefore sets its "other" option-kind slot and
// hashes "optkind|<kind>"; the kernel's option-kind mapping must agree.
func TestCommanderZoneFeatMatchesPolicynet(t *testing.T) {
	d := commanderZoneDecision()
	e := &Encoder{}
	for i, want := range []struct {
		mode uint8
		kind string
		ok   uint8
	}{
		{0, "command_zone", okCommandZone},
		{1, "leave", okLeave},
	} {
		o := e.optionOf(&d, d.Options[i])
		if o.dk != dkCommanderZone || o.kind != want.ok {
			t.Fatalf("option %d encoded dk %d kind %d, want dkCommanderZone and kind %d", i, o.dk, o.kind, want.ok)
		}
		k := kinds[o.kind]
		if k.slot != len(optionKinds) {
			t.Fatalf("%q: kernel slot %d, want policynet's other slot %d", want.kind, k.slot, len(optionKinds))
		}
		var v view.View
		g := policynet.EncodeOptionsWith(policynet.FeaturesEntity, &v, 0, decision.KCommanderZone,
			[]decision.Option{{Kind: want.kind}}, nil)
		var other, yes, no bool
		for _, s := range g[0].Slots {
			switch s.Row {
			case uint16(optKindOffset + len(optionKinds)):
				other = true
			case uint16(optKindOffset + int(okYes)):
				yes = true
			case uint16(optKindOffset + int(okNo)):
				no = true
			}
		}
		if !other || yes || no {
			t.Fatalf("%q: policynet slots other=%v yes=%v no=%v, want the other slot only", want.kind, other, yes, no)
		}
		if len(g[0].Hashed) != 1 || g[0].Hashed[0].Row != k.row {
			t.Fatalf("%q: kernel optkind row %#x, policynet hashed %v", want.kind, k.row, g[0].Hashed)
		}
	}
}

// TestCommanderZoneFeatListPos: gorge lists command_zone before leave, so the
// two options rank 0 and 1 (not both 0 by sharing the commander's zone key).
func TestCommanderZoneFeatListPos(t *testing.T) {
	d := commanderZoneDecision()
	if d.Options[0].Source() != d.Options[1].Source() {
		t.Fatalf("setup: options name different sources %d and %d", d.Options[0].Source(), d.Options[1].Source())
	}
	e := &Encoder{}
	if n := e.listShape(&d, -1); n != 2 {
		t.Fatalf("list length %d, want 2", n)
	}
	if got := [2]int{e.ListPos(0), e.ListPos(1)}; got != [2]int{0, 1} {
		t.Fatalf("list positions %v, want [0 1]", got)
	}
}
